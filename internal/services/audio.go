package services

import (
	"encoding/binary"
	"errors"
	"math"

	"examshield/internal/store"
)

// Audio clips arrive as little-endian signed 16-bit PCM, 16 kHz mono.
const (
	AudioSampleRate  = 16000
	MaxAudioSeconds  = 5
	speechFlagThresh = 0.6
)

var ErrBadAudio = errors.New("audio must be 16-bit PCM, 16 kHz mono, 0.25–5 s")

// DecodePCM16 converts raw PCM16LE bytes to float32 samples in [-1,1].
func DecodePCM16(b []byte) ([]float32, error) {
	if len(b)%2 != 0 || len(b) < AudioSampleRate/2 || len(b) > AudioSampleRate*2*MaxAudioSeconds {
		return nil, ErrBadAudio
	}
	out := make([]float32, len(b)/2)
	for i := range out {
		out[i] = float32(int16(binary.LittleEndian.Uint16(b[2*i:]))) / 32768
	}
	return out, nil
}

// EvaluateSpeech flags sustained speech near the candidate.
func EvaluateSpeech(prob float64) []store.Flag {
	if prob < speechFlagThresh {
		return nil
	}
	return []store.Flag{{
		EventType:  "VOICE_DETECTED",
		Severity:   string(AIEvents["VOICE_DETECTED"].Severity),
		Confidence: math.Round(prob*1000) / 1000,
	}}
}
