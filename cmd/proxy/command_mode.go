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

// rpcError represents a JSON-RPC 2.0 error object
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Define JSON-RPC 2.0 request and response structs
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type toolCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

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
	// Parse JSON-RPC request
	var rpcReq jsonRPCRequest
	if err := json.Unmarshal(reqBytes, &rpcReq); err != nil {
		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error: &rpcError{
				Code:    -32700,
				Message: "Parse error: invalid JSON",
			},
		}
		return json.Marshal(resp)
	}

	// Validate JSON-RPC version
	if rpcReq.JSONRPC != "2.0" {
		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      rpcReq.ID,
			Error: &rpcError{
				Code:    -32600,
				Message: "Invalid Request: jsonrpc must be '2.0'",
			},
		}
		return json.Marshal(resp)
	}

	// Map JSON-RPC method to HTTP method and path
	var httpMethod, httpPath string
	var httpBody string
	switch rpcReq.Method {
	case "tools/list":
		httpMethod = http.MethodGet
		httpPath = "/tools"
	case "tools/call":
		// params must include Name and proxyPath
		var tool toolCall
		if err := json.Unmarshal(rpcReq.Params, &tool); err != nil {
			resp := jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      rpcReq.ID,
				Error: &rpcError{
					Code:    -32602,
					Message: "Invalid params: " + err.Error(),
				},
			}
			return json.Marshal(resp)
		}
		if tool.Name == "" {
			resp := jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      rpcReq.ID,
				Error: &rpcError{
					Code:    -32602,
					Message: "Invalid params: Name is required",
				},
			}
			return json.Marshal(resp)
		}
		httpMethod = http.MethodPost
		// Compose path with Name
		httpPath = "/tool/" + tool.Name
		httpBody = string(tool.Arguments)
	case "resources/list":
		httpMethod = http.MethodGet
		httpPath = "/resources"
	case "resources/call":
		// params must include resourceName and proxyPath
		var params struct {
			ResourceName string          `json:"resourceName"`
			ProxyPath    string          `json:"proxyPath"`
			Body         json.RawMessage `json:"body,omitempty"`
		}
		if err := json.Unmarshal(rpcReq.Params, &params); err != nil {
			resp := jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      rpcReq.ID,
				Error: &rpcError{
					Code:    -32602,
					Message: "Invalid params: " + err.Error(),
				},
			}
			return json.Marshal(resp)
		}
		if params.ResourceName == "" {
			resp := jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      rpcReq.ID,
				Error: &rpcError{
					Code:    -32602,
					Message: "Invalid params: resourceName is required",
				},
			}
			return json.Marshal(resp)
		}
		httpMethod = http.MethodPost
		httpPath = "/resource/" + params.ResourceName
		if params.ProxyPath != "" {
			if !strings.HasPrefix(params.ProxyPath, "/") {
				httpPath += "/"
			}
			httpPath += params.ProxyPath
		}
		httpBody = string(params.Body)
	default:
		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      rpcReq.ID,
			Error: &rpcError{
				Code:    -32601,
				Message: "Method not found",
			},
		}
		return json.Marshal(resp)
	}

	// Create HTTP request
	req, err := http.NewRequest(httpMethod, httpPath, strings.NewReader(httpBody))
	if err != nil {
		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      rpcReq.ID,
			Error: &rpcError{
				Code:    -32603,
				Message: "Internal error: failed to create HTTP request",
			},
		}
		return json.Marshal(resp)
	}

	// Create a ResponseRecorder to capture the response
	recorder := &responseRecorder{
		header: http.Header{},
		body:   &bytes.Buffer{},
	}

	// Serve the request using the Gin engine
	ps.engine.ServeHTTP(recorder, req)

	// Build JSON-RPC response result
	result := map[string]interface{}{
		"status":  recorder.status,
		"headers": recorder.header,
		"body":    recorder.body.String(),
	}

	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      rpcReq.ID,
		Result:  result,
	}
	respBytes, err := json.Marshal(resp)
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
