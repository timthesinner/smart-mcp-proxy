package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"smart-mcp-proxy/internal/config" // Keep config import for setup

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestCommandProxy creates ProxyServer and CommandProxy for testing.
func setupTestCommandProxy(t *testing.T) (*CommandProxy, []*httptest.Server) {
	// Use the same backend server setup as HTTP tests
	server1, server1Conf := testHttpServer("server1", []string{"tool1", "tool2"}, []string{"res1"}, []string{"r-tool1", "r-tool2"}, []string{"r-res1"})
	server2, server2Conf := testHttpServer("server2", []string{"tool3"}, []string{"res2"}, []string{"r-tool3"}, []string{"r-res2"})

	cfg := &config.Config{
		MCPServers: []config.MCPServerConfig{server1Conf, server2Conf},
	}

	// 1. Create the core ProxyServer
	ps, err := NewProxyServer(cfg)
	require.NoError(t, err)
	require.NotNil(t, ps)

	// 2. Create the CommandProxy using the ProxyServer
	cmdProxy, err := NewCommandProxy(ps)
	require.NoError(t, err)
	require.NotNil(t, cmdProxy)

	return cmdProxy, []*httptest.Server{server1, server2}
}

type testToolsAndResourceResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  struct {
		Tools      []config.ToolInfo     `json:"tools,omitempty"`
		Resources  []config.ResourceInfo `json:"resources,omitempty"`
		NextCursor string                `json:"nextCursor,omitempty"`
	} `json:"result,omitempty"`
	Error interface{} `json:"error,omitempty"`
}

// TestCommandHandleToolsList tests the "tools/list" JSON-RPC method.
func TestCommandHandleToolsList(t *testing.T) {
	cmdProxy, servers := setupTestCommandProxy(t)
	for _, server := range servers {
		defer server.Close()
	}

	// Create a JSON-RPC request for tools/list
	rpcReq := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	}
	reqBytes, err := json.Marshal(rpcReq)
	require.NoError(t, err)

	// Call the handler directly
	respBytes, err := cmdProxy.handleCommandRequest(reqBytes)
	require.NoError(t, err)
	require.NotNil(t, respBytes)

	// Parse the JSON-RPC response
	var rpcResp testToolsAndResourceResponse
	err = json.Unmarshal(respBytes, &rpcResp)
	require.NoError(t, err)

	// Assertions on the response
	assert.Equal(t, "2.0", rpcResp.JSONRPC)
	assert.Equal(t, float64(1), rpcResp.ID) // JSON numbers are float64 by default
	assert.Nil(t, rpcResp.Error)
	require.NotNil(t, rpcResp.Result)
	require.NotNil(t, rpcResp.Result.Tools)

	assert.Len(t, rpcResp.Result.Tools, 3) // tool1, tool2, tool3
	foundTools := make(map[string]bool)
	for _, tool := range rpcResp.Result.Tools {
		assert.NotEmpty(t, tool.Name)
		assert.NotNil(t, tool.InputSchema)
		foundTools[tool.Name] = true
	}
	assert.True(t, foundTools["tool1"])
	assert.True(t, foundTools["tool2"])
	assert.True(t, foundTools["tool3"])
}

// TestCommandHandleResourcesList tests the "resources/list" JSON-RPC method.
func TestCommandHandleResourcesList(t *testing.T) {
	cmdProxy, servers := setupTestCommandProxy(t)
	for _, server := range servers {
		defer server.Close()
	}

	rpcReq := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      "res-list-req",
		Method:  "resources/list",
	}
	reqBytes, err := json.Marshal(rpcReq)
	require.NoError(t, err)

	respBytes, err := cmdProxy.handleCommandRequest(reqBytes)
	require.NoError(t, err)

	var rpcResp testToolsAndResourceResponse
	err = json.Unmarshal(respBytes, &rpcResp)
	require.NoError(t, err)

	assert.Equal(t, "2.0", rpcResp.JSONRPC)
	assert.Equal(t, "res-list-req", rpcResp.ID)
	assert.Nil(t, rpcResp.Error)
	require.NotNil(t, rpcResp.Result)
	require.NotNil(t, rpcResp.Result.Resources)

	assert.Len(t, rpcResp.Result.Resources, 2) // res1, res2
	foundResources := make(map[string]bool)
	for _, res := range rpcResp.Result.Resources {
		assert.NotEmpty(t, res.Name)
		foundResources[res.Name] = true
	}
	assert.True(t, foundResources["res1"])
	assert.True(t, foundResources["res2"])
}

