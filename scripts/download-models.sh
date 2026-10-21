#!/usr/bin/env bash
# Fetches/exports the ONNX models into ./models. Every model is optional:
# anything missing falls back to the pure-Go detector (faces, embeddings,
# VAD) or is reported unsupported (gaze, head pose, objects).
#
# Verify every file you download (sha256) against a copy you trust before an
# exam: model drift silently changes thresholds. Expected shapes are checked
# at load time (see internal/ai/onnx.go) and mismatching files are rejected.
#
# Requirements: curl, unzip; python3 + pip for the YOLO/L2CS/6DRepNet exports.
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p models
cd models

fetch() { # url dest
  [ -f "$2" ] && { echo "✓ $2 (exists)"; return; }
  echo "↓ $2"; curl -fL --retry 3 -o "$2.part" "$1" && mv "$2.part" "$2"
}

# 1. ArcFace (InsightFace buffalo_l, w600k_r50: input [N,3,112,112] → [N,512]).
if [ ! -f arcface.onnx ]; then
  fetch https://github.com/deepinsight/insightface/releases/download/v0.7/buffalo_l.zip buffalo_l.zip
  unzip -o -j buffalo_l.zip w600k_r50.onnx && mv w600k_r50.onnx arcface.onnx && rm -f buffalo_l.zip
fi

# 2. Silero VAD v5 (inputs input/state/sr).
fetch https://raw.githubusercontent.com/snakers4/silero-vad/v5.1/src/silero_vad/data/silero_vad.onnx silero_vad.onnx

# 3. YOLOv8n (COCO, [1,84,8400]) and YOLOv8n-face ([1,5|20,8400]) via Ultralytics.
if command -v python3 >/dev/null; then
  if [ ! -f yolov8n.onnx ] || [ ! -f yolov8n-face.onnx ]; then
    python3 -m pip install --quiet "ultralytics==8.2.103"
  fi
  [ -f yolov8n.onnx ] || python3 - <<'PY'
from ultralytics import YOLO
YOLO("yolov8n.pt").export(format="onnx", imgsz=640, opset=12, simplify=True)
PY
  if [ ! -f yolov8n-face.onnx ]; then
    echo "→ yolov8n-face: place a YOLOv8n-face .pt (e.g. from github.com/akanametov/yolov8-face releases) as yolov8n-face.pt, then re-run."
    [ -f yolov8n-face.pt ] && python3 - <<'PY'
from ultralytics import YOLO
YOLO("yolov8n-face.pt").export(format="onnx", imgsz=640, opset=12, simplify=True)
PY
  fi
else
  echo "! python3 not found: skipping YOLO exports (objects + ONNX face detection fall back)."
fi

# 4. L2CS-Net (gaze) and 6DRepNet (head pose) have no official ONNX release.
#    Export them from their PyTorch repos:
#      L2CS-Net : github.com/Ahmednull/L2CS-Net  → torch.onnx.export(model, x[1,3,448,448], "l2cs_net.onnx",
#                 output_names=["yaw","pitch"], opset_version=12)
#      6DRepNet : github.com/thohemp/6DRepNet    → torch.onnx.export(model, x[1,3,224,224], "6drepnet.onnx",
#                 output_names=["rotation"], opset_version=12)
for f in l2cs_net.onnx 6drepnet.onnx; do
  [ -f "$f" ] || echo "• $f not present (export it manually; capability stays disabled until then)"
done

echo
sha256sum ./*.onnx 2>/dev/null || true
