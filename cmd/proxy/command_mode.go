package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http" // Keep for http status codes and header manipulation
	"os"
	"strings"
	// Note: config import removed as types are handled by ProxyServer methods
	// Gin is no longer needed here
)

// rpcError represents a JSON-RPC 2.0 error object
type rpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"` // Optional data field
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

// --- Structs for specific RPC method parameters ---

// Params for tools/call
type toolCallParams struct {
	ServerName string          `json:"serverName"` // Added serverName
	ToolName   string          `json:"toolName"`   // Renamed from Name
	Arguments  json.RawMessage `json:"arguments,omitempty"`
}

// Params for resources/access (renamed from resources/call for clarity)
type resourceAccessParams struct {
	ServerName   string          `json:"serverName"` // Added serverName
	ResourceName string          `json:"resourceName"`
	ProxyPath    string          `json:"proxyPath"` // Path within the resource context
	Method       string          `json:"method"`    // HTTP Method (GET, POST, etc.)
	Headers      http.Header     `json:"headers,omitempty"`
	Body         json.RawMessage `json:"body,omitempty"`
}

// --- End Param Structs ---

// CommandProxy implements the Proxy interface for STDIO transport
type CommandProxy struct {
	ps *ProxyServer // Reference to the core ProxyServer logic
}

// NewCommandProxy creates a new CommandProxy instance.
// It takes a pre-configured ProxyServer instance.
func NewCommandProxy(ps *ProxyServer) (*CommandProxy, error) {
	if ps == nil {
		return nil, fmt.Errorf("ProxyServer instance cannot be nil")
	}
	return &CommandProxy{
		ps: ps,
	}, nil
}

// Run starts the command mode loop, reading from stdin and writing to stdout.
func (c *CommandProxy) Run() error {
	log.Println("Starting MCP Proxy in Command Mode")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Bytes()
		// Use the handleCommandRequest method associated with the CommandProxy instance
		respBytes, err := c.handleCommandRequest(line)
		if err != nil {
			// Log error to stderr, but try to send a JSON-RPC error response
			fmt.Fprintf(os.Stderr, "Error processing command request: %v\n", err)
			// Attempt to create a generic error response if possible
			// This part might need refinement based on where the error occurred
			errorResp := jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      nil, // ID might be unknown if parsing failed early
				Error: &rpcError{
					Code:    -32603, // Internal error
					Message: fmt.Sprintf("Internal server error: %v", err),
				},
			}
			// Try to parse ID from the raw line if possible for better error reporting
			var basicReq struct {
				ID interface{} `json:"id"`
			}
			_ = json.Unmarshal(line, &basicReq) // Ignore error, ID might still be nil
			errorResp.ID = basicReq.ID

			respBytes, _ = json.Marshal(errorResp) // Marshal the error response
			// Fallthrough to write the error response
		}

		if respBytes != nil {
			os.Stdout.Write(respBytes)
			os.Stdout.Write([]byte("\n")) // Ensure newline separator
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
		return err // Return the error from the scanner
	}
	log.Println("MCP Proxy Command Mode finished.")
	return nil
}

// Shutdown is a placeholder for command mode; typically no explicit shutdown needed.
func (c *CommandProxy) Shutdown(ctx context.Context) error {
	log.Println("CommandProxy Shutdown called (no-op).")
	// No network server to shut down, MCP server shutdown handled by main/HTTPProxy
	return nil
}

// handleCommandRequest processes a single MCP request line (JSON-RPC).
// Now a method on CommandProxy to access c.ps.
func (c *CommandProxy) handleCommandRequest(reqBytes []byte) ([]byte, error) {
	// 1. Parse JSON-RPC request
	var rpcReq jsonRPCRequest
	if err := json.Unmarshal(reqBytes, &rpcReq); err != nil {
		return marshalRPCError(nil, -32700, "Parse error: invalid JSON", nil)
	}

	// 2. Validate JSON-RPC version
	if rpcReq.JSONRPC != "2.0" {
		return marshalRPCError(rpcReq.ID, -32600, "Invalid Request: jsonrpc must be '2.0'", nil)
	}

	// 3. Handle the specific method
	var result interface{}
	var rpcErr *rpcError

	switch rpcReq.Method {
	case "tools/list":
		// Directly call the ProxyServer method
		result = c.ps.ListTools()
	case "restrictedTools/list": // Added method for restricted tools
		result = c.ps.ListRestrictedTools()
	case "resources/list":
		// Directly call the ProxyServer method
		result = c.ps.ListResources()
	case "restrictedResources/list": // Added method for restricted resources
		result = c.ps.ListRestrictedResources()
	case "tools/call":
		rpcErr = c.handleToolCall(rpcReq.ID, rpcReq.Params, &result)
	case "resources/access": // Renamed from resources/call
		rpcErr = c.handleResourceAccess(rpcReq.ID, rpcReq.Params, &result)
	default:
		rpcErr = &rpcError{Code: -32601, Message: "Method not found"}
	}

	// 4. Construct JSON-RPC Response adhering to spec (result XOR error)
	// Explicitly nil result if there was an error to ensure omitempty works correctly,
	// as 'result' might hold stale data from previous calls within the same test.
	if rpcErr != nil {
	}

	// Construct response conditionally to ensure Result is omitted on error
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      rpcReq.ID,
	}
	if rpcErr != nil {
		resp.Error = rpcErr
		// Do not assign resp.Result
	} else {
		resp.Result = result // Assign result only on success
	}

	// 5. Marshal JSON-RPC Response
	return json.Marshal(resp) // Let the caller handle potential marshal error
}

