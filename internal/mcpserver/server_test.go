package mcpserver

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

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

	tools := toolNames(t, server)
	for _, name := range []string{
		"confluence_search_cql",
		"confluence_list_page_comments",
		"confluence_get_comment",
		"confluence_list_comment_children",
	} {
		if !tools[name] {
			t.Fatalf("readonly server missing tool %q", name)
		}
	}
	for _, name := range []string{
		"confluence_create_footer_comment",
		"confluence_update_footer_comment",
		"confluence_delete_footer_comment",
	} {
		if tools[name] {
			t.Fatalf("readonly server unexpectedly registered write tool %q", name)
		}
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

	tools := toolNames(t, server)
	for _, name := range []string{
		"confluence_create_footer_comment",
		"confluence_update_footer_comment",
		"confluence_delete_footer_comment",
	} {
		if !tools[name] {
			t.Fatalf("readwrite server missing tool %q", name)
		}
	}
}

func toolNames(t *testing.T, server *mcp.Server) map[string]bool {
	t.Helper()

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.0"}, nil)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect failed: %v", err)
	}
	defer serverSession.Close()

	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect failed: %v", err)
	}
	defer clientSession.Close()

	result, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools failed: %v", err)
	}

	names := make(map[string]bool, len(result.Tools))
	for _, tool := range result.Tools {
		names[tool.Name] = true
	}
	return names
}
