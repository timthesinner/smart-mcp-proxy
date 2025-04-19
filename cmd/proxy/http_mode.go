package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"smart-mcp-proxy/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HTTPProxy implements the Proxy interface for HTTP transport
type HTTPProxy struct {
	ps  *ProxyServer
	srv *http.Server
}

func NewHTTPProxy(cfg *config.Config) (*HTTPProxy, error) {
	servers, err := config.NewMCPServers(cfg)
	if err != nil {
		return nil, err
	}

	ps := &ProxyServer{
		engine:     gin.Default(),
		mcpServers: servers,
	}

	requestCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mcp_proxy_requests_total",
			Help: "Total number of requests received",
		},
		[]string{"method", "endpoint", "status"},
	)
	requestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mcp_proxy_request_duration_seconds",
			Help:    "Histogram of request durations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	prometheus.MustRegister(requestCounter, requestDuration)

	// Add logging middleware
	ps.engine.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)

		// Log request details
		log.Printf("%s %s %d %s", c.Request.Method, c.Request.URL.Path, c.Writer.Status(), duration)

		// Update Prometheus metrics
		statusCode := fmt.Sprintf("%d", c.Writer.Status())
		requestCounter.WithLabelValues(c.Request.Method, c.FullPath(), statusCode).Inc()
		requestDuration.WithLabelValues(c.Request.Method, c.FullPath()).Observe(duration.Seconds())
	})

	// Add /metrics endpoint
	ps.engine.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Setup routes
	ps.engine.GET("/tools", ps.handleTools)
	ps.engine.GET("/restricted-tools", ps.handleRestrictedTools)
	ps.engine.GET("/resources", ps.handleResources)
	ps.engine.GET("/restricted-resources", ps.handleRestrictedResources)
	ps.engine.Any("/tool/:toolName/*proxyPath", ps.handleToolProxy)
	ps.engine.Any("/resource/:resourceName/*proxyPath", ps.handleResourceProxy)

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      ps.engine,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	return &HTTPProxy{
		ps:  ps,
		srv: srv,
	}, nil
}

func (h *HTTPProxy) Run() error {
	log.Println("Starting MCP Proxy Server on :8080")
	done := make(chan struct{})
	go func() {
		if err := h.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
		close(done)
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("\nShutting down MCP Proxy Server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := h.srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	h.ps.Shutdown()

	log.Println("MCP Proxy Server has been shut down gracefully")
	<-done
	return nil
}

func (h *HTTPProxy) Shutdown(ctx context.Context) error {
	return h.srv.Shutdown(ctx)
}
