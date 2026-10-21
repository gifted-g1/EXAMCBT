package ai

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg" // register decoders
	_ "image/png"
	"math"
)

func sqrt(x float64) float64 { return math.Sqrt(x) }

// MaxFramePixels bounds decoded frames (decompression-bomb guard): 4096².
const MaxFramePixels = 4096 * 4096

// Decode decodes a JPEG/PNG after checking its declared dimensions.
func Decode(data []byte) (image.Image, error) {
	if len(data) == 0 {
		return nil, ErrEmptyImage
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	if cfg.Width < 8 || cfg.Height < 8 || cfg.Width*cfg.Height > MaxFramePixels {
		return nil, fmt.Errorf("image dimensions %dx%d out of range", cfg.Width, cfg.Height)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	return img, err
}

// rgbAt returns 8-bit RGB at (x,y), clamping to bounds.
func rgbAt(img image.Image, x, y int) (float32, float32, float32) {
	b := img.Bounds()
	if x < b.Min.X {
		x = b.Min.X
	} else if x >= b.Max.X {
		x = b.Max.X - 1
	}
	if y < b.Min.Y {
		y = b.Min.Y
	} else if y >= b.Max.Y {
		y = b.Max.Y - 1
	}
	switch m := img.(type) {
	case *image.YCbCr:
		yi := m.YOffset(x, y)
		ci := m.COffset(x, y)
		r, g, bl := color.YCbCrToRGB(m.Y[yi], m.Cb[ci], m.Cr[ci])
		return float32(r), float32(g), float32(bl)
	case *image.RGBA:
		i := m.PixOffset(x, y)
		return float32(m.Pix[i]), float32(m.Pix[i+1]), float32(m.Pix[i+2])
	case *image.NRGBA:
		i := m.PixOffset(x, y)
		return float32(m.Pix[i]), float32(m.Pix[i+1]), float32(m.Pix[i+2])
	}
	r, g, bl, _ := img.At(x, y).RGBA()
	return float32(r >> 8), float32(g >> 8), float32(bl >> 8)
}

// Norm describes per-channel normalisation: (v/scale - mean) / std.
type Norm struct {
	Scale float32
	Mean  [3]float32
	Std   [3]float32
}

var (
	// NormUnit maps 0..255 to 0..1 (YOLO).
	NormUnit = Norm{Scale: 255, Mean: [3]float32{0, 0, 0}, Std: [3]float32{1, 1, 1}}
	// NormImageNet is torchvision's ImageNet normalisation (L2CS, 6DRepNet).
	NormImageNet = Norm{Scale: 255, Mean: [3]float32{0.485, 0.456, 0.406}, Std: [3]float32{0.229, 0.224, 0.225}}
	// NormArcFace maps 0..255 to -1..1.
	NormArcFace = Norm{Scale: 127.5, Mean: [3]float32{1, 1, 1}, Std: [3]float32{1, 1, 1}}
)

// ToCHW resamples src (bilinear) to w×h and writes a normalised RGB tensor
// in CHW order.
func ToCHW(src image.Image, w, h int, n Norm) []float32 {
	out := make([]float32, 3*w*h)
	b := src.Bounds()
	sx := float64(b.Dx()) / float64(w)
	sy := float64(b.Dy()) / float64(h)
	plane := w * h
	for y := 0; y < h; y++ {
		fy := (float64(y)+0.5)*sy - 0.5
		y0 := int(math.Floor(fy))
		wy := float32(fy - float64(y0))
		for x := 0; x < w; x++ {
			fx := (float64(x)+0.5)*sx - 0.5
			x0 := int(math.Floor(fx))
			wx := float32(fx - float64(x0))
			r00, g00, b00 := rgbAt(src, b.Min.X+x0, b.Min.Y+y0)
			r10, g10, b10 := rgbAt(src, b.Min.X+x0+1, b.Min.Y+y0)
			r01, g01, b01 := rgbAt(src, b.Min.X+x0, b.Min.Y+y0+1)
			r11, g11, b11 := rgbAt(src, b.Min.X+x0+1, b.Min.Y+y0+1)
			lerp := func(a, b, c, d float32) float32 {
				top := a + (b-a)*wx
				bot := c + (d-c)*wx
				return top + (bot-top)*wy
			}
			px := [3]float32{lerp(r00, r10, r01, r11), lerp(g00, g10, g01, g11), lerp(b00, b10, b01, b11)}
			i := y*w + x
			for ch := 0; ch < 3; ch++ {
				out[ch*plane+i] = (px[ch]/n.Scale - n.Mean[ch]) / n.Std[ch]
			}
		}
	}
	return out
}

// Letterbox describes how a frame was scaled and padded into a square input.
type Letterbox struct {
	Scale      float64
	PadX, PadY int
	Size       int
	SrcW, SrcH int
	SrcMinX    int
	SrcMinY    int
}

// LetterboxCHW resizes src keeping aspect ratio into size×size, padding with
// grey (114), and returns the YOLO-normalised tensor.
func LetterboxCHW(src image.Image, size int) ([]float32, Letterbox) {
	b := src.Bounds()
	scale := math.Min(float64(size)/float64(b.Dx()), float64(size)/float64(b.Dy()))
	nw, nh := int(math.Round(float64(b.Dx())*scale)), int(math.Round(float64(b.Dy())*scale))
	padX, padY := (size-nw)/2, (size-nh)/2
	lb := Letterbox{Scale: scale, PadX: padX, PadY: padY, Size: size, SrcW: b.Dx(), SrcH: b.Dy(),
		SrcMinX: b.Min.X, SrcMinY: b.Min.Y}

	resized := ToCHW(src, nw, nh, NormUnit)
	out := make([]float32, 3*size*size)
	grey := float32(114.0 / 255.0)
	for i := range out {
		out[i] = grey
	}
	for ch := 0; ch < 3; ch++ {
		for y := 0; y < nh; y++ {
			copy(out[ch*size*size+(y+padY)*size+padX:ch*size*size+(y+padY)*size+padX+nw],
				resized[ch*nw*nh+y*nw:ch*nw*nh+(y+1)*nw])
		}
	}
	return out, lb
}

// ToSource maps a box in letterboxed coordinates back to the source image.
func (lb Letterbox) ToSource(cx, cy, w, h float64) Box {
	x0 := (cx - w/2 - float64(lb.PadX)) / lb.Scale
	y0 := (cy - h/2 - float64(lb.PadY)) / lb.Scale
	bw := w / lb.Scale
	bh := h / lb.Scale
	x0 = math.Max(0, x0)
	y0 = math.Max(0, y0)
	bw = math.Min(bw, float64(lb.SrcW)-x0)
	bh = math.Min(bh, float64(lb.SrcH)-y0)
	return Box{X: lb.SrcMinX + int(x0), Y: lb.SrcMinY + int(y0), W: int(bw), H: int(bh)}
}

// Crop returns the sub-image for r expanded by margin (fraction of size),
// clipped to bounds. Falls back to a copy when SubImage is unsupported.
func Crop(img image.Image, r image.Rectangle, margin float64) image.Image {
	dx := int(float64(r.Dx()) * margin)
	dy := int(float64(r.Dy()) * margin)
	r = image.Rect(r.Min.X-dx, r.Min.Y-dy, r.Max.X+dx, r.Max.Y+dy).Intersect(img.Bounds())
	if r.Empty() {
		return img
	}
	if s, ok := img.(interface {
		SubImage(image.Rectangle) image.Image
	}); ok {
		return s.SubImage(r)
	}
	dst := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	for y := 0; y < r.Dy(); y++ {
		for x := 0; x < r.Dx(); x++ {
			dst.Set(x, y, img.At(r.Min.X+x, r.Min.Y+y))
		}
	}
	return dst
}
