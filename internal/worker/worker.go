// Package worker is the AI inference worker: it consumes media jobs from the
// events.frames stream, runs the models in parallel, applies the proctoring
// rules and publishes results to the findings stream.
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"image"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"examshield/internal/ai"
	"examshield/internal/services"
	"examshield/internal/store"
)

type Worker struct {
	rdb       *store.Redis
	blobs     store.Blobs
	engine    ai.Engine
	matcher   *services.FaceMatcher
	consumer  string
	pool      int
	log       *slog.Logger
	refs      *refCache
	jobBudget time.Duration
}

func New(rdb *store.Redis, blobs store.Blobs, engine ai.Engine, matcher *services.FaceMatcher, consumer string, pool int, log *slog.Logger) *Worker {
	return &Worker{rdb: rdb, blobs: blobs, engine: engine, matcher: matcher, consumer: consumer,
		pool: max(1, pool), log: log, refs: newRefCache(512), jobBudget: 20 * time.Second}
}

// Run processes jobs until ctx is cancelled, then drains in-flight jobs.
func (w *Worker) Run(ctx context.Context) error {
	if err := w.rdb.EnsureGroup(ctx, store.StreamFrames, store.GroupAIWorkers); err != nil {
		return err
	}
	jobs := make(chan store.Message, w.pool*2)
	var wg sync.WaitGroup
	for i := 0; i < w.pool; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for m := range jobs {
				w.process(m)
			}
		}()
	}
	defer func() {
		close(jobs)
		wg.Wait()
	}()

	pending := true
	lastClaim := time.Time{}
	for ctx.Err() == nil {
		var msgs []store.Message
		var err error
		if time.Since(lastClaim) > store.StaleClaimInterval {
			lastClaim = time.Now()
			msgs, err = w.rdb.ClaimStale(ctx, store.StreamFrames, store.GroupAIWorkers, w.consumer, store.StaleMinIdle, int64(w.pool))
			if err != nil {
				w.log.Error("worker: claim stale", "err", err)
			}
		}
		if len(msgs) == 0 {
			msgs, err = w.rdb.ReadGroup(ctx, store.StreamFrames, store.GroupAIWorkers, w.consumer, int64(w.pool), 2*time.Second, pending)
		}
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			w.log.Error("worker: read stream", "err", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(2 * time.Second):
			}
			continue
		}
		if pending && len(msgs) == 0 {
			pending = false
			continue
		}
		for _, m := range msgs {
			select {
			case jobs <- m:
			case <-ctx.Done():
				return nil
			}
		}
	}
	return nil
}

// process never uses the Run context for its own work, so cancelling Run
// lets in-flight jobs finish (bounded by jobBudget) instead of being lost.
func (w *Worker) process(m store.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), w.jobBudget)
	defer cancel()
	var job store.FrameJob
	if err := json.Unmarshal(m.Data, &job); err != nil {
		w.log.Error("worker: malformed job dropped", "id", m.ID, "err", err)
		_ = w.rdb.Ack(ctx, store.StreamFrames, store.GroupAIWorkers, m.ID)
		return
	}
	msg, err := w.Analyze(ctx, job)
	if err != nil {
		// Missing/corrupt media won't get better on retry: ACK and log.
		w.log.Warn("worker: job skipped", "session_id", job.SessionID, "kind", job.Kind, "err", err)
		_ = w.rdb.Ack(ctx, store.StreamFrames, store.GroupAIWorkers, m.ID)
		return
	}
	if err := w.rdb.Publish(ctx, store.StreamFindings, msg); err != nil {
		w.log.Error("worker: publish findings (left pending for retry)", "err", err)
		return
	}
	_ = w.rdb.Ack(ctx, store.StreamFrames, store.GroupAIWorkers, m.ID)
}

// Analyze runs every applicable model on one job.
func (w *Worker) Analyze(ctx context.Context, job store.FrameJob) (*store.FindingsMsg, error) {
	data, err := w.blobs.Get(ctx, job.Key)
	if err != nil {
		return nil, err
	}
	out := &store.FindingsMsg{ExamID: job.ExamID, SessionID: job.SessionID, MediaKey: job.Key,
		CapturedAt: job.CapturedAt, Backend: backendSummary(w.engine.Backends()), Inferences: map[string]json.RawMessage{}}
	switch job.Kind {
	case "audio":
		pcm, err := services.DecodePCM16(data)
		if err != nil {
			return nil, err
		}
		p, err := w.engine.SpeechProbability(pcm)
		if err != nil {
			return nil, err
		}
		out.Inferences["vad"] = mustJSON(map[string]any{"speech_prob": p, "backend": w.engine.Backends()["audio"]})
		out.Flags = services.EvaluateSpeech(p)
		return out, nil
	case "frame":
		img, err := ai.Decode(data)
		if err != nil {
			return nil, err
		}
		return out, w.analyzeFrame(ctx, img, job, out)
	}
	return nil, errors.New("unknown job kind")
}

