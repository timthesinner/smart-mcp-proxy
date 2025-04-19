package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync" // Import sync package
	"syscall"
	"time"

	"smart-mcp-proxy/internal/config" // Keep config import for types like ToolInfo

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HTTPProxy implements the Proxy interface for HTTP transport
type HTTPProxy struct {
	ps     *ProxyServer // Reference to the core ProxyServer logic
	engine *gin.Engine
	srv    *http.Server
}

// Package-level variables for Prometheus metrics to be initialized once.
var (
	httpMetricsOnce   sync.Once
	httpRequestsTotal *prometheus.CounterVec
	httpRequestDur    *prometheus.HistogramVec
)

// NewHTTPProxy creates a new HTTPProxy instance.
// It takes a pre-configured ProxyServer instance.
func NewHTTPProxy(ps *ProxyServer, listenAddr string) (*HTTPProxy, error) {
	if ps == nil {
		return nil, fmt.Errorf("ProxyServer instance cannot be nil")
	}

	engine := gin.Default()

	// --- Prometheus Metrics Setup ---
	// Use sync.Once to ensure metrics are registered only once globally.
	httpMetricsOnce.Do(func() {
		// Define temporary variables inside the closure first
		reqCounter := prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcp_proxy_http_requests_total",
				Help: "Total number of HTTP requests received by the proxy",
			},
			[]string{"method", "endpoint", "status"},
		)
		reqDuration := prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "mcp_proxy_http_request_duration_seconds",
				Help:    "Histogram of HTTP request durations",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "endpoint"},
		)
		// Register metrics
		prometheus.MustRegister(reqCounter, reqDuration)
		// Assign to package-level variables AFTER registration
		httpRequestsTotal = reqCounter
		httpRequestDur = reqDuration
		log.Println("Prometheus metrics registered for HTTP proxy.")
	})
	// --- End Prometheus Metrics Setup ---

	// --- Middleware Setup ---
	engine.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)

		// Log request details
		log.Printf("HTTP Request: %s %s %d %s", c.Request.Method, c.Request.URL.Path, c.Writer.Status(), duration)

		// Update Prometheus metrics using the package-level variables
		statusCode := fmt.Sprintf("%d", c.Writer.Status())
		// Use c.FullPath() to get the route path template (e.g., /tool/:serverName/:toolName/*proxyPath)
		if httpRequestsTotal != nil { // Check if initialized
			httpRequestsTotal.WithLabelValues(c.Request.Method, c.FullPath(), statusCode).Inc()
		}
		if httpRequestDur != nil { // Check if initialized
			httpRequestDur.WithLabelValues(c.Request.Method, c.FullPath()).Observe(duration.Seconds())
		}
	})
	// --- End Middleware Setup ---

	// Create the HTTPProxy instance *before* setting up routes,
	// so the handlers have access to the instance (h.ps).
	h := &HTTPProxy{
		ps:     ps,
		engine: engine,
	}

	// --- Route Setup ---
	engine.GET("/metrics", gin.WrapH(promhttp.Handler()))
	engine.GET("/tools", h.handleTools)
	engine.GET("/restricted-tools", h.handleRestrictedTools)
	engine.GET("/resources", h.handleResources)
	engine.GET("/restricted-resources", h.handleRestrictedResources)
	// Use /*proxyPath to capture the rest of the path
	engine.Any("/tool/:serverName/:toolName/*proxyPath", h.handleToolProxy)
	engine.Any("/resource/:serverName/:resourceName/*proxyPath", h.handleResourceProxy)
	// --- End Route Setup ---

	// --- HTTP Server Setup ---
	srv := &http.Server{
		Addr:         listenAddr, // Use provided listen address
		Handler:      engine,
		ReadTimeout:  15 * time.Second, // Increased slightly
		WriteTimeout: 30 * time.Second, // Increased slightly
		IdleTimeout:  60 * time.Second,
	}
	h.srv = srv // Assign the configured server to the struct
	// --- End HTTP Server Setup ---

	return h, nil
}

// handleTools handles the /tools endpoint using the ProxyServer logic
func (h *HTTPProxy) handleTools(c *gin.Context) {
	allTools := h.ps.ListTools()
	c.JSON(http.StatusOK, gin.H{"tools": allTools})
}

// handleRestrictedTools handles the /restricted-tools endpoint
func (h *HTTPProxy) handleRestrictedTools(c *gin.Context) {
	allTools := h.ps.ListRestrictedTools()
	c.JSON(http.StatusOK, gin.H{"tools": allTools})
}

// handleResources handles the /resources endpoint
func (h *HTTPProxy) handleResources(c *gin.Context) {
	allResources := h.ps.ListResources()
	c.JSON(http.StatusOK, gin.H{"resources": allResources})
}

