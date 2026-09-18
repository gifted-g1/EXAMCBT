"""
Behavior analysis: turns a single monitoring frame, plus a small amount
of per-student rolling state, into zero or more classified events.

Design principle (per platform spec): this module NEVER concludes a
student is cheating. It only detects observable conditions (no face,
multiple faces, face displacement, sustained look-away) and assigns a
severity for human review. Escalation across repeated events is a
signal, not a verdict.
"""
import time
from dataclasses import dataclass, field
from typing import Dict, List

import numpy as np

from app.face_detection.detector import FaceBox, FaceDetector

LOOK_AWAY_GRACE_SECONDS = 5.0
NO_FACE_GRACE_SECONDS = 3.0


@dataclass
class DetectedEvent:
    event_type: str
    severity: str
    confidence: float
    description: str


@dataclass
class StudentMonitoringState:
    last_face_seen_at: float = field(default_factory=time.time)
    face_missing_since: float | None = None
    last_face_center: tuple[float, float] | None = None
    consecutive_multi_face_frames: int = 0


class BehaviorAnalyzer:
    def __init__(self) -> None:
        self._detector = FaceDetector()
        self._state: Dict[str, StudentMonitoringState] = {}

    def _state_for(self, student_id: str) -> StudentMonitoringState:
        if student_id not in self._state:
            self._state[student_id] = StudentMonitoringState()
        return self._state[student_id]

    def analyze(self, student_id: str, frame_bgr: np.ndarray) -> List[DetectedEvent]:
        state = self._state_for(student_id)
        faces = self._detector.detect(frame_bgr)
        now = time.time()
        events: List[DetectedEvent] = []

        if len(faces) == 0:
            if state.face_missing_since is None:
                state.face_missing_since = now
            missing_for = now - state.face_missing_since
            if missing_for >= NO_FACE_GRACE_SECONDS:
                events.append(DetectedEvent(
                    event_type="FACE_NOT_VISIBLE",
                    severity="MEDIUM" if missing_for < 15 else "HIGH",
                    confidence=0.85,
                    description="Student face is no longer visible in the camera frame",
                ))
            return events

        # Face(s) present: clear the missing-face timer.
        state.face_missing_since = None
        state.last_face_seen_at = now

        if len(faces) > 1:
            state.consecutive_multi_face_frames += 1
            severity = "HIGH" if state.consecutive_multi_face_frames >= 3 else "MEDIUM"
            events.append(DetectedEvent(
                event_type="MULTIPLE_FACES",
                severity=severity,
                confidence=min(0.99, 0.7 + 0.05 * state.consecutive_multi_face_frames),
                description="More than one face detected in the camera frame",
            ))
            return events
        else:
            state.consecutive_multi_face_frames = 0

        # Single face: check for large positional jumps (possible
        # camera tampering, student leaning far out of frame, etc).
        primary = self._largest(faces)
        center = (primary.x + primary.w / 2, primary.y + primary.h / 2)
        if state.last_face_center is not None:
            dx = center[0] - state.last_face_center[0]
            dy = center[1] - state.last_face_center[1]
            distance = (dx ** 2 + dy ** 2) ** 0.5
            frame_diagonal = (frame_bgr.shape[0] ** 2 + frame_bgr.shape[1] ** 2) ** 0.5
            if frame_diagonal > 0 and (distance / frame_diagonal) > 0.35:
                events.append(DetectedEvent(
                    event_type="SIGNIFICANT_FACE_POSITION_CHANGE",
                    severity="LOW",
                    confidence=0.6,
                    description="Significant change in face position detected between frames",
                ))
        state.last_face_center = center

        return events

    @staticmethod
    def _largest(faces: List[FaceBox]) -> FaceBox:
        return max(faces, key=lambda b: b.w * b.h)
