// Command fakecam converts a still image into a one-frame Y4M video that
// Chromium can use as a fake webcam (--use-file-for-fake-video-capture), so
// the Playwright E2E test can exercise real face verification.
//
//	go run ./scripts/fakecam internal/testdata/face.jpg web/e2e/face.y4m
package main

import (
	"bufio"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"os"
)

const w, h = 640, 480

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: fakecam <in.jpg> <out.y4m>")
		os.Exit(2)
	}
	f, err := os.Open(os.Args[1])
	check(err)
	src, _, err := image.Decode(f)
	check(err)
	f.Close()

	// Fit the image into 640x480 on a light grey background.
	b := src.Bounds()
	scale := min(float64(w)/float64(b.Dx()), float64(h)/float64(b.Dy()))
	dw, dh := int(float64(b.Dx())*scale), int(float64(b.Dy())*scale)
	ox, oy := (w-dw)/2, (h-dh)/2
	at := func(x, y int) color.Color {
		if x < ox || y < oy || x >= ox+dw || y >= oy+dh {
			return color.Gray{Y: 220}
		}
		return src.At(b.Min.X+int(float64(x-ox)/scale), b.Min.Y+int(float64(y-oy)/scale))
	}

	yp := make([]byte, w*h)
	up := make([]byte, w*h/4)
	vp := make([]byte, w*h/4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, bl, _ := at(x, y).RGBA()
			yy, cb, cr := color.RGBToYCbCr(uint8(r>>8), uint8(g>>8), uint8(bl>>8))
			yp[y*w+x] = yy
			if x%2 == 0 && y%2 == 0 {
				up[(y/2)*(w/2)+x/2] = cb
				vp[(y/2)*(w/2)+x/2] = cr
			}
		}
	}
	out, err := os.Create(os.Args[2])
	check(err)
	bw := bufio.NewWriter(out)
	fmt.Fprintf(bw, "YUV4MPEG2 W%d H%d F15:1 Ip A1:1 C420jpeg\n", w, h)
	for i := 0; i < 2; i++ {
		bw.WriteString("FRAME\n")
		bw.Write(yp)
		bw.Write(up)
		bw.Write(vp)
	}
	check(bw.Flush())
	check(out.Close())
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
