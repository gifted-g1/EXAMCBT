// Package ai provides the inference engine used for identity verification
// and live proctoring. Two backends exist:
//
//   - ONNX (build tag "onnx", cgo + libonnxruntime): YOLOv8-face, ArcFace,
//     L2CS-Net, 6DRepNet, YOLOv8n and Silero VAD.
//   - Go fallback (always compiled): pigo cascade face detection, a gradient
//     feature embedding, a face-geometry head-pose heuristic and an energy
//     VAD. No gaze or object detection.
//
// Load composes them per capability: every model that loads uses ONNX, every
// model that is missing or fails its shape check falls back to Go (or is
// reported unsupported), so one bad model file never disables the pipeline.
package ai

import (
	"errors"
	"image"
	"log/slog"
	"strings"
)

// ErrUnsupported means the active backend cannot perform this analysis.
var ErrUnsupported = errors.New("ai: capability not available in this backend")

// ErrEmptyImage is returned for nil/zero-sized frames.
var ErrEmptyImage = errors.New("ai: empty image")

type Box struct {
	X, Y, W, H int
	Score      float32
}

func (b Box) Rect() image.Rectangle { return image.Rect(b.X, b.Y, b.X+b.W, b.Y+b.H) }
func (b Box) Area() int             { return b.W * b.H }

type Object struct {
	Label string  `json:"label"`
	Score float32 `json:"score"`
	Box   Box     `json:"box"`
}

// Pose angles are in degrees. Yaw: left/right, Pitch: up/down.
type Pose struct {
	Yaw   float64 `json:"yaw"`
	Pitch float64 `json:"pitch"`
	Roll  float64 `json:"roll"`
}

type Gaze struct {
	Yaw   float64 `json:"yaw"`
	Pitch float64 `json:"pitch"`
}

// Engine is safe for concurrent use.
type Engine interface {
	// Backends describes which implementation serves each capability.
	Backends() map[string]string
	DetectFaces(img image.Image) ([]Box, error)
	Embed(img image.Image, face Box) ([]float32, error)
	HeadPose(img image.Image, face Box) (*Pose, error)
	Gaze(img image.Image, face Box) (*Gaze, error)
	DetectObjects(img image.Image) ([]Object, error)
	// SpeechProbability returns the max speech probability over a 16 kHz
	// mono clip.
	SpeechProbability(pcm []float32) (float64, error)
	Close() error
}

// Capabilities that can be requested from Load.
type Need struct {
	Faces, Embed, Pose, Gaze, Objects, Audio bool
}

func NeedAll() Need { return Need{true, true, true, true, true, true} }

// Options configures Load.
type Options struct {
	Backend    string // auto | onnx | go
	ModelDir   string
	ONNXLib    string
	UseCUDA    bool
	CascadeDir string // where pigo's "facefinder" lives (defaults to ModelDir)
	Log        *slog.Logger
}

// Load builds the composite engine.
func Load(opt Options, need Need) (Engine, error) {
	if opt.Log == nil {
		opt.Log = slog.Default()
	}
	if opt.CascadeDir == "" {
		opt.CascadeDir = opt.ModelDir
	}
	fb, err := newFallback(opt.CascadeDir)
	if err != nil {
		return nil, err
	}
	c := &composite{fallback: fb, names: map[string]string{}}

	backend := strings.ToLower(opt.Backend)
	if backend == "onnx" || backend == "auto" {
		ox, err := newONNX(opt, need)
		switch {
		case err == nil:
			c.onnx = ox
		case backend == "onnx":
			return nil, err
		default:
			opt.Log.Warn("ai: ONNX backend unavailable, using Go fallback", "err", err)
		}
	}
	c.resolve()
	opt.Log.Info("ai: engine ready", "backends", c.names)
	return c, nil
}

// onnxModels is implemented by the onnx-tagged backend; any nil method set
// means that model did not load.
type onnxModels interface {
	has(capability string) bool
	Engine
}

type composite struct {
	onnx     onnxModels
	fallback *fallback
	names    map[string]string
}

func (c *composite) pick(capability string) Engine {
	if c.onnx != nil && c.onnx.has(capability) {
		return c.onnx
	}
	return c.fallback
}

func (c *composite) resolve() {
	for _, capb := range []string{"faces", "embed", "pose", "gaze", "objects", "audio"} {
		if c.onnx != nil && c.onnx.has(capb) {
			c.names[capb] = "onnx"
		} else if c.fallback.has(capb) {
			c.names[capb] = "go"
		} else {
			c.names[capb] = "none"
		}
	}
}

func (c *composite) Backends() map[string]string { return c.names }

func (c *composite) DetectFaces(img image.Image) ([]Box, error) {
	if empty(img) {
		return nil, ErrEmptyImage
	}
	return c.pick("faces").DetectFaces(img)
}

func (c *composite) Embed(img image.Image, face Box) ([]float32, error) {
	if empty(img) {
		return nil, ErrEmptyImage
	}
	return c.pick("embed").Embed(img, face)
}

func (c *composite) HeadPose(img image.Image, face Box) (*Pose, error) {
	if empty(img) {
		return nil, ErrEmptyImage
	}
	return c.pick("pose").HeadPose(img, face)
}

func (c *composite) Gaze(img image.Image, face Box) (*Gaze, error) {
	if empty(img) {
		return nil, ErrEmptyImage
	}
	return c.pick("gaze").Gaze(img, face)
}

func (c *composite) DetectObjects(img image.Image) ([]Object, error) {
	if empty(img) {
		return nil, ErrEmptyImage
	}
	return c.pick("objects").DetectObjects(img)
}

func (c *composite) SpeechProbability(pcm []float32) (float64, error) {
	if len(pcm) == 0 {
		return 0, ErrEmptyImage
	}
	return c.pick("audio").SpeechProbability(pcm)
}

func (c *composite) Close() error {
	if c.onnx != nil {
		return c.onnx.Close()
	}
	return nil
}

func empty(img image.Image) bool {
	return img == nil || img.Bounds().Dx() < 8 || img.Bounds().Dy() < 8
}

// LargestFace returns the biggest box, or nil.
func LargestFace(boxes []Box) *Box {
	var best *Box
	for i := range boxes {
		if best == nil || boxes[i].Area() > best.Area() {
			best = &boxes[i]
		}
	}
	return best
}

// Cosine returns the cosine similarity of two embeddings, clamped to [0,1].
func Cosine(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	s := dot / (sqrt(na) * sqrt(nb))
	if s < 0 {
		return 0
	}
	if s > 1 {
		return 1
	}
	return s
}
