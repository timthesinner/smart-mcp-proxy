package config

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"
)

// MCPServerConfig represents the configuration for a single MCP server.
type MCPServerConfig struct {
	Name             string                 `json:"name"`
	Address          string                 `json:"address,omitempty"`
	Command          string                 `json:"command,omitempty"`
	Args             []string               `json:"args,omitempty"`
	Env              map[string]interface{} `json:"env,omitempty"`
	AllowedTools     []string               `json:"allowed_tools,omitempty"`
	AllowedResources []string               `json:"allowed_resources,omitempty"`
}

// Config represents the overall configuration for the MCP Proxy Server.
type Config struct {
	MCPServers []MCPServerConfig `json:"mcp_servers"`
}

// Validate validates the Config struct.
func (c *Config) Validate() error {
	if len(c.MCPServers) == 0 {
		return errors.New("no MCP servers defined in configuration")
	}

	names := make(map[string]struct{})
	for i, server := range c.MCPServers {
		if strings.TrimSpace(server.Name) == "" {
			return fmt.Errorf("mcp_servers[%d]: name is required", i)
		}
		if _, exists := names[server.Name]; exists {
			return fmt.Errorf("mcp_servers[%d]: duplicate server name '%s'", i, server.Name)
		}
		names[server.Name] = struct{}{}

		if strings.TrimSpace(server.Address) == "" && strings.TrimSpace(server.Command) == "" {
			return fmt.Errorf("mcp_servers[%d]: either address or command is required", i)
		}

		// AllowedTools and AllowedResources can be empty or nil, meaning no restrictions.
	}

	return nil
}

// MCPServer represents a running MCP server instance.
type MCPServer struct {
	Config MCPServerConfig

	// For HTTP/SSE MCP servers
	httpClient *http.Client

	// For stdio-based MCP servers
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	// Optional override for HandleStdioRequest for testing/mocking
	HandleStdioRequestFunc func(reqBytes []byte) ([]byte, error)

	// Process supervision
	mu         sync.Mutex
	restarting bool
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup

	// Cached list of tools and resources exposed by the MCP server
	tools     []ToolInfo
	resources []ResourceInfo

	// Cached list of tools and resources restricted by the MCP server
	restrictedTools     []ToolInfo
	restrictedResources []ResourceInfo
}

// ResourceInfo represents detailed information about a resource exposed by the MCP server.
type ResourceInfo struct {
	URI         string `json:"uri,omitempty"`
	URITemplate string `json:"uriTemplate,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ToolInfo represents detailed information about a tool exposed by the MCP server.
type ToolInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	Annotations map[string]interface{} `json:"annotations,omitempty"`
}

// CallToolRequestParams represents the parameters for a 'tools/call' JSON-RPC request.
type CallToolRequestParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ToolError represents an error returned by a tool execution.
type ToolError struct {
	Message string      `json:"message"`
	Code    string      `json:"code,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// ImageSource represents the source data for an image content block.
type ImageSource struct {
	Type      string `json:"type"`      // e.g., "base64"
	MediaType string `json:"mediaType"` // e.g., "image/png"
	Data      string `json:"data"`
}

// ContentBlock represents a single block of content within a CallToolResult.
// It uses omitempty and pointers to handle the union nature of different block types.
type ContentBlock struct {
	Type string `json:"type"` // "text", "image", "tool_use", "tool_result"

	// Fields for type="text"
	Text *string `json:"text,omitempty"`

	// Fields for type="image"
	Source *ImageSource `json:"source,omitempty"`

	// Fields for type="tool_use"
	ToolUseID *string                `json:"toolUseId,omitempty"`
	ToolName  *string                `json:"name,omitempty"` // Note: reusing 'name' tag
	Input     map[string]interface{} `json:"input,omitempty"`

	// Fields for type="tool_result"
	// ToolUseID is also used here (defined above)
	Content *string    `json:"content,omitempty"` // Assuming string content for now
	IsError *bool      `json:"isError,omitempty"`
	Error   *ToolError `json:"error,omitempty"` // Renamed from ToolResultError for consistency
}

// CallToolResult represents the result object for a 'tools/call' JSON-RPC response.
type CallToolResult struct {
	Content   []ContentBlock `json:"content"`
	IsError   bool           `json:"isError"`             // Overall error status for the tool call itself
	ToolError *ToolError     `json:"toolError,omitempty"` // Error details if the call itself failed (distinct from tool_result block errors)
}

// GetTools returns a copy of the current list of tools exposed by the MCP server.
func (s *MCPServer) GetTools() []ToolInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	toolsCopy := make([]ToolInfo, len(s.tools))
	copy(toolsCopy, s.tools)
	return toolsCopy
}

