# Smart MCP Proxy - Architectural Mindmap

*   **Source References:** See [`source_references.md`](./source_references.md) for links to specific files mentioned below.
*   **Dependencies:** See [`dependencies.md`](./dependencies.md) for key external libraries.

## 1. Root: `smart-mcp-proxy`

*   **Purpose:** A secure, configurable gateway for multiple Model Context Protocol (MCP) servers. It centralizes token management, unifies server configurations, selectively exposes tools/resources based on allow-lists, and provides transparent logging via HTTP or STDIO interfaces.

## 2. Metadata & Core Info

*   **Owner:** `timthesinner`
*   **Repo:** `smart-mcp-proxy`
*   **Language:** Golang v1.24.2
*   **Containerization:** Docker
    *   Includes Dockerfile (multi-stage build) and example Compose files (prod, dev, DinD).
*   **Docker Repo:** `ghcr.io/timthesinner/smart-mcp-proxy`
*   **Configuration:** Environmental Variables / JSON Config File
    *   Priority: `-config` flag > `MCP_PROXY_CONFIG` env var.
    *   Validation: Ensures unique server names and address/command presence.
    *   Includes example configuration and Go code for loading/validation.
*   **CI/CD:** Github Actions (Assumed from metadata).
*   **License:** MIT

## 3. Architecture Overview

*   **Flow:** Client(s) <--(HTTP/STDIO)--> Smart MCP Proxy <--(MCP over HTTP/STDIO)--> MCP Server(s)
*   **Core Component:** `ProxyServer`
    *   Holds the list of initialized `config.MCPServer` instances.
    *   Provides methods for finding servers (`findMCPServerBy...`), listing allowed/restricted tools/resources (`ListTools`, `ListResources`, etc.), and orchestrating tool calls (`CallTool`) and resource requests (`ProxyRequest`).
    *   Acts as the central logic hub shared between operating modes.
*   **Operating Modes:** Determined at startup. `MCP_PROXY_MODE` env var > `-mode` flag > default ('command').
    *   **HTTP Mode:** Listens on HTTP (default `:8080`), uses Gin framework, handles standard MCP HTTP endpoints.
    *   **Command/STDIO Mode:** Communicates via STDIN/STDOUT using JSON-RPC 2.0 messages, logs to STDERR.

## 4. Configuration & Server Management

*   **Loading (`LoadConfig`):** Reads JSON config file specified by flag or env var.
*   **Validation (`Config.Validate`):** Checks for empty server list, unique names, and presence of `address` or `command`.
*   **Runtime Server (`MCPServer` struct):**
    *   Represents a configured downstream server.
    *   Holds config, HTTP client (if HTTP type) or `exec.Cmd` + stdio pipes (if stdio type).
    *   Manages stdio process lifecycle (start, monitor, restart, shutdown) using context, WaitGroup, and mutexes.
    *   Caches discovered tools/resources (allowed and restricted) via `refreshToolsAndResources`.
*   **Stdio Process Management:**
    *   `startStdioProcess`: Launches command with context, sets up env vars, pipes, starts `monitorProcess`.
    *   `monitorProcess`: Goroutine logging stderr, waiting for exit. Restarts process on unexpected exit with 3s backoff, unless shutdown is in progress.
    *   `Shutdown`: Cancels context, waits for monitor goroutine (5s timeout), kills process if needed, closes pipes.
*   **Tool/Resource Discovery (`refreshToolsAndResources`):**
    *   Fetches lists from downstream servers (HTTP GET or stdio JSON-RPC `tools/list`, `resources/list`). Supports pagination for stdio.
    *   Filters results based on `AllowedTools`/`AllowedResources` config (empty list means allow all).
    *   Updates cached lists (`tools`, `resources`, `restrictedTools`, `restrictedResources`).
    *   *Note:* Periodic refresh logic exists (`startPeriodicRefresh`) but is currently disabled.
*   **Communication:**
    *   `HandleStdioRequest`: Sends JSON request + newline to stdin, reads response line from stdout (mutex-protected).
    *   `fetchToolsAndResourcesHTTP`: Handles HTTP GET for discovery.
    *   `fetchToolsAndResourcesStdio`: Handles stdio JSON-RPC for discovery.
*   **Allow Listing (`IsToolAllowed`, `IsResourceAllowed`):** Checks name against configured allow list.

## 5. Application Entrypoint

*   **Responsibilities:**
    *   Parses command-line flags (`-config`, `-mode`).
    *   Determines config path and operating mode based on flags and environment variables (env vars take precedence).
    *   Loads configuration (`config.LoadConfig`). Exits on failure.
    *   Initializes the core `ProxyServer` instance (`NewProxyServer`). Exits on failure.
    *   Initializes the mode-specific proxy (`NewHTTPProxy` or `NewCommandProxy`), passing the `ProxyServer` instance. Exits on failure.
    *   Calls `Run()` on the mode-specific proxy instance. Exits on failure.
    *   *Note:* Does not explicitly handle OS signals; relies on mode-specific `Run` implementations.

## 6. Core Proxy Logic

