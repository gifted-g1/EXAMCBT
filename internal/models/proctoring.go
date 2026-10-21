package models

import "encoding/json"

// ProctoringSettings are the per-exam integrity switches. Each field is a
// pointer so "never set" (nil, e.g. exams created before the setting
// existed) can mean "on": proctoring is opt-out, never silently off.
type ProctoringSettings struct {
	FaceVerification   *bool
	CameraMonitoring   *bool
	AudioMonitoring    *bool
	FullscreenRequired *bool
	TabSwitchDetection *bool
	ClipboardBlocking  *bool
}

// Proctoring is the resolved, all-explicit form used by the API and UI.
type Proctoring struct {
	FaceVerification   bool `json:"face_verification"`
	CameraMonitoring   bool `json:"camera_monitoring"`
	AudioMonitoring    bool `json:"audio_monitoring"`
	FullscreenRequired bool `json:"fullscreen_required"`
	TabSwitchDetection bool `json:"tab_switch_detection"`
	ClipboardBlocking  bool `json:"clipboard_blocking"`
}

func on(b *bool) bool { return b == nil || *b }

// Resolve returns every switch with nil treated as on.
func (p ProctoringSettings) Resolve() Proctoring {
	return Proctoring{
		FaceVerification:   on(p.FaceVerification),
		CameraMonitoring:   on(p.CameraMonitoring),
		AudioMonitoring:    on(p.AudioMonitoring),
		FullscreenRequired: on(p.FullscreenRequired),
		TabSwitchDetection: on(p.TabSwitchDetection),
		ClipboardBlocking:  on(p.ClipboardBlocking),
	}
}

// NeedsMedia reports whether the student's browser must use camera or
// microphone (which requires a secure https:// origin).
func (p Proctoring) NeedsMedia() bool {
	return p.FaceVerification || p.CameraMonitoring || p.AudioMonitoring
}

// Settings converts resolved values back into stored settings.
func (p Proctoring) Settings() ProctoringSettings {
	b := func(v bool) *bool { return &v }
	return ProctoringSettings{
		FaceVerification:   b(p.FaceVerification),
		CameraMonitoring:   b(p.CameraMonitoring),
		AudioMonitoring:    b(p.AudioMonitoring),
		FullscreenRequired: b(p.FullscreenRequired),
		TabSwitchDetection: b(p.TabSwitchDetection),
		ClipboardBlocking:  b(p.ClipboardBlocking),
	}
}

// MarshalJSON always emits the resolved values, so API clients, the Redis
// exam cache and the database column all see explicit booleans.
func (p ProctoringSettings) MarshalJSON() ([]byte, error) { return json.Marshal(p.Resolve()) }

// UnmarshalJSON accepts a partial object: absent keys stay nil (= on).
func (p *ProctoringSettings) UnmarshalJSON(b []byte) error {
	var raw struct {
		FaceVerification   *bool `json:"face_verification"`
		CameraMonitoring   *bool `json:"camera_monitoring"`
		AudioMonitoring    *bool `json:"audio_monitoring"`
		FullscreenRequired *bool `json:"fullscreen_required"`
		TabSwitchDetection *bool `json:"tab_switch_detection"`
		ClipboardBlocking  *bool `json:"clipboard_blocking"`
	}
	if string(b) == "null" {
		*p = ProctoringSettings{}
		return nil
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	*p = ProctoringSettings(raw)
	return nil
}
