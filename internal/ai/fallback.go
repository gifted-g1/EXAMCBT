package ai

import (
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"sort"

	pigo "github.com/esimov/pigo/core"
)

// fallback is the pure-Go backend. It needs only the pigo "facefinder"
// cascade (~240 KB, shipped in models/). Head pose and gaze are reported
// unsupported: a frontal cascade simply stops finding the face when a
// student turns away, which the rule engine reports as NO_FACE.
type fallback struct {
	classifier *pigo.Pigo
}

func newFallback(dir string) (*fallback, error) {
	data, err := os.ReadFile(filepath.Join(dir, "facefinder"))
	if err != nil {
		return nil, fmt.Errorf("ai: read pigo cascade: %w", err)
	}
	cl, err := pigo.NewPigo().Unpack(data)
	if err != nil {
		return nil, fmt.Errorf("ai: unpack pigo cascade: %w", err)
	}
	return &fallback{classifier: cl}, nil
}

func (f *fallback) has(capability string) bool {
	switch capability {
	case "faces", "embed", "audio":
		return true
	}
	return false
}

func (f *fallback) Backends() map[string]string { return nil }
func (f *fallback) Close() error                { return nil }

// cascadeAngles are the in-plane rotations scanned (pigo: 1.0 = 360°):
// upright, and about ±20° for tilted heads.
var cascadeAngles = []float64{0, 0.055, 0.945}

// DetectFaces runs the pigo cascade at several tilts, merges overlapping
// hits, and drops detections that contain little skin-coloured area (pigo's
// frontal cascade fires on textured backgrounds such as blinds or windows).
// Dark frames are histogram-equalised first.
func (f *fallback) DetectFaces(img image.Image) ([]Box, error) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	pixels := grayscale(img)
	if meanByte(pixels) < 90 {
		equalizeBytes(pixels)
	}
	minSize := int(math.Max(40, float64(min(w, h))/8))
	params := pigo.CascadeParams{
		MinSize:     minSize,
		MaxSize:     min(w, h),
		ShiftFactor: 0.1,
		ScaleFactor: 1.1,
		ImageParams: pigo.ImageParams{Pixels: pixels, Rows: h, Cols: w, Dim: w},
	}
	// RunCascade reads only the immutable cascade, so one classifier can be
	// shared across goroutines.
	var cands []Box
	for _, angle := range cascadeAngles {
		dets := f.classifier.ClusterDetections(f.classifier.RunCascade(params, angle), 0.2)
		for _, d := range dets {
			if d.Q < 5.0 {
				continue
			}
			r := d.Scale / 2
			cands = append(cands, Box{X: b.Min.X + d.Col - r, Y: b.Min.Y + d.Row - r, W: d.Scale, H: d.Scale,
				Score: float32(math.Min(1, float64(d.Q)/20))})
		}
	}
	// Merge the same face found at different angles: keep the best score.
	sort.Slice(cands, func(i, j int) bool { return cands[i].Score > cands[j].Score })
	var out []Box
	for _, c := range cands {
		dup := false
		for _, o := range out {
			if IoU(c, o) > 0.3 || centreInside(c, o) || centreInside(o, c) {
				dup = true
				break
			}
		}
		if dup || skinRatio(img, c) < minSkinRatio {
			continue
		}
		out = append(out, c)
	}
	return out, nil
}

// minSkinRatio: share of skin-chromaticity pixels required in the central
// part of a face box. Low enough for glasses, beards and coloured lighting.
const minSkinRatio = 0.2

