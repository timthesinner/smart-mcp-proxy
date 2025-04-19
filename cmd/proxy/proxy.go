package main

import "context"

// Proxy defines the interface for MCP proxy servers.
type Proxy interface {
	Run() error
	Shutdown(ctx context.Context) error
}
