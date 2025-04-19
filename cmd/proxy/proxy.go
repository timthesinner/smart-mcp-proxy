package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"smart-mcp-proxy/internal/config"
)

// Proxy defines the interface for MCP proxy servers.
type Proxy interface {
	Run() error
	Shutdown(ctx context.Context) error
}

// ProxyServer holds the MCP server backends and common logic
type ProxyServer struct {
	mcpServers []*config.MCPServer
}

// RestrictedToolInfo adds ServerName to ToolInfo
type RestrictedToolInfo struct {
	config.ToolInfo
	ServerName string `json:"serverName"`
}

// RestrictedResourceInfo adds ServerName to ResourceInfo
type RestrictedResourceInfo struct {
	config.ResourceInfo
	ServerName string `json:"serverName"`
}

// NewProxyServer creates a new ProxyServer instance with initialized MCP servers
func NewProxyServer(cfg *config.Config) (*ProxyServer, error) {
	servers, err := config.NewMCPServers(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize MCP servers: %w", err)
	}

	ps := &ProxyServer{
		mcpServers: servers,
	}
	return ps, nil
}

// Shutdown gracefully shuts down all MCP servers.
func (ps *ProxyServer) Shutdown() {
	log.Println("Shutting down proxy server...")
	for _, server := range ps.mcpServers {
		if err := server.Shutdown(); err != nil {
			log.Printf("Error shutting down MCP server %s: %v", server.Config.Name, err)
		}
	}
	log.Println("Proxy server shutdown complete.")
}

// findMCPServerByName finds an MCP server by its name.
func (ps *ProxyServer) findMCPServerByName(name string) *config.MCPServer {
	for _, server := range ps.mcpServers {
		if server.Config.Name == name {
			return server
		}
	}
	return nil
}

// findMCPServerByTool finds the MCP server that allows the given tool
func (ps *ProxyServer) findMCPServerByTool(toolName string) *config.MCPServer {
	for _, server := range ps.mcpServers {
		if server.IsToolAllowed(toolName) {
			return server
		}
	}
	return nil
}

// findMCPServerByResource finds the MCP server that allows the given resource
func (ps *ProxyServer) findMCPServerByResource(resourceName string) *config.MCPServer {
	for _, server := range ps.mcpServers {
		if server.IsResourceAllowed(resourceName) {
			return server
		}
	}
	return nil
}

// ListTools collects ToolInfo from all MCP servers.
func (ps *ProxyServer) ListTools() []config.ToolInfo {
	var allTools []config.ToolInfo
	for _, server := range ps.mcpServers {
		tools := server.GetTools()
		allTools = append(allTools, tools...)
	}
	return allTools
}

// ListRestrictedTools collects RestrictedToolInfo from all MCP servers.
func (ps *ProxyServer) ListRestrictedTools() []RestrictedToolInfo {
	var allTools []RestrictedToolInfo
	for _, server := range ps.mcpServers {
		tools := server.GetRestrictedTools()
		for _, tool := range tools {
			allTools = append(allTools, RestrictedToolInfo{ToolInfo: tool, ServerName: server.Config.Name})
		}
	}
	return allTools
}

// ListResources collects ResourceInfo from all MCP servers.
func (ps *ProxyServer) ListResources() []config.ResourceInfo {
	var allResources []config.ResourceInfo
	for _, server := range ps.mcpServers {
		resources := server.GetResources()
		allResources = append(allResources, resources...)
	}
	return allResources
}

// ListRestrictedResources collects RestrictedResourceInfo from all MCP servers.
func (ps *ProxyServer) ListRestrictedResources() []RestrictedResourceInfo {
	var allResources []RestrictedResourceInfo
	for _, server := range ps.mcpServers {
		resources := server.GetRestrictedResources()
		for _, resource := range resources {
			allResources = append(allResources, RestrictedResourceInfo{ResourceInfo: resource, ServerName: server.Config.Name})
		}
	}
	return allResources
}

// ProxyRequestInput holds necessary info for proxying a request.
type ProxyRequestInput struct {
	Server *config.MCPServer
	Method string
	Path   string
	Query  string
	Header http.Header
	Body   io.Reader
}

// ProxyResponseOutput holds the response data from the proxied server.
type ProxyResponseOutput struct {
	Status  int
	Headers http.Header
	Body    []byte
}

// ProxyRequest handles the core logic of forwarding a request to an MCP server.
// It determines whether to use HTTP or Stdio based on the server config.
func (ps *ProxyServer) ProxyRequest(input ProxyRequestInput) (*ProxyResponseOutput, error) {
	server := input.Server
	if server == nil {
		return nil, fmt.Errorf("target server cannot be nil")
	}

	log.Printf("Proxying request: %s %s%s to server %s (%s)", input.Method, input.Path, input.Query, server.Config.Name, server.Config.Address)

	if server.Config.Command != "" {
		// Correctly call the refactored stdio proxy method
		return ps.proxyStdioRequestInternal(input)
	}
	return ps.proxyHttpRequest(input)
}

