package services

import (
	"examshield/internal/ai"
	"examshield/internal/store"
)

// EvaluateFaces turns a face count into NO_FACE / MULTIPLE_FACES flags.
// significant counts faces at least a quarter the size of the largest, so a
// tiny background face (poster, passer-by far away) is ignored.
func EvaluateFaces(boxes []ai.Box) []store.Flag {
	best := ai.LargestFace(boxes)
	if best == nil {
		return []store.Flag{{EventType: "NO_FACE", Severity: string(AIEvents["NO_FACE"].Severity), Confidence: 0.9}}
	}
	n := 0
	var second float32
	for _, b := range boxes {
		if float64(b.Area()) >= 0.25*float64(best.Area()) {
			n++
			if b != *best && b.Score > second {
				second = b.Score
			}
		}
	}
	if n > 1 {
		return []store.Flag{{EventType: "MULTIPLE_FACES", Severity: string(AIEvents["MULTIPLE_FACES"].Severity),
			Confidence: float64(second), Metadata: map[string]any{"faces": n}}}
	}
	return nil
}

// EvaluateObjects flags prohibited items and extra people.
func EvaluateObjects(objs []ai.Object) []store.Flag {
	var flags []store.Flag
	best := map[string]float32{}
	persons := 0
	for _, o := range objs {
		if o.Label == "person" {
			persons++
		}
		if o.Score > best[o.Label] {
			best[o.Label] = o.Score
		}
	}
	add := func(event, label string) {
		if s, ok := best[label]; ok {
			flags = append(flags, store.Flag{EventType: event, Severity: string(AIEvents[event].Severity),
				Confidence: float64(s), Metadata: map[string]any{"object": label}})
		}
	}
	add("PHONE_DETECTED", "cell phone")
	add("BOOK_DETECTED", "book")
	add("EXTRA_DEVICE", "laptop")
	if persons > 1 {
		flags = append(flags, store.Flag{EventType: "EXTRA_PERSON", Severity: string(AIEvents["EXTRA_PERSON"].Severity),
			Confidence: float64(best["person"]), Metadata: map[string]any{"persons": persons}})
	}
	return flags
}
