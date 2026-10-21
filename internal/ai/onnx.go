//go:build onnx

package ai

import (
	"errors"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

// Model files expected in MODEL_DIR (see scripts/download-models.sh).
const (
	fileFace    = "yolov8n-face.onnx"
	fileArcFace = "arcface.onnx"
	fileGaze    = "l2cs_net.onnx"
	filePose    = "6drepnet.onnx"
	fileObjects = "yolov8n.onnx"
	fileVAD     = "silero_vad.onnx"
)

var initOnce sync.Once
var initErr error

func initORT(lib string) error {
	initOnce.Do(func() {
		if lib == "" {
			lib = defaultORTLib()
		}
		ort.SetSharedLibraryPath(lib)
		initErr = ort.InitializeEnvironment()
	})
	return initErr
}

func defaultORTLib() string {
	switch runtime.GOOS {
	case "windows":
		return "onnxruntime.dll"
	case "darwin":
		return "libonnxruntime.dylib"
	}
	return "/usr/lib/libonnxruntime.so"
}

// model wraps one DynamicAdvancedSession. ORT's Run is thread-safe and the
// dynamic session takes fresh tensors per call, so no mutex is needed.
type model struct {
	sess    *ort.DynamicAdvancedSession
	inputs  []ort.InputOutputInfo
	outputs []ort.InputOutputInfo
}

func (m *model) destroy() {
	if m != nil && m.sess != nil {
		_ = m.sess.Destroy()
	}
}

type onnxEngine struct {
	face, arc, gaze, pose, objects, vad *model
	vadV5                               bool
	faceChannels, objChannels, anchors  int
}

func newONNX(opt Options, need Need) (onnxModels, error) {
	if err := initORT(opt.ONNXLib); err != nil {
		return nil, fmt.Errorf("init onnxruntime: %w", err)
	}
	e := &onnxEngine{}
	load := func(file string, check func(in, out []ort.InputOutputInfo) error) *model {
		path := filepath.Join(opt.ModelDir, file)
		if _, err := os.Stat(path); err != nil {
			opt.Log.Warn("ai: model missing; capability falls back", "model", file)
			return nil
		}
		m, err := openModel(path, opt.UseCUDA, check)
		if err != nil {
			opt.Log.Error("ai: model rejected; capability falls back", "model", file, "err", err)
			return nil
		}
		return m
	}

	if need.Faces {
		e.face = load(fileFace, func(in, out []ort.InputOutputInfo) error {
			if err := expectInput(in, 640); err != nil {
				return err
			}
			if len(out[0].Dimensions) != 3 || (out[0].Dimensions[1] != 5 && out[0].Dimensions[1] != 20) {
				return fmt.Errorf("face output %v: want [1,5|20,N]", out[0].Dimensions)
			}
			e.faceChannels = int(out[0].Dimensions[1])
			return nil
		})
	}
	if need.Embed {
		e.arc = load(fileArcFace, func(in, out []ort.InputOutputInfo) error {
			if err := expectInput(in, 112); err != nil {
				return err
			}
			if d := out[0].Dimensions; len(d) != 2 || d[1] != 512 {
				return fmt.Errorf("arcface output %v: want [N,512]", d)
			}
			return nil
		})
	}
	if need.Gaze {
		e.gaze = load(fileGaze, func(in, out []ort.InputOutputInfo) error {
			if err := expectInput(in, 448); err != nil {
				return err
			}
			if len(out) != 2 || out[0].Dimensions[len(out[0].Dimensions)-1] != 90 {
				return fmt.Errorf("l2cs outputs %d: want 2×[1,90]", len(out))
			}
			return nil
		})
	}
	if need.Pose {
		e.pose = load(filePose, func(in, out []ort.InputOutputInfo) error {
			if err := expectInput(in, 224); err != nil {
				return err
			}
			if out[0].Dimensions.FlattenedSize() != 9 && out[0].Dimensions[len(out[0].Dimensions)-1] != 3 {
				return fmt.Errorf("6drepnet output %v: want [1,3,3]", out[0].Dimensions)
			}
			return nil
		})
	}
	if need.Objects {
		e.objects = load(fileObjects, func(in, out []ort.InputOutputInfo) error {
			if err := expectInput(in, 640); err != nil {
				return err
			}
			if d := out[0].Dimensions; len(d) != 3 || d[1] != 84 {
				return fmt.Errorf("yolov8 output %v: want [1,84,N] (COCO-80)", d)
			}
			e.objChannels = 84
			return nil
		})
	}
	if need.Audio {
		e.vad = load(fileVAD, func(in, out []ort.InputOutputInfo) error {
			names := map[string]bool{}
			for _, i := range in {
				names[i.Name] = true
			}
			switch {
			case names["input"] && names["state"] && names["sr"]:
				e.vadV5 = true
			case names["input"] && names["h"] && names["c"] && names["sr"]:
				e.vadV5 = false
			default:
				return fmt.Errorf("silero inputs %v: want v4 (input,sr,h,c) or v5 (input,state,sr)", names)
			}
			return nil
		})
	}
	if e.face == nil && e.arc == nil && e.gaze == nil && e.pose == nil && e.objects == nil && e.vad == nil {
		return nil, errors.New("no ONNX models could be loaded")
	}
	return e, nil
}

func openModel(path string, cuda bool, check func(in, out []ort.InputOutputInfo) error) (*model, error) {
	in, out, err := ort.GetInputOutputInfo(path)
	if err != nil {
		return nil, err
	}
	if len(in) == 0 || len(out) == 0 {
		return nil, errors.New("model has no inputs/outputs")
	}
	if check != nil {
		if err := check(in, out); err != nil {
			return nil, err
		}
	}
	opts, err := ort.NewSessionOptions()
	if err != nil {
		return nil, err
	}
	defer opts.Destroy()
	_ = opts.SetIntraOpNumThreads(max(1, runtime.NumCPU()/2))
	if cuda {
		if cu, err := ort.NewCUDAProviderOptions(); err == nil {
			// If CUDA can't be appended (no GPU/driver) we stay on CPU.
			_ = opts.AppendExecutionProviderCUDA(cu)
			cu.Destroy()
		}
	}
	names := func(xs []ort.InputOutputInfo) []string {
		s := make([]string, len(xs))
		for i, x := range xs {
			s[i] = x.Name
		}
		return s
	}
	sess, err := ort.NewDynamicAdvancedSession(path, names(in), names(out), opts)
	if err != nil {
		return nil, err
	}
	return &model{sess: sess, inputs: in, outputs: out}, nil
}

// expectInput checks a single NCHW image input of size×size (dynamic dims ok).
func expectInput(in []ort.InputOutputInfo, size int64) error {
	d := in[0].Dimensions
	if len(d) != 4 || (d[1] != 3 && d[1] != -1) || (d[2] != size && d[2] > 0) || (d[3] != size && d[3] > 0) {
		return fmt.Errorf("input %v: want [N,3,%d,%d]", d, size, size)
	}
	return nil
}

// run executes m with float32 image input and returns ORT-allocated outputs
// as float32 slices. All tensors are destroyed before returning.
func (m *model) runImage(data []float32, size int64) ([][]float32, [][]int64, error) {
	in, err := ort.NewTensor(ort.NewShape(1, 3, size, size), data)
	if err != nil {
		return nil, nil, err
	}
	defer in.Destroy()
	return m.run([]ort.Value{in})
}

func (m *model) run(inputs []ort.Value) ([][]float32, [][]int64, error) {
	outs := make([]ort.Value, len(m.outputs))
	if err := m.sess.Run(inputs, outs); err != nil {
		return nil, nil, err
	}
	defer func() {
		for _, o := range outs {
			if o != nil {
				o.Destroy()
			}
		}
	}()
	data := make([][]float32, len(outs))
	shapes := make([][]int64, len(outs))
	for i, o := range outs {
		t, ok := o.(*ort.Tensor[float32])
		if !ok {
			return nil, nil, fmt.Errorf("output %d is not float32", i)
		}
		src := t.GetData()
		if len(src) == 0 {
			return nil, nil, fmt.Errorf("output %d is empty", i)
		}
		data[i] = append([]float32(nil), src...)
		shapes[i] = t.GetShape()
	}
	return data, shapes, nil
}

func (e *onnxEngine) has(c string) bool {
	switch c {
	case "faces":
		return e.face != nil
	case "embed":
		return e.arc != nil
	case "gaze":
		return e.gaze != nil
	case "pose":
		return e.pose != nil
	case "objects":
		return e.objects != nil
	case "audio":
		return e.vad != nil
	}
	return false
}

func (e *onnxEngine) Backends() map[string]string { return nil }

func (e *onnxEngine) DetectFaces(img image.Image) ([]Box, error) {
	tensor, lb := LetterboxCHW(img, 640)
	out, shapes, err := e.face.runImage(tensor, 640)
	if err != nil {
		return nil, err
	}
	anchors := int(shapes[0][2])
	objs := DecodeYOLO(out[0], e.faceChannels, anchors, 1, 0.45, 0.45, lb, map[int]string{0: "face"})
	boxes := make([]Box, len(objs))
	for i, o := range objs {
		boxes[i] = o.Box
	}
	return boxes, nil
}

func (e *onnxEngine) Embed(img image.Image, face Box) ([]float32, error) {
	crop := Crop(img, squareAround(face), 0.08)
	out, _, err := e.arc.runImage(ToCHW(crop, 112, 112, NormArcFace), 112)
	if err != nil {
		return nil, err
	}
	return L2Normalize(out[0]), nil
}

func (e *onnxEngine) HeadPose(img image.Image, face Box) (*Pose, error) {
	crop := Crop(img, squareAround(face), 0.2)
	out, _, err := e.pose.runImage(ToCHW(crop, 224, 224, NormImageNet), 224)
	if err != nil {
		return nil, err
	}
	p := EulerFromRotation(out[0])
	return &p, nil
}

func (e *onnxEngine) Gaze(img image.Image, face Box) (*Gaze, error) {
	crop := Crop(img, squareAround(face), 0.1)
	out, _, err := e.gaze.runImage(ToCHW(crop, 448, 448, NormImageNet), 448)
	if err != nil {
		return nil, err
	}
	// Output order follows the model's declared names: L2CS exports are
	// (yaw, pitch) unless the names say otherwise.
	yawIdx, pitchIdx := 0, 1
	if e.gaze.outputs[0].Name == "pitch" {
		yawIdx, pitchIdx = 1, 0
	}
	return &Gaze{Yaw: BinnedAngle(out[yawIdx]), Pitch: BinnedAngle(out[pitchIdx])}, nil
}

func (e *onnxEngine) DetectObjects(img image.Image) ([]Object, error) {
	tensor, lb := LetterboxCHW(img, 640)
	out, shapes, err := e.objects.runImage(tensor, 640)
	if err != nil {
		return nil, err
	}
	return DecodeYOLO(out[0], e.objChannels, int(shapes[0][2]), 80, 0.40, 0.45, lb, ProctorObjects), nil
}

// SpeechProbability runs Silero VAD over 512-sample windows (16 kHz) with
// recurrent state carried across windows of this clip only.
func (e *onnxEngine) SpeechProbability(pcm []float32) (float64, error) {
	const win = 512
	// "sr" is a rank-0 scalar in current exports, [1] in some older ones.
	var sr ort.Value
	var err error
	if srRank(e.vad.inputs) == 0 {
		sr, err = ort.NewScalar(int64(16000))
	} else {
		sr, err = ort.NewTensor(ort.NewShape(1), []int64{16000})
	}
	if err != nil {
		return 0, err
	}
	defer sr.Destroy()
	stateSize := int64(128)
	if !e.vadV5 {
		stateSize = 64
	}
	h := make([]float32, 2*stateSize)
	c := make([]float32, 2*stateSize)
	best := 0.0
	for i := 0; i+win <= len(pcm); i += win {
		in, err := ort.NewTensor(ort.NewShape(1, win), append([]float32(nil), pcm[i:i+win]...))
		if err != nil {
			return 0, err
		}
		hs, err := ort.NewTensor(ort.NewShape(2, 1, stateSize), h)
		if err != nil {
			in.Destroy()
			return 0, err
		}
		var inputs []ort.Value
		var cs *ort.Tensor[float32]
		if e.vadV5 {
			inputs = orderInputs(e.vad.inputs, map[string]ort.Value{"input": in, "state": hs, "sr": sr})
		} else {
			cs, err = ort.NewTensor(ort.NewShape(2, 1, stateSize), c)
			if err != nil {
				in.Destroy()
				hs.Destroy()
				return 0, err
			}
			inputs = orderInputs(e.vad.inputs, map[string]ort.Value{"input": in, "h": hs, "c": cs, "sr": sr})
		}
		out, _, err := e.vad.run(inputs)
		in.Destroy()
		hs.Destroy()
		if cs != nil {
			cs.Destroy()
		}
		if err != nil {
			return 0, err
		}
		best = math.Max(best, float64(out[0][0]))
		copy(h, out[1])
		if !e.vadV5 && len(out) > 2 {
			copy(c, out[2])
		}
	}
	return best, nil
}

func srRank(info []ort.InputOutputInfo) int {
	for _, in := range info {
		if in.Name == "sr" {
			return len(in.Dimensions)
		}
	}
	return 0
}

func orderInputs(info []ort.InputOutputInfo, byName map[string]ort.Value) []ort.Value {
	out := make([]ort.Value, len(info))
	for i, in := range info {
		out[i] = byName[in.Name]
	}
	return out
}

func (e *onnxEngine) Close() error {
	for _, m := range []*model{e.face, e.arc, e.gaze, e.pose, e.objects, e.vad} {
		m.destroy()
	}
	return nil
}

// squareAround grows a box to a square around its centre (models expect
// square face crops).
func squareAround(b Box) image.Rectangle {
	s := max(b.W, b.H)
	cx, cy := b.X+b.W/2, b.Y+b.H/2
	return image.Rect(cx-s/2, cy-s/2, cx+s/2, cy+s/2)
}
