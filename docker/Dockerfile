# Use latest LTS Debian base image
FROM debian:bookworm-slim

# Set environment variables for non-interactive installs
ENV DEBIAN_FRONTEND=noninteractive

# Install dependencies: curl, gnupg, ca-certificates, python3, nodejs (LTS)
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    gnupg \
    ca-certificates \
    python3 \
    python3-pip \
    && rm -rf /var/lib/apt/lists/*

# Install Node.js LTS (using NodeSource)
RUN curl -fsSL https://deb.nodesource.com/setup_lts.x | bash - && \
    apt-get install -y nodejs && \
    rm -rf /var/lib/apt/lists/*

# Create a non-root user to run the Go process
RUN useradd -m -d /home/proxyuser -s /bin/bash proxyuser

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum first for caching
COPY go.mod go.sum ./

# Download Go modules (assuming Go is installed in build stage, but here we only prepare Dockerfile)
# This step is placeholder for multi-stage build, actual Go build will be done in build stage

# Copy the rest of the source code
COPY . .

# Change ownership to non-root user
RUN chown -R proxyuser:proxyuser /app

# Switch to non-root user
USER proxyuser

# Expose port 8080
EXPOSE 8080

# Entrypoint to start the Go HTTP server process
ENTRYPOINT ["./cmd/proxy/proxy"]