// TestCommandHandleToolCall tests the "tools/call" JSON-RPC method.
func TestCommandHandleToolCall(t *testing.T) {
	cmdProxy, servers := setupTestCommandProxy(t)
	for _, server := range servers {
		defer server.Close()
	}

	// --- Test valid call ---
	// Use config.CallToolRequestParams now
	toolParams := config.CallToolRequestParams{
		Name:      "tool1", // Use Name field
		Arguments: map[string]interface{}{"arg1": "value1"},
	}
	paramsBytes, _ := json.Marshal(toolParams)
	rpcReq := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      "tool-call-1",
		Method:  "tools/call",
		Params:  paramsBytes,
	}
	reqBytes, _ := json.Marshal(rpcReq)

	respBytes, err := cmdProxy.handleCommandRequest(reqBytes)
	require.NoError(t, err)

	// Unmarshal and check the valid response
	{
		var rpcResp jsonRPCResponse // Scope rpcResp to this block
		err = json.Unmarshal(respBytes, &rpcResp)
		require.NoError(t, err)

		assert.Equal(t, "tool-call-1", rpcResp.ID)
		// Correct Assertion: Error should be nil on success
		assert.Nil(t, rpcResp.Error, "Expected no error for successful tool call")
		// Correct Assertion: Result should NOT be nil on success
		require.NotNil(t, rpcResp.Result, "Expected result for successful tool call")

		// Check the structure of the result (should be config.CallToolResult)
		// Need to remarshal and unmarshal the result into the expected struct
		resultBytes, err := json.Marshal(rpcResp.Result)
		require.NoError(t, err, "Failed to remarshal result map")

		var callResult config.CallToolResult
		err = json.Unmarshal(resultBytes, &callResult)
		require.NoError(t, err, "Failed to unmarshal result into config.CallToolResult")

		// Assertions on CallToolResult
		require.Len(t, callResult.Content, 1, "Expected one content block")
		assert.Equal(t, "text", callResult.Content[0].Type)
		// The mock server returns JSON: {"status": "tool /tool/tool1 called"}
		// The new CallTool logic for HTTP should parse this JSON into the result.
		// Let's check if the text content matches the expected JSON string from the mock.
		require.NotNil(t, callResult.Content[0].Text, "Text content should not be nil")
		expectedBody := `{"status":"tool /tool/tool1 called"}`
		assert.JSONEq(t, expectedBody, *callResult.Content[0].Text, "Body content mismatch") // Dereference pointer
		// Or, if the CallTool logic is expected to parse the inner JSON:
		// assert.Equal(t, "tool /tool/tool1 called", *callResult.Content[0].Text) // Adjust based on actual CallTool behavior
	}

	// --- Test tool not found (server finding is now internal to CallTool) ---
	toolParams.Name = "nonexistentTool" // Change to a tool that doesn't exist
	paramsBytes, _ = json.Marshal(toolParams)
	rpcReq.ID = "tool-call-err-1"
	rpcReq.Params = paramsBytes
	reqBytes, _ = json.Marshal(rpcReq)

	respBytes, err = cmdProxy.handleCommandRequest(reqBytes)
	require.NoError(t, err)
	// Unmarshal and check the "server not found" error response
	{
		var rpcResp jsonRPCResponse // Scope rpcResp to this block
		err = json.Unmarshal(respBytes, &rpcResp)
		require.NoError(t, err)

		assert.Equal(t, "tool-call-err-1", rpcResp.ID)
		// Correct Assertion: Result should be nil on error
		assert.Nil(t, rpcResp.Result, "Expected nil result for server not found error")
		// Correct Assertion: Error should NOT be nil on error
		require.NotNil(t, rpcResp.Error, "Expected error for tool not found")
		// Corrected expected error code and message for tool not found
		assert.Equal(t, -32000, rpcResp.Error.Code) // Generic server error code used in handleToolCall
		assert.Contains(t, rpcResp.Error.Message, "Failed to execute tool 'nonexistentTool'")
		// Check the underlying error message stored in Data
		require.NotNil(t, rpcResp.Error.Data, "Error data should not be nil for tool not found")
		errorDataStr, ok := rpcResp.Error.Data.(string)
		require.True(t, ok, "Error data should be a string")
		// Check that the underlying error data contains the ErrToolNotFound message
		// The exact format might be "tool not found or not provided by any configured server: nonexistentTool"
		assert.Contains(t, errorDataStr, ErrToolNotFound.Error())
		assert.Contains(t, errorDataStr, "nonexistentTool") // Ensure the specific tool name is still present
	}

	// --- Test invalid params (missing name) ---
	// Use a map directly to represent invalid params missing 'name'
	invalidParams := map[string]interface{}{
		"arguments": map[string]interface{}{"arg1": "value1"},
	}
	paramsBytes, _ = json.Marshal(invalidParams)
	rpcReq.ID = "tool-call-err-invalid"
	rpcReq.Params = paramsBytes
	reqBytes, _ = json.Marshal(rpcReq)

	respBytes, err = cmdProxy.handleCommandRequest(reqBytes)
	require.NoError(t, err)
	{
		var rpcResp jsonRPCResponse
		err = json.Unmarshal(respBytes, &rpcResp)
		require.NoError(t, err)

		assert.Equal(t, "tool-call-err-invalid", rpcResp.ID)
		assert.Nil(t, rpcResp.Result)
		require.NotNil(t, rpcResp.Error)
		assert.Equal(t, -32602, rpcResp.Error.Code) // Invalid Params code
		assert.Contains(t, rpcResp.Error.Message, "Invalid params for tools/call: 'name' is required")
	}

	// Note: The "tool not allowed on server" test case is removed because
	// the command mode `tools/call` no longer takes `serverName`. The routing
	// to the correct server based on the tool name happens inside `ProxyServer.CallTool`.
	// We tested the "tool not found" case above, which covers scenarios where no server handles the tool.

	// The lines previously commented out here have been removed.
	// The redundant test block checking for "tool not allowed on server"
	// which started here has been removed as it's no longer applicable.
}

type testRestrictedToolsAndResourceResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  struct {
		Tools      []RestrictedToolInfo     `json:"tools,omitempty"`
		Resources  []RestrictedResourceInfo `json:"resources,omitempty"`
		NextCursor string                   `json:"nextCursor,omitempty"`
	} `json:"result,omitempty"`
	Error interface{} `json:"error,omitempty"`
}

// TestCommandHandleRestrictedToolsList tests the "restrictedTools/list" JSON-RPC method.
func TestCommandHandleRestrictedToolsList(t *testing.T) {
	cmdProxy, servers := setupTestCommandProxy(t)
	for _, server := range servers {
		defer server.Close()
	}

	rpcReq := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      "restricted-tools-1",
		Method:  "restrictedTools/list", // Correct method name
	}
	reqBytes, err := json.Marshal(rpcReq)
	require.NoError(t, err)

	respBytes, err := cmdProxy.handleCommandRequest(reqBytes)
	require.NoError(t, err)

	var rpcResp testRestrictedToolsAndResourceResponse
	err = json.Unmarshal(respBytes, &rpcResp)
	require.NoError(t, err)

	assert.Equal(t, "2.0", rpcResp.JSONRPC)
	assert.Equal(t, "restricted-tools-1", rpcResp.ID)
	assert.Nil(t, rpcResp.Error)
	require.NotNil(t, rpcResp.Result, "Expected non-nil result for restrictedTools/list, got error: %+v", rpcResp.Error)
	require.NotNil(t, rpcResp.Result.Tools)

	assert.Len(t, rpcResp.Result.Tools, 3) // tool1, tool2, tool3
	foundTools := make(map[string]string)  // Map tool name to server name
	for _, tool := range rpcResp.Result.Tools {
		assert.NotEmpty(t, tool.Name)
		assert.NotEmpty(t, tool.ServerName) // Check ServerName
		assert.NotNil(t, tool.InputSchema)
		foundTools[tool.Name] = tool.ServerName
	}
	assert.Equal(t, "server1", foundTools["r-tool1"])
	assert.Equal(t, "server1", foundTools["r-tool2"])
	assert.Equal(t, "server2", foundTools["r-tool3"])
}

