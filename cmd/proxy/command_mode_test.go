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
	server1, server1Conf := testHttpServer("server1", []string{"tool1", "tool2"}, []string{"res1"})
	server2, server2Conf := testHttpServer("server2", []string{"tool3"}, []string{"res2"})

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
	var rpcResp jsonRPCResponse
	err = json.Unmarshal(respBytes, &rpcResp)
	require.NoError(t, err)

	// Assertions on the response
	assert.Equal(t, "2.0", rpcResp.JSONRPC)
	assert.Equal(t, float64(1), rpcResp.ID) // JSON numbers are float64 by default
	assert.Nil(t, rpcResp.Error)
	require.NotNil(t, rpcResp.Result)

	// Assertions on the result (should be a list of ToolInfo)
	resultData, err := json.Marshal(rpcResp.Result)
	require.NoError(t, err)

	var toolsResult []config.ToolInfo // Expecting direct list from ProxyServer.ListTools
	err = json.Unmarshal(resultData, &toolsResult)
	require.NoError(t, err)

	assert.Len(t, toolsResult, 3) // tool1, tool2, tool3
	foundTools := make(map[string]bool)
	for _, tool := range toolsResult {
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

	var rpcResp jsonRPCResponse
	err = json.Unmarshal(respBytes, &rpcResp)
	require.NoError(t, err)

	assert.Equal(t, "2.0", rpcResp.JSONRPC)
	assert.Equal(t, "res-list-req", rpcResp.ID)
	assert.Nil(t, rpcResp.Error)
	require.NotNil(t, rpcResp.Result)

	resultData, err := json.Marshal(rpcResp.Result)
	require.NoError(t, err)

	var resourcesResult []config.ResourceInfo
	err = json.Unmarshal(resultData, &resourcesResult)
	require.NoError(t, err)

	assert.Len(t, resourcesResult, 2) // res1, res2
	foundResources := make(map[string]bool)
	for _, res := range resourcesResult {
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
	toolParams := toolCallParams{
		ServerName: "server1",
		ToolName:   "tool1",
		Arguments:  json.RawMessage(`{"arg1": "value1"}`),
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

		// Check the structure of the result (map with status, headers, body)
		resultMap, ok := rpcResp.Result.(map[string]interface{})
		require.True(t, ok, "Result should be a map")
		assert.Contains(t, resultMap, "status")
		assert.Contains(t, resultMap, "headers")
		assert.Contains(t, resultMap, "body")
		assert.Equal(t, float64(200), resultMap["status"]) // Status should be 200 OK from mock server
		bodyStr, _ := resultMap["body"].(string)
		assert.Contains(t, bodyStr, `tool /tool/tool1 called`) // Check body from mock server
	}

	// --- Test server not found ---
	toolParams.ServerName = "serverX"
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
		require.NotNil(t, rpcResp.Error, "Expected error for server not found")
		assert.Equal(t, -32001, rpcResp.Error.Code) // Custom server error code
		assert.Contains(t, rpcResp.Error.Message, "Server 'serverX' not found")
	}

	// --- Test tool not allowed ---
	toolParams.ServerName = "server1"
	toolParams.ToolName = "tool3" // tool3 is on server2
	paramsBytes, _ = json.Marshal(toolParams)
	rpcReq.ID = "tool-call-err-2"
	rpcReq.Params = paramsBytes
	reqBytes, _ = json.Marshal(rpcReq)

	respBytes, err = cmdProxy.handleCommandRequest(reqBytes)
	require.NoError(t, err)
	// Unmarshal and check the "tool not allowed" error response
	{
		var rpcResp jsonRPCResponse // Scope rpcResp to this block
		err = json.Unmarshal(respBytes, &rpcResp)
		require.NoError(t, err)

		assert.Equal(t, "tool-call-err-2", rpcResp.ID)
		// Correct Assertion: Result should be nil on error
		assert.Nil(t, rpcResp.Result, "Expected nil result for tool not allowed error")
		// Correct Assertion: Error should NOT be nil on error
		require.NotNil(t, rpcResp.Error, "Expected error for tool not allowed")
		assert.Equal(t, -32002, rpcResp.Error.Code) // Custom tool not allowed error code
		assert.Contains(t, rpcResp.Error.Message, "Tool 'tool3' not allowed on server 'server1'")
	}

}

// Add similar test cases for resources/access if needed
// TestCommandHandleResourceAccess ...
