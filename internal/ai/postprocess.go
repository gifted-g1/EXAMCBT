package ai

import (
	"math"
	"sort"
)

// DecodeYOLO parses a YOLOv8-style output of shape [1, C, N] (channel-major),
// where rows 0..3 are cx,cy,w,h in letterbox pixels and rows 4.. are class
// scores (C=84 for COCO; C=5 or 20 for YOLOv8-face, where row 4 is the face
// score and any remaining rows are landmarks). Only classes listed in keep
// are returned (nil keep = all), then class-wise NMS is applied.
func DecodeYOLO(out []float32, channels, anchors int, numClasses int, conf, iou float32, lb Letterbox, keep map[int]string) []Object {
	if channels < 4+numClasses || len(out) < channels*anchors {
		return nil
	}
	var dets []Object
	classOf := make([]int, 0, 32)
	for a := 0; a < anchors; a++ {
		best, bestScore := -1, float32(0)
		for c := 0; c < numClasses; c++ {
			s := out[(4+c)*anchors+a]
			if s > bestScore {
				best, bestScore = c, s
			}
		}
		if best < 0 || bestScore < conf {
			continue
		}
		label, ok := "", true
		if keep != nil {
			label, ok = keep[best]
		}
		if !ok {
			continue
		}
		cx, cy := float64(out[0*anchors+a]), float64(out[1*anchors+a])
		w, h := float64(out[2*anchors+a]), float64(out[3*anchors+a])
		box := lb.ToSource(cx, cy, w, h)
		box.Score = bestScore
		dets = append(dets, Object{Label: label, Score: bestScore, Box: box})
		classOf = append(classOf, best)
	}
	return nms(dets, classOf, iou)
}

func nms(dets []Object, classes []int, iouThr float32) []Object {
	idx := make([]int, len(dets))
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(a, b int) bool { return dets[idx[a]].Score > dets[idx[b]].Score })
	suppressed := make([]bool, len(dets))
	var out []Object
	for _, i := range idx {
		if suppressed[i] {
			continue
		}
		out = append(out, dets[i])
		for _, j := range idx {
			if j == i || suppressed[j] || classes[j] != classes[i] {
				continue
			}
			if IoU(dets[i].Box, dets[j].Box) > iouThr {
				suppressed[j] = true
			}
		}
	}
	return out
}

// IoU is intersection-over-union of two boxes.
func IoU(a, b Box) float32 {
	inter := a.Rect().Intersect(b.Rect())
	if inter.Empty() {
		return 0
	}
	i := float32(inter.Dx() * inter.Dy())
	u := float32(a.Area()+b.Area()) - i
	if u <= 0 {
		return 0
	}
	return i / u
}

// BinnedAngle converts L2CS-Net's 90 classification logits into degrees:
// softmax expectation over bin indices, ×4°, −180°.
func BinnedAngle(logits []float32) float64 {
	if len(logits) == 0 {
		return 0
	}
	maxv := logits[0]
	for _, v := range logits {
		if v > maxv {
			maxv = v
		}
	}
	var sum, exp float64
	for i, v := range logits {
		e := math.Exp(float64(v - maxv))
		sum += e
		exp += e * float64(i)
	}
	return exp/sum*4 - 180
}

// EulerFromRotation converts 6DRepNet's 3×3 rotation matrix (row-major) to
// pitch/yaw/roll in degrees, matching the reference implementation.
func EulerFromRotation(r []float32) Pose {
	if len(r) < 9 {
		return Pose{}
	}
	r00, r10, r20 := float64(r[0]), float64(r[3]), float64(r[6])
	r21, r22 := float64(r[7]), float64(r[8])
	r11, r12 := float64(r[4]), float64(r[5])
	sy := math.Sqrt(r00*r00 + r10*r10)
	var x, y, z float64
	if sy > 1e-6 {
		x = math.Atan2(r21, r22)
		y = math.Atan2(-r20, sy)
		z = math.Atan2(r10, r00)
	} else {
		x = math.Atan2(-r12, r11)
		y = math.Atan2(-r20, sy)
		z = 0
	}
	deg := 180 / math.Pi
	return Pose{Pitch: x * deg, Yaw: y * deg, Roll: z * deg}
}

// L2Normalize scales v to unit length in place.
func L2Normalize(v []float32) []float32 {
	var n float64
	for _, x := range v {
		n += float64(x) * float64(x)
	}
	if n == 0 {
		return v
	}
	inv := float32(1 / math.Sqrt(n))
	for i := range v {
		v[i] *= inv
	}
	return v
}

// COCO class ids the object detector reports.
var ProctorObjects = map[int]string{
	0:  "person",
	63: "laptop",
	67: "cell phone",
	73: "book",
}
