// Package faceverify turns a cropped face image into a fixed-length
// feature vector and compares vectors by cosine similarity.
//
// NOTE: this extractor (a normalized Sobel-gradient-magnitude
// histogram over a downscaled, equalized crop) is intentionally
// lightweight so the service has no heavy model-download dependency
// and needs no cgo/GPU. It sits behind a stable Extract() interface —
// swap in a proper face-embedding model (ArcFace/FaceNet, via ONNX
// runtime bindings) for production accuracy without touching anything
// else in the service.
package faceverify

import (
	"image"
	"math"
	"sync"
)

const (
	embeddingSize  = 48 // downscale target: 48x48
	matchThreshold = 0.80
)

type Result struct {
	Matched    bool
	Confidence float64
}

type Verifier struct {
	mu         sync.RWMutex
	references map[string][]float64 // student_id -> feature vector
}

func New() *Verifier {
	return &Verifier{references: make(map[string][]float64)}
}

// RegisterReference stores the student's reference embedding. In this
// scaffold the store is in-memory; a production deployment persists
// reference embeddings (never raw images) via the Go backend and this
// service fetches/caches them by student_id instead.
func (v *Verifier) RegisterReference(studentID string, face image.Image) {
	vec := extract(face)
	v.mu.Lock()
	v.references[studentID] = vec
	v.mu.Unlock()
}

// Verify compares a captured face against the student's stored
// reference. If no reference exists yet, this capture becomes the
// reference (first-authorized-use registration), matching the
// "register on first verified use" workflow the exam relies on.
func (v *Verifier) Verify(studentID string, face image.Image) Result {
	candidate := extract(face)

	v.mu.RLock()
	reference, ok := v.references[studentID]
	v.mu.RUnlock()

	if !ok {
		v.mu.Lock()
		v.references[studentID] = candidate
		v.mu.Unlock()
		return Result{Matched: true, Confidence: 1.0}
	}

	sim := cosineSimilarity(candidate, reference)
	return Result{Matched: sim >= matchThreshold, Confidence: round4(sim)}
}

// extract computes a normalized gradient-magnitude feature vector
// over a downscaled grayscale version of the face crop.
func extract(img image.Image) []float64 {
	gray := toGrayscale(img)
	resized := resizeNearest(gray, embeddingSize, embeddingSize)
	equalized := equalizeHistogram(resized)
	magnitude := sobelMagnitude(equalized, embeddingSize, embeddingSize)

	vec := make([]float64, len(magnitude))
	var norm float64
	for i, v := range magnitude {
		vec[i] = v
		norm += v * v
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range vec {
			vec[i] /= norm
		}
	}
	return vec
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot float64
	for i := range a {
		dot += a[i] * b[i]
	}
	if dot < 0 {
		dot = 0
	}
	if dot > 1 {
		dot = 1
	}
	return dot
}

func round4(f float64) float64 {
	return math.Round(f*10000) / 10000
}
