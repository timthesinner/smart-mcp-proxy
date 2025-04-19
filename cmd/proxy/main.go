package main

import (
	"flag"
	"log"
	"os"

	"smart-mcp-proxy/internal/config"
)

func main() {
	// Define command-line flags
	configPathFlag := flag.String("config", "", "Path to MCP proxy config file")
	modeFlag := flag.String("mode", "", "Run mode: 'http' or 'command' (default 'http')")
	flag.Parse()

	// Determine config path from flag or environment variable
	configPath := *configPathFlag
	if configPath == "" {
		configPath = os.Getenv("MCP_PROXY_CONFIG")
	}
	if configPath == "" {
		log.Fatal("MCP_PROXY_CONFIG environment variable or -config flag must be set")
	}

	// Determine mode: Environment variable takes precedence over flag
	mode := os.Getenv("MCP_PROXY_MODE")
	if mode == "" {
		mode = *modeFlag // Use flag only if env var is not set
	}
	if mode == "" {
		mode = "command" // Default to command if both env var and flag are empty
	}

	// Load config
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Create the core ProxyServer instance first
	ps, err := NewProxyServer(cfg)
	if err != nil {
		log.Fatalf("failed to create core proxy server: %v", err)
	}

	var proxy Proxy
	switch mode {
	case "http":
		// Define listen address (could be from config or flag later)
		listenAddr := ":8080" // Default address
		proxy, err = NewHTTPProxy(ps, listenAddr)
		if err != nil {
			log.Fatalf("failed to create HTTP proxy: %v", err)
		}
	case "command":
		// Pass the ProxyServer instance to NewCommandProxy
		proxy, err = NewCommandProxy(ps) // Assuming NewCommandProxy will take *ProxyServer
		if err != nil {
			log.Fatalf("failed to create command proxy: %v", err)
		}
	default:
		log.Fatalf("invalid mode: %s, must be 'http' or 'command'", mode)
	}

	if err := proxy.Run(); err != nil {
		log.Fatalf("proxy run error: %v", err)
	}
}
