package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"smart-mcp-proxy/internal/config"

	"github.com/gin-gonic/gin"
)

// CommandProxy implements the Proxy interface for STDIO transport
type CommandProxy struct {
	ps *ProxyServer
}

func NewCommandProxy(cfg *config.Config) (*CommandProxy, error) {
	servers, err := config.NewMCPServers(cfg)
	if err != nil {
		return nil, err
	}

	ps := &ProxyServer{
		engine:     gin.Default(),
		mcpServers: servers,
	}

	// Setup routes
	ps.engine.GET("/tools", ps.handleTools)
	ps.engine.GET("/restricted-tools", ps.handleRestrictedTools)
	ps.engine.GET("/resources", ps.handleResources)
	ps.engine.GET("/restricted-resources", ps.handleRestrictedResources)
	ps.engine.Any("/tool/:toolName/*proxyPath", ps.handleToolProxy)
	ps.engine.Any("/resource/:resourceName/*proxyPath", ps.handleResourceProxy)

	return &CommandProxy{
		ps: ps,
	}, nil
}

func (c *CommandProxy) Run() error {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Bytes()
		respBytes, err := handleCommandRequest(c.ps, line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error handling command request: %v\n", err)
			continue
		}
		os.Stdout.Write(respBytes)
		os.Stdout.Write([]byte("\n"))
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "error reading stdin: %v\n", err)
		return err
	}
	return nil
}

func (c *CommandProxy) Shutdown(ctx context.Context) error {
	// No special shutdown needed for command mode
	return nil
}

// handleCommandRequest processes a single MCP request in command mode
func handleCommandRequest(ps *ProxyServer, reqBytes []byte) ([]byte, error) {
	// For simplicity, assume the request is a JSON object with method, path, headers, body
	// We will simulate an HTTP request and get the response

	// Create a dummy HTTP request from reqBytes
	var mcpRequest struct {
		Method  string              `json:"method"`
		Path    string              `json:"path"`
		Query   string              `json:"query"`
		Headers map[string][]string `json:"headers"`
		Body    string              `json:"body"`
	}
	err := json.Unmarshal(reqBytes, &mcpRequest)
	if err != nil {
		return nil, fmt.Errorf("invalid request JSON: %w", err)
	}

	// Build HTTP request
	urlStr := mcpRequest.Path
	if mcpRequest.Query != "" {
		urlStr += "?" + mcpRequest.Query
	}
	req, err := http.NewRequest(mcpRequest.Method, urlStr, strings.NewReader(mcpRequest.Body))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	for k, vv := range mcpRequest.Headers {
		for _, v := range vv {
			req.Header.Add(k, v)
		}
	}

	// Create a ResponseRecorder to capture the response
	recorder := &responseRecorder{
		header: http.Header{},
		body:   &bytes.Buffer{},
	}

	// Serve the request using the Gin engine
	ps.engine.ServeHTTP(recorder, req)

	// Build MCP response JSON
	mcpResponse := map[string]interface{}{
		"status":  recorder.status,
		"headers": recorder.header,
		"body":    recorder.body.String(),
	}
	respBytes, err := json.Marshal(mcpResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}
	return respBytes, nil
}

// responseRecorder is an implementation of http.ResponseWriter to capture response
type responseRecorder struct {
	header http.Header
	body   *bytes.Buffer
	status int
}

func (r *responseRecorder) Header() http.Header {
	return r.header
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	return r.body.Write(b)
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
}
