package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"examshield/internal/ai"
	"examshield/internal/models"
)

func f(v float64) *float64 { return &v }

func TestPenaltyAndIntegrity(t *testing.T) {
	assert.Equal(t, 8.0, Penalty(models.SevHigh, nil))
	assert.Equal(t, 4.0, Penalty(models.SevHigh, f(0.5)))
	assert.Equal(t, 20.0, Penalty(models.SevCritical, f(7)), "confidence clamped to 1")

	vs := []models.Violation{
		{Severity: models.SevHigh},                                           // -8
		{Severity: models.SevMedium, AIConfidence: f(0.5)},                   // -1.5
		{Severity: models.SevCritical, ReviewStatus: models.ReviewDismissed}, // ignored
	}
	assert.Equal(t, 90.5, IntegrityFrom(vs))

	var many []models.Violation
	for i := 0; i < 10; i++ {
		many = append(many, models.Violation{Severity: models.SevCritical})
	}
	assert.Equal(t, 0.0, IntegrityFrom(many), "floored at 0")
}

func TestEvaluateFaces(t *testing.T) {
	assert.Equal(t, "NO_FACE", EvaluateFaces(nil)[0].EventType)
	one := []ai.Box{{W: 100, H: 100, Score: 0.9}}
	assert.Empty(t, EvaluateFaces(one))
	// A tiny background face is ignored; a comparable one is flagged.
	assert.Empty(t, EvaluateFaces(append(one, ai.Box{X: 300, W: 20, H: 20, Score: 0.9})))
	flags := EvaluateFaces(append(one, ai.Box{X: 300, W: 80, H: 80, Score: 0.7}))
	assert.Equal(t, "MULTIPLE_FACES", flags[0].EventType)
	assert.InDelta(t, 0.7, flags[0].Confidence, 1e-6)
}

func TestEvaluateObjects(t *testing.T) {
	flags := EvaluateObjects([]ai.Object{
		{Label: "person", Score: 0.9}, {Label: "person", Score: 0.8},
		{Label: "cell phone", Score: 0.6}, {Label: "cell phone", Score: 0.7},
	})
	types := map[string]float64{}
	for _, fl := range flags {
		types[fl.EventType] = fl.Confidence
	}
	assert.InDelta(t, 0.7, types["PHONE_DETECTED"], 1e-6)
	assert.Contains(t, types, "EXTRA_PERSON")
	assert.NotContains(t, types, "BOOK_DETECTED")
}

func TestPoseAndGaze(t *testing.T) {
	assert.Empty(t, EvaluatePose(&ai.Pose{Yaw: 10, Pitch: -20}))
	fl := EvaluatePose(&ai.Pose{Yaw: 50})
	assert.Equal(t, "LOOKING_AWAY", fl[0].EventType)
	assert.InDelta(t, 0.929, fl[0].Confidence, 1e-3)
	assert.Empty(t, EvaluateGaze(&ai.Gaze{Yaw: 5}))
	assert.Equal(t, "GAZE_OFF_SCREEN", EvaluateGaze(&ai.Gaze{Pitch: -40})[0].EventType)
	assert.Nil(t, EvaluatePose(nil))
}

func TestSpeechAndPCM(t *testing.T) {
	assert.Empty(t, EvaluateSpeech(0.3))
	assert.Equal(t, "VOICE_DETECTED", EvaluateSpeech(0.9)[0].EventType)
	_, err := DecodePCM16(make([]byte, 101))
	assert.Error(t, err)
	_, err = DecodePCM16(make([]byte, 100))
	assert.Error(t, err, "too short")
	pcm, err := DecodePCM16(make([]byte, 32000))
	assert.NoError(t, err)
	assert.Len(t, pcm, 16000)
}

func TestCalibration(t *testing.T) {
	onnx := calibrations["onnx"]
	assert.InDelta(t, 0.5, onnx.score(0.30), 1e-9)
	assert.Less(t, onnx.score(0.20), 0.25, "impostor")
	assert.Greater(t, onnx.score(0.45), 0.75, "same person")
	goCal := calibrations["go"]
	assert.Less(t, goCal.score(0.80), 0.75)
}

func TestDeriveFP(t *testing.T) {
	assert.Equal(t, "", DeriveFP("short"))
	assert.Equal(t, "", DeriveFP("has spaces in it!!!!!!"))
	a := DeriveFP("abcdefghijklmnop1234")
	assert.Len(t, a, 64)
	assert.Equal(t, a, DeriveFP("abcdefghijklmnop1234"))
	assert.NotEqual(t, a, DeriveFP("abcdefghijklmnop1235"))
}

func TestClientEventsDoNotIncludeAIEvents(t *testing.T) {
	for k := range AIEvents {
		_, clash := ClientEvents[k]
		assert.False(t, clash, "%s must not be reportable by the browser", k)
	}
}

func TestCSVSafe(t *testing.T) {
	assert.Equal(t, "'=HYPERLINK(1)", csvSafe("=HYPERLINK(1)"))
	assert.Equal(t, "'+1", csvSafe("+1"))
	assert.Equal(t, "Ada", csvSafe("Ada"))
}

func TestMatricFromFilename(t *testing.T) {
	assert.Equal(t, "IFT/22/9203", MatricFromFilename("IFT-22-9203.jpg"))
	assert.Equal(t, "IFT/22/9203", MatricFromFilename(`C:\photos\ift-22-9203.PNG`))
	assert.Equal(t, "", MatricFromFilename("passport.jpg"))
}