// skinRatio measures skin-coloured pixels in the central 50% of a box using
// normalised rg chromaticity (r = R/(R+G+B), g = G/(R+G+B)). Chromaticity is
// independent of brightness, so it behaves the same for light and dark skin
// and for dim rooms; grey/blue backgrounds score near zero.
func skinRatio(img image.Image, box Box) float64 {
	r := box.Rect()
	dx, dy := r.Dx()/4, r.Dy()/4
	r = image.Rect(r.Min.X+dx, r.Min.Y+dy, r.Max.X-dx, r.Max.Y-dy).Intersect(img.Bounds())
	if r.Empty() {
		return 0
	}
	step := max(1, r.Dx()/40)
	skin, total := 0, 0
	for y := r.Min.Y; y < r.Max.Y; y += step {
		for x := r.Min.X; x < r.Max.X; x += step {
			R, G, B := rgbAt(img, x, y)
			sum := R + G + B
			total++
			if sum < 30 { // too dark to judge colour
				continue
			}
			nr, ng := R/sum, G/sum
			if nr > 0.36 && nr < 0.60 && ng > 0.25 && ng < 0.37 && R > B {
				skin++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(skin) / float64(total)
}

func centreInside(a, b Box) bool {
	cx, cy := a.X+a.W/2, a.Y+a.H/2
	return cx >= b.X && cx <= b.X+b.W && cy >= b.Y && cy <= b.Y+b.H
}

func meanByte(px []uint8) float64 {
	if len(px) == 0 {
		return 0
	}
	var s int
	for _, p := range px {
		s += int(p)
	}
	return float64(s) / float64(len(px))
}

func equalizeBytes(px []uint8) {
	f := make([]float64, len(px))
	for i, p := range px {
		f[i] = float64(p)
	}
	equalize(f)
	for i, v := range f {
		px[i] = uint8(v)
	}
}

// Embed computes a spatial uniform-LBP histogram descriptor (Ahonen et al.)
// over the central part of the face box: a 6×6 grid of 59-bin histograms on
// a histogram-equalised 96×96 crop, Hellinger-normalised. LBP tolerates
// lighting changes and small misalignment far better than raw pixels or
// gradients, but it is still not a face-recognition model: FaceMatcher marks
// results "low assurance" and does not let them block a student on its own.
func (f *fallback) Embed(img image.Image, face Box) ([]float32, error) {
	const size, grid = 96, 6
	// Central 80% of the detector box: drop hair/background at the edges.
	r := face.Rect()
	dx, dy := r.Dx()/10, r.Dy()/10
	crop := Crop(img, image.Rect(r.Min.X+dx, r.Min.Y+dy, r.Max.X-dx, r.Max.Y-dy), 0)
	chw := ToCHW(crop, size, size, Norm{Scale: 1, Std: [3]float32{1, 1, 1}})
	gray := make([]float64, size*size)
	for i := range gray {
		gray[i] = 0.299*float64(chw[i]) + 0.587*float64(chw[size*size+i]) + 0.114*float64(chw[2*size*size+i])
	}
	equalize(gray)

	cell := (size - 2) / grid
	vec := make([]float32, grid*grid*59)
	for y := 1; y < size-1; y++ {
		for x := 1; x < size-1; x++ {
			cx, cy := min((x-1)/cell, grid-1), min((y-1)/cell, grid-1)
			vec[(cy*grid+cx)*59+uniformLBP[lbpCode(gray, size, x, y)]]++
		}
	}
	// Hellinger: L1-normalise each cell histogram, then square-root.
	for c := 0; c < grid*grid; c++ {
		h := vec[c*59 : (c+1)*59]
		var sum float32
		for _, v := range h {
			sum += v
		}
		if sum == 0 {
			continue
		}
		for i := range h {
			h[i] = float32(math.Sqrt(float64(h[i] / sum)))
		}
	}
	return L2Normalize(vec), nil
}

func lbpCode(px []float64, w, x, y int) uint8 {
	c := px[y*w+x]
	var code uint8
	nb := [8][2]int{{-1, -1}, {0, -1}, {1, -1}, {1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}}
	for i, d := range nb {
		if px[(y+d[1])*w+x+d[0]] >= c {
			code |= 1 << i
		}
	}
	return code
}

// uniformLBP maps the 256 8-bit LBP codes to 59 bins: one per "uniform"
// pattern (≤2 bit transitions) plus one shared bin for the rest.
var uniformLBP = func() [256]int {
	var m [256]int
	next := 0
	for c := 0; c < 256; c++ {
		transitions := 0
		for i := 0; i < 8; i++ {
			if (c>>i)&1 != (c>>((i+1)%8))&1 {
				transitions++
			}
		}
		if transitions <= 2 {
			m[c] = next
			next++
		} else {
			m[c] = 58
		}
	}
	return m
}()

func (f *fallback) HeadPose(image.Image, Box) (*Pose, error)    { return nil, ErrUnsupported }
func (f *fallback) Gaze(image.Image, Box) (*Gaze, error)        { return nil, ErrUnsupported }
func (f *fallback) DetectObjects(image.Image) ([]Object, error) { return nil, ErrUnsupported }

// SpeechProbability is an energy + zero-crossing voice activity heuristic
// over 30 ms frames: the share of frames that look like voiced speech,
// scaled so that ~40% voiced frames reads as certain speech.
func (f *fallback) SpeechProbability(pcm []float32) (float64, error) {
	const frame = 480 // 30 ms @ 16 kHz
	if len(pcm) < frame {
		return 0, nil
	}
	voiced, total := 0, 0
	for i := 0; i+frame <= len(pcm); i += frame {
		var energy float64
		zc := 0
		for j := i; j < i+frame; j++ {
			energy += float64(pcm[j]) * float64(pcm[j])
			if j > i && (pcm[j] >= 0) != (pcm[j-1] >= 0) {
				zc++
			}
		}
		rms := math.Sqrt(energy / frame)
		zcr := float64(zc) / frame
		if rms > 0.02 && zcr > 0.01 && zcr < 0.30 {
			voiced++
		}
		total++
	}
	return math.Min(1, float64(voiced)/float64(total)/0.4), nil
}

func grayscale(img image.Image) []uint8 {
	b := img.Bounds()
	out := make([]uint8, b.Dx()*b.Dy())
	i := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl := rgbAt(img, x, y)
			out[i] = uint8(0.299*r + 0.587*g + 0.114*bl)
			i++
		}
	}
	return out
}

func equalize(px []float64) {
	var hist [256]int
	for _, p := range px {
		hist[clamp255(p)]++
	}
	var cdf [256]int
	run := 0
	for i, c := range hist {
		run += c
		cdf[i] = run
	}
	cdfMin := 0
	for _, v := range cdf {
		if v > 0 {
			cdfMin = v
			break
		}
	}
	den := float64(len(px) - cdfMin)
	if den <= 0 {
		return
	}
	for i, p := range px {
		px[i] = math.Round(float64(cdf[clamp255(p)]-cdfMin) / den * 255)
	}
}

func clamp255(p float64) int {
	v := int(p)
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}
