// Package facedetect wraps pigo (github.com/esimov/pigo), a pure-Go,
// cascade-based face detector. Chosen deliberately over cgo bindings to
// OpenCV so this service has zero system-library dependencies and
// compiles to a single static binary.
package facedetect

import (
	"fmt"
	"image"

	pigo "github.com/esimov/pigo/core"
)

type Box struct {
	X, Y, W, H int
	Confidence float32
}

type Detector struct {
	classifier *pigo.Pigo
}

// New loads the pretrained cascade file. cascadePath is typically
// "data/facefinder" shipped alongside the service binary.
func New(cascadePath string) (*Detector, error) {
	cascadeFile, err := readCascade(cascadePath)
	if err != nil {
		return nil, fmt.Errorf("reading cascade file: %w", err)
	}
	p := pigo.NewPigo()
	classifier, err := p.Unpack(cascadeFile)
	if err != nil {
		return nil, fmt.Errorf("unpacking cascade: %w", err)
	}
	return &Detector{classifier: classifier}, nil
}

// Detect returns every face pigo finds above a modest confidence
// threshold, largest-area first not guaranteed — callers should pick
// the box they need (e.g. largest) explicitly.
func (d *Detector) Detect(img image.Image) []Box {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	pixels := pigo.RgbToGrayscale(img)

	params := pigo.CascadeParams{
		MinSize:     60,
		MaxSize:     1000,
		ShiftFactor: 0.1,
		ScaleFactor: 1.1,
		ImageParams: pigo.ImageParams{
			Pixels: pixels,
			Rows:   height,
			Cols:   width,
			Dim:    width,
		},
	}

	dets := d.classifier.RunCascade(params, 0.0)
	dets = d.classifier.ClusterDetections(dets, 0.2)

	var boxes []Box
	for _, det := range dets {
		if det.Q < 5.0 { // confidence threshold: filters weak/spurious detections
			continue
		}
		r := det.Scale / 2
		boxes = append(boxes, Box{
			X:          det.Col - r,
			Y:          det.Row - r,
			W:          det.Scale,
			H:          det.Scale,
			Confidence: det.Q,
		})
	}
	return boxes
}

// LargestFace returns the largest-area detected face, or nil if none.
func LargestFace(boxes []Box) *Box {
	if len(boxes) == 0 {
		return nil
	}
	largest := boxes[0]
	for _, b := range boxes[1:] {
		if b.W*b.H > largest.W*largest.H {
			largest = b
		}
	}
	return &largest
}