// GetRestrictedTools returns a copy of the current list of tools not exposed by the MCP server.
func (s *MCPServer) GetRestrictedTools() []ToolInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	toolsCopy := make([]ToolInfo, len(s.restrictedTools))
	copy(toolsCopy, s.restrictedTools)
	return toolsCopy
}

// GetResources returns a copy of the current list of resources exposed by the MCP server.
func (s *MCPServer) GetResources() []ResourceInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	resourcesCopy := make([]ResourceInfo, len(s.resources))
	copy(resourcesCopy, s.resources)
	return resourcesCopy
}

// GetRestrictedResources returns a copy of the current list of resources not exposed by the MCP server.
func (s *MCPServer) GetRestrictedResources() []ResourceInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	resourcesCopy := make([]ResourceInfo, len(s.restrictedResources))
	copy(resourcesCopy, s.restrictedResources)
	return resourcesCopy
}

// LoadConfig loads the configuration from a JSON file.
// The path to the config file can be provided via the configPath argument.
// If configPath is empty, it will look for the environment variable MCP_PROXY_CONFIG.
func LoadConfig(configPath string) (*Config, error) {
	if configPath == "" {
		configPath = os.Getenv("MCP_PROXY_CONFIG")
		if configPath == "" {
			return nil, errors.New("configuration path not provided and MCP_PROXY_CONFIG environment variable is not set")
		}
	}

	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return &cfg, nil
}

// NewMCPServers creates MCPServer instances from config.
func NewMCPServers(cfg *Config) ([]*MCPServer, error) {
	servers := make([]*MCPServer, 0, len(cfg.MCPServers))
	for _, sc := range cfg.MCPServers {
		server := &MCPServer{
			Config: sc,
		}

		if sc.Address != "" {
			// Initialize HTTP client for HTTP/SSE MCP server
			server.httpClient = &http.Client{
				Timeout: 30 * time.Second,
			}
			// Fetch initial tools and resources for HTTP/SSE server
			if err := server.refreshToolsAndResources(); err != nil {
				fmt.Printf("failed to fetch tools/resources for server %s: %v\n", sc.Name, err)
			}
			// Start periodic refresh
			//go server.startPeriodicRefresh()
		} else if sc.Command != "" {
			// Initialize stdio-based MCP server
			if err := server.startStdioProcess(); err != nil {
				return nil, err
			}
			// Fetch initial tools and resources for stdio server
			if err := server.refreshToolsAndResources(); err != nil {
				fmt.Printf("failed to fetch tools/resources for server %s: %v", sc.Name, err)
			}
			// Start periodic refresh
			//go server.startPeriodicRefresh()
		} else {
			return nil, errors.New("mcp server config must have either address or command")
		}

		servers = append(servers, server)
	}
	return servers, nil
}

// startStdioProcess launches the stdio-based MCP server process and sets up pipes and supervision.
func (s *MCPServer) startStdioProcess() error {
	s.mu.Lock()

	if s.restarting {
		s.mu.Unlock()
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.ctx = ctx
	s.cancel = cancel

	cmd := exec.CommandContext(ctx, s.Config.Command, s.Config.Args...)
	envVars := make([]string, 0, len(s.Config.Env))
	for k, v := range s.Config.Env {
		envVars = append(envVars, fmt.Sprintf("%s=%v", k, v))
	}
	cmd.Env = append(os.Environ(), append(cmd.Env, envVars...)...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		s.mu.Unlock()
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.mu.Unlock()
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		s.mu.Unlock()
		return err
	}

	s.cmd = cmd
	s.stdin = stdin
	s.stdout = stdout
	s.stderr = stderr

	s.mu.Unlock()

	if err := cmd.Start(); err != nil {
		return err
	}

	s.wg.Add(1)
	go s.monitorProcess()

	return nil
}

// refreshToolsAndResources fetches the list of tools and resources from the MCP server.
func (s *MCPServer) refreshToolsAndResources() error {
	var toolInfos []ToolInfo
	var resourceInfos []ResourceInfo
	var err error

	if s.Config.Command != "" {
		// stdio-based MCP server: send request to get tools and resources
		toolInfos, resourceInfos, err = s.fetchToolsAndResourcesStdio()
		if err != nil {
			return err
		}
	} else if s.Config.Address != "" {
		// HTTP/SSE MCP server: send HTTP requests to get tools and resources
		toolInfos, resourceInfos, err = s.fetchToolsAndResourcesHTTP()
	} else {
		return errors.New("mcp server config must have either address or command")
	}

	if err != nil {
		return err
	}

	var allowedTools []ToolInfo
	var restrictedTools []ToolInfo
	for _, tool := range toolInfos {
		if len(s.Config.AllowedTools) == 0 || slices.Contains(s.Config.AllowedTools, tool.Name) {
			allowedTools = append(allowedTools, tool)
		} else {
			restrictedTools = append(restrictedTools, tool)
		}
	}

	var allowedResources []ResourceInfo
	var restrictedResources []ResourceInfo
	for _, resource := range resourceInfos {
		if len(s.Config.AllowedResources) == 0 || slices.Contains(s.Config.AllowedResources, resource.Name) {
			allowedResources = append(allowedResources, resource)
		} else {
			restrictedResources = append(restrictedResources, resource)
		}
	}

	// Assign allowed ToolInfo and ResourceInfo slices to MCPServer fields
	s.tools = allowedTools
	s.restrictedTools = restrictedTools
	s.resources = allowedResources
	s.restrictedResources = restrictedResources
	return nil
}

// startPeriodicRefresh starts a goroutine that refreshes tools and resources every 15 minutes.
func (s *MCPServer) startPeriodicRefresh() {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if err := s.refreshToolsAndResources(); err != nil {
				log.Printf("Error refreshing tools/resources for MCP server %s: %v", s.Config.Name, err)
			}
		}
	}
}

