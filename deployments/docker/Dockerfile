FROM golang:1.22-bookworm AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /hvc-server ./cmd/server

FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    ffmpeg \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /hvc-server /usr/local/bin/hvc-server
COPY configs/config.yaml /etc/hvc/config.yaml

EXPOSE 8080 9090

ENTRYPOINT ["hvc-server"]
CMD ["--config", "/etc/hvc/config.yaml"]
