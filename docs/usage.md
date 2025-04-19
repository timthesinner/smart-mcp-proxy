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

## Running Modes

The MCP Proxy Server supports two primary modes of operation:

### HTTP Mode

This is the default mode where the proxy listens on HTTP port 8080 and routes client requests to configured MCP servers. To run in HTTP mode:

```bash
./smart-mcp-proxy -config /path/to/config.json
```

or set the environment variable and run:

```bash
export MCP_PROXY_CONFIG=/path/to/config.json
./smart-mcp-proxy
```

### Command (STDIO) Mode

In command mode, the proxy communicates with the MCP client via STDIN/STDOUT instead of HTTP. This mode is useful for integration scenarios requiring direct stdio communication. To run in command mode:

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


## Command Mode (STDIO)

The proxy can also run in "command mode" where it communicates with the MCP client via STDIN/STDOUT instead of HTTP. This mode is useful for integration scenarios requiring direct stdio communication.

To run in command mode, use the `-mode command` flag or set the environment variable `MCP_PROXY_MODE=command`:

```bash
./smart-mcp-proxy -config /path/to/config.json -mode command
```

Or:

```bash
export MCP_PROXY_MODE=command
./smart-mcp-proxy
```

## Docker Usage

The Docker image defaults to running in command mode. To override and run in HTTP mode, set the `MCP_PROXY_MODE` environment variable when running the container:

```bash
docker run -e MCP_PROXY_CONFIG=/path/to/config.json -e MCP_PROXY_MODE=http smart-mcp-proxy
```

## Launch Configuration

A VSCode launch configuration is provided to run the proxy in command mode locally. See `.vscode/launch.json` for details.

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