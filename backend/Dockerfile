# Build context for this Dockerfile is the REPOSITORY ROOT (not
# backend/), because it needs both frontend/ and backend/ — see
# docker-compose.yml's `context: .` / `dockerfile: backend/Dockerfile`.

# Stage 1 — build the React frontend
FROM node:20-slim AS frontend-build
WORKDIR /frontend
COPY frontend/package.json ./
RUN npm install
COPY frontend/ ./
RUN npm run build

# Stage 2 — build the Go binary, with the frontend build embedded via go:embed
FROM golang:1.22-bookworm AS backend-build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
# The embedded frontend must be in place *before* `go build`, since
# go:embed reads internal/webui/dist at compile time.
RUN rm -rf internal/webui/dist
COPY --from=frontend-build /frontend/dist ./internal/webui/dist
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/examshield-server ./cmd/server

# Stage 3 — minimal runtime image
FROM gcr.io/distroless/static-debian12
COPY --from=backend-build /out/examshield-server /examshield-server
EXPOSE 8080
ENTRYPOINT ["/examshield-server"]
