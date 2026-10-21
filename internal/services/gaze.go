package services

import (
	"math"

	"examshield/internal/ai"
	"examshield/internal/store"
)

// Head-pose and gaze thresholds (degrees). A student reading the bottom of
// the screen pitches down ~20–25°, so pitch tolerance is wider than yaw.
const (
	headYawLimit   = 35.0
	headPitchLimit = 30.0
	gazeYawLimit   = 30.0
	gazePitchLimit = 25.0
)

// EvaluatePose flags a head turned away from the screen.
func EvaluatePose(p *ai.Pose) []store.Flag {
	if p == nil {
		return nil
	}
	excess := math.Max(math.Abs(p.Yaw)/headYawLimit, math.Abs(p.Pitch)/headPitchLimit)
	if excess < 1 {
		return nil
	}
	return []store.Flag{{
		EventType:  "LOOKING_AWAY",
		Severity:   string(AIEvents["LOOKING_AWAY"].Severity),
		Confidence: confidenceFromExcess(excess),
		Metadata:   map[string]any{"yaw": round1(p.Yaw), "pitch": round1(p.Pitch), "roll": round1(p.Roll)},
	}}
}

// EvaluateGaze flags eyes directed off-screen while the head faces forward.
func EvaluateGaze(g *ai.Gaze) []store.Flag {
	if g == nil {
		return nil
	}
	excess := math.Max(math.Abs(g.Yaw)/gazeYawLimit, math.Abs(g.Pitch)/gazePitchLimit)
	if excess < 1 {
		return nil
	}
	return []store.Flag{{
		EventType:  "GAZE_OFF_SCREEN",
		Severity:   string(AIEvents["GAZE_OFF_SCREEN"].Severity),
		Confidence: confidenceFromExcess(excess),
		Metadata:   map[string]any{"yaw": round1(g.Yaw), "pitch": round1(g.Pitch)},
	}}
}

// confidenceFromExcess maps "how far past the limit" (1.0 = at limit) to a
// confidence of 0.5 at the limit rising to ~1 at 1.5× the limit.
func confidenceFromExcess(x float64) float64 {
	return math.Round(math.Min(1, 0.5+(x-1))*1000) / 1000
}

func round1(f float64) float64 { return math.Round(f*10) / 10 }
