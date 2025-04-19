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