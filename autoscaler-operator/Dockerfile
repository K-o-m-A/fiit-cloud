# ---- build stage ----
FROM golang:1.26-alpine AS builder

WORKDIR /workspace

# Cache dependencies first
COPY go.mod go.sum ./
RUN go mod download

# Copy sources
COPY main.go .
COPY pkg/ pkg/

# Build a statically linked binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -a -ldflags="-s -w" \
    -o /workspace/operator \
    .

# ---- final stage ----
FROM gcr.io/distroless/static:nonroot

WORKDIR /

COPY --from=builder /workspace/operator /operator

# Run as non-root (distroless nonroot UID = 65532)
USER 65532:65532

ENTRYPOINT ["/operator"]
