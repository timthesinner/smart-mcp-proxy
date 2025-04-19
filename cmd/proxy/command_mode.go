package main

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"smart-mcp-proxy/internal/config"

	"github.com/gin-gonic/gin"
)

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
