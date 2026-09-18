FROM golang:1.22-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/examshield-server ./cmd/server

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/examshield-server /examshield-server
EXPOSE 8080
ENTRYPOINT ["/examshield-server"]
