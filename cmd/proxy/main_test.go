package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"smart-mcp-proxy/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func testHttpServer(serverName string, allowedTools []string, allowedResources []string) (*httptest.Server, config.MCPServerConfig) {
	mux := http.NewServeMux()

	mux.HandleFunc("/tools", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		var tools []config.ToolInfo
		for _, tool := range allowedTools {
			tools = append(tools, config.ToolInfo{Name: tool, InputSchema: map[string]interface{}{}})
		}

		bytes, _ := json.Marshal(map[string]interface{}{"tools": tools})
		w.Write(bytes)
	})

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

	server := httptest.NewServer(mux)
	conf := config.MCPServerConfig{
		Name:             serverName,
		Address:          server.URL,
		AllowedTools:     allowedTools,
		AllowedResources: allowedResources,
	}

	return server, conf
}

// Helper to create a ProxyServer with test MCP servers
func setupTestProxyServer() (*ProxyServer, []*httptest.Server) {
	server1, server1Conf := testHttpServer("server1", []string{"tool1", "tool2"}, []string{"res1"})
	server2, server2Conf := testHttpServer("server2", []string{"tool3"}, []string{"res2"})

	cfg := &config.Config{
		MCPServers: []config.MCPServerConfig{server1Conf, server2Conf},
	}

	servers, err := config.NewMCPServers(cfg)
	fmt.Println(servers, err)
	ps := &ProxyServer{
		engine:     gin.Default(),
		mcpServers: servers,
	}
	ps.engine.GET("/tools", ps.handleTools)
	ps.engine.GET("/resources", ps.handleResources)
	ps.engine.Any("/tool/:toolName/*proxyPath", ps.handleToolProxy)
	ps.engine.Any("/resource/:resourceName/*proxyPath", ps.handleResourceProxy)
	return ps, []*httptest.Server{server1, server2}
}

// TestFindMCPServerByTool tests finding MCP server by tool name.
func TestFindMCPServerByTool(t *testing.T) {
	ps, servers := setupTestProxyServer()
	for _, server := range servers {
		defer server.Close()
	}

	server := ps.findMCPServerByTool("tool1")
	assert.NotNil(t, server)
	assert.Equal(t, "server1", server.Config.Name)

	server = ps.findMCPServerByTool("tool3")
	assert.NotNil(t, server)
	assert.Equal(t, "server2", server.Config.Name)

	server = ps.findMCPServerByTool("toolX")
	assert.Nil(t, server)
}

func TestHandleTools(t *testing.T) {
	ps, servers := setupTestProxyServer()
	for _, server := range servers {
		defer server.Close()
	}

	req := httptest.NewRequest("GET", "/tools", nil)
	w := httptest.NewRecorder()
	ps.engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Tools []config.ToolInfo `json:"tools"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.NotEmpty(t, resp.Tools)

	// Check that returned tools have expected fields
	for _, tool := range resp.Tools {
		assert.NotEmpty(t, tool.Name)
		// InputSchema can be empty map but should not be nil
		assert.NotNil(t, tool.InputSchema)
	}
}

func TestHandleResources(t *testing.T) {
	ps, servers := setupTestProxyServer()
	for _, server := range servers {
		defer server.Close()
	}

	req := httptest.NewRequest("GET", "/resources", nil)
	w := httptest.NewRecorder()
	ps.engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Resources []config.ResourceInfo `json:"resources"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.NotEmpty(t, resp.Resources)

	// Check that returned resources have expected fields
	for _, res := range resp.Resources {
		assert.NotEmpty(t, res.Name)
	}
}
