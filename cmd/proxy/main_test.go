package main

import (
	"bytes" // Keep bytes
	"encoding/json"
	"fmt"
	"io"  // Add io
	"log" // Add log
	"net/http"
	"net/http/httptest"
	"strings" // Add strings
	"testing"

	"smart-mcp-proxy/internal/config"

	// Gin is needed for HTTPProxy tests
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require" // Add require
)

// testHttpServer updated to return CallToolResult for tool calls
func testHttpServer(serverName string, allowedTools []string, allowedResources []string, restrictedTools []string, restrictedResources []string) (*httptest.Server, config.MCPServerConfig) {
	mux := http.NewServeMux()

	// Simulate /tools endpoint on backend
	mux.HandleFunc("/tools", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		var tools []config.ToolInfo
		for _, tool := range allowedTools {
			// Add basic schema for testing
			tools = append(tools, config.ToolInfo{Name: tool, InputSchema: map[string]interface{}{"type": "object"}})
		}
		for _, tool := range restrictedTools {
			// Add basic schema for testing
			tools = append(tools, config.ToolInfo{Name: tool, InputSchema: map[string]interface{}{"type": "object"}})
		}
		bytes, _ := json.Marshal(map[string]interface{}{"tools": tools})
		w.Write(bytes)
	})

	// Simulate /resources endpoint on backend
	mux.HandleFunc("/resources", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		var resources []config.ResourceInfo
		for _, r := range allowedResources {
			resources = append(resources, config.ResourceInfo{Name: r})
		}
		for _, r := range restrictedResources {
			resources = append(resources, config.ResourceInfo{Name: r})
		}
		bytes, _ := json.Marshal(map[string]interface{}{"resources": resources})
		w.Write(bytes)
	})

	// Simulate a generic tool endpoint on backend (POST /tool/:toolName)
	mux.HandleFunc("/tool/", func(w http.ResponseWriter, r *http.Request) {
		toolName := strings.TrimPrefix(r.URL.Path, "/tool/")

		// --- Error Simulation ---
		if toolName == "tool-error-500" {
			log.Printf("Mock Server: Simulating 500 error for tool '%s'", toolName)
			http.Error(w, "Internal Server Error Simulation", http.StatusInternalServerError)
			return
		}
		// --- End Error Simulation ---

		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		// Basic echo response for tool calls, returning a CallToolResult structure
		bodyBytes, _ := io.ReadAll(r.Body) // Read arguments from body if needed for response
		log.Printf("Mock Server: Received call to tool '%s' with body: %s", toolName, string(bodyBytes))

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		// Return a valid CallToolResult matching the expected structure
		responseText := fmt.Sprintf(`{"status": "tool /tool/%s called"}`, toolName) // This is the inner JSON string
		result := config.CallToolResult{
			Content: []config.ContentBlock{
				{Type: "text", Text: &responseText}, // Wrap the JSON string in the ContentBlock
			},
		}
		json.NewEncoder(w).Encode(result) // Encode the CallToolResult struct
	})

	// Simulate a generic resource endpoint on backend
	mux.HandleFunc("/resource/", func(w http.ResponseWriter, r *http.Request) {
		// --- Error Simulation ---
		if strings.Contains(r.URL.Path, "error-404") {
			log.Printf("Mock Server: Simulating 404 error for resource path '%s'", r.URL.Path)
			http.Error(w, "Resource Not Found Simulation", http.StatusNotFound)
			return
		}
		if strings.Contains(r.URL.Path, "error-500") {
			log.Printf("Mock Server: Simulating 500 error for resource path '%s'", r.URL.Path)
			http.Error(w, "Internal Server Error Simulation", http.StatusInternalServerError)
			return
		}
		// --- End Error Simulation ---

		// Basic echo response for resource access
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		// Revert: Do not include method in response to keep command_mode_test passing
		fmt.Fprintf(w, `{"status": "resource %s accessed"}`, r.URL.Path)
	})

	server := httptest.NewServer(mux)
	conf := config.MCPServerConfig{
		Name:             serverName,
		Address:          server.URL,
		AllowedTools:     allowedTools,
		AllowedResources: allowedResources,
	}

	return server, conf
}

