# MCP Proxy Server Configuration Schema

This document describes the JSON configuration schema used by the MCP Proxy Server.

## Configuration Structure

The configuration file is a JSON object with the following structure:

```json
{
  "mcp_servers": [
    {
      "name": "string",
      "address": "string",
      "command": "string",
      "args": ["string", "..."],
      "env": {"KEY": "value", "...": "..."},
      "allowed_tools": ["string", "..."],
      "allowed_resources": ["string", "..."]
    }
  ]
}
```

### Fields

- `mcp_servers` (array, required): List of MCP server configurations.

Each MCP server configuration object contains:

- `name` (string, required): Unique name identifier for the MCP server.
- `address` (string, optional): Network address of the MCP server (e.g., `127.0.0.1:50051` or `mcp.example.com:443`). Required if `command` is not specified.
- `command` (string, optional): Command to start a stdio-based MCP server locally. Required if `address` is not specified.
- `args` (array of strings, optional): Arguments to pass to the command when starting a stdio-based MCP server.
- `env` (object, optional): Environment variables to set when starting the stdio-based MCP server, specified as key-value pairs.
- `allowed_tools` (array of strings, optional): List of tool names allowed for this MCP server. If omitted or empty, all tools are allowed.
- `allowed_resources` (array of strings, optional): List of resource URIs allowed for this MCP server. If omitted or empty, all resources are allowed.

### Required vs Optional Fields

- Either `address` or `command` must be specified for each MCP server.
- `name` is mandatory and must be unique.
- `allowed_tools` and `allowed_resources` are optional; if omitted or empty, no restrictions apply.

## Validation Rules

- At least one MCP server must be defined.
- Each MCP server must have a unique, non-empty `name`.
- Each MCP server must have at least one of `address` or `command` specified.
- `allowed_tools` and `allowed_resources` are optional and can be empty or omitted to allow all.

## Example

Example configuration showing both HTTP and stdio-based MCP servers:

```json
{
  "mcp_servers": [
    {
      "name": "http-mcp-server",
      "address": "127.0.0.1:50051",
      "allowed_tools": ["search_package_docs", "get_completions"],
      "allowed_resources": ["repo://owner/repo/refs/heads/main/contents/file.go"]
    },
    {
      "name": "stdio-mcp-server",
      "command": "/usr/local/bin/mcp-stdio-server",
      "args": ["--verbose", "--config", "/etc/mcp/config.yaml"],
      "env": {
        "MCP_LOG_LEVEL": "debug",
        "MCP_TIMEOUT": "30s"
      },
      "allowed_tools": ["search_package_docs"],
      "allowed_resources": ["repo://owner/repo/refs/heads/main/contents/file.go"]
    }
  ]
}
```

## Environment Variable

The path to the configuration file can be set using the environment variable `MCP_PROXY_CONFIG`.

## Notes

- The proxy server enforces allow-lists for tools and resources per MCP server.
- If allow-lists are empty or omitted, no restrictions are applied.
- For stdio-based MCP servers, the proxy will start the specified command with optional arguments and environment variables, managing the process lifecycle.

## Troubleshooting Tips

- Ensure the configuration file is valid JSON and follows the schema.
- Verify that `command` paths are executable and arguments are correct.
- Check logs for errors related to MCP server startup or connection issues.
- Use the example configuration at `configs/example-config.json` as a reference.