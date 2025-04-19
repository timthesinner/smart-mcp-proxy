package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"smart-mcp-proxy/internal/config"

	// Gin is needed for HTTPProxy tests
	"github.com/stretchr/testify/assert"
)

// testHttpServer remains the same - simulates a backend MCP server
func testHttpServer(serverName string, allowedTools []string, allowedResources []string) (*httptest.Server, config.MCPServerConfig) {
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
		bytes, _ := json.Marshal(map[string]interface{}{"resources": resources})
		w.Write(bytes)
	})

	// Simulate a generic tool endpoint on backend
	mux.HandleFunc("/tool/", func(w http.ResponseWriter, r *http.Request) {
		// Basic echo response for tool calls
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status": "tool %s called"}`, r.URL.Path)
	})

	// Simulate a generic resource endpoint on backend
	mux.HandleFunc("/resource/", func(w http.ResponseWriter, r *http.Request) {
		// Basic echo response for resource access
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
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
	server1, server1Conf := testHttpServer("server1", []string{"tool1", "tool2"}, []string{"res1"})
	server2, server2Conf := testHttpServer("server2", []string{"tool3"}, []string{"res2"})

	cfg := &config.Config{
		MCPServers: []config.MCPServerConfig{server1Conf, server2Conf},
	}

	// 1. Create the core ProxyServer
	ps, err := NewProxyServer(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, ps)

	// 2. Create the HTTPProxy using the ProxyServer
	// Use a dummy listen address for testing; it won't actually bind.
	httpProxy, err := NewHTTPProxy(ps, ":0") // ":0" is often used for ephemeral ports in tests
	assert.NoError(t, err)
	assert.NotNil(t, httpProxy)

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
	// Should get tools from both servers
	assert.Len(t, resp.Tools, 3) // tool1, tool2, tool3

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

// TestHTTPHandleToolProxy tests the tool proxy endpoint via the HTTPProxy.
func TestHTTPHandleToolProxy(t *testing.T) {
	httpProxy, _, servers := setupTestHTTPProxy(t)
	for _, server := range servers {
		defer server.Close()
	}

	// Test valid tool proxy
	req := httptest.NewRequest("GET", "/tool/server1/tool1/some/path", nil)
	w := httptest.NewRecorder()
	httpProxy.engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "tool /tool/tool1/some/path called") // Check backend response

	// Test tool on different server
	req = httptest.NewRequest("POST", "/tool/server2/tool3/another/path", nil) // Use POST
	w = httptest.NewRecorder()
	httpProxy.engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "tool /tool/tool3/another/path called")

	// Test tool not allowed on server
	req = httptest.NewRequest("GET", "/tool/server1/tool3/path", nil) // tool3 not on server1
	w = httptest.NewRecorder()
	httpProxy.engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "tool 'tool3' not allowed on server 'server1'")

	// Test server not found
	req = httptest.NewRequest("GET", "/tool/serverX/tool1/path", nil)
	w = httptest.NewRecorder()
	httpProxy.engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "server 'serverX' not found")
}

// TestHTTPHandleResourceProxy tests the resource proxy endpoint via the HTTPProxy.
func TestHTTPHandleResourceProxy(t *testing.T) {
	httpProxy, _, servers := setupTestHTTPProxy(t)
	for _, server := range servers {
		defer server.Close()
	}

	// Test valid resource proxy
	req := httptest.NewRequest("GET", "/resource/server1/res1/data", nil)
	w := httptest.NewRecorder()
	httpProxy.engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "resource /resource/res1/data accessed")

	// Test resource on different server
	req = httptest.NewRequest("PUT", "/resource/server2/res2/config", nil) // Use PUT
	w = httptest.NewRecorder()
	httpProxy.engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "resource /resource/res2/config accessed")

	// Test resource not allowed on server
	req = httptest.NewRequest("GET", "/resource/server1/res2/value", nil) // res2 not on server1
	w = httptest.NewRecorder()
	httpProxy.engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "resource 'res2' not allowed on server 'server1'")

	// Test server not found
	req = httptest.NewRequest("GET", "/resource/serverX/res1/info", nil)
	w = httptest.NewRecorder()
	httpProxy.engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "server 'serverX' not found")
}
