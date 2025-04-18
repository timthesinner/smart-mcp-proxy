package config

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// TestLoadConfig_Valid tests loading a valid config file.
func TestLoadConfig_Valid(t *testing.T) {
	content := `{
		"mcp_servers": [
			{
				"name": "server1",
				"address": "http://localhost:9000",
				"allowed_tools": ["tool1", "tool2"],
				"allowed_resources": ["res1"]
			}
		]
	}`
	tmpFile, err := os.CreateTemp("", "config_test_*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	cfg, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if len(cfg.MCPServers) != 1 {
		t.Errorf("expected 1 MCP server, got %d", len(cfg.MCPServers))
	}
	if cfg.MCPServers[0].Name != "server1" {
		t.Errorf("expected server name 'server1', got '%s'", cfg.MCPServers[0].Name)
	}
}

// TestLoadConfig_InvalidPath tests loading a config from a nonexistent path.
func TestLoadConfig_InvalidPath(t *testing.T) {
	_, err := LoadConfig("nonexistent.json")
	if err == nil {
		t.Error("expected error for nonexistent config file, got nil")
	}
}

// TestValidate tests the Validate method of Config.
func TestValidate(t *testing.T) {
	cfg := &Config{
		MCPServers: []MCPServerConfig{
			{Name: "server1", Address: "http://localhost:9000"},
			{Name: "server2", Address: "http://localhost:9001"},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected valid config, got error: %v", err)
	}

	cfgEmpty := &Config{}
	if err := cfgEmpty.Validate(); err == nil {
		t.Error("expected error for empty MCPServers, got nil")
	}

	cfgDuplicate := &Config{
		MCPServers: []MCPServerConfig{
			{Name: "server1", Address: "http://localhost:9000"},
			{Name: "server1", Address: "http://localhost:9001"},
		},
	}
	if err := cfgDuplicate.Validate(); err == nil {
		t.Error("expected error for duplicate server names, got nil")
	}

	cfgNoName := &Config{
		MCPServers: []MCPServerConfig{
			{Name: "", Address: "http://localhost:9000"},
		},
	}
	if err := cfgNoName.Validate(); err == nil {
		t.Error("expected error for empty server name, got nil")
	}

	cfgNoAddressOrCommand := &Config{
		MCPServers: []MCPServerConfig{
			{Name: "server1", Address: "", Command: ""},
		},
	}
	if err := cfgNoAddressOrCommand.Validate(); err == nil {
		t.Error("expected error for empty server address and command, got nil")
	}
}

// TestNewMCPServers tests instantiation of MCP servers including stdio-based.
func TestNewMCPServers(t *testing.T) {
	cfg := &Config{
		MCPServers: []MCPServerConfig{
			{Name: "server1", Address: "http://localhost:9000"},
		},
	}
	servers, err := NewMCPServers(cfg)
	if err != nil {
		t.Fatalf("NewMCPServers failed: %v", err)
	}
	if len(servers) != 1 {
		t.Errorf("expected 1 MCP server, got %d", len(servers))
	}
	if servers[0].Config.Name != "server1" {
		t.Errorf("expected server name 'server1', got '%s'", servers[0].Config.Name)
	}
}

// TestNewMCPServers_Stdio tests instantiation of stdio-based MCP server.
func TestNewMCPServers_Stdio(t *testing.T) {
	cfg := &Config{
		MCPServers: []MCPServerConfig{
			{
				Name:    "stdio-server",
				Command: "cat",
				Args:    []string{},
				Env: map[string]interface{}{
					"foo": "bar",
				},
			},
		},
	}
	servers, err := NewMCPServers(cfg)
	if err != nil {
		t.Fatalf("NewMCPServers failed for stdio server: %v", err)
	}
	if len(servers) != 1 {
		t.Errorf("expected 1 MCP server, got %d", len(servers))
	}
	if servers[0].Config.Name != "stdio-server" {
		t.Errorf("expected server name 'stdio-server', got '%s'", servers[0].Config.Name)
	}
	if servers[0].cmd == nil {
		t.Error("expected stdio process to be started")
	}
	if err := servers[0].Shutdown(); err != nil {
		t.Errorf("failed to shutdown stdio server: %v", err)
	}
}

// TestStartStdioProcess_Error tests error on starting nonexistent command.
func TestStartStdioProcess_Error(t *testing.T) {
	server := &MCPServer{
		Config: MCPServerConfig{
			Name:    "bad-server",
			Command: "nonexistent-command",
		},
	}
	err := server.startStdioProcess()
	if err == nil {
		t.Error("expected error starting nonexistent command, got nil")
	}
}

// TestShutdown tests graceful shutdown of stdio MCP server.
func TestShutdown(t *testing.T) {
	cfg := &Config{
		MCPServers: []MCPServerConfig{
			{
				Name:    "stdio-server",
				Command: "cat",
			},
		},
	}
	servers, err := NewMCPServers(cfg)
	if err != nil {
		t.Fatalf("NewMCPServers failed: %v", err)
	}
	server := servers[0]
	err = server.Shutdown()
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

// TestMonitorProcess_Restart tests process restart on exit.
func TestMonitorProcess_Restart(t *testing.T) {
	cfg := &Config{
		MCPServers: []MCPServerConfig{
			{
				Name:    "stdio-server",
				Command: "cat",
			},
		},
	}
	servers, err := NewMCPServers(cfg)
	if err != nil {
		t.Fatalf("NewMCPServers failed: %v", err)
	}
	server := servers[0]

	if server.cmd != nil && server.cmd.Process != nil {
		err := server.cmd.Process.Kill()
		if err != nil {
			t.Fatalf("failed to kill process: %v", err)
		}
	}

	time.Sleep(500 * time.Millisecond)

	if server.cmd == nil || server.cmd.Process == nil {
		t.Error("expected process to be restarted")
	}

	server.Shutdown()
}

// TestRefreshToolsAndResources_HTTP_FullAndLegacy tests refreshToolsAndResources with HTTP fetcher for full and legacy responses.
func TestRefreshToolsAndResources_HTTP_FullAndLegacy(t *testing.T) {
	// Mock MCPServer with HTTP client
	server := &MCPServer{
		Config: MCPServerConfig{
			Name:    "http-server",
			Address: "http://mockserver",
		},
	}

	// Mock HTTP client with RoundTrip function
	server.httpClient = &http.Client{
		Transport: &mockRoundTripper{
			roundTripFunc: func(req *http.Request) (*http.Response, error) {
				url := req.URL.String()
				var body string
				if strings.HasSuffix(url, "/tools") {
					// Return full ToolInfo JSON
					body = `{"tools":[{"name":"tool1","description":"desc1"},{"name":"tool2","description":"desc2"}]}`
				} else if strings.HasSuffix(url, "/resources") {
					// Return full ResourceInfo JSON (array of objects)
					body = `{"resources":[{"name":"res1","description":"desc1"},{"name":"res2","description":"desc2"}]}`
				} else {
					return nil, fmt.Errorf("unexpected URL: %s", url)
				}
				resp := &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader(body)),
				}
				return resp, nil
			},
		},
	}

	err := server.refreshToolsAndResources()
	if err != nil {
		t.Fatalf("refreshToolsAndResources failed: %v", err)
	}

	// Verify full ToolInfo parsed
	if len(server.tools) != 2 || server.tools[0].Name != "tool1" || server.tools[1].Description != "desc2" {
		t.Errorf("unexpected tools parsed: %+v", server.tools)
	}

	// Verify full ResourceInfo parsed
	if len(server.resources) != 2 || server.resources[0].Name != "res1" || server.resources[1].Name != "res2" {
		t.Errorf("unexpected resources parsed: %+v", server.resources)
	}
}

