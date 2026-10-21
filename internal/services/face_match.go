package services

import (
	"errors"
	"image"
	"math"

	"examshield/internal/ai"
)

var (
	ErrNoFace        = errors.New("no face detected")
	ErrMultipleFaces = errors.New("more than one face detected")
)

// calibration maps a raw cosine similarity onto a 0..1 match score with a
// logistic curve: score = 1 / (1 + e^(−k·(cos − mid))).
//
// ArcFace (w600k_r50) same-person cosines typically fall around 0.35–0.75 and
// impostors below 0.25, so mid=0.30, k=15 puts the 0.75 score threshold at
// cos ≈ 0.373. The Go fallback's gradient features are far less
// discriminative (everything face-like scores 0.6+), hence mid=0.85, k=30
// (0.75 ⇔ cos ≈ 0.887) and the "low" assurance label.
type calibration struct{ mid, k float64 }

var calibrations = map[string]calibration{
	"onnx": {mid: 0.30, k: 15},
	"go":   {mid: 0.85, k: 30},
}

func (c calibration) score(cos float64) float64 {
	return 1 / (1 + math.Exp(-c.k*(cos-c.mid)))
}

// FaceMatcher compares faces with the engine's embedding model.
type FaceMatcher struct {
	engine    ai.Engine
	threshold float64
	enforce   string // auto | always | never
}

func NewFaceMatcher(engine ai.Engine, threshold float64, enforce string) *FaceMatcher {
	if enforce == "" {
		enforce = "auto"
	}
	return &FaceMatcher{engine: engine, threshold: threshold, enforce: enforce}
}

// Enforced reports whether a failed match blocks a student. In "auto" mode
// only a high-assurance model (ArcFace) may refuse entry; the Go fallback
// is too weak to deny a student by itself, so its mismatches go to a proctor
// as FACE_REVIEW_REQUIRED instead.
func (m *FaceMatcher) Enforced() bool {
	switch m.enforce {
	case "always":
		return true
	case "never":
		return false
	}
	return m.Assurance() == "high"
}

func (m *FaceMatcher) Threshold() float64 { return m.threshold }

// Assurance is "high" with ArcFace, "low" with the Go fallback.
func (m *FaceMatcher) Assurance() string {
	if m.engine.Backends()["embed"] == "onnx" {
		return "high"
	}
	return "low"
}

// SingleFace finds exactly one face (the largest if extras are much smaller,
// e.g. a face printed on a poster behind the student).
func (m *FaceMatcher) SingleFace(img image.Image) (*ai.Box, int, error) {
	boxes, err := m.engine.DetectFaces(img)
	if err != nil {
		return nil, 0, err
	}
	best := ai.LargestFace(boxes)
	if best == nil {
		return nil, 0, ErrNoFace
	}
	significant := 0
	for _, b := range boxes {
		// A second face counts only if it is reasonably large and about as
		// confident as the main one; weak hits are detector noise.
		if float64(b.Area()) >= 0.25*float64(best.Area()) && b.Score >= 0.5*best.Score {
			significant++
		}
	}
	return best, significant, nil
}

// Embedding returns the embedding of the single dominant face in img.
func (m *FaceMatcher) Embedding(img image.Image) ([]float32, error) {
	face, n, err := m.SingleFace(img)
	if err != nil {
		return nil, err
	}
	if n > 1 {
		return nil, ErrMultipleFaces
	}
	return m.engine.Embed(img, *face)
}

// Score converts two embeddings into a calibrated match score in [0,1].
func (m *FaceMatcher) Score(a, b []float32) float64 {
	cal := calibrations[m.engine.Backends()["embed"]]
	if cal.k == 0 {
		cal = calibrations["go"]
	}
	return math.Round(cal.score(ai.Cosine(a, b))*1000) / 1000
}

// Matches reports whether score passes the configured threshold.
func (m *FaceMatcher) Matches(score float64) bool { return score >= m.threshold }