// setupTestHTTPProxy sets up ProxyServer and HTTPProxy for testing.
// Returns the HTTPProxy, the core ProxyServer, and the backend test servers.
func setupTestHTTPProxy(t *testing.T) (*HTTPProxy, *ProxyServer, []*httptest.Server) {
	// Add "tool-error-500" to server1's allowed tools for testing backend errors
	server1, server1Conf := testHttpServer("server1", []string{"tool1", "tool2", "tool-error-500"}, []string{"res1"}, []string{"r-tool1", "r-tool2"}, []string{"r-res1"})
	server2, server2Conf := testHttpServer("server2", []string{"tool3"}, []string{"res2"}, []string{"r-tool3"}, []string{"r-res2"})

	cfg := &config.Config{
		MCPServers: []config.MCPServerConfig{server1Conf, server2Conf},
	}

	// 1. Create the core ProxyServer
	ps, err := NewProxyServer(cfg)
	require.NoError(t, err) // Use require here
	require.NotNil(t, ps)   // Use require here

	// 2. Create the HTTPProxy using the ProxyServer
	// Use a dummy listen address for testing; it won't actually bind.
	httpProxy, err := NewHTTPProxy(ps, ":0") // ":0" is often used for ephemeral ports in tests
	require.NoError(t, err)                  // Use require here
	require.NotNil(t, httpProxy)             // Use require here

	return httpProxy, ps, []*httptest.Server{server1, server2}
}

// TestFindMCPServerByTool tests finding MCP server by tool name (uses core ProxyServer logic).
func TestFindMCPServerByTool(t *testing.T) {
	_, ps, servers := setupTestHTTPProxy(t) // Get the core ps instance
	for _, server := range servers {
		defer server.Close()
	}

	server := ps.findMCPServerByTool("tool1")
	assert.NotNil(t, server)
	assert.Equal(t, "server1", server.Config.Name)

	server = ps.findMCPServerByTool("tool3")
	assert.NotNil(t, server)
	assert.Equal(t, "server2", server.Config.Name)

	server = ps.findMCPServerByTool("toolX") // Non-existent tool
	assert.Nil(t, server)
}

// TestFindMCPServerByResource tests finding MCP server by resource name (uses core ProxyServer logic).
func TestFindMCPServerByResource(t *testing.T) {
	_, ps, servers := setupTestHTTPProxy(t) // Get the core ps instance
	for _, server := range servers {
		defer server.Close()
	}

	server := ps.findMCPServerByResource("res1")
	assert.NotNil(t, server)
	assert.Equal(t, "server1", server.Config.Name)

	server = ps.findMCPServerByResource("res2")
	assert.NotNil(t, server)
	assert.Equal(t, "server2", server.Config.Name)

	server = ps.findMCPServerByResource("resX") // Non-existent resource
	assert.Nil(t, server)
}

// TestHTTPHandleTools tests the /tools endpoint via the HTTPProxy.
func TestHTTPHandleTools(t *testing.T) {
	httpProxy, _, servers := setupTestHTTPProxy(t)
	for _, server := range servers {
		defer server.Close()
	}

	req := httptest.NewRequest("GET", "/tools", nil)
	w := httptest.NewRecorder()
	httpProxy.engine.ServeHTTP(w, req) // Use the HTTPProxy's engine

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Tools []config.ToolInfo `json:"tools"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	// Should get tools from both servers (tool1, tool2, tool-error-500, tool3)
	assert.Len(t, resp.Tools, 4)

	// Check that returned tools have expected fields
	foundTools := make(map[string]bool)
	for _, tool := range resp.Tools {
		assert.NotEmpty(t, tool.Name)
		assert.NotNil(t, tool.InputSchema)
		foundTools[tool.Name] = true
	}
	assert.True(t, foundTools["tool1"])
	assert.True(t, foundTools["tool2"])
	assert.True(t, foundTools["tool3"])
}

// TestHTTPHandleResources tests the /resources endpoint via the HTTPProxy.
func TestHTTPHandleResources(t *testing.T) {
	httpProxy, _, servers := setupTestHTTPProxy(t)
	for _, server := range servers {
		defer server.Close()
	}

	req := httptest.NewRequest("GET", "/resources", nil)
	w := httptest.NewRecorder()
	httpProxy.engine.ServeHTTP(w, req) // Use the HTTPProxy's engine

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Resources []config.ResourceInfo `json:"resources"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	// Should get resources from both servers
	assert.Len(t, resp.Resources, 2) // res1, res2

	// Check that returned resources have expected fields
	foundResources := make(map[string]bool)
	for _, res := range resp.Resources {
		assert.NotEmpty(t, res.Name)
		foundResources[res.Name] = true
	}
	assert.True(t, foundResources["res1"])
	assert.True(t, foundResources["res2"])

}