// fetchToolsAndResourcesHTTP fetches tools and resources from HTTP/SSE MCP server.
//
// The /tools endpoint is expected to return JSON with an array of ToolInfo objects:
//
//	{
//	  "tools": [
//	    {
//	      "name": "tool1",
//	      "description": "Description of tool1",
//	      "inputSchema": {...},
//	      "annotations": {...}
//	    },
//	    ...
//	  ]
//	}
//
// The /resources endpoint is expected to return JSON with an array of ResourceInfo objects:
//
//	{
//	  "resources": [
//	    {
//	      "uri": "resource-uri",
//	      "uriTemplate": "template",
//	      "name": "resourceName",
//	      "description": "Description of resource",
//	      "mimeType": "application/json"
//	    },
//	    ...
//	  ]
//	}
//
// This function supports backward compatibility with legacy responses where tools and resources
// are arrays of strings. In such cases, a warning is logged and the strings are converted to
// ToolInfo and ResourceInfo with only the Name field populated.
func (s *MCPServer) fetchToolsAndResourcesHTTP() ([]ToolInfo, []ResourceInfo, error) {
	toolsURL := fmt.Sprintf("%s/tools", s.Config.Address)
	resourcesURL := fmt.Sprintf("%s/resources", s.Config.Address)

	toolsResp, err := s.httpClient.Get(toolsURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get tools: %w", err)
	}
	defer toolsResp.Body.Close()

	if toolsResp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("tools endpoint returned status %d", toolsResp.StatusCode)
	}

	// Decode full ToolInfo array response
	var toolsDataFull struct {
		Tools []ToolInfo `json:"tools"`
	}
	err = json.NewDecoder(toolsResp.Body).Decode(&toolsDataFull)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode tools response: %w", err)
	}

	resourcesResp, err := s.httpClient.Get(resourcesURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get resources: %w", err)
	}
	defer resourcesResp.Body.Close()

	if resourcesResp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("resources endpoint returned status %d", resourcesResp.StatusCode)
	}

	// Decode full ResourceInfo array response
	var resourcesDataFull struct {
		Resources []ResourceInfo `json:"resources"`
	}
	err = json.NewDecoder(resourcesResp.Body).Decode(&resourcesDataFull)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode resources response: %w", err)
	}

	return toolsDataFull.Tools, resourcesDataFull.Resources, nil
}

type stdioToolsAndResourceInfo struct {
	Result struct {
		Tools      []ToolInfo     `json:"tools,omitempty"`
		Resources  []ResourceInfo `json:"resources,omitempty"`
		NextCursor string         `json:"nextCursor,omitempty"`
	} `json:"result"`
	Error interface{} `json:"error"`
}

