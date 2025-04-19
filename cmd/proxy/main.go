package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"smart-mcp-proxy/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ProxyServer holds the Gin engine and MCP server backends
type ProxyServer struct {
	engine     *gin.Engine
	mcpServers []*config.MCPServer
}

// NewProxyServer creates a new ProxyServer instance
func NewProxyServer(cfg *config.Config) (*ProxyServer, error) {
	servers, err := config.NewMCPServers(cfg)
	if err != nil {
		return nil, err
	}

	requestCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mcp_proxy_requests_total",
			Help: "Total number of requests received",
		},
		[]string{"method", "endpoint", "status"},
	)
	requestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mcp_proxy_request_duration_seconds",
			Help:    "Histogram of request durations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	prometheus.MustRegister(requestCounter, requestDuration)

	ps := &ProxyServer{
		engine:     gin.Default(),
		mcpServers: servers,
	}

	// Add logging middleware
	ps.engine.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)

		// Log request details
		log.Printf("%s %s %d %s", c.Request.Method, c.Request.URL.Path, c.Writer.Status(), duration)

		// Update Prometheus metrics
		statusCode := fmt.Sprintf("%d", c.Writer.Status())
		requestCounter.WithLabelValues(c.Request.Method, c.FullPath(), statusCode).Inc()
		requestDuration.WithLabelValues(c.Request.Method, c.FullPath()).Observe(duration.Seconds())
	})

	// Add /metrics endpoint
	ps.engine.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Setup routes
	ps.engine.GET("/tools", ps.handleTools)
	ps.engine.GET("/restricted-tools", ps.handleRestrictedTools)
	ps.engine.GET("/resources", ps.handleResources)
	ps.engine.GET("/restricted-resources", ps.handleRestrictedResources)
	ps.engine.Any("/tool/:toolName/*proxyPath", ps.handleToolProxy)
	ps.engine.Any("/resource/:resourceName/*proxyPath", ps.handleResourceProxy)

	return ps, nil
}

// Shutdown gracefully shuts down all MCP servers.
func (ps *ProxyServer) Shutdown() {
	for _, server := range ps.mcpServers {
		if err := server.Shutdown(); err != nil {
			log.Printf("Error shutting down MCP server %s: %v", server.Config.Name, err)
		}
	}
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

// handleTools handles the /tools endpoint
func (ps *ProxyServer) handleTools(c *gin.Context) {
	// Collect full ToolInfo objects from all MCP servers
	var allTools []config.ToolInfo
	for _, server := range ps.mcpServers {
		tools := server.GetTools()
		allTools = append(allTools, tools...)
	}
	// Return full ToolInfo objects as JSON array under "tools" key
	// This complies with MCP spec by providing detailed tool information
	c.JSON(http.StatusOK, gin.H{"tools": allTools})
}

type RestrictedToolInfo struct {
	config.ToolInfo
	ServerName string `json:"serverName"`
}

// handleRestrictedTools handles the /restricted-tools endpoint
func (ps *ProxyServer) handleRestrictedTools(c *gin.Context) {
	var allTools []RestrictedToolInfo
	for _, server := range ps.mcpServers {
		tools := server.GetRestrictedTools()
		for _, tool := range tools {
			allTools = append(allTools, RestrictedToolInfo{ToolInfo: tool, ServerName: server.Config.Name})
		}
	}

	// Return full ToolInfo objects as JSON array under "tools" key
	// This complies with MCP spec by providing detailed tool information
	c.JSON(http.StatusOK, gin.H{"tools": allTools})
}

// handleResources handles the /resources endpoint
func (ps *ProxyServer) handleResources(c *gin.Context) {
	// Collect full ResourceInfo objects from all MCP servers
	var allResources []config.ResourceInfo
	for _, server := range ps.mcpServers {
		resources := server.GetResources()
		allResources = append(allResources, resources...)
	}
	// Return full ResourceInfo objects as JSON array under "resources" key
	// This complies with MCP spec by providing detailed resource information
	c.JSON(http.StatusOK, gin.H{"resources": allResources})
}

type RestrictedResourceInfo struct {
	config.ResourceInfo
	ServerName string `json:"serverName"`
}

// handleRestrictedResources handles the /restricted-resources endpoint
func (ps *ProxyServer) handleRestrictedResources(c *gin.Context) {
	// Collect full ResourceInfo objects from all MCP servers
	var allResources []RestrictedResourceInfo
	for _, server := range ps.mcpServers {
		resources := server.GetRestrictedResources()
		for _, resource := range resources {
			allResources = append(allResources, RestrictedResourceInfo{ResourceInfo: resource, ServerName: server.Config.Name})
		}
	}
	// Return full ResourceInfo objects as JSON array under "resources" key
	// This complies with MCP spec by providing detailed resource information
	c.JSON(http.StatusOK, gin.H{"resources": allResources})
}

// handleToolProxy proxies requests to the specified tool
func (ps *ProxyServer) handleToolProxy(c *gin.Context) {
	toolName := c.Param("toolName")
	server := ps.findMCPServerByTool(toolName)
	if server == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "tool not allowed"})
		return
	}
	ps.proxyRequest(c, server)
}

// handleResourceProxy proxies requests to the specified resource
func (ps *ProxyServer) handleResourceProxy(c *gin.Context) {
	resourceName := c.Param("resourceName")
	server := ps.findMCPServerByResource(resourceName)
	if server == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "resource not allowed"})
		return
	}
	ps.proxyRequest(c, server)
}

