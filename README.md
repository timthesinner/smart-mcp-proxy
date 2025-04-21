# Smart MCP Proxy
<p align="left">
  <img src="docs/icon.png" alt="Smart MCP Proxy Logo" width="150"/>
</p>


Smart MCP Proxy is a powerful, secure, and configurable gateway that centralizes access to multiple Model Context Protocol (MCP) servers. It enables organizations to manage tokens centrally, unify MCP server configurations, and expose only the tools and resources they choose — all through a single, transparent HTTP interface with detailed logging.

## Table of Contents

- [Introduction](#introduction)
- [Value Proposition](#value-proposition)
- [Features](#features)
- [Architecture](#architecture)
- [Configuration](#configuration)
- [Operating Modes](#operating-modes)
- [Usage](#usage)
- [Docker Usage](#docker-usage)
- [Development](#development)
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

Smart MCP Proxy is configured primarily via a JSON file specifying MCP servers, allowed tools, and resources. The configuration file path can be set via the `MCP_PROXY_CONFIG` environment variable or the `-config` command-line argument.

See the detailed configuration documentation in [docs/configuration.md](docs/configuration.md) and the example configuration at [configs/example-config.json](configs/example-config.json).

## Operating Modes

The proxy can operate in two modes:

- **HTTP Mode (default):** The proxy listens on an HTTP port (default 8080) and communicates with clients using the standard MCP HTTP protocol.
- **Command/STDIO Mode:** The proxy communicates with a single client via standard input (STDIN) and standard output (STDOUT), using the MCP command protocol. Logs are written to standard error (STDERR). This mode is useful for direct integration with tools or scripts.

You can select the mode using:
- The `-mode` command-line flag (e.g., `-mode command`).
- The `MCP_PROXY_MODE` environment variable (e.g., `export MCP_PROXY_MODE=command`).

If neither is specified, the proxy defaults to HTTP mode.

## Usage

### Running Locally

**HTTP Mode (Default):**

```bash
# Using command-line flag for config
./smart-mcp-proxy -config /path/to/config.json

# Using environment variable for config
export MCP_PROXY_CONFIG=/path/to/config.json
./smart-mcp-proxy
```

**Command/STDIO Mode:**

```bash
# Using command-line flags
./smart-mcp-proxy -config /path/to/config.json -mode command

# Using environment variables
export MCP_PROXY_CONFIG=/path/to/config.json
export MCP_PROXY_MODE=command
./smart-mcp-proxy
```

For stdio-based MCP servers defined in the configuration, the proxy manages the lifecycle of the server process, including starting and stopping it as needed.

See detailed usage instructions in [docs/usage.md](docs/usage.md).

## Docker Usage

The official Docker image `ghcr.io/timthesinner/smart-mcp-proxy:latest` supports `amd64` and `arm64` architectures.

### Default Mode (Command/STDIO)

The Docker image defaults to running the proxy in **Command/STDIO mode**.

```bash
# Example: Run in default Command/STDIO mode, mounting a local config
docker run --rm -i \
  -v ./configs/example-config.json:/app/config.json:ro \
  -e MCP_PROXY_CONFIG=/app/config.json \
  ghcr.io/timthesinner/smart-mcp-proxy:latest
```
*Note: `-i` is used to attach STDIN for command mode.*

### Overriding to HTTP Mode

To run the container in HTTP mode, set the `MCP_PROXY_MODE` environment variable to `http` and expose the port (default 8080).

```bash
# Example: Run in HTTP mode, mounting a local config and exposing port 8080
docker run --rm -d \
  -p 8080:8080 \
  -v ./configs/example-config.json:/app/config.json:ro \
  -e MCP_PROXY_CONFIG=/app/config.json \
  -e MCP_PROXY_MODE=http \
  ghcr.io/timthesinner/smart-mcp-proxy:latest
```

### Docker-in-Docker (DinD)

For scenarios requiring the proxy to interact with Docker (e.g., managing stdio servers running in containers), you might need to mount the host's Docker socket or use a DinD sidecar. See [docs/usage.md](docs/usage.md) for details and security considerations.

## Development

### Running Pre-commit Checks Locally

To run the pre-commit checks on your local machine, execute:

```bash
go fmt ./...
go test ./...
```

Ensure your code is properly formatted and all tests pass before committing.

### VS Code Launch Configuration

A VS Code launch configuration named **"Launch Proxy (STDIO Mode)"** is provided in `.vscode/launch.json`. This allows you to easily run and debug the proxy directly in Command/STDIO mode using the example configuration file. Access it via the "Run and Debug" panel in VS Code.

## Documentation

- [Configuration Reference](docs/configuration.md)
- [Usage Guide](docs/usage.md)
- [Model Context Protocol Specification](https://modelcontextprotocol.io/specification/2025-03-26)

## License

[MIT](LICENSE)