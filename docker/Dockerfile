# syntax=docker/dockerfile:experimental
# Multi-stage build for smart-mcp-proxy supporting amd64 and arm64

# Builder stage
FROM --platform=$BUILDPLATFORM golang:1.21-bullseye AS builder

# Set working directory inside builder
WORKDIR /src

# Copy go.mod and go.sum for dependency caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the Go binary for the target platform with correct name
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=$(dpkg --print-architecture) go build -o /smart-mcp-proxy ./cmd/proxy/main.go

# Final stage
FROM --platform=$TARGETPLATFORM debian:bookworm-slim

# Set environment variables for non-interactive installs
ENV DEBIAN_FRONTEND=noninteractive

# Install dependencies: curl, gnupg, ca-certificates, python3, python3-pip, nodejs, docker (for docker-in-docker)
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    gnupg \
    ca-certificates \
    python3 \
    python3-pip \
    docker.io \
    && rm -rf /var/lib/apt/lists/*

# Install Node.js LTS (using NodeSource)
RUN curl -fsSL https://deb.nodesource.com/setup_lts.x | bash - && \
    apt-get install -y nodejs && \
    rm -rf /var/lib/apt/lists/*

# Create a non-root user to run the Go process
RUN useradd -m -d /home/proxyuser -s /bin/bash proxyuser

# Copy the Go binary from builder stage
COPY --from=builder /smart-mcp-proxy /usr/local/bin/smart-mcp-proxy

# Change ownership to non-root user
RUN chown proxyuser:proxyuser /usr/local/bin/smart-mcp-proxy

# Set working directory
WORKDIR /app

# Switch to non-root user
USER proxyuser

# Expose port 8080
EXPOSE 8080

# Default command to run the Go binary
CMD ["/usr/local/bin/smart-mcp-proxy"]