*   **`ProxyServer` Methods:**
    *   `findMCPServerBy...()`: Finds the *first* matching server in the list.
    *   `ListTools()`, `ListResources()`, `ListRestricted...()`: Aggregate results from all configured servers' cached lists.
    *   `CallTool()`: Finds server, delegates to `callStdioTool` or `callHttpTool`. Returns sentinel errors (`ErrToolNotFound`, `ErrBackendCommunication`, `ErrInternalProxy`).
    *   `ProxyRequest()`: Takes `ProxyRequestInput`, delegates to `proxyStdioRequestInternal` or `proxyHttpRequest`. Returns `ProxyResponseOutput`.
*   **Tool Call Implementation:**
    *   `callStdioTool`: Marshals JSON-RPC request, uses `server.HandleStdioRequest`, unmarshals `config.CallToolResult`. Maps errors.
    *   `callHttpTool`: Creates HTTP POST to `/tool/{toolName}`, sets 30s timeout, executes request, checks status, unmarshals `config.CallToolResult`. Maps errors.
*   **Resource Proxy Implementation:**
    *   `proxyHttpRequest`: Creates new HTTP request matching input, sets 30s timeout, executes, returns `ProxyResponseOutput`.
    *   `proxyStdioRequestInternal`: Creates MCP stdio protocol request (map), marshals, uses `server.HandleStdioRequest`, unmarshals MCP stdio response (map), converts to `ProxyResponseOutput`.
*   **Helpers:** `copyHeaders` (filters hop-by-hop), `singleJoiningSlash`.

## 7. HTTP Mode Logic

*   **Structure (`HTTPProxy`):** Holds `ProxyServer`, Gin engine, `http.Server`.
*   **Initialization (`NewHTTPProxy`):** Sets up Gin, Prometheus metrics (`sync.Once`), logging middleware, routes, and HTTP server with timeouts.
*   **Middleware:** Logs request details (method, path, status, duration) and updates Prometheus metrics (`httpRequestsTotal`, `httpRequestDur`).
*   **Routes:**
    *   `/metrics`: Prometheus handler.
    *   `/tools`, `/resources`, `/restricted-tools`, `/restricted-resources`: GET handlers calling `ps.List...`.
    *   `/tool/:toolName`: POST handler (`handleToolCall`).
    *   `/resource/:serverName/:resourceName/*proxyPath`: ANY handler (`handleResourceProxy`).
*   **Handlers:**
    *   `handleToolCall`: Binds JSON body, calls `ps.CallTool`, maps errors to HTTP status codes (404, 502, 500), returns result/error JSON.
    *   `handleResourceProxy`: Finds server, checks allowance, constructs backend path, delegates to `proxyRequest`.
    *   `proxyRequest` (Helper): Calls `ps.ProxyRequest`, handles proxy errors (returns 502), handles backend errors (logs, returns 502), copies headers/status/body for success.
*   **Lifecycle (`Run`):** Starts HTTP server, waits for SIGINT/SIGTERM, initiates graceful server shutdown (10s timeout), calls `ps.Shutdown()`.

## 8. Command/STDIO Mode Logic

*   **Protocol:** JSON-RPC 2.0 over STDIN/STDOUT. Defines request/response/error structs.
*   **Structure (`CommandProxy`):** Holds `ProxyServer`.
*   **Main Loop (`Run`):** Reads lines from stdin, calls `handleCommandRequest`, writes response/error JSON to stdout + newline. Logs errors to stderr.
*   **Shutdown (`Shutdown`):** No-op; relies on external `ProxyServer` shutdown.
*   **Request Handling (`handleCommandRequest`):** Parses request, validates version, dispatches based on method.
    *   List methods: Call `ps.List...`, format result.
    *   `tools/call`: Delegates to `handleToolCall`.
    *   `resources/access`: Delegates to `handleResourceAccess`.
    *   Other: Returns "Method not found" (-32601).
*   **Handlers:**
    *   `handleToolCall`: Parses params, calls `ps.CallTool`, maps errors to JSON-RPC error (-32000), returns result.
    *   `handleResourceAccess`: Parses params, finds server (-32001), checks allowance (-32002), creates `ProxyRequestInput`, calls `ps.ProxyRequest` (-32003), formats `ProxyResponseOutput` for JSON-RPC result (attempts JSON parse of body, falls back to string).
*   **Error Codes:** Uses standard JSON-RPC codes and custom codes (-32000 to -32003).

## 9. Dockerization

*   **Dockerfile:** Multi-stage build. Copies binary, exposes 8080. Defaults to Command/STDIO mode entrypoint.
*   **Compose Files:** Examples for production, development, and Docker-in-Docker scenarios.
*   **Usage:** Runs in Command/STDIO mode by default. Override to HTTP mode via `MCP_PROXY_MODE=http` env var and exposing port.

## 10. Testing

*   **Strategy:** Unit tests covering configuration, core proxy logic (`ProxyServer` methods tested via mode tests), HTTP handlers, and command mode request handling.
*   **Frameworks:** Standard `testing` package, `net/http/httptest`.
*   **Key Test Files:** Located in relevant package directories.
*   **Running Tests:** `go test ./...`

## 11. Documentation

*   Includes README, detailed configuration guide, usage guide (including Docker/DinD), project icon, and specific Docker examples.