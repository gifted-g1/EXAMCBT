package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Stream names.
const (
	StreamFrames   = "events.frames"
	StreamFindings = "findings"

	GroupAIWorkers = "ai-workers"
	GroupAPI       = "api-findings"

	streamMaxLen = 50_000
)

// FrameJob asks the AI worker to analyse one captured media item.
type FrameJob struct {
	Kind         string `json:"kind"` // "frame" | "audio"
	ExamID       int64  `json:"exam_id"`
	SessionID    int64  `json:"session_id"`
	Key          string `json:"key"`
	ReferenceKey string `json:"reference_key,omitempty"`
	CapturedAt   int64  `json:"captured_at"` // unix ms
}

// Flag is a rule-engine verdict for one media item.
type Flag struct {
	EventType  string         `json:"event_type"`
	Severity   string         `json:"severity"`
	Confidence float64        `json:"confidence"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// FindingsMsg carries the result of analysing a FrameJob back to the API,
// which persists it and pushes alerts to proctors. The worker never writes to
// MySQL; the API is the single writer.
type FindingsMsg struct {
	ExamID     int64                      `json:"exam_id"`
	SessionID  int64                      `json:"session_id"`
	MediaKey   string                     `json:"media_key"`
	CapturedAt int64                      `json:"captured_at"`
	Backend    string                     `json:"backend"`
	Inferences map[string]json.RawMessage `json:"inferences"`
	Flags      []Flag                     `json:"flags"`
}

// Publish appends v (JSON-encoded) to stream, capping its length.
func (r *Redis) Publish(ctx context.Context, stream string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return r.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		MaxLen: streamMaxLen,
		Approx: true,
		Values: map[string]any{"data": string(b)},
	}).Err()
}

// EnsureGroup creates a consumer group starting at the stream's beginning,
// ignoring "already exists".
func (r *Redis) EnsureGroup(ctx context.Context, stream, group string) error {
	err := r.XGroupCreateMkStream(ctx, stream, group, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}
	return nil
}

// Message is one stream entry decoded into its raw JSON payload.
type Message struct {
	ID   string
	Data []byte
}

// ReadGroup blocks up to block for new messages for consumer. Messages this
// consumer claimed earlier but never ACKed are redelivered first (start "0").
func (r *Redis) ReadGroup(ctx context.Context, stream, group, consumer string, count int64, block time.Duration, pending bool) ([]Message, error) {
	start := ">"
	if pending {
		start = "0"
	}
	res, err := r.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{stream, start},
		Count:    count,
		Block:    block,
	}).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Message
	for _, s := range res {
		for _, m := range s.Messages {
			raw, _ := m.Values["data"].(string)
			out = append(out, Message{ID: m.ID, Data: []byte(raw)})
		}
	}
	return out, nil
}

func (r *Redis) Ack(ctx context.Context, stream, group string, ids ...string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.XAck(ctx, stream, group, ids...).Err()
}

// ConsumerName builds a stable consumer name for this process.
func ConsumerName(role, host string, pid int) string {
	return fmt.Sprintf("%s-%s-%d", role, host, pid)
}

// ClaimStale takes over messages that another consumer read but never ACKed
// for at least minIdle — e.g. an API or worker that crashed or was replaced
// (consumer names change across restarts). Without this, such messages would
// stay pending forever and their findings would be lost.
func (r *Redis) ClaimStale(ctx context.Context, stream, group, consumer string, minIdle time.Duration, count int64) ([]Message, error) {
	msgs, _, err := r.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream: stream, Group: group, Consumer: consumer, MinIdle: minIdle, Start: "0-0", Count: count,
	}).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]Message, 0, len(msgs))
	for _, m := range msgs {
		raw, _ := m.Values["data"].(string)
		out = append(out, Message{ID: m.ID, Data: []byte(raw)})
	}
	return out, nil
}

// StaleClaimInterval / StaleMinIdle tune ClaimStale for both consumers.
const (
	StaleClaimInterval = 30 * time.Second
	StaleMinIdle       = 60 * time.Second
)
