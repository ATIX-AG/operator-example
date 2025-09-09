# syntax=docker/dockerfile:1.7

############################
# Build the operator binary
############################
FROM golang:1.24 AS builder

# Speed up builds
ENV GOCACHE=/go-cache \
    GOMODCACHE=/gomod-cache \
    CGO_ENABLED=0 \
    GOOS=linux

WORKDIR /app/operator-example

# Copy Go module files first (for better caching)
COPY operator-example/go.mod go.mod
COPY operator-example/go.sum go.sum

# Download deps (cached)
RUN --mount=type=cache,target=/gomod-cache \
    --mount=type=cache,target=/go-cache \
    go mod download

# Copy the rest of the source
COPY operator-example/cmd/main.go cmd/main.go
COPY operator-example/api/ api/
COPY operator-example/internal/ internal/

# Build the static binary
RUN --mount=type=cache,target=/gomod-cache \
    --mount=type=cache,target=/go-cache \
    go build -a -installsuffix cgo -o /app/operator-example/operator ./cmd/main.go


######################################
# Minimal runtime image (distroless)
######################################
# If your app makes HTTPS requests, you’ll want CA certs.
# Use "base" instead of "static" to include them.
FROM gcr.io/distroless/base-debian12:nonroot

WORKDIR /

# Copy only the binary (not the whole source tree)
COPY --from=builder /app/operator-example/operator /operator

# Run as non-root (UID/GID provided by :nonroot images)
USER nonroot:nonroot

ENTRYPOINT ["/operator"]