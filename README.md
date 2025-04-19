# Smart MCP Proxy

Smart MCP Proxy is a powerful, secure, and configurable gateway that centralizes access to multiple Model Context Protocol (MCP) servers. It enables organizations to manage tokens centrally, unify MCP server configurations, and expose only the tools and resources they choose — all through a single, transparent HTTP interface with detailed logging.

## Table of Contents

- [Introduction](#introduction)
- [Value Proposition](#value-proposition)
- [Features](#features)
- [Architecture](#architecture)
- [Configuration](#configuration)
- [Usage](#usage)
- [CI/CD Pipeline](#cicd-pipeline)
- [Documentation](#documentation)
- [License](#license)

## Introduction

Smart MCP Proxy acts as a secure gateway between clients and multiple MCP servers, exposing a curated subset of tools and resources. It simplifies client interactions by presenting a unified API while enforcing fine-grained access control and transparent logging.

## Value Proposition

- **Centralized Token Management:** Manage authentication tokens in one place, reducing complexity and improving security.
- **Unified MCP Server Configuration:** Configure multiple MCP servers in a single proxy configuration file.
- **Fine-Grained Exposure:** Selectively expose only approved tools and resources from each MCP server.
- **Cost Savings Through Smaller Tool Footprint:** By exposing only the necessary tools and resources, Smart MCP Proxy reduces token overhead and operational costs, enabling organizations to optimize resource usage and minimize expenses.
- **Transparent Logging:** Detailed logs of all requests and responses for auditing and troubleshooting.

## Features

- Configurable list of MCP servers (HTTP and stdio-based).
- Tool and resource allow-listing per MCP server.
- Automatic lifecycle management of stdio-based MCP servers.
- Implements all standard MCP HTTP endpoints.
- Transparent request and response logging.
- Supports environment variable and command-line configuration.
- Example configuration provided for quick start.

## Architecture

```
+-------------------+         +-------------------+         +-------------------+
|                   |         |                   |         |                   |
|   Client(s)       +-------->+  Smart MCP Proxy  +-------->+  MCP Server(s)    |
|                   |  HTTP   |   (this project)  |   MCP   |  (downstream)     |
+-------------------+         +-------------------+         +-------------------+
                                   |         |         |
                                   |         |         |
                                   v         v         v
                              [Tool A]   [Tool B]   [Tool C]
```

- The proxy listens on HTTP port 8080.
- Routes client requests to allowed tools on configured MCP servers.
- Exposes only explicitly whitelisted tools and resources.

## Configuration

Smart MCP Proxy is configured via a JSON file specifying MCP servers, allowed tools, and resources. The configuration file path can be set via the `MCP_PROXY_CONFIG` environment variable or passed as a command-line argument.

See the detailed configuration documentation in [docs/configuration.md](docs/configuration.md) and the example configuration at [configs/example-config.json](configs/example-config.json).

## Usage

Run the proxy server with the configuration file specified:

```bash
./smart-mcp-proxy -config /path/to/config.json
```

Or set the environment variable and run:

```bash
export MCP_PROXY_CONFIG=/path/to/config.json
./smart-mcp-proxy
```

For stdio-based MCP servers, the proxy manages the lifecycle of the server process, including starting and stopping it as needed.

See detailed usage instructions in [docs/usage.md](docs/usage.md).

## CI/CD Pipeline

The Smart MCP Proxy project includes a robust CI/CD pipeline to ensure code quality and streamline deployment:

- **Pre-commit Checks:** Before committing code, the pipeline runs `go fmt` to enforce Go code formatting standards and `go test` to execute all tests, ensuring code correctness and consistency.
- **Multi-Architecture Docker Build:** The pipeline builds Docker images for both `amd64` and `arm64` architectures, enabling broad compatibility across different hardware platforms.
- **Docker Image Push:** Built images are pushed to the GitHub Container Registry, making them available for deployment and use.
- **Required Secrets:** To push Docker images, the pipeline requires GitHub secrets such as `CR_PAT` (Container Registry Personal Access Token) for authentication.

### Running Pre-commit Checks Locally

To run the pre-commit checks on your local machine, execute:

```bash
go fmt ./...
go test ./...
```

Ensure your code is properly formatted and all tests pass before committing.

### Pulling and Running the Docker Image

You can pull the latest multi-architecture Docker image from GitHub Container Registry:

```bash
docker pull ghcr.io/timthesinner/smart-mcp-proxy:latest
```

Run the Docker container with:

```bash
docker run -p 8080:8080 ghcr.io/timthesinner/smart-mcp-proxy:latest
```

This will start the Smart MCP Proxy server, listening on port 8080.

Supported architectures for the Docker image are `amd64` and `arm64`.

## Documentation

- [Configuration Reference](docs/configuration.md)
- [Usage Guide](docs/usage.md)
- [Model Context Protocol Specification](https://modelcontextprotocol.io/specification/2025-03-26)

## License

[MIT](LICENSE)