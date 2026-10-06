// Package httpserver exposes the AI service over plain net/http — no
// web framework — matching the same minimalist-stack philosophy the
// rest of the platform follows. The Go backend is the only caller;
// this is an internal service, never exposed to the public internet
// or called directly by the React frontend.
package httpserver

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log/slog"
	"net/http"
	"strings"

	"examshield-ai/internal/behavior"
	"examshield-ai/internal/facedetect"
	"examshield-ai/internal/faceverify"
)

type Server struct {
	detector *facedetect.Detector
	verifier *faceverify.Verifier
	analyzer *behavior.Analyzer
	logger   *slog.Logger
}

func New(detector *facedetect.Detector, verifier *faceverify.Verifier, analyzer *behavior.Analyzer, logger *slog.Logger) *Server {
	return &Server{detector: detector, verifier: verifier, analyzer: analyzer, logger: logger}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/verify-face", s.handleVerifyFace)
	mux.HandleFunc("/analyze-frame", s.handleAnalyzeFrame)
	mux.HandleFunc("/detect-face", s.handleDetectFace)
	mux.HandleFunc("/detect-multiple-faces", s.handleDetectMultipleFaces)
	return s.withLogging(mux)
}

func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.logger.Info("request", "method", r.Method, "path", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type verifyFaceRequest struct {
	StudentID   string `json:"student_id"`
	ImageBase64 string `json:"image_base64"`
	IsReference bool   `json:"is_reference"`
}

func (s *Server) handleVerifyFace(w http.ResponseWriter, r *http.Request) {
	var req verifyFaceRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.StudentID == "" || req.ImageBase64 == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "student_id and image_base64 are required"})
		return
	}

	img, err := decodeImage(req.ImageBase64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "could not decode image data"})
		return
	}

	boxes := s.detector.Detect(img)
	face := facedetect.LargestFace(boxes)
	if face == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"matched": false, "confidence": 0.0, "reason": "no_face_detected",
		})
		return
	}
	cropped := cropBox(img, *face)

	if req.IsReference {
		s.verifier.RegisterReference(req.StudentID, cropped)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"matched": true, "confidence": 1.0, "reason": "reference_registered",
		})
		return
	}

	result := s.verifier.Verify(req.StudentID, cropped)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"matched": result.Matched, "confidence": result.Confidence,
	})
}

type analyzeFrameRequest struct {
	StudentID   string `json:"student_id"`
	ImageBase64 string `json:"image_base64"`
}

func (s *Server) handleAnalyzeFrame(w http.ResponseWriter, r *http.Request) {
	var req analyzeFrameRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.StudentID == "" || req.ImageBase64 == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "student_id and image_base64 are required"})
		return
	}

	img, err := decodeImage(req.ImageBase64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "could not decode image data"})
		return
	}

	events := s.analyzer.Analyze(req.StudentID, img)
	if events == nil {
		events = []behavior.Event{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"events": events})
}

type imageOnlyRequest struct {
	ImageBase64 string `json:"image_base64"`
}

func (s *Server) handleDetectFace(w http.ResponseWriter, r *http.Request) {
	var req imageOnlyRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	img, err := decodeImage(req.ImageBase64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "could not decode image data"})
		return
	}
	boxes := s.detector.Detect(img)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"face_detected": len(boxes) > 0, "count": len(boxes),
	})
}

func (s *Server) handleDetectMultipleFaces(w http.ResponseWriter, r *http.Request) {
	var req imageOnlyRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	img, err := decodeImage(req.ImageBase64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "could not decode image data"})
		return
	}
	boxes := s.detector.Detect(img)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"multiple_faces_detected": len(boxes) > 1, "count": len(boxes),
	})
}

// --- helpers ---

func decodeJSON(w http.ResponseWriter, r *http.Request, dst interface{}) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// decodeImage accepts a raw base64 string or a data: URI and decodes
// it fully in memory. Frames are never written to disk, since raw
// camera footage must not be retained unnecessarily.
func decodeImage(imageBase64 string) (image.Image, error) {
	data := imageBase64
	if idx := strings.Index(data, ","); idx != -1 && strings.HasPrefix(data, "data:") {
		data = data[idx+1:]
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	return img, nil
}

func cropBox(img image.Image, box facedetect.Box) image.Image {
	b := img.Bounds()
	x0, y0 := clamp(box.X, b.Min.X, b.Max.X), clamp(box.Y, b.Min.Y, b.Max.Y)
	x1, y1 := clamp(box.X+box.W, b.Min.X, b.Max.X), clamp(box.Y+box.H, b.Min.Y, b.Max.Y)
	rect := image.Rect(x0, y0, x1, y1)

	type subImager interface {
		SubImage(r image.Rectangle) image.Image
	}
	if si, ok := img.(subImager); ok {
		return si.SubImage(rect)
	}
	return img
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