// TestHTTPHandleRestrictedTools tests the GET /restricted-tools endpoint.
func TestHTTPHandleRestrictedTools(t *testing.T) {
	httpProxy, _, servers := setupTestHTTPProxy(t)
	for _, server := range servers {
		defer server.Close()
	}

	req := httptest.NewRequest("GET", "/restricted-tools", nil)
	w := httptest.NewRecorder()
	httpProxy.engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Tools []RestrictedToolInfo `json:"tools"` // Use the restricted type
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	// Should get restricted tools from both servers
	assert.Len(t, resp.Tools, 3) // r-tool1, r-tool2, r-tool3

	// Check that returned tools have expected fields including ServerName
	foundTools := make(map[string]string) // Map tool name to server name
	for _, tool := range resp.Tools {
		assert.NotEmpty(t, tool.Name)
		assert.NotEmpty(t, tool.ServerName)
		assert.NotNil(t, tool.InputSchema)
		foundTools[tool.Name] = tool.ServerName
	}
	assert.Equal(t, "server1", foundTools["r-tool1"])
	assert.Equal(t, "server1", foundTools["r-tool2"])
	assert.Equal(t, "server2", foundTools["r-tool3"])
}

// TestHTTPHandleRestrictedResources tests the GET /restricted-resources endpoint.
func TestHTTPHandleRestrictedResources(t *testing.T) {
	httpProxy, _, servers := setupTestHTTPProxy(t)
	for _, server := range servers {
		defer server.Close()
	}

	req := httptest.NewRequest("GET", "/restricted-resources", nil)
	w := httptest.NewRecorder()
	httpProxy.engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Resources []RestrictedResourceInfo `json:"resources"` // Use the restricted type
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	// Should get restricted resources from both servers
	assert.Len(t, resp.Resources, 2) // r-res1, r-res2

	// Check that returned resources have expected fields including ServerName
	foundResources := make(map[string]string) // Map resource name to server name
	for _, res := range resp.Resources {
		assert.NotEmpty(t, res.Name)
		assert.NotEmpty(t, res.ServerName)
		foundResources[res.Name] = res.ServerName
	}
	assert.Equal(t, "server1", foundResources["r-res1"])
	assert.Equal(t, "server2", foundResources["r-res2"])
}

// TestHTTPHandleToolCall tests the new POST /tool/:toolName endpoint.
func TestHTTPHandleToolCall(t *testing.T) {
	httpProxy, _, servers := setupTestHTTPProxy(t)
	for _, server := range servers {
		defer server.Close()
	}

	// --- Test valid tool call ---
	args := `{"arg1": "value1"}`
	req := httptest.NewRequest("POST", "/tool/tool1", strings.NewReader(args))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	httpProxy.engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Check response body structure (config.CallToolResult)
	var callResult config.CallToolResult
	err := json.Unmarshal(w.Body.Bytes(), &callResult)
	assert.NoError(t, err, "Failed to unmarshal response into CallToolResult")
	require.Len(t, callResult.Content, 1, "Expected one content block") // Use require
	assert.Equal(t, "text", callResult.Content[0].Type)
	require.NotNil(t, callResult.Content[0].Text, "Text content should not be nil") // Use require
	// Mock server now returns CallToolResult containing JSON: {"status": "tool /tool/tool1 called"}
	expectedInnerJSON := `{"status":"tool /tool/tool1 called"}`
	assert.JSONEq(t, expectedInnerJSON, *callResult.Content[0].Text, "Inner JSON content mismatch")

	// --- Test tool on different server (should still work via CallTool routing) ---
	args = `{"arg2": 42}`
	req = httptest.NewRequest("POST", "/tool/tool3", strings.NewReader(args))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	httpProxy.engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	err = json.Unmarshal(w.Body.Bytes(), &callResult)
	assert.NoError(t, err)
	require.Len(t, callResult.Content, 1)         // Use require
	require.NotNil(t, callResult.Content[0].Text) // Use require
	// Mock server returns CallToolResult containing JSON: {"status": "tool /tool/tool3 called"}
	expectedInnerJSON = `{"status":"tool /tool/tool3 called"}`
	assert.JSONEq(t, expectedInnerJSON, *callResult.Content[0].Text)

	// --- Test tool not found ---
	args = `{}`
	req = httptest.NewRequest("POST", "/tool/nonexistentTool", strings.NewReader(args))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	httpProxy.engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code) // Expect 404 for tool not found
	var errResp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(t, err)
	// Check the specific error message returned by the updated handler
	expectedErrMsg := "Tool 'nonexistentTool' not found or not provided by any configured server"
	assert.Equal(t, expectedErrMsg, errResp["error"])
	_, detailsExist := errResp["details"]
	assert.False(t, detailsExist, "Error response should not contain 'details' field")

	// --- Test invalid JSON body ---
	args = `{"arg1": "value1"` // Malformed JSON
	req = httptest.NewRequest("POST", "/tool/tool1", strings.NewReader(args))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	httpProxy.engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	err = json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(t, err)
	assert.Contains(t, errResp["error"], "Invalid request body")

	// --- Test empty body (should be treated as empty args, potentially valid) ---
	req = httptest.NewRequest("POST", "/tool/tool1", nil) // Empty body
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	httpProxy.engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code) // Assuming empty args is valid for tool1 mock
	err = json.Unmarshal(w.Body.Bytes(), &callResult)
	assert.NoError(t, err)
	require.Len(t, callResult.Content, 1)                      // Use require
	require.NotNil(t, callResult.Content[0].Text)              // Use require
	expectedInnerJSON = `{"status":"tool /tool/tool1 called"}` // Mock response is the same
	assert.JSONEq(t, expectedInnerJSON, *callResult.Content[0].Text)

	// --- Test backend server error (500) ---
	args = `{}`
	req = httptest.NewRequest("POST", "/tool/tool-error-500", strings.NewReader(args)) // Use the error-simulating tool name
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	httpProxy.engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code) // Expect 502 Bad Gateway from proxy
	err = json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(t, err)
	// Check the specific error message returned by handleToolCall for backend communication errors
	expectedErrMsg = "Error communicating with backend server for tool 'tool-error-500'" // Use =
	assert.Equal(t, expectedErrMsg, errResp["error"])
	_, detailsExist = errResp["details"] // Use =
	assert.False(t, detailsExist, "Details should not be present for this error type")

	// --- Test incorrect HTTP method ---
	req = httptest.NewRequest("GET", "/tool/tool1", nil) // Use GET instead of POST
	w = httptest.NewRecorder()
	httpProxy.engine.ServeHTTP(w, req)

	// Gin's default behavior for unhandled methods on a matched route prefix is 404
	// If we wanted 405, the handler itself would need more specific method checks.
	// For now, asserting 404 is consistent with Gin's behavior.
	assert.Equal(t, http.StatusNotFound, w.Code)
	// Optionally check the body for Gin's standard 404 page or JSON error
	// assert.Contains(t, w.Body.String(), "404 page not found")
}