// proxyRequest forwards the incoming request to the MCP server and writes back the response
func (ps *ProxyServer) proxyRequest(c *gin.Context, server *config.MCPServer) {
	// Log incoming request
	log.Printf("Incoming request: %s %s from %s", c.Request.Method, c.Request.URL.String(), c.ClientIP())

	// Check if this is a stdio-based MCP server
	if server.Config.Command != "" {
		// Handle stdio MCP server proxying
		ps.proxyStdioRequest(c, server)
		return
	}

	// Otherwise, handle HTTP MCP server proxying as before
	// Build the target URL
	targetURL, err := url.Parse(server.Config.Address)
	if err != nil {
		log.Printf("Invalid MCP server address: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid MCP server address"})
		return
	}

	// Append the original request path and query
	targetURL.Path = singleJoiningSlash(targetURL.Path, c.Request.URL.Path)
	targetURL.RawQuery = c.Request.URL.RawQuery

	// Create new request to MCP server
	bodyBytes, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("Failed to read request body: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read request body"})
		return
	}
	// Restore the io.ReadCloser to its original state
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))

	req, err := http.NewRequest(c.Request.Method, targetURL.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create request"})
		return
	}

	// Copy headers
	copyHeaders(c.Request.Header, req.Header)

	// Set a timeout context
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	// Perform the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to reach MCP server: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to reach MCP server"})
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	copyHeaders(resp.Header, c.Writer.Header())

	// Write status code
	c.Status(resp.StatusCode)

	// Copy response body
	_, err = io.Copy(c.Writer, resp.Body)
	if err != nil {
		log.Printf("Error copying response body: %v", err)
	}

	// Log response status
	log.Printf("Response status: %d for %s %s", resp.StatusCode, c.Request.Method, c.Request.URL.String())
}

func (ps *ProxyServer) proxyStdioRequest(c *gin.Context, server *config.MCPServer) {
	// Read the full request body
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("Failed to read request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	// Build MCP protocol request object
	mcpRequest := map[string]interface{}{
		"method":  c.Request.Method,
		"path":    c.Request.URL.Path,
		"query":   c.Request.URL.RawQuery,
		"headers": c.Request.Header,
		"body":    string(bodyBytes),
	}

	// Serialize to JSON
	reqBytes, err := json.Marshal(mcpRequest)
	if err != nil {
		log.Printf("Failed to marshal MCP request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal MCP request"})
		return
	}

	// Use MCPServer method to handle stdio request
	respBytes, err := server.HandleStdioRequest(reqBytes)
	if err != nil {
		log.Printf("Failed to communicate with MCP server: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to communicate with MCP server"})
		return
	}

	// Parse JSON response
	var mcpResponse struct {
		Status  int                 `json:"status"`
		Headers map[string][]string `json:"headers"`
		Body    string              `json:"body"`
	}
	err = json.Unmarshal(respBytes, &mcpResponse)
	if err != nil {
		log.Printf("Failed to unmarshal MCP response: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid MCP server response"})
		return
	}

	// Copy headers to response
	for k, v := range mcpResponse.Headers {
		for _, vv := range v {
			c.Writer.Header().Add(k, vv)
		}
	}

	// Write status code
	c.Status(mcpResponse.Status)

	// Write body
	_, err = c.Writer.Write([]byte(mcpResponse.Body))
	if err != nil {
		log.Printf("Failed to write response body: %v", err)
	}

	// Log response status
	log.Printf("Response status: %d for %s %s", mcpResponse.Status, c.Request.Method, c.Request.URL.String())
}

// copyHeaders copies HTTP headers from source to destination
func copyHeaders(src http.Header, dst http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

// singleJoiningSlash joins two URL paths with a single slash
func singleJoiningSlash(a, b string) string {
	switch {
	case strings.HasSuffix(a, "/") && strings.HasPrefix(b, "/"):
		return a + b[1:]
	case !strings.HasSuffix(a, "/") && !strings.HasPrefix(b, "/"):
		return a + "/" + b
	}
	return a + b
}

func main() {
	// Define command-line flags
	configPathFlag := flag.String("config", "", "Path to MCP proxy config file")
	modeFlag := flag.String("mode", "", "Run mode: 'http' or 'command' (default 'http')")
	flag.Parse()

	// Determine config path from flag or environment variable
	configPath := *configPathFlag
	if configPath == "" {
		configPath = os.Getenv("MCP_PROXY_CONFIG")
	}
	if configPath == "" {
		log.Fatal("MCP_PROXY_CONFIG environment variable or -config flag must be set")
	}

	// Determine mode from flag or environment variable
	mode := *modeFlag
	if mode == "" {
		mode = os.Getenv("MCP_PROXY_MODE")
	}
	if mode == "" {
		mode = "http"
	}

	// Load config
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	var proxy Proxy
	switch mode {
	case "http":
		proxy, err = NewHTTPProxy(cfg)
		if err != nil {
			log.Fatalf("failed to create HTTP proxy: %v", err)
		}
	case "command":
		proxy, err = NewCommandProxy(cfg)
		if err != nil {
			log.Fatalf("failed to create command proxy: %v", err)
		}
	default:
		log.Fatalf("invalid mode: %s, must be 'http' or 'command'", mode)
	}

	if err := proxy.Run(); err != nil {
		log.Fatalf("proxy run error: %v", err)
	}
}

func runCommandMode(cfg *config.Config) {
	// In command mode, the proxy communicates with MCP client via STDIN/STDOUT
	// All logging goes to STDERR

	ps, err := NewProxyServer(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create proxy server: %v\n", err)
		os.Exit(1)
	}

	// Use a scanner to read lines from stdin
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Bytes()
		// Process the MCP request line and write response to stdout
		respBytes, err := handleCommandRequest(ps, line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error handling command request: %v\n", err)
			continue
		}
		os.Stdout.Write(respBytes)
		os.Stdout.Write([]byte("\n"))
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "error reading stdin: %v\n", err)
	}
}
