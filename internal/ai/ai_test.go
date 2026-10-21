package ai

import (
	"image"
	"image/color"
	"io"
	"log/slog"
	"math"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testEngine(t *testing.T) Engine {
	t.Helper()
	e, err := Load(Options{Backend: "auto", ModelDir: "../../models", Log: slog.New(slog.NewTextHandler(io.Discard, nil))}, NeedAll())
	require.NoError(t, err)
	return e
}

func face(t *testing.T) image.Image {
	data, err := os.ReadFile("../testdata/face.jpg")
	require.NoError(t, err)
	img, err := Decode(data)
	require.NoError(t, err)
	return img
}

func TestFallbackDetectsAndEmbedsFace(t *testing.T) {
	e := testEngine(t)
	img := face(t)
	boxes, err := e.DetectFaces(img)
	require.NoError(t, err)
	require.Len(t, boxes, 1)
	b := boxes[0]
	assert.True(t, b.W > 80 && b.H > 80, "face box %+v", b)

	emb, err := e.Embed(img, b)
	require.NoError(t, err)
	assert.InDelta(t, 1.0, Cosine(emb, emb), 1e-6)
}

func TestEmptyFramesAreRejected(t *testing.T) {
	e := testEngine(t)
	_, err := e.DetectFaces(nil)
	assert.ErrorIs(t, err, ErrEmptyImage)
	_, err = e.DetectFaces(image.NewRGBA(image.Rect(0, 0, 2, 2)))
	assert.ErrorIs(t, err, ErrEmptyImage)
	_, err = e.SpeechProbability(nil)
	assert.Error(t, err)
	_, err = Decode(nil)
	assert.Error(t, err)
	_, err = Decode([]byte("not an image"))
	assert.Error(t, err)
}

func TestBlankFrameHasNoFace(t *testing.T) {
	e := testEngine(t)
	img := image.NewRGBA(image.Rect(0, 0, 320, 240))
	for i := range img.Pix {
		img.Pix[i] = 200
	}
	boxes, err := e.DetectFaces(img)
	require.NoError(t, err)
	assert.Empty(t, boxes)
}

func TestFallbackReportsUnsupported(t *testing.T) {
	e := testEngine(t)
	img := face(t)
	_, err := e.DetectObjects(img)
	assert.ErrorIs(t, err, ErrUnsupported)
	_, err = e.Gaze(img, Box{W: 10, H: 10})
	assert.ErrorIs(t, err, ErrUnsupported)
	assert.Equal(t, "none", e.Backends()["objects"])
}

func TestFallbackVAD(t *testing.T) {
	e := testEngine(t)
	silence := make([]float32, 16000)
	p, err := e.SpeechProbability(silence)
	require.NoError(t, err)
	assert.Less(t, p, 0.1)

	tone := make([]float32, 16000) // 220 Hz voiced-like signal
	for i := range tone {
		tone[i] = float32(0.3 * math.Sin(2*math.Pi*220*float64(i)/16000))
	}
	p, err = e.SpeechProbability(tone)
	require.NoError(t, err)
	assert.Greater(t, p, 0.6)
}

func TestDecodeYOLOAndNMS(t *testing.T) {
	// 84 channels x 3 anchors; anchor 0 and 1 overlap (same phone), anchor 2 is a book.
	const ch, n = 84, 3
	out := make([]float32, ch*n)
	set := func(a int, cx, cy, w, h float32, cls int, score float32) {
		out[0*n+a], out[1*n+a], out[2*n+a], out[3*n+a] = cx, cy, w, h
		out[(4+cls)*n+a] = score
	}
	set(0, 320, 320, 100, 200, 67, 0.9)
	set(1, 325, 322, 100, 200, 67, 0.6)
	set(2, 100, 100, 50, 50, 73, 0.5)
	lb := Letterbox{Scale: 1, Size: 640, SrcW: 640, SrcH: 640}
	objs := DecodeYOLO(out, ch, n, 80, 0.4, 0.45, lb, ProctorObjects)
	require.Len(t, objs, 2)
	assert.Equal(t, "cell phone", objs[0].Label)
	assert.InDelta(t, 0.9, objs[0].Score, 1e-6)
	assert.Equal(t, "book", objs[1].Label)
	assert.Equal(t, Box{X: 270, Y: 220, W: 100, H: 200, Score: 0.9}, objs[0].Box)

	// Malformed output never panics.
	assert.Nil(t, DecodeYOLO(out[:10], ch, n, 80, 0.4, 0.45, lb, nil))
}

func TestLetterboxMapsBack(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1280, 720))
	tensor, lb := LetterboxCHW(img, 640)
	assert.Len(t, tensor, 3*640*640)
	assert.InDelta(t, 0.5, lb.Scale, 1e-9)
	assert.Equal(t, 140, lb.PadY)
	// The centre of the letterboxed image is the centre of the source.
	b := lb.ToSource(320, 320, 100, 100)
	assert.Equal(t, Box{X: 540, Y: 260, W: 200, H: 200}, b)
	// Grey padding row.
	assert.InDelta(t, 114.0/255, tensor[0], 1e-6)
}

