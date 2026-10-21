# syntax=docker/dockerfile:1.7
#
# One image, four binaries: api, ai-worker, migrate, seed.
# api and ai-worker are built with -tags onnx (cgo) and ship ONNX Runtime
# 1.19.0, the C API version onnxruntime_go v1.12 is written against.
# Model files are NOT baked in: mount them at /app/models (see
# scripts/download-models.sh). Missing models fall back to pure Go.

ARG ORT_VERSION=1.19.0

# ---- 1. React UI ---------------------------------------------------------------
FROM node:20.18.0-bookworm-slim AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

# ---- 2. Go binaries + ONNX Runtime ------------------------------------------
FROM golang:1.22.8-bookworm AS build
ARG ORT_VERSION
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
RUN set -eux; \
    case "${TARGETARCH:-amd64}" in amd64) ort_arch=x64 ;; arm64) ort_arch=aarch64 ;; *) echo "unsupported arch"; exit 1 ;; esac; \
    curl -fsSL "https://github.com/microsoft/onnxruntime/releases/download/v${ORT_VERSION}/onnxruntime-linux-${ort_arch}-${ORT_VERSION}.tgz" \
      | tar -xz -C /opt; \
    mv /opt/onnxruntime-linux-${ort_arch}-${ORT_VERSION} /opt/onnxruntime
RUN CGO_ENABLED=1 go build -trimpath -tags onnx -ldflags="-s -w" -o /out/api ./cmd/api \
 && CGO_ENABLED=1 go build -trimpath -tags onnx -ldflags="-s -w" -o /out/ai-worker ./cmd/ai-worker \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/seed ./cmd/seed

# ---- 3. Runtime ----------------------------------------------------------------
FROM debian:bookworm-slim AS runtime
ARG ORT_VERSION
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl tzdata \
 && rm -rf /var/lib/apt/lists/* \
 && useradd --system --uid 10001 --home /app examshield
COPY --from=build /opt/onnxruntime/lib/libonnxruntime.so.${ORT_VERSION} /usr/lib/libonnxruntime.so
COPY --from=build /out/ /app/
# pigo cascade for the Go fallback (tiny); ONNX models are mounted at runtime.
COPY models/facefinder /app/models/facefinder
WORKDIR /app
USER examshield
ENV MODEL_DIR=/app/models \
    ONNXRUNTIME_LIB=/usr/lib/libonnxruntime.so \
    AI_BACKEND=auto
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=3s --retries=5 CMD curl -fsS http://127.0.0.1:8080/healthz || exit 1
CMD ["/app/api"]