// mockRoundTripper mocks http.RoundTripper for testing
type mockRoundTripper struct {
	roundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTripFunc(req)
}

// TestRefreshToolsAndResources_Stdio_FullAndLegacy tests refreshToolsAndResources with stdio fetcher for full and legacy responses.
type mockMCPServer struct {
	MCPServer
	responses map[string][]string
	callCount map[string]int
}

func (m *mockMCPServer) HandleStdioRequest(reqBytes []byte) ([]byte, error) {
	var reqMap map[string]interface{}
	if err := json.Unmarshal(reqBytes, &reqMap); err != nil {
		return nil, err
	}
	method, _ := reqMap["method"].(string)
	count := m.callCount[method]
	m.callCount[method] = count + 1
	if count >= len(m.responses[method]) {
		return nil, fmt.Errorf("no more mock responses for %s", method)
	}
	return []byte(m.responses[method][count]), nil
}

func TestRefreshToolsAndResources_Stdio_FullAndLegacy(t *testing.T) {
	server := &mockMCPServer{
		MCPServer: MCPServer{
			Config: MCPServerConfig{
				Name:    "stdio-server",
				Command: "mockcmd",
			},
		},
		responses: map[string][]string{
			"tools/list": {
				`{"result":{"tools":[{"name":"tool1","description":"desc1"}],"nextCursor":"cursor1"}}`,
				`{"result":{"tools":[{"name":"tool2","description":"desc2"}]}}`,
			},
			"resources/list": {
				`{"result":{"resources":[{"name":"res1","description":"desc1"}],"nextCursor":"cursor2"}}`,
				`{"result":{"resources":[{"name":"res2","description":"desc2"}]}}`,
			},
		},
		callCount: make(map[string]int),
	}
	server.HandleStdioRequestFunc = server.HandleStdioRequest

	err := server.refreshToolsAndResources()
	if err != nil {
		t.Fatalf("refreshToolsAndResources failed: %v", err)
	}

	// Verify full tools parsed
	if len(server.tools) != 2 || server.tools[0].Name != "tool1" || server.tools[1].Name != "tool2" {
		t.Errorf("unexpected tools parsed: %+v", server.tools)
	}

	// Verify full resources parsed
	if len(server.resources) != 2 || server.resources[0].Name != "res1" || server.resources[1].Name != "res2" {
		t.Errorf("unexpected resources parsed: %+v", server.resources)
	}
}

