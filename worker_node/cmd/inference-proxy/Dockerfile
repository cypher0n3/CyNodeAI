# Build inference-proxy binary (worker_node module; requires go_shared_libs from workspace).
# Build context must be repo root: podman build -f worker_node/cmd/inference-proxy/Dockerfile -t cynodeai-inference-proxy:dev .
# Use a minimal go.work so we do not need to copy orchestrator or cynork.
FROM golang:1.25-alpine AS builder
WORKDIR /build
RUN printf '%s\n' 'go 1.25.7' 'use (' './go_shared_libs' './worker_node' ')' > go.work
COPY go_shared_libs ./go_shared_libs
COPY worker_node ./worker_node
RUN go build -o /inference-proxy ./worker_node/cmd/inference-proxy

# Minimal runtime image.
FROM alpine:3.20
COPY --from=builder /inference-proxy /inference-proxy
EXPOSE 11434
CMD ["/inference-proxy"]
