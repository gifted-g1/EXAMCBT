// Package services holds Exam Shield's business logic: authentication, the
// exam lifecycle, student sessions, proctoring rules and integrity scoring.
package services

import (
	"math"
	"time"

	"examshield/internal/models"
)

// EventSpec describes one violation type.
type EventSpec struct {
	Severity models.Severity
	// Cooldown suppresses repeats of the same event for one session, so a
	// student who looks away for 30 s is flagged once, not 15 times.
	Cooldown time.Duration
}

// ClientEvents are the only event types a browser may report. Anything else
// is rejected, so a tampered client cannot inject AI-style findings.
var ClientEvents = map[string]EventSpec{
	"TAB_SWITCH":         {models.SevMedium, 5 * time.Second},
	"WINDOW_BLUR":        {models.SevLow, 5 * time.Second},
	"FULLSCREEN_EXIT":    {models.SevHigh, 5 * time.Second},
	"COPY_PASTE":         {models.SevMedium, 5 * time.Second},
	"CONTEXT_MENU":       {models.SevLow, 10 * time.Second},
	"DEVTOOLS_SHORTCUT":  {models.SevHigh, 10 * time.Second},
	"PRINT_SCREEN":       {models.SevMedium, 10 * time.Second},
	"CAMERA_UNAVAILABLE": {models.SevHigh, 30 * time.Second},
	"MIC_UNAVAILABLE":    {models.SevMedium, 30 * time.Second},
	"MULTIPLE_DISPLAYS":  {models.SevHigh, 60 * time.Second},
	"NETWORK_OFFLINE":    {models.SevLow, 30 * time.Second},
}

// AIEvents are produced only by the rule engine.
var AIEvents = map[string]EventSpec{
	"NO_FACE":              {models.SevMedium, 15 * time.Second},
	"MULTIPLE_FACES":       {models.SevHigh, 15 * time.Second},
	"FACE_MISMATCH":        {models.SevCritical, 30 * time.Second},
	"LOOKING_AWAY":         {models.SevLow, 15 * time.Second},
	"GAZE_OFF_SCREEN":      {models.SevLow, 15 * time.Second},
	"PHONE_DETECTED":       {models.SevHigh, 20 * time.Second},
	"BOOK_DETECTED":        {models.SevMedium, 30 * time.Second},
	"EXTRA_DEVICE":         {models.SevMedium, 30 * time.Second},
	"EXTRA_PERSON":         {models.SevHigh, 20 * time.Second},
	"VOICE_DETECTED":       {models.SevMedium, 20 * time.Second},
	"NO_REFERENCE_PHOTO":   {models.SevMedium, 0},
	"FACE_REVIEW_REQUIRED": {models.SevHigh, 0},
}

// SeverityWeight is the integrity penalty for a certain (confidence 1.0)
// violation of each severity.
var SeverityWeight = map[models.Severity]float64{
	models.SevLow:      1,
	models.SevMedium:   3,
	models.SevHigh:     8,
	models.SevCritical: 20,
}

// Penalty is the integrity-score deduction for one violation:
//
//	penalty = weight(severity) × confidence
//
// where confidence is the AI confidence in [0,1] (1 for client and proctor
// events). The session score is 100 − Σ penalties of non-dismissed
// violations, floored at 0.
func Penalty(sev models.Severity, confidence *float64) float64 {
	c := 1.0
	if confidence != nil {
		c = math.Max(0, math.Min(1, *confidence))
	}
	return SeverityWeight[sev] * c
}

// IntegrityFrom recomputes a score from scratch (used after a review
// dismisses or confirms a violation).
func IntegrityFrom(vs []models.Violation) float64 {
	score := 100.0
	for _, v := range vs {
		if v.ReviewStatus == models.ReviewDismissed {
			continue
		}
		score -= Penalty(v.Severity, v.AIConfidence)
	}
	return math.Max(0, math.Round(score*100)/100)
}