// handleToolCall handles the logic for the "tools/call" RPC method.
func (c *CommandProxy) handleToolCall(reqID interface{}, params json.RawMessage, result *interface{}) *rpcError {
	var toolParams toolCallParams
	if err := json.Unmarshal(params, &toolParams); err != nil {
		return &rpcError{Code: -32602, Message: "Invalid params for tools/call", Data: err.Error()}
	}
	if toolParams.ServerName == "" || toolParams.ToolName == "" {
		return &rpcError{Code: -32602, Message: "Invalid params for tools/call: serverName and toolName are required"}
	}

	server := c.ps.findMCPServerByName(toolParams.ServerName)
	if server == nil {
		return &rpcError{Code: -32001, Message: fmt.Sprintf("Server '%s' not found", toolParams.ServerName)}
	}
	if !server.IsToolAllowed(toolParams.ToolName) {
		return &rpcError{Code: -32002, Message: fmt.Sprintf("Tool '%s' not allowed on server '%s'", toolParams.ToolName, toolParams.ServerName)}
	}

	// Prepare input for ProxyRequest
	// Assume tool calls are POST requests to a path like /tool/{toolName}
	// Headers might be passed in params in a real scenario, but keeping simple for now.
	input := ProxyRequestInput{
		Server: server,
		Method: http.MethodPost, // Common for tool calls
		Path:   fmt.Sprintf("/tool/%s", toolParams.ToolName),
		Query:  "", // Query params usually not used for tool calls
		Header: make(http.Header),
		Body:   bytes.NewReader(toolParams.Arguments), // Pass arguments as body
	}
	input.Header.Set("Content-Type", "application/json") // Assume JSON arguments

	respOutput, err := c.ps.ProxyRequest(input)
	if err != nil {
		return &rpcError{Code: -32003, Message: "Failed to proxy tool call", Data: err.Error()}
	}

	// Format the result for JSON-RPC
	*result = map[string]interface{}{
		"status":  respOutput.Status,
		"headers": respOutput.Headers,
		"body":    string(respOutput.Body), // Return body as string for JSON-RPC
	}
	return nil // Success
}

// handleResourceAccess handles the logic for the "resources/access" RPC method.
func (c *CommandProxy) handleResourceAccess(reqID interface{}, params json.RawMessage, result *interface{}) *rpcError {
	var resourceParams resourceAccessParams
	if err := json.Unmarshal(params, &resourceParams); err != nil {
		return &rpcError{Code: -32602, Message: "Invalid params for resources/access", Data: err.Error()}
	}
	if resourceParams.ServerName == "" || resourceParams.ResourceName == "" || resourceParams.Method == "" {
		return &rpcError{Code: -32602, Message: "Invalid params for resources/access: serverName, resourceName, and method are required"}
	}

	server := c.ps.findMCPServerByName(resourceParams.ServerName)
	if server == nil {
		return &rpcError{Code: -32001, Message: fmt.Sprintf("Server '%s' not found", resourceParams.ServerName)}
	}
	if !server.IsResourceAllowed(resourceParams.ResourceName) {
		return &rpcError{Code: -32002, Message: fmt.Sprintf("Resource '%s' not allowed on server '%s'", resourceParams.ResourceName, resourceParams.ServerName)}
	}

	// Construct the target path, ensuring proxyPath starts correctly
	targetPath := fmt.Sprintf("/resource/%s", resourceParams.ResourceName)
	if resourceParams.ProxyPath != "" {
		if !strings.HasPrefix(resourceParams.ProxyPath, "/") {
			targetPath += "/"
		}
		targetPath += resourceParams.ProxyPath
	}

	// Prepare input for ProxyRequest
	input := ProxyRequestInput{
		Server: server,
		Method: resourceParams.Method,
		Path:   targetPath,
		Query:  "", // Query params could be added if needed via params struct
		Header: resourceParams.Headers,
		Body:   bytes.NewReader(resourceParams.Body),
	}
	if input.Header == nil {
		input.Header = make(http.Header) // Ensure header is not nil
	}
	// Potentially set default Content-Type if body is present and header isn't set
	if len(resourceParams.Body) > 0 && input.Header.Get("Content-Type") == "" {
		input.Header.Set("Content-Type", "application/json") // Default assumption
	}

	respOutput, err := c.ps.ProxyRequest(input)
	if err != nil {
		return &rpcError{Code: -32003, Message: "Failed to proxy resource access", Data: err.Error()}
	}

	// Format the result for JSON-RPC
	*result = map[string]interface{}{
		"status":  respOutput.Status,
		"headers": respOutput.Headers,
		"body":    string(respOutput.Body), // Return body as string
	}
	return nil // Success
}

// marshalRPCError is a helper to create and marshal a JSON-RPC error response.
func marshalRPCError(id interface{}, code int, message string, data interface{}) ([]byte, error) {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &rpcError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	return json.Marshal(resp)
}
