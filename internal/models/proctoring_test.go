package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProctoringDefaultsOnAndPartialJSON(t *testing.T) {
	// Never set (old exams / NULL column): everything on.
	var zero ProctoringSettings
	r := zero.Resolve()
	assert.True(t, r.FaceVerification && r.CameraMonitoring && r.AudioMonitoring &&
		r.FullscreenRequired && r.TabSwitchDetection && r.ClipboardBlocking)
	assert.True(t, r.NeedsMedia())

	// Partial JSON: only the named switches change.
	var p ProctoringSettings
	require.NoError(t, json.Unmarshal([]byte(`{"camera_monitoring":false,"face_verification":false}`), &p))
	r = p.Resolve()
	assert.False(t, r.CameraMonitoring)
	assert.False(t, r.FaceVerification)
	assert.True(t, r.AudioMonitoring, "unspecified stays on")

	// Marshalling always emits all six resolved values.
	b, err := json.Marshal(p)
	require.NoError(t, err)
	assert.JSONEq(t, `{"face_verification":false,"camera_monitoring":false,"audio_monitoring":true,
		"fullscreen_required":true,"tab_switch_detection":true,"clipboard_blocking":true}`, string(b))

	off := Proctoring{}
	assert.False(t, off.NeedsMedia())
	assert.Equal(t, off, off.Settings().Resolve())
}
