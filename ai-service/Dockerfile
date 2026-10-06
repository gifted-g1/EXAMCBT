# Stage 1 — build
FROM golang:1.22-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/examshield-ai ./cmd/server

# Stage 2 — minimal runtime image
FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=build /out/examshield-ai /app/examshield-ai
COPY data/facefinder /app/data/facefinder
ENV CASCADE_PATH=/app/data/facefinder
EXPOSE 9000
ENTRYPOINT ["/app/examshield-ai"]
