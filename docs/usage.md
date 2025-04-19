# MCP Proxy Server Usage

This document provides instructions on how to set up and run the MCP Proxy Server using a JSON configuration file.

## Overview

The MCP Proxy Server acts as a proxy to one or more MCP servers, managing connections and enforcing allow-lists for tools and resources.

## Configuration File

The MCP Proxy Server requires a JSON configuration file specifying the MCP servers to connect to, along with allowed tools and resources for each server.

## Setting the Configuration File

You can specify the path to the configuration file in one of two ways:

1. Pass the path as a command-line argument when starting the proxy server.
2. Set the environment variable `MCP_PROXY_CONFIG` to the path of the configuration file.

If neither is provided, the server will fail to start.

## Running the MCP Proxy Server

Example command to run the proxy server with a configuration file:

```bash
./smart-mcp-proxy -config /path/to/config.json
```

Or set the environment variable and run:

```bash
export MCP_PROXY_CONFIG=/path/to/config.json
./smart-mcp-proxy
```

## Example Configuration

See the example configuration file at `configs/example-config.json` for a sample setup, including examples of both HTTP-based and stdio-based MCP server configurations.

## Operating Modes

The MCP Proxy Server supports two primary modes of operation:

1.  **HTTP Mode (Default):**
    - The proxy listens on an HTTP port (default 8080).
    - Communicates with clients using the standard MCP HTTP protocol.
    - Suitable for typical client-server interactions over the network.

2.  **Command/STDIO Mode:**
    - The proxy communicates with a single client via standard input (STDIN) and standard output (STDOUT).
    - Uses the MCP command protocol.
    - Logs are written to standard error (STDERR).
    - Useful for direct integration with tools, scripts, or environments where HTTP is not desired (e.g., certain IDE extensions).

### Selecting the Mode

You can select the operating mode using either a command-line flag or an environment variable:

- **Command-Line Flag:** Use the `-mode` flag followed by `http` or `command`.
  ```bash
  # Run in Command/STDIO mode
  ./smart-mcp-proxy -config /path/to/config.json -mode command

  # Explicitly run in HTTP mode (though it's the default)
  ./smart-mcp-proxy -config /path/to/config.json -mode http
  ```

- **Environment Variable:** Set the `MCP_PROXY_MODE` environment variable to `http` or `command`.
  ```bash
  # Run in Command/STDIO mode
  export MCP_PROXY_MODE=command
  export MCP_PROXY_CONFIG=/path/to/config.json
  ./smart-mcp-proxy

  # Explicitly run in HTTP mode
  export MCP_PROXY_MODE=http
  export MCP_PROXY_CONFIG=/path/to/config.json
  ./smart-mcp-proxy
  ```

If neither the flag nor the environment variable is set, the proxy defaults to **HTTP mode**. The command-line flag takes precedence over the environment variable if both are set.

## Docker Usage

The official Docker image `ghcr.io/timthesinner/smart-mcp-proxy:latest` is available and supports `amd64` and `arm64` architectures.

### Default Mode (Command/STDIO)

By default, the Docker container runs the proxy in **Command/STDIO mode**. This is often suitable for integrations where the container's STDIN/STDOUT is connected to another process.

```bash
# Example: Run in default Command/STDIO mode
# Mount your config file and pass its path via environment variable
docker run --rm -i \
  -v /path/to/your/config.json:/app/config.json:ro \
  -e MCP_PROXY_CONFIG=/app/config.json \
  ghcr.io/timthesinner/smart-mcp-proxy:latest
```
*Note the use of `-i` (interactive) to keep STDIN open for the command mode.*

### Overriding to HTTP Mode

To run the container in HTTP mode, you must:
1. Set the `MCP_PROXY_MODE` environment variable to `http`.
2. Expose the proxy's port (default 8080) from the container.

```bash
# Example: Run in HTTP mode
# Mount config, set mode to http, and map port 8080
docker run --rm -d \
  -p 8080:8080 \
  -v /path/to/your/config.json:/app/config.json:ro \
  -e MCP_PROXY_CONFIG=/app/config.json \
  -e MCP_PROXY_MODE=http \
  --name my-mcp-proxy \
  ghcr.io/timthesinner/smart-mcp-proxy:latest
```
*Note the use of `-d` (detached) and `-p` (port mapping) for HTTP mode.*

