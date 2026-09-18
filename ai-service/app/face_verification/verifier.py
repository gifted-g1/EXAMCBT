"""
Face verification: turns a detected face crop into a fixed-length
feature vector and compares it against a student's stored reference
vector using cosine similarity.

NOTE: the feature extractor here (normalized, flattened grayscale
histogram-of-gradients-lite) is intentionally lightweight so the
service has no heavy model-download dependency. For production
accuracy, swap FeatureExtractor internals for a proper face-embedding
model (e.g. ArcFace/FaceNet) behind the same extract() interface —
nothing else in the service needs to change.
"""
from dataclasses import dataclass

import cv2
import numpy as np

EMBEDDING_SIZE = (64, 64)
MATCH_THRESHOLD = 0.80


@dataclass
class VerificationResult:
    matched: bool
    confidence: float


class FeatureExtractor:
    def extract(self, face_bgr: np.ndarray) -> np.ndarray:
        gray = cv2.cvtColor(face_bgr, cv2.COLOR_BGR2GRAY)
        resized = cv2.resize(gray, EMBEDDING_SIZE)
        equalized = cv2.equalizeHist(resized)

        gx = cv2.Sobel(equalized, cv2.CV_32F, 1, 0, ksize=3)
        gy = cv2.Sobel(equalized, cv2.CV_32F, 0, 1, ksize=3)
        magnitude = cv2.magnitude(gx, gy)

        vector = magnitude.flatten().astype(np.float32)
        norm = np.linalg.norm(vector)
        if norm > 0:
            vector = vector / norm
        return vector


class FaceVerifier:
    def __init__(self) -> None:
        self._extractor = FeatureExtractor()
        # In-memory reference store keyed by student_id, in this
        # scaffold. A production deployment persists reference
        # embeddings (not raw images) in the database via Go, and this
        # service would fetch/cache them by student_id instead.
        self._references: dict[str, np.ndarray] = {}

    def register_reference(self, student_id: str, face_bgr: np.ndarray) -> None:
        self._references[student_id] = self._extractor.extract(face_bgr)

    def verify(self, student_id: str, face_bgr: np.ndarray) -> VerificationResult:
        candidate = self._extractor.extract(face_bgr)
        reference = self._references.get(student_id)
        if reference is None:
            # No reference on file yet: treat this capture as the
            # registration event so the exam isn't blocked, matching
            # the "register on first authorized use" workflow.
            self.register_reference(student_id, face_bgr)
            return VerificationResult(matched=True, confidence=1.0)

        similarity = float(np.dot(candidate, reference))
        similarity = max(0.0, min(1.0, similarity))
        return VerificationResult(matched=similarity >= MATCH_THRESHOLD, confidence=round(similarity, 4))