func TestToCHWNormalisation(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{255, 0, 255, 255})
		}
	}
	v := ToCHW(img, 2, 2, NormArcFace)
	assert.InDelta(t, 1, v[0], 1e-6)  // R plane
	assert.InDelta(t, -1, v[4], 1e-6) // G plane
}

func TestBinnedAngleAndEuler(t *testing.T) {
	logits := make([]float32, 90)
	logits[45] = 50 // all mass on bin 45 → 45*4-180 = 0°
	assert.InDelta(t, 0, BinnedAngle(logits), 1e-3)

	id := []float32{1, 0, 0, 0, 1, 0, 0, 0, 1}
	p := EulerFromRotation(id)
	assert.InDelta(t, 0, p.Yaw, 1e-9)
	assert.InDelta(t, 0, p.Pitch, 1e-9)

	// 30° yaw about the Y axis.
	th := 30 * math.Pi / 180
	ry := []float32{float32(math.Cos(th)), 0, float32(math.Sin(th)), 0, 1, 0, float32(-math.Sin(th)), 0, float32(math.Cos(th))}
	assert.InDelta(t, 30, EulerFromRotation(ry).Yaw, 1e-4)
}

func TestCosine(t *testing.T) {
	assert.Equal(t, 0.0, Cosine(nil, nil))
	assert.Equal(t, 0.0, Cosine([]float32{1}, []float32{1, 2}))
	assert.Equal(t, 0.0, Cosine([]float32{1, 0}, []float32{-1, 0}), "clamped at 0")
	assert.InDelta(t, 1.0, Cosine([]float32{1, 2}, []float32{2, 4}), 1e-9)
}

// A face on a busy, blinds-like background used to yield a second (false)
// face from the pigo cascade, which rejected genuine students as
// MULTIPLE_FACES. The skin-chromaticity filter must keep only the real face,
// also when the frame is dark or the head is tilted.
func TestFallbackIgnoresTexturedBackground(t *testing.T) {
	e := testEngine(t)
	src := face(t)
	sb := src.Bounds()
	for _, tc := range []struct {
		name  string
		gain  float64
		angle float64
	}{{"normal", 1, 0}, {"dark", 0.3, 0}, {"tilted", 1, 0.2}, {"dark+tilted", 0.35, -0.2}} {
		frame := image.NewRGBA(image.Rect(0, 0, 640, 480))
		for y := 0; y < 480; y++ {
			for x := 0; x < 640; x++ {
				// grey vertical blinds
				v := uint8(90 + 60*((x/12)%2))
				c := color.RGBA{v, v, uint8(min(255, int(v)+10)), 255}
				// photo centred, scaled to 400 px tall, rotated by angle
				cx, cy := float64(x-320), float64(y-240)
				rx := cx*math.Cos(tc.angle) - cy*math.Sin(tc.angle)
				ry := cx*math.Sin(tc.angle) + cy*math.Cos(tc.angle)
				s := float64(sb.Dy()) / 400
				sx, sy := int(rx*s)+sb.Dx()/2, int(ry*s)+sb.Dy()/2
				if sx >= 0 && sy >= 0 && sx < sb.Dx() && sy < sb.Dy() {
					r, g, b, _ := src.At(sb.Min.X+sx, sb.Min.Y+sy).RGBA()
					c = color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 255}
				}
				k := func(u uint8) uint8 { return uint8(math.Min(255, float64(u)*tc.gain)) }
				frame.SetRGBA(x, y, color.RGBA{k(c.R), k(c.G), k(c.B), 255})
			}
		}
		boxes, err := e.DetectFaces(frame)
		require.NoError(t, err)
		require.Len(t, boxes, 1, tc.name)
		cx := boxes[0].X + boxes[0].W/2
		assert.InDelta(t, 320, cx, 80, "%s: box should be on the face, got %+v", tc.name, boxes[0])
	}
}
