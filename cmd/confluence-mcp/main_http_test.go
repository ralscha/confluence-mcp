package main

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"confluence-mcp/internal/config"
	"confluence-mcp/internal/confluence"
)

func TestRunHTTP(t *testing.T) {
	cfg := &config.Config{
		ConfluenceBaseURL:  "https://test.atlassian.net",
		ConfluenceEmail:    "test@example.com",
		ConfluenceAPIToken: "test-token",
		Mode:               config.ModeReadOnly,
		Transport:          config.TransportHTTP,
		HTTPAddr:           ":0",
	}

	client, err := confluence.NewClient(cfg.ConfluenceBaseURL, cfg.ConfluenceEmail, cfg.ConfluenceAPIToken, &http.Client{
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0.0.0"}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := runHTTP(ctx, cfg, server); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("runHTTP failed: %v", err)
	}

	_ = client
}