func (w *Worker) analyzeFrame(ctx context.Context, img image.Image, job store.FrameJob, out *store.FindingsMsg) error {
	var mu sync.Mutex
	put := func(name string, v any, flags []store.Flag) {
		mu.Lock()
		defer mu.Unlock()
		out.Inferences[name] = mustJSON(v)
		out.Flags = append(out.Flags, flags...)
	}

	g, gctx := errgroup.WithContext(ctx)

	// Object detection is independent of faces.
	g.Go(func() error {
		objs, err := w.engine.DetectObjects(img)
		if errors.Is(err, ai.ErrUnsupported) {
			return nil
		}
		if err != nil {
			return err
		}
		put("yolov8n", objs, services.EvaluateObjects(objs))
		return nil
	})

	// Faces, then per-face models in parallel.
	g.Go(func() error {
		boxes, err := w.engine.DetectFaces(img)
		if err != nil {
			return err
		}
		put("face_detect", map[string]any{"count": len(boxes), "boxes": boxes}, services.EvaluateFaces(boxes))
		face := ai.LargestFace(boxes)
		if face == nil {
			return nil
		}
		fg, _ := errgroup.WithContext(gctx)
		fg.Go(func() error { return w.faceMatch(gctx, img, *face, job.ReferenceKey, put) })
		fg.Go(func() error {
			p, err := w.engine.HeadPose(img, *face)
			if errors.Is(err, ai.ErrUnsupported) {
				return nil
			}
			if err != nil {
				return err
			}
			put("head_pose", p, services.EvaluatePose(p))
			return nil
		})
		fg.Go(func() error {
			gz, err := w.engine.Gaze(img, *face)
			if errors.Is(err, ai.ErrUnsupported) {
				return nil
			}
			if err != nil {
				return err
			}
			put("gaze", gz, services.EvaluateGaze(gz))
			return nil
		})
		return fg.Wait()
	})
	return g.Wait()
}

// faceMatch compares the live face with the selfie captured at verification.
func (w *Worker) faceMatch(ctx context.Context, img image.Image, face ai.Box, refKey string, put func(string, any, []store.Flag)) error {
	if refKey == "" {
		return nil
	}
	ref, err := w.reference(ctx, refKey)
	if err != nil {
		return nil // reference purged or unreadable: skip, don't fail the frame
	}
	emb, err := w.engine.Embed(img, face)
	if err != nil {
		return err
	}
	score := w.matcher.Score(emb, ref)
	var flags []store.Flag
	// Only a high-assurance model may raise FACE_MISMATCH: the Go fallback's
	// scores are recorded for the proctor but never turned into accusations.
	if w.matcher.Assurance() == "high" && !w.matcher.Matches(score) {
		flags = []store.Flag{{EventType: "FACE_MISMATCH", Severity: string(services.AIEvents["FACE_MISMATCH"].Severity),
			Confidence: 1 - score, Metadata: map[string]any{"match_score": score, "assurance": w.matcher.Assurance()}}}
	}
	put("face_match", map[string]any{"score": score, "assurance": w.matcher.Assurance()}, flags)
	return nil
}

func (w *Worker) reference(ctx context.Context, key string) ([]float32, error) {
	if v, ok := w.refs.get(key); ok {
		return v, nil
	}
	data, err := w.blobs.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	img, err := ai.Decode(data)
	if err != nil {
		return nil, err
	}
	emb, err := w.matcher.Embedding(img)
	if err != nil {
		return nil, err
	}
	w.refs.put(key, emb)
	return emb, nil
}

func backendSummary(m map[string]string) string {
	if m["faces"] == "onnx" {
		return "onnx"
	}
	return "go"
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`null`)
	}
	return b
}

// refCache is a small bounded cache of reference embeddings.
type refCache struct {
	mu    sync.Mutex
	cap   int
	order []string
	items map[string][]float32
}

func newRefCache(n int) *refCache { return &refCache{cap: n, items: map[string][]float32{}} }

func (c *refCache) get(k string) ([]float32, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.items[k]
	return v, ok
}

func (c *refCache) put(k string, v []float32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.items[k]; ok {
		return
	}
	if len(c.order) >= c.cap {
		delete(c.items, c.order[0])
		c.order = c.order[1:]
	}
	c.order = append(c.order, k)
	c.items[k] = v
}
