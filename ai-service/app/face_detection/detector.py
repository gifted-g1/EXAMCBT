"""
Face detection built on OpenCV's Haar cascade classifiers. This module
has exactly one job: given an image, find zero or more face bounding
boxes. It knows nothing about identity, verification, or exam policy.
"""
from dataclasses import dataclass
from typing import List

import cv2
import numpy as np


@dataclass
class FaceBox:
    x: int
    y: int
    w: int
    h: int


class FaceDetector:
    def __init__(self) -> None:
        cascade_path = cv2.data.haarcascades + "haarcascade_frontalface_default.xml"
        self._cascade = cv2.CascadeClassifier(cascade_path)
        if self._cascade.empty():
            raise RuntimeError("failed to load haar cascade classifier")

    def detect(self, bgr_image: np.ndarray) -> List[FaceBox]:
        gray = cv2.cvtColor(bgr_image, cv2.COLOR_BGR2GRAY)
        gray = cv2.equalizeHist(gray)
        faces = self._cascade.detectMultiScale(
            gray,
            scaleFactor=1.1,
            minNeighbors=5,
            minSize=(60, 60),
        )
        return [FaceBox(x=int(x), y=int(y), w=int(w), h=int(h)) for (x, y, w, h) in faces]

    def crop_largest_face(self, bgr_image: np.ndarray) -> np.ndarray | None:
        boxes = self.detect(bgr_image)
        if not boxes:
            return None
        largest = max(boxes, key=lambda b: b.w * b.h)
        return bgr_image[largest.y: largest.y + largest.h, largest.x: largest.x + largest.w]