// fetchToolsAndResourcesStdio fetches tools and resources from stdio MCP server.
func (s *MCPServer) fetchToolsAndResourcesStdio() ([]ToolInfo, []ResourceInfo, error) {
	// Define a helper function to send a request and parse response
	sendRequest := func(method string) ([]stdioToolsAndResourceInfo, error) {
		var allItems []stdioToolsAndResourceInfo
		cursor := ""
		for {
			params := map[string]interface{}{}
			if cursor != "" {
				params["cursor"] = cursor
			}
			req := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  method,
				"params":  params,
			}
			reqBytes, err := json.Marshal(req)
			if err != nil {
				return allItems, err
			}

			respBytes, err := s.HandleStdioRequest(reqBytes)
			if err != nil {
				log.Printf("Failed to handle MCP server request: %s", string(respBytes))
				return allItems, err
			}

			var resp stdioToolsAndResourceInfo
			if err := json.Unmarshal(respBytes, &resp); err != nil {
				log.Printf("Failed to unmarshal MCP server response: %s", string(respBytes))
				return allItems, err
			}

			if resp.Error != nil {
				// If error message is "Method not found", do not return an error
				if errMap, ok := resp.Error.(map[string]interface{}); ok {
					if msg, ok := errMap["message"].(string); ok && msg == "Method not found" {
						return allItems, nil
					}
				}

				return allItems, fmt.Errorf("error response: %v %s", resp.Error, string(respBytes))
			}

			allItems = append(allItems, resp)
			if resp.Result.NextCursor == "" {
				break
			}
			cursor = resp.Result.NextCursor
		}
		return allItems, nil
	}

	var tools []ToolInfo
	toolResp, toolErr := sendRequest("tools/list")
	if toolErr != nil {
		fmt.Printf("failed to fetch tools: %v", toolErr)
	} else {
		for _, tr := range toolResp {
			tools = append(tools, tr.Result.Tools...)
		}
	}

	var resources []ResourceInfo
	resourceResp, resourceErr := sendRequest("resources/list")
	if resourceErr != nil {
		fmt.Printf("failed to fetch resources: %v", resourceErr)
	} else {
		for _, rr := range resourceResp {
			resources = append(resources, rr.Result.Resources...)
		}
	}

	var err error
	if toolErr != nil {
		err = fmt.Errorf("failed to fetch tools for server %s: %w", s.Config.Name, toolErr)
	} else if resourceErr != nil {
		err = fmt.Errorf("failed to fetch resources for server %s: %w", s.Config.Name, toolErr)
	}

	return tools, resources, err
}

// monitorProcess monitors the stdio MCP server process and restarts it if it exits unexpectedly.
func (s *MCPServer) monitorProcess() {
	defer s.wg.Done()

	stderrScanner := bufio.NewScanner(s.stderr)
	go func() {
		for stderrScanner.Scan() {
			log.Printf("MCP server %s stderr: %s", s.Config.Name, stderrScanner.Text())
		}
	}()

	err := s.cmd.Wait()
	if err != nil {
		log.Printf("MCP server %s exited with error: %v", s.Config.Name, err)
	} else {
		log.Printf("MCP server %s exited", s.Config.Name)
	}

	s.mu.Lock()

	if s.restarting {
		s.mu.Unlock()
		return
	}

	// Check if context is done (shutdown)
	select {
	case <-s.ctx.Done():
		// Context canceled, do not restart
		s.mu.Unlock()
		return
	default:
	}

	s.restarting = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.restarting = false
		s.mu.Unlock()
	}()

	// Backoff delay before restart to avoid rapid restart loops
	backoff := 3 * time.Second
	log.Printf("Waiting %v before restarting MCP server %s", backoff, s.Config.Name)
	time.Sleep(backoff)

	// Restart the process
	if err := s.startStdioProcess(); err != nil {
		log.Printf("Failed to restart MCP server %s: %v", s.Config.Name, err)
	}
}

// Shutdown gracefully shuts down the MCP server process.
func (s *MCPServer) Shutdown() error {
	if s.cancel != nil {
		s.cancel()
	}

	// Give process some time to exit gracefully
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Process exited gracefully
	case <-time.After(5 * time.Second):
		// Timeout, kill the process forcefully
		s.mu.Lock()
		if s.cmd != nil && s.cmd.Process != nil {
			log.Printf("Force killing MCP server %s", s.Config.Name)
			s.cmd.Process.Kill()
		}
		s.mu.Unlock()
	}

	// Close pipes
	s.mu.Lock()
	if s.stdin != nil {
		s.stdin.Close()
	}
	if s.stdout != nil {
		s.stdout.Close()
	}
	if s.stderr != nil {
		s.stderr.Close()
	}
	s.mu.Unlock()

	return nil
}

// IsToolAllowed checks if a tool is allowed for this MCP server.
func (s *MCPServer) IsToolAllowed(toolName string) bool {
	if len(s.Config.AllowedTools) == 0 {
		return true
	}
	for _, t := range s.Config.AllowedTools {
		if t == toolName {
			return true
		}
	}
	return false
}

// IsResourceAllowed checks if a resource is allowed for this MCP server.
func (s *MCPServer) IsResourceAllowed(resourceName string) bool {
	if len(s.Config.AllowedResources) == 0 {
		return true
	}
	for _, r := range s.Config.AllowedResources {
		if r == resourceName {
			return true
		}
	}
	return false
}

// HandleStdioRequest sends the serialized request to the stdio MCP server and reads the response.
func (s *MCPServer) HandleStdioRequest(reqBytes []byte) ([]byte, error) {
	if s.HandleStdioRequestFunc != nil {
		return s.HandleStdioRequestFunc(reqBytes)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Write request followed by newline
	_, err := s.stdin.Write(append(reqBytes, '\n'))
	if err != nil {
		return nil, err
	}

	reader := bufio.NewReader(s.stdout)

	// Read response line
	respBytes, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}

	return respBytes, nil
}
