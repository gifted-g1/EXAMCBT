// Package behavior turns a stream of per-student face detections into
// classified monitoring events.
//
// Design principle (unchanged from the spec): this package NEVER
// concludes a student is cheating. It only detects observable
// conditions (no face, multiple faces, large position jumps) and
// assigns a severity for human review. Escalation across repeated
// frames is a signal, not a verdict.
package behavior

import (
	"image"
	"math"
	"sync"
	"time"

	"examshield-ai/internal/facedetect"
)

const (
	noFaceGraceSeconds = 3.0
)

type Event struct {
	EventType   string  `json:"event_type"`
	Severity    string  `json:"severity"`
	Confidence  float64 `json:"confidence"`
	Description string  `json:"description"`
}

type studentState struct {
	faceMissingSince        *time.Time
	lastFaceCenterX         float64
	lastFaceCenterY         float64
	hasLastCenter           bool
	consecutiveMultiFaceCnt int
}

type Analyzer struct {
	detector *facedetect.Detector

	mu    sync.Mutex
	state map[string]*studentState
}

func New(detector *facedetect.Detector) *Analyzer {
	return &Analyzer{detector: detector, state: make(map[string]*studentState)}
}

func (a *Analyzer) stateFor(studentID string) *studentState {
	s, ok := a.state[studentID]
	if !ok {
		s = &studentState{}
		a.state[studentID] = s
	}
	return s
}

func (a *Analyzer) Analyze(studentID string, img image.Image) []Event {
	a.mu.Lock()
	defer a.mu.Unlock()

	state := a.stateFor(studentID)
	boxes := a.detector.Detect(img)
	now := time.Now()
	var events []Event

	if len(boxes) == 0 {
		if state.faceMissingSince == nil {
			state.faceMissingSince = &now
		}
		missingFor := now.Sub(*state.faceMissingSince).Seconds()
		if missingFor >= noFaceGraceSeconds {
			severity := "MEDIUM"
			if missingFor >= 15 {
				severity = "HIGH"
			}
			events = append(events, Event{
				EventType:   "FACE_NOT_VISIBLE",
				Severity:    severity,
				Confidence:  0.85,
				Description: "Student face is no longer visible in the camera frame",
			})
		}
		return events
	}

	// Face(s) present: clear the missing-face timer.
	state.faceMissingSince = nil

	if len(boxes) > 1 {
		state.consecutiveMultiFaceCnt++
		severity := "MEDIUM"
		if state.consecutiveMultiFaceCnt >= 3 {
			severity = "HIGH"
		}
		confidence := math.Min(0.99, 0.7+0.05*float64(state.consecutiveMultiFaceCnt))
		events = append(events, Event{
			EventType:   "MULTIPLE_FACES",
			Severity:    severity,
			Confidence:  confidence,
			Description: "More than one face detected in the camera frame",
		})
		return events
	}
	state.consecutiveMultiFaceCnt = 0

	// Single face: check for large positional jumps (possible camera
	// tampering, student leaning far out of frame, etc).
	primary := facedetect.LargestFace(boxes)
	centerX := float64(primary.X) + float64(primary.W)/2
	centerY := float64(primary.Y) + float64(primary.H)/2

	if state.hasLastCenter {
		dx := centerX - state.lastFaceCenterX
		dy := centerY - state.lastFaceCenterY
		distance := math.Sqrt(dx*dx + dy*dy)

		bounds := img.Bounds()
		diag := math.Sqrt(float64(bounds.Dx()*bounds.Dx() + bounds.Dy()*bounds.Dy()))
		if diag > 0 && distance/diag > 0.35 {
			events = append(events, Event{
				EventType:   "SIGNIFICANT_FACE_POSITION_CHANGE",
				Severity:    "LOW",
				Confidence:  0.6,
				Description: "Significant change in face position detected between frames",
			})
		}
	}
	state.lastFaceCenterX = centerX
	state.lastFaceCenterY = centerY
	state.hasLastCenter = true

	return events
}
