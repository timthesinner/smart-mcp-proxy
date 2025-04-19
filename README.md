# Smart MCP Proxy

Smart MCP Proxy is a powerful, secure, and configurable gateway that centralizes access to multiple Model Context Protocol (MCP) servers. It enables organizations to manage tokens centrally, unify MCP server configurations, and expose only the tools and resources they choose — all through a single, transparent HTTP interface with detailed logging.

## Table of Contents

- [Introduction](#introduction)
- [Value Proposition](#value-proposition)
- [Features](#features)
- [Architecture](#architecture)
- [Configuration](#configuration)
- [Usage](#usage)
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

### Command Mode (STDIO)

The proxy can also run in "command mode" where it communicates with the MCP client via STDIN/STDOUT instead of HTTP. This mode is useful for integration scenarios requiring direct stdio communication.

### Running in HTTP Mode

To run the proxy in the default HTTP mode, use:

```bash
./smart-mcp-proxy -config /path/to/config.json
```

or set the environment variable and run:

```bash
export MCP_PROXY_CONFIG=/path/to/config.json
./smart-mcp-proxy
```

### Running in Command (STDIO) Mode

To run the proxy in command mode, which uses STDIN/STDOUT for communication and logs to STDERR, use:

```bash
./smart-mcp-proxy -config /path/to/config.json -mode command
```

or set the environment variable:

```bash
export MCP_PROXY_MODE=command
export MCP_PROXY_CONFIG=/path/to/config.json
./smart-mcp-proxy
```

### Docker Usage

The Docker image runs the proxy in command mode by default. To run the container:

```bash
docker run -e MCP_PROXY_CONFIG=/path/to/config.json smart-mcp-proxy
```

To override the mode to HTTP mode at runtime:

```bash
docker run -e MCP_PROXY_MODE=http -e MCP_PROXY_CONFIG=/path/to/config.json smart-mcp-proxy
```

### VSCode Launch Configuration

A launch configuration is provided to run the proxy in command mode for local development and debugging. Use the "Run MCP Proxy in Command Mode" configuration in VSCode's Run and Debug panel. It uses the example config file by default.


To run in command mode, use the `-mode command` flag or set the environment variable `MCP_PROXY_MODE=command`:

```bash
./smart-mcp-proxy -config /path/to/config.json -mode command
```

Or:

```bash
export MCP_PROXY_MODE=command
./smart-mcp-proxy
```

### Docker Usage

The Docker image defaults to running in command mode. To override and run in HTTP mode, set the `MCP_PROXY_MODE` environment variable when running the container:

```bash
docker run -e MCP_PROXY_CONFIG=/path/to/config.json -e MCP_PROXY_MODE=http smart-mcp-proxy
```

### Launch Configuration

A VSCode launch configuration is provided to run the proxy in command mode locally. See `.vscode/launch.json` for details.

For stdio-based MCP servers, the proxy manages the lifecycle of the server process, including starting and stopping it as needed.

See detailed usage instructions in [docs/usage.md](docs/usage.md).

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
docker run -p 8080:8080 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v ./config.json:/app/config.json:ro \
  -e MCP_PROXY_CONFIG=/app/config.json \
  --privileged \
  --user "$(id -u):$(id -g)" \
  ghcr.io/timthesinner/smart-mcp-proxy:latest
```

This will start the Smart MCP Proxy server, listening on port 8080.

Supported architectures for the Docker image are `amd64` and `arm64`.

## Documentation

- [Configuration Reference](docs/configuration.md)
- [Usage Guide](docs/usage.md)
- [Model Context Protocol Specification](https://modelcontextprotocol.io/specification/2025-03-26)

## License

[MIT](LICENSE)