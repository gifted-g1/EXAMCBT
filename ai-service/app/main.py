"""
Exam Shield AI Service.

Deliberately built on Python's standard library `http.server` rather
than Flask/FastAPI/Django, per the project's stack restriction that
Python is used ONLY for AI/computer-vision logic — no general-purpose
web framework. Go is the only application/business API layer; this
service exposes a minimal internal HTTP surface that only the Go
backend calls.

Endpoints:
    POST /verify-face              -> compare/register a student's face
    POST /analyze-frame            -> run behavior detection on a frame
    POST /detect-face              -> boolean: is a face present
    POST /detect-multiple-faces    -> boolean: is more than one face present
    GET  /healthz                  -> liveness check
"""
import base64
import json
import logging
import os
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any, Callable, Dict

import cv2
import numpy as np

from app.behavior_detection.analyzer import BehaviorAnalyzer
from app.face_detection.detector import FaceDetector
from app.face_verification.verifier import FaceVerifier

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
logger = logging.getLogger("examshield-ai")

detector = FaceDetector()
verifier = FaceVerifier()
analyzer = BehaviorAnalyzer()


def decode_image(image_base64: str) -> np.ndarray:
    """Decode a base64 JPEG/PNG frame into a BGR OpenCV image.

    Raises ValueError on malformed input; frames are decoded entirely
    in memory and are never written to disk, since raw camera footage
    must not be retained unnecessarily.
    """
    if "," in image_base64 and image_base64.strip().startswith("data:"):
        image_base64 = image_base64.split(",", 1)[1]
    raw = base64.b64decode(image_base64, validate=False)
    arr = np.frombuffer(raw, dtype=np.uint8)
    image = cv2.imdecode(arr, cv2.IMREAD_COLOR)
    if image is None:
        raise ValueError("could not decode image data")
    return image


def handle_verify_face(body: Dict[str, Any]) -> Dict[str, Any]:
    student_id = body.get("student_id")
    image_b64 = body.get("image_base64")
    is_reference = bool(body.get("is_reference", False))
    if not student_id or not image_b64:
        raise ValueError("student_id and image_base64 are required")

    image = decode_image(image_b64)
    face = detector.crop_largest_face(image)
    if face is None:
        return {"matched": False, "confidence": 0.0, "reason": "no_face_detected"}

    if is_reference:
        verifier.register_reference(student_id, face)
        return {"matched": True, "confidence": 1.0, "reason": "reference_registered"}

    result = verifier.verify(student_id, face)
    return {"matched": result.matched, "confidence": result.confidence}


def handle_analyze_frame(body: Dict[str, Any]) -> Dict[str, Any]:
    student_id = body.get("student_id")
    image_b64 = body.get("image_base64")
    if not student_id or not image_b64:
        raise ValueError("student_id and image_base64 are required")

    image = decode_image(image_b64)
    events = analyzer.analyze(student_id, image)
    return {
        "events": [
            {
                "event_type": e.event_type,
                "severity": e.severity,
                "confidence": e.confidence,
                "description": e.description,
            }
            for e in events
        ]
    }


def handle_detect_face(body: Dict[str, Any]) -> Dict[str, Any]:
    image = decode_image(body.get("image_base64", ""))
    faces = detector.detect(image)
    return {"face_detected": len(faces) > 0, "count": len(faces)}


def handle_detect_multiple_faces(body: Dict[str, Any]) -> Dict[str, Any]:
    image = decode_image(body.get("image_base64", ""))
    faces = detector.detect(image)
    return {"multiple_faces_detected": len(faces) > 1, "count": len(faces)}


ROUTES: Dict[str, Callable[[Dict[str, Any]], Dict[str, Any]]] = {
    "/verify-face": handle_verify_face,
    "/analyze-frame": handle_analyze_frame,
    "/detect-face": handle_detect_face,
    "/detect-multiple-faces": handle_detect_multiple_faces,
}


class Handler(BaseHTTPRequestHandler):
    server_version = "ExamShieldAI/1.0"

    def log_message(self, fmt: str, *args: Any) -> None:  # noqa: A003
        logger.info("%s - %s", self.address_string(), fmt % args)

    def _send_json(self, status: int, payload: Dict[str, Any]) -> None:
        data = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self) -> None:  # noqa: N802
        if self.path == "/healthz":
            self._send_json(200, {"status": "ok"})
            return
        self._send_json(404, {"error": "not found"})

    def do_POST(self) -> None:  # noqa: N802
        handler = ROUTES.get(self.path)
        if handler is None:
            self._send_json(404, {"error": "not found"})
            return

        try:
            length = int(self.headers.get("Content-Length", "0"))
            raw_body = self.rfile.read(length) if length > 0 else b"{}"
            body = json.loads(raw_body.decode("utf-8"))
        except (ValueError, json.JSONDecodeError):
            self._send_json(400, {"error": "invalid JSON body"})
            return

        try:
            result = handler(body)
            self._send_json(200, result)
        except ValueError as e:
            self._send_json(400, {"error": str(e)})
        except Exception:  # noqa: BLE001
            logger.exception("unhandled error processing %s", self.path)
            self._send_json(500, {"error": "internal AI service error"})


def main() -> None:
    port = int(os.environ.get("AI_SERVICE_PORT", "9000"))
    host = os.environ.get("AI_SERVICE_HOST", "0.0.0.0")
    server = ThreadingHTTPServer((host, port), Handler)
    logger.info("examshield-ai listening on %s:%s", host, port)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()


if __name__ == "__main__":
    main()
