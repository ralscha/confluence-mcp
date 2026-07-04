package mcpserver

import (
	"net/http"
	"testing"
	"time"

	"confluence-mcp/internal/config"
	"confluence-mcp/internal/confluence"
)

func TestNewServer_ReadOnly(t *testing.T) {
	cfg := &config.Config{
		ConfluenceBaseURL:  "https://test.atlassian.net",
		ConfluenceEmail:    "test@example.com",
		ConfluenceAPIToken: "test-token",
		Mode:               config.ModeReadOnly,
	}

	client, err := confluence.NewClient(cfg.ConfluenceBaseURL, cfg.ConfluenceEmail, cfg.ConfluenceAPIToken, &http.Client{
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	server := NewServer(cfg, client)
	if server == nil {
		t.Fatal("expected server, got nil")
	}
}

func TestNewServer_ReadWrite(t *testing.T) {
	cfg := &config.Config{
		ConfluenceBaseURL:  "https://test.atlassian.net",
		ConfluenceEmail:    "test@example.com",
		ConfluenceAPIToken: "test-token",
		Mode:               config.ModeReadWrite,
	}

	client, err := confluence.NewClient(cfg.ConfluenceBaseURL, cfg.ConfluenceEmail, cfg.ConfluenceAPIToken, &http.Client{
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	server := NewServer(cfg, client)
	if server == nil {
		t.Fatal("expected server, got nil")
	}
}
