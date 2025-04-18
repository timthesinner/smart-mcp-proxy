# Smart MCP Proxy

A secure, configurable Model Context Protocol (MCP) proxy server that exposes a curated set of tools from multiple downstream MCP servers. This project enables fine-grained control over which tools are accessible, providing a robust security boundary and a unified HTTP interface.

---

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Features](#features)
- [Technology Stack](#technology-stack)
- [Docker Containerization](#docker-containerization)
- [Configuration](#configuration)
- [Security Considerations](#security-considerations)
- [Development Setup](#development-setup)
- [Usage](#usage)
- [Contributing](#contributing)
- [License](#license)

---

## Overview

**Smart MCP Proxy** acts as a gateway between clients and a set of MCP servers, exposing only a user-defined subset of tools and endpoints. This allows organizations to enforce security policies, reduce attack surface, and present a simplified API to clients.

- **Runs as an HTTP server (port 8080) in a Docker container**
- **Proxies requests to a configurable list of MCP servers**
- **Exposes only whitelisted tools to clients**
- **Implements standard MCP HTTP endpoints**
- **Built with Go (Gin framework), managed with Go modules**
- **Docker image includes latest NodeJS and Python runtimes for downstream compatibility**

---

## Architecture

```
+-------------------+         +-------------------+         +-------------------+
|                   |         |                   |         |                   |
|   Client(s)       +-------->+  Smart MCP Proxy  +-------->+  MCP Server(s)    |
|                   | 8080    |   (this project)  |   MCP   |  (downstream)     |
+-------------------+         +-------------------+         +-------------------+
                                   |         |         |
                                   |         |         |
                                   v         v         v
                              [Tool A]   [Tool B]   [Tool C]
```

- The proxy receives HTTP requests on port 8080.
- Requests are routed to allowed tools on downstream MCP servers.
- Only tools explicitly configured are exposed to clients.

---

## Features

- **Configurable MCP Server List:** Specify which MCP servers to proxy.
- **Tool Whitelisting:** Expose only selected tools from each server.
- **Security-First Docker Image:** Non-root user, minimal base, latest LTS, NodeJS & Python runtimes.
- **Standard MCP Endpoints:** Implements all required MCP HTTP endpoints.
- **Extensible:** Easily add/remove MCP servers and tools via configuration.

---

## Technology Stack

- **Go (Golang):** Main application logic and HTTP server (Gin framework)
- **Docker:** Containerization, multi-stage build, security best practices
- **NodeJS & Python:** Latest versions installed for downstream tool compatibility
- **Go Modules:** Dependency management

---

## Docker Containerization

- **Base Image:** Latest LTS Linux (e.g., Ubuntu LTS or Debian Slim)
- **Runtimes:** Installs latest NodeJS and Python
- **Non-root User:** Application runs as a non-root user for security
- **Port:** Exposes HTTP server on port 8080
- **Layer Caching:** Optimized Dockerfile for build speed and cache efficiency

### Example Dockerfile (to be implemented)

```dockerfile
FROM ubuntu:22.04

# Install dependencies, NodeJS, Python, Go, create non-root user, etc.
# ...
# EXPOSE 8080
# USER appuser
# CMD ["./smart-mcp-proxy"]
```

---

## Configuration

The proxy is configured via a file or environment variables (to be defined), specifying:

- List of downstream MCP servers (host, port, protocol)
- Tools to expose from each server
- Authentication/authorization settings (future enhancement)

---

## Security Considerations

- **Principle of Least Privilege:** Only expose explicitly whitelisted tools.
- **Non-root Execution:** Docker container runs as a non-root user.
- **Minimal Base Image:** Reduces attack surface.
- **No direct access to downstream servers except via proxy logic.**
- **Future:** Add authentication, rate limiting, and audit logging.

---

## Development Setup

1. **Clone the repository:**
   ```sh
   git clone https://github.com/timthesinner/smart-mcp-proxy.git
   cd smart-mcp-proxy
   ```

2. **Install Go (latest stable):**
   - https://golang.org/doc/install

3. **Install Docker:**
   - https://docs.docker.com/get-docker/

4. **Build and run the container:**
   ```sh
   docker build -t smart-mcp-proxy .
   docker run -p 8080:8080 smart-mcp-proxy
   ```

5. **Local development:**
   ```sh
   go mod tidy
   go run main.go
   ```

---

## Usage

- **API Documentation:** To be defined (will follow MCP HTTP standards)
- **Configuration:** Specify MCP servers and tool whitelist in config file or environment variables

---

## Contributing

Contributions are welcome! Please open issues and pull requests.

---

## License

[MIT](LICENSE)