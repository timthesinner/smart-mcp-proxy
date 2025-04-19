package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http/httptest"
	"os"
	"testing"

	"smart-mcp-proxy/internal/config"
)

func setupTestCommandProxyServer() (*config.Config, []*httptest.Server) {
	server1, server1Conf := testHttpServer("server1", []string{"tool1", "tool2"}, []string{"res1"})
	server2, server2Conf := testHttpServer("server2", []string{"tool3"}, []string{"res2"})

	cfg := &config.Config{
		MCPServers: []config.MCPServerConfig{server1Conf, server2Conf},
	}

	return cfg, []*httptest.Server{server1, server2}
}

func TestRunCommandMode(t *testing.T) {
	cfg, servers := setupTestCommandProxyServer()
	for _, server := range servers {
		defer server.Close()
	}

	mcpReq := map[string]interface{}{
		"method":  "GET",
		"path":    "/tools",
		"query":   "",
		"headers": map[string][]string{},
		"body":    "",
	}

	reqBytes, err := json.Marshal(mcpReq)
	if err != nil {
		t.Fatalf("failed to marshal MCP request: %v", err)
	}

	r, w := createPipe(t)

	oldStdin := setStdin(r)
	defer restoreStdin(oldStdin)

	oldStdout := setStdout(w)
	defer restoreStdout(oldStdout)

	go func() {
		defer w.Close()
		w.Write(reqBytes)
		w.Write([]byte("\n"))
	}()

	go runCommandMode(cfg)

	respData := readAll(t, r)

	var resp map[string]interface{}
	err = json.Unmarshal(respData, &resp)
	if err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if _, ok := resp["tools"]; !ok {
		t.Errorf("response missing 'tools' key %s", respData)
	}
}

// Helper functions for pipe and stdio redirection

func createPipe(t *testing.T) (*os.File, *os.File) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	return r, w
}

func setStdin(r *os.File) *os.File {
	oldStdin := os.Stdin
	os.Stdin = r
	return oldStdin
}

func restoreStdin(oldStdin *os.File) {
	os.Stdin = oldStdin
}

func setStdout(w *os.File) *os.File {
	oldStdout := os.Stdout
	os.Stdout = w
	return oldStdout
}

func restoreStdout(oldStdout *os.File) {
	os.Stdout = oldStdout
}

func readAll(t *testing.T, r *os.File) []byte {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}
	return data
}