// proxyHttpRequest forwards the request to an HTTP-based MCP server.
func (ps *ProxyServer) proxyHttpRequest(input ProxyRequestInput) (*ProxyResponseOutput, error) {
	server := input.Server
	targetURL, err := url.Parse(server.Config.Address)
	if err != nil {
		log.Printf("Invalid MCP server address '%s': %v", server.Config.Address, err)
		return nil, fmt.Errorf("invalid MCP server address: %w", err)
	}

	// Append the original request path and query
	targetURL.Path = singleJoiningSlash(targetURL.Path, input.Path)
	targetURL.RawQuery = input.Query

	// Read body for the new request
	bodyBytes, err := ioutil.ReadAll(input.Body)
	if err != nil {
		log.Printf("Failed to read request body for proxying: %v", err)
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	req, err := http.NewRequest(input.Method, targetURL.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		log.Printf("Failed to create proxy request: %v", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Copy headers
	copyHeaders(input.Header, req.Header)

	// Set a timeout context (TODO: Make timeout configurable)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	// Perform the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to reach MCP server '%s': %v", server.Config.Name, err)
		return nil, fmt.Errorf("failed to reach MCP server: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body from server '%s': %v", server.Config.Name, err)
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	log.Printf("Response status from %s: %d for %s %s", server.Config.Name, resp.StatusCode, input.Method, input.Path)

	return &ProxyResponseOutput{
		Status:  resp.StatusCode,
		Headers: resp.Header,
		Body:    respBodyBytes,
	}, nil
}

// proxyStdioRequestInternal handles the logic for stdio proxying, matching the new input/output structure.
// Renamed from proxyStdioRequest to avoid conflict with the old signature if it exists elsewhere temporarily.
func (ps *ProxyServer) proxyStdioRequestInternal(input ProxyRequestInput) (*ProxyResponseOutput, error) {
	server := input.Server

	// Read the full request body
	bodyBytes, err := io.ReadAll(input.Body)
	if err != nil {
		log.Printf("Failed to read request body for stdio proxying: %v", err)
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	// Build MCP protocol request object
	mcpRequest := map[string]interface{}{
		"method":  input.Method,
		"path":    input.Path,
		"query":   input.Query,
		"headers": input.Header,
		"body":    string(bodyBytes), // MCP stdio expects body as string
	}

	// Serialize to JSON
	reqBytes, err := json.Marshal(mcpRequest)
	if err != nil {
		log.Printf("Failed to marshal MCP request for stdio: %v", err)
		return nil, fmt.Errorf("failed to marshal MCP request: %w", err)
	}

	// Use MCPServer method to handle stdio request
	respBytes, err := server.HandleStdioRequest(reqBytes)
	if err != nil {
		log.Printf("Failed to communicate with stdio MCP server '%s': %v", server.Config.Name, err)
		return nil, fmt.Errorf("failed to communicate with MCP server: %w", err)
	}

	// Parse JSON response from stdio server
	var mcpResponse struct {
		Status  int                 `json:"status"`
		Headers map[string][]string `json:"headers"`
		Body    string              `json:"body"` // MCP stdio returns body as string
	}
	err = json.Unmarshal(respBytes, &mcpResponse)
	if err != nil {
		log.Printf("Failed to unmarshal MCP response from stdio server '%s': %v", server.Config.Name, err)
		// Log the raw response for debugging
		log.Printf("Raw response from %s: %s", server.Config.Name, string(respBytes))
		return nil, fmt.Errorf("invalid MCP server response: %w", err)
	}

	log.Printf("Response status from stdio %s: %d for %s %s", server.Config.Name, mcpResponse.Status, input.Method, input.Path)

	// Convert headers map to http.Header
	respHeaders := make(http.Header)
	for k, v := range mcpResponse.Headers {
		respHeaders[k] = v
	}

	return &ProxyResponseOutput{
		Status:  mcpResponse.Status,
		Headers: respHeaders,
		Body:    []byte(mcpResponse.Body),
	}, nil
}

// copyHeaders copies HTTP headers from source to destination
func copyHeaders(src http.Header, dst http.Header) {
	for k, vv := range src {
		// Filter out hop-by-hop headers (like Connection, Proxy-Authenticate, etc.)
		// This is a basic filter, a more robust solution might be needed.
		if k == "Connection" || k == "Proxy-Connection" || k == "Keep-Alive" || k == "Proxy-Authenticate" || k == "Proxy-Authorization" || k == "Te" || k == "Trailers" || k == "Transfer-Encoding" || k == "Upgrade" {
			continue
		}
		dst[k] = append([]string(nil), vv...) // Create a copy of the slice
	}
}

// singleJoiningSlash joins two URL paths with a single slash
func singleJoiningSlash(a, b string) string {
	aSlash := strings.HasSuffix(a, "/")
	bSlash := strings.HasPrefix(b, "/")
	switch {
	case aSlash && bSlash:
		return a + b[1:]
	case !aSlash && !bSlash:
		// Ensure 'b' starts with a slash if it's not empty
		if b != "" {
			return a + "/" + b
		}
		return a // If b is empty, just return a
	default: // One has slash, one doesn't
		return a + b
	}
}