// handleRestrictedResources handles the /restricted-resources endpoint
func (h *HTTPProxy) handleRestrictedResources(c *gin.Context) {
	allResources := h.ps.ListRestrictedResources()
	c.JSON(http.StatusOK, gin.H{"resources": allResources})
}

// handleToolProxy proxies requests to the specified tool on a specific server
func (h *HTTPProxy) handleToolProxy(c *gin.Context) {
	serverName := c.Param("serverName")
	toolName := c.Param("toolName")
	proxyPath := c.Param("proxyPath") // Includes leading slash

	server := h.ps.findMCPServerByName(serverName)
	if server == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("server '%s' not found", serverName)})
		return
	}

	// Check if the specific server allows this tool (optional, depends on desired behavior)
	// If the server was found by name, we might assume access is granted,
	// or we could double-check with IsToolAllowed. Let's double-check for safety.
	if !server.IsToolAllowed(toolName) {
		c.JSON(http.StatusForbidden, gin.H{"error": fmt.Sprintf("tool '%s' not allowed on server '%s'", toolName, serverName)})
		return
	}

	// Construct the path for the target server request
	// The path should likely include the tool name as part of the MCP server's expected API
	// Example: /tool/actual-tool-name/proxied/path
	// Adjust this based on how MCP servers expect tool proxy paths.
	// Assuming the target path should be constructed like this:
	targetPath := fmt.Sprintf("/tool/%s%s", toolName, proxyPath) // Ensure proxyPath starts with /

	h.proxyRequest(c, server, targetPath)
}

// handleResourceProxy proxies requests to the specified resource on a specific server
func (h *HTTPProxy) handleResourceProxy(c *gin.Context) {
	serverName := c.Param("serverName")
	resourceName := c.Param("resourceName")
	proxyPath := c.Param("proxyPath") // Includes leading slash

	server := h.ps.findMCPServerByName(serverName)
	if server == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("server '%s' not found", serverName)})
		return
	}

	// Double-check if the server allows this resource
	if !server.IsResourceAllowed(resourceName) {
		c.JSON(http.StatusForbidden, gin.H{"error": fmt.Sprintf("resource '%s' not allowed on server '%s'", resourceName, serverName)})
		return
	}

	// Construct the target path for the resource request
	// Example: /resource/actual-resource-name/proxied/path
	targetPath := fmt.Sprintf("/resource/%s%s", resourceName, proxyPath) // Ensure proxyPath starts with /

	h.proxyRequest(c, server, targetPath)
}

// proxyRequest is a helper for handleToolProxy and handleResourceProxy
func (h *HTTPProxy) proxyRequest(c *gin.Context, server *config.MCPServer, targetPath string) {
	input := ProxyRequestInput{
		Server: server,
		Method: c.Request.Method,
		Path:   targetPath, // Use the constructed target path
		Query:  c.Request.URL.RawQuery,
		Header: c.Request.Header,
		Body:   c.Request.Body, // Pass the original body reader
	}

	respOutput, err := h.ps.ProxyRequest(input)
	if err != nil {
		// Log the detailed error from ProxyRequest
		log.Printf("Error proxying request to server %s: %v", server.Config.Name, err)
		// Return a generic error to the client
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to proxy request to backend server"})
		return
	}

	// Copy headers from backend response to client response
	copyHeaders(respOutput.Headers, c.Writer.Header())

	// Write status code
	c.Status(respOutput.Status)

	// Write body if present
	if len(respOutput.Body) > 0 {
		_, err = c.Writer.Write(respOutput.Body)
		if err != nil {
			// Log error, but response status/headers might already be sent
			log.Printf("Error writing response body to client: %v", err)
		}
	}
}

// Run starts the HTTP server and waits for a shutdown signal.
func (h *HTTPProxy) Run() error {
	log.Printf("Starting MCP Proxy HTTP Server on %s", h.srv.Addr)
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := h.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server ListenAndServe error: %s\n", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("\nShutting down MCP Proxy HTTP Server...")

	// Shutdown Gin server
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // Increased timeout
	defer cancel()
	if err := h.srv.Shutdown(ctx); err != nil {
		log.Printf("HTTP Server forced to shutdown: %v", err)
		// Even if HTTP server shutdown fails, try to shutdown MCP servers
	} else {
		log.Println("HTTP Server shutdown complete.")
	}

	// Shutdown underlying MCP servers
	h.ps.Shutdown() // Call shutdown on the core ProxyServer

	log.Println("MCP Proxy HTTP Server has been shut down gracefully")
	<-done // Wait for ListenAndServe goroutine to finish
	return nil
}

// Shutdown gracefully shuts down the HTTP server.
func (h *HTTPProxy) Shutdown(ctx context.Context) error {
	log.Println("Initiating HTTPProxy Shutdown...")
	// Shutdown the HTTP server first
	err := h.srv.Shutdown(ctx)
	// Then shutdown the underlying ProxyServer (MCP connections)
	h.ps.Shutdown() // Ensure MCP servers are also shut down
	return err
}