## VS Code Launch Configuration for Development

For local development and debugging, a VS Code launch configuration is provided in `.vscode/launch.json`:

- **`Launch Proxy (STDIO Mode)`:** This configuration runs the proxy directly in Command/STDIO mode using the `configs/example-config.json` file. It's useful for testing the STDIO communication channel. You can find and run this configuration from the "Run and Debug" panel in VS Code.

## Docker-in-Docker (DinD) Support and Security Considerations

The smart-mcp-proxy project supports Docker-in-Docker (DinD) usage in two primary ways: by mounting the host Docker socket directly or by using a DinD sidecar container. Each approach has distinct security implications and usage scenarios.

### Mounting the Host Docker Socket

You can enable DinD support by mounting the host's Docker socket (`/var/run/docker.sock`) into the `smart-mcp-proxy` container. This allows the proxy server to communicate directly with the host Docker daemon.

**Security Implications:**

- Mounting the Docker socket grants the container full control over the host's Docker daemon.
- This effectively provides root-level access to the host system via Docker.
- Use this method only in trusted environments where you control the container and host security.
- Avoid using this approach in untrusted or multi-tenant environments.

**Enabling Socket Mounting:**

- In `docker-compose.yml` and `docker-compose-dev.yml`, mount the Docker socket as a volume:
  ```yaml
  volumes:
    - /var/run/docker.sock:/var/run/docker.sock
  ```
- Optionally, run the container in privileged mode to enable additional capabilities if required.

### Using the DinD Sidecar Approach

For CI/CD pipelines or scenarios where mounting the host Docker socket is not possible or desirable, you can run the MCP Proxy Server alongside a Docker-in-Docker (DinD) sidecar service.

This approach provides full Docker daemon isolation by running a separate Docker daemon inside a privileged container. The MCP Proxy Server is configured to communicate with this DinD daemon over TCP.

### Example Setup

An example Docker Compose file `docker-compose-dind-example.yml` is provided in the project root. It includes:

- A `dind` service using the `docker:dind` image, running in privileged mode with TLS disabled.
- The `smart-mcp-proxy` service configured with the environment variable `DOCKER_HOST=tcp://dind:2375` to connect to the DinD daemon.
- A named volume `dind-storage` for Docker daemon storage.

### When to Use

- In CI/CD pipelines where the host Docker socket cannot be mounted.
- When full isolation of the Docker daemon is required.
- For advanced scenarios requiring a separate Docker daemon lifecycle.

To use this setup, run:

```bash
docker-compose -f docker-compose-dind-example.yml up
```

## Using stdio-based MCP Servers

For MCP servers configured with the `command` field (stdio-based), the proxy server will start the specified command as a local process. You can optionally specify command-line arguments using the `args` field and environment variables using the `env` field in the configuration.

The proxy manages the lifecycle of the stdio-based MCP server process, including starting and stopping it as needed.

## Process Management and Troubleshooting

- Ensure the command path is correct and executable.
- Verify that any required arguments and environment variables are correctly specified.
- Check the MCP server logs for errors or startup issues.
- The proxy server logs connection attempts and validation errors; review these logs for troubleshooting.
- If the stdio-based MCP server fails to start or crashes, the proxy will attempt to restart it according to its internal policies.
- For debugging, run the stdio MCP server command manually to verify it starts correctly outside the proxy.

## Logs and Debugging

The proxy server will log connection attempts and validation errors. Ensure your configuration file is valid JSON and follows the schema described in the configuration documentation.

## Advanced Usage

- Multi-server setups: Configure multiple MCP servers with different allowed tools and resources.
- Custom tool/resource exposure: Fine-tune which tools and resources are exposed per MCP server.
- Environment variable overrides: Use environment variables to override configuration settings for flexible deployments.

## FAQ and Troubleshooting

- Q: What happens if a stdio-based MCP server crashes?
  A: The proxy will attempt to restart it automatically based on internal policies.

- Q: How do I restrict access to specific tools?
  A: Use the `allowed_tools` field in the configuration to whitelist tools per MCP server.

- Q: Can I expose all tools and resources without restrictions?
  A: Yes, omit or leave `allowed_tools` and `allowed_resources` empty to allow all.

- Q: Where can I find logs?
  A: Logs are output to the proxy server's standard output and can be redirected as needed.