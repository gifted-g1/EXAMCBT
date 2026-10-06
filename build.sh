#!/usr/bin/env bash
# Builds the React frontend, embeds it into the Go backend via
# go:embed, and produces a single backend binary that serves the UI,
# the REST/WebSocket API, and (in LAN mode) a genuinely offline exam —
# all from one process on one port.
#
# This is the same sequence backend/Dockerfile runs inside its
# multi-stage build; use this script for a native (non-Docker) build,
# e.g. the LAN-mode setup described in the README where the backend
# needs to run directly on the invigilating machine.
set -euo pipefail
cd "$(dirname "$0")"

echo "==> Building frontend..."
cd frontend
npm install
npm run build
cd ..

echo "==> Embedding frontend into backend..."
rm -rf backend/internal/webui/dist
cp -r frontend/dist backend/internal/webui/dist

echo "==> Building backend..."
cd backend
go build -o ../examshield-server ./cmd/server
cd ..

echo "==> Done: ./examshield-server"