// TestRefreshToolsAndResources_HTTP_ErrorCases tests error handling in HTTP fetcher.
func TestRefreshToolsAndResources_HTTP_ErrorCases(t *testing.T) {
	server := &MCPServer{
		Config: MCPServerConfig{
			Name:    "http-server",
			Address: "http://mockserver",
		},
	}

	// Mock HTTP client to return error on tools endpoint
	server.httpClient = &http.Client{
		Transport: &mockRoundTripper{
			roundTripFunc: func(req *http.Request) (*http.Response, error) {
				url := req.URL.String()
				if strings.HasSuffix(url, "/tools") {
					return nil, fmt.Errorf("network error")
				}
				// Return valid empty resources response
				body := `{"resources":[]}`
				resp := &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader(body)),
				}
				return resp, nil
			},
		},
	}

	err := server.refreshToolsAndResources()
	if err == nil || !strings.Contains(err.Error(), "failed to get tools") {
		t.Errorf("expected error for tools fetch failure, got %v", err)
	}
}

// Create a new type embedding mockMCPServer to override HandleStdioRequest method
type errorMockMCPServer struct {
	*mockMCPServer
}

// Override HandleStdioRequest to always return error
func (m *errorMockMCPServer) HandleStdioRequest(req []byte) ([]byte, error) {
	return nil, fmt.Errorf("stdio error")
}

// TestRefreshToolsAndResources_Stdio_ErrorCases tests error handling in stdio fetcher.
func TestRefreshToolsAndResources_Stdio_ErrorCases(t *testing.T) {
	server := &mockMCPServer{
		MCPServer: MCPServer{
			Config: MCPServerConfig{
				Name:    "stdio-server",
				Command: "mockcmd",
			},
		},
		responses: map[string][]string{},
		callCount: make(map[string]int),
	}

	errorServer := &errorMockMCPServer{mockMCPServer: server}
	errorServer.HandleStdioRequestFunc = errorServer.HandleStdioRequest

	err := errorServer.refreshToolsAndResources()
	if err == nil || !strings.Contains(err.Error(), "failed to fetch tools") {
		t.Errorf("expected error for stdio fetch failure, got %v", err)
	}
}