// TestCommandHandleRestrictedResourcesList tests the "restrictedResources/list" JSON-RPC method.
func TestCommandHandleRestrictedResourcesList(t *testing.T) {
	cmdProxy, servers := setupTestCommandProxy(t)
	for _, server := range servers {
		defer server.Close()
	}

	rpcReq := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      "restricted-res-1",
		Method:  "restrictedResources/list", // Correct method name
	}
	reqBytes, err := json.Marshal(rpcReq)
	require.NoError(t, err)

	respBytes, err := cmdProxy.handleCommandRequest(reqBytes)
	require.NoError(t, err)

	var rpcResp testRestrictedToolsAndResourceResponse
	err = json.Unmarshal(respBytes, &rpcResp)
	require.NoError(t, err)

	assert.Equal(t, "2.0", rpcResp.JSONRPC)
	assert.Equal(t, "restricted-res-1", rpcResp.ID)
	assert.Nil(t, rpcResp.Error)
	require.NotNil(t, rpcResp.Result, "Expected non-nil result for restrictedResources/list, got error: %+v", rpcResp.Error)
	require.NotNil(t, rpcResp.Result.Resources)

	assert.Len(t, rpcResp.Result.Resources, 2) // res1, res2
	foundResources := make(map[string]string)  // Map resource name to server name
	for _, res := range rpcResp.Result.Resources {
		assert.NotEmpty(t, res.Name)
		assert.NotEmpty(t, res.ServerName) // Check ServerName
		foundResources[res.Name] = res.ServerName
	}
	assert.Equal(t, "server1", foundResources["r-res1"])
	assert.Equal(t, "server2", foundResources["r-res2"])
}

// TestCommandHandleResourceAccess tests the "resources/access" JSON-RPC method.
func TestCommandHandleResourceAccess(t *testing.T) {
	cmdProxy, servers := setupTestCommandProxy(t)
	for _, server := range servers {
		defer server.Close()
	}

	// --- Test valid access (GET) ---
	resParams := resourceAccessParams{
		ServerName:   "server1",
		ResourceName: "res1",
		ProxyPath:    "/some/path?query=1", // Include path and query
		Method:       "GET",
		Headers:      map[string]string{"X-Test-Header": "Value1"},
		// No body for GET
	}
	paramsBytes, _ := json.Marshal(resParams)
	rpcReq := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      "res-access-1",
		Method:  "resources/access",
		Params:  paramsBytes,
	}
	reqBytes, _ := json.Marshal(rpcReq)

	respBytes, err := cmdProxy.handleCommandRequest(reqBytes)
	require.NoError(t, err)

	// Unmarshal and check the valid response
	{
		var rpcResp jsonRPCResponse
		err = json.Unmarshal(respBytes, &rpcResp)
		require.NoError(t, err)

		assert.Equal(t, "res-access-1", rpcResp.ID)
		assert.Nil(t, rpcResp.Error, "Expected no error for successful resource access")
		require.NotNil(t, rpcResp.Result, "Expected result for successful resource access")

		resultMap, ok := rpcResp.Result.(map[string]interface{})
		require.True(t, ok, "Result should be a map")
		assert.Contains(t, resultMap, "status")
		assert.Contains(t, resultMap, "headers")
		assert.Contains(t, resultMap, "body")
		assert.Equal(t, float64(200), resultMap["status"]) // Status should be 200 OK

		// Check headers (returned headers are http.Header -> map[string][]string)
		respHeaders, ok := resultMap["headers"].(map[string]interface{})
		require.True(t, ok, "Headers should be a map")
		// Mock server adds Content-Type
		assert.Contains(t, respHeaders, "Content-Type")
		// Mock server does NOT echo back X-Test-Header, so remove assertions for it

		// Check body (mock server returns JSON string)
		bodyResult := resultMap["body"]
		bodyMap := make(map[string]interface{})
		// Check if bodyResult is already a map (expected case)
		if bMap, ok := bodyResult.(map[string]interface{}); ok {
			bodyMap = bMap
		} else if bodyStr, ok := bodyResult.(string); ok {
			// Fallback: If it's a string, try unmarshalling
			err = json.Unmarshal([]byte(bodyStr), &bodyMap)
			require.NoError(t, err, "Failed to unmarshal body string: %s", bodyStr)
		} else {
			require.FailNow(t, "Body result is neither a map nor a string", "Body type: %T, Body value: %v", bodyResult, bodyResult)
		}
		// Assert based on actual mock server response: {"status": "resource /resource/res1/some/path?query=1 accessed"}
		assert.Equal(t, "resource /resource/res1/some/path?query=1 accessed", bodyMap["status"])
	}

	// --- Test Invalid Params (missing method) ---
	resParams = resourceAccessParams{
		ServerName:   "server1",
		ResourceName: "res1",
	}
	paramsBytes, _ = json.Marshal(resParams)
	rpcReq.ID = "res-access-err-1"
	rpcReq.Params = paramsBytes
	reqBytes, _ = json.Marshal(rpcReq)

	respBytes, err = cmdProxy.handleCommandRequest(reqBytes)
	require.NoError(t, err)
	{
		var rpcResp jsonRPCResponse
		err = json.Unmarshal(respBytes, &rpcResp)
		require.NoError(t, err)
		assert.Equal(t, "res-access-err-1", rpcResp.ID)
		assert.Nil(t, rpcResp.Result)
		require.NotNil(t, rpcResp.Error)
		assert.Equal(t, -32602, rpcResp.Error.Code) // Invalid Params
		assert.Contains(t, rpcResp.Error.Message, "serverName, resourceName, and method are required")
	}

	// --- Test Resource Not Allowed ---
	resParams = resourceAccessParams{
		ServerName:   "server1",
		ResourceName: "res2", // res2 is on server2
		Method:       "GET",
	}
	paramsBytes, _ = json.Marshal(resParams)
	rpcReq.ID = "res-access-err-2"
	rpcReq.Params = paramsBytes
	reqBytes, _ = json.Marshal(rpcReq)

	respBytes, err = cmdProxy.handleCommandRequest(reqBytes)
	require.NoError(t, err)
	{
		var rpcResp jsonRPCResponse
		err = json.Unmarshal(respBytes, &rpcResp)
		require.NoError(t, err)
		assert.Equal(t, "res-access-err-2", rpcResp.ID)
		assert.Nil(t, rpcResp.Result)
		require.NotNil(t, rpcResp.Error)
		assert.Equal(t, -32002, rpcResp.Error.Code) // Custom Resource Not Allowed
		assert.Contains(t, rpcResp.Error.Message, "Resource 'res2' not allowed on server 'server1'")
	}
}

