package faceverify

import (
	"image"
	"math"
)

// toGrayscale converts to a flat []float64 luminance array plus its
// original dimensions, using the standard Rec. 601 luma weights.
type grayImage struct {
	pixels []float64
	w, h   int
}

func toGrayscale(img image.Image) grayImage {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	pixels := make([]float64, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			// RGBA() returns 16-bit components; normalize to 8-bit range.
			lum := 0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(bl>>8)
			pixels[y*w+x] = lum
		}
	}
	return grayImage{pixels: pixels, w: w, h: h}
}

// resizeNearest performs simple nearest-neighbor resampling — enough
// for a low-resolution feature vector and avoids pulling in an image
// resampling library.
func resizeNearest(src grayImage, newW, newH int) grayImage {
	dst := make([]float64, newW*newH)
	for y := 0; y < newH; y++ {
		srcY := y * src.h / newH
		for x := 0; x < newW; x++ {
			srcX := x * src.w / newW
			dst[y*newW+x] = src.pixels[srcY*src.w+srcX]
		}
	}
	return grayImage{pixels: dst, w: newW, h: newH}
}

// equalizeHistogram applies basic histogram equalization over the 0-255
// luminance range to reduce the effect of lighting differences between
// the reference capture and later verification attempts.
func equalizeHistogram(src grayImage) grayImage {
	const bins = 256
	var histogram [bins]int
	for _, p := range src.pixels {
		v := int(p)
		if v < 0 {
			v = 0
		}
		if v > 255 {
			v = 255
		}
		histogram[v]++
	}

	var cdf [bins]int
	running := 0
	for i, count := range histogram {
		running += count
		cdf[i] = running
	}
	total := len(src.pixels)
	if total == 0 {
		return src
	}
	cdfMin := 0
	for _, v := range cdf {
		if v > 0 {
			cdfMin = v
			break
		}
	}

	var lut [bins]float64
	denom := float64(total - cdfMin)
	for i := 0; i < bins; i++ {
		if denom <= 0 {
			lut[i] = float64(i)
			continue
		}
		lut[i] = math.Round(float64(cdf[i]-cdfMin) / denom * 255)
	}

	out := make([]float64, len(src.pixels))
	for i, p := range src.pixels {
		v := int(p)
		if v < 0 {
			v = 0
		}
		if v > 255 {
			v = 255
		}
		out[i] = lut[v]
	}
	return grayImage{pixels: out, w: src.w, h: src.h}
}

// sobelMagnitude computes the Sobel gradient magnitude at every pixel,
// used as the feature vector's raw values (edges/contours of the face
// carry more identity signal than raw pixel intensity).
func sobelMagnitude(src grayImage, w, h int) []float64 {
	gx := [3][3]float64{{-1, 0, 1}, {-2, 0, 2}, {-1, 0, 1}}
	gy := [3][3]float64{{-1, -2, -1}, {0, 0, 0}, {1, 2, 1}}

	out := make([]float64, w*h)
	at := func(x, y int) float64 {
		if x < 0 {
			x = 0
		}
		if x >= w {
			x = w - 1
		}
		if y < 0 {
			y = 0
		}
		if y >= h {
			y = h - 1
		}
		return src.pixels[y*w+x]
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var sx, sy float64
			for ky := -1; ky <= 1; ky++ {
				for kx := -1; kx <= 1; kx++ {
					v := at(x+kx, y+ky)
					sx += gx[ky+1][kx+1] * v
					sy += gy[ky+1][kx+1] * v
				}
			}
			out[y*w+x] = math.Sqrt(sx*sx + sy*sy)
		}
	}
	return out
}