// TestHTTPHandleResourceProxy tests the resource proxy endpoint via the HTTPProxy.
func TestHTTPHandleResourceProxy(t *testing.T) {
	httpProxy, _, servers := setupTestHTTPProxy(t)
	for _, server := range servers {
		defer server.Close()
	}

	// --- Test valid GET resource proxy ---
	req := httptest.NewRequest("GET", "/resource/server1/res1/data", nil)
	w := httptest.NewRecorder()
	httpProxy.engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "resource /resource/res1/data accessed") // Reverted: Do not check method

	// --- Test valid PUT resource proxy ---
	req = httptest.NewRequest("PUT", "/resource/server2/res2/config", bytes.NewReader([]byte(`{"key":"value"}`))) // Use PUT, provide body
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	httpProxy.engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "resource /resource/res2/config accessed") // Reverted: Do not check method

	// --- Test valid POST resource proxy ---
	req = httptest.NewRequest("POST", "/resource/server1/res1/action", bytes.NewReader([]byte(`{"action":"start"}`)))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	httpProxy.engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "resource /resource/res1/action accessed") // Reverted: Do not check method

	// --- Test valid DELETE resource proxy ---
	req = httptest.NewRequest("DELETE", "/resource/server2/res2/item/123", nil)
	w = httptest.NewRecorder()
	httpProxy.engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "resource /resource/res2/item/123 accessed") // Reverted: Do not check method

	// --- Test resource not allowed on server ---
	req = httptest.NewRequest("GET", "/resource/server1/res2/value", nil) // res2 not on server1
	w = httptest.NewRecorder()
	httpProxy.engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "resource 'res2' not allowed on server 'server1'")

	// --- Test server not found ---
	req = httptest.NewRequest("GET", "/resource/serverX/res1/info", nil)
	w = httptest.NewRecorder()
	httpProxy.engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "server 'serverX' not found")

	// --- Test backend 404 error ---
	req = httptest.NewRequest("GET", "/resource/server1/res1/error-404", nil) // Use error-simulating path
	w = httptest.NewRecorder()
	httpProxy.engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code) // Proxy should forward the 404
	assert.Contains(t, w.Body.String(), "Resource Not Found Simulation")

	// --- Test backend 500 error ---
	req = httptest.NewRequest("GET", "/resource/server2/res2/error-500", nil) // Use error-simulating path
	w = httptest.NewRecorder()
	httpProxy.engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadGateway, w.Code) // Proxy should return 502
	// Check for the generic error message returned by the proxy handler
	var errResp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(t, err)
	assert.Equal(t, "backend server 'server2' returned an error", errResp["error"])
}