// TestCommandHandleErrors tests general JSON-RPC error handling.
func TestCommandHandleErrors(t *testing.T) {
	cmdProxy, servers := setupTestCommandProxy(t)
	for _, server := range servers {
		defer server.Close()
	}

	testCases := []struct {
		name        string
		reqBytes    []byte
		expectedID  interface{}
		expectedErr *rpcError
	}{
		{
			name:        "Parse Error (Invalid JSON)",
			reqBytes:    []byte(`{"jsonrpc": "2.0", "id": 1, "method": "test"`), // Malformed JSON
			expectedID:  nil,                                                    // ID might be unparseable
			expectedErr: &rpcError{Code: -32700, Message: "Parse error: invalid JSON"},
		},
		{
			name:        "Invalid Request (Wrong Version)",
			reqBytes:    []byte(`{"jsonrpc": "1.0", "id": 2, "method": "tools/list"}`),
			expectedID:  float64(2),
			expectedErr: &rpcError{Code: -32600, Message: "Invalid Request: jsonrpc must be '2.0'"},
		},
		{
			name:        "Method Not Found",
			reqBytes:    []byte(`{"jsonrpc": "2.0", "id": "m-err", "method": "nonexistent/method"}`),
			expectedID:  "m-err",
			expectedErr: &rpcError{Code: -32601, Message: "Method not found"},
		},
		{
			name:        "Invalid Params (tools/call missing name)",
			reqBytes:    []byte(`{"jsonrpc": "2.0", "id": "p-err-1", "method": "tools/call", "params": {"arguments": {}}}`), // Missing name
			expectedID:  "p-err-1",
			expectedErr: &rpcError{Code: -32602, Message: "Invalid params for tools/call: 'name' is required"}, // Updated message
		},
		{
			name:        "Invalid Params (resources/access missing name)",
			reqBytes:    []byte(`{"jsonrpc": "2.0", "id": "p-err-2", "method": "resources/access", "params": {"serverName": "server1", "method": "GET"}}`), // Missing resourceName
			expectedID:  "p-err-2",
			expectedErr: &rpcError{Code: -32602, Message: "Invalid params for resources/access: serverName, resourceName, and method are required"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			respBytes, err := cmdProxy.handleCommandRequest(tc.reqBytes)
			// handleCommandRequest itself shouldn't error for valid JSON-RPC structure errors
			require.NoError(t, err)

			var rpcResp jsonRPCResponse
			err = json.Unmarshal(respBytes, &rpcResp)
			require.NoError(t, err, "Failed to unmarshal error response: %s", string(respBytes))

			assert.Equal(t, tc.expectedID, rpcResp.ID)
			assert.Nil(t, rpcResp.Result)
			require.NotNil(t, rpcResp.Error)
			assert.Equal(t, tc.expectedErr.Code, rpcResp.Error.Code)
			// Only check contains for message, as data might differ slightly (e.g., underlying parse error details)
			assert.Contains(t, rpcResp.Error.Message, tc.expectedErr.Message)
		})
	}
}
