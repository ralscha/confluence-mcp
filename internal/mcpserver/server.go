package mcpserver

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"confluence-mcp/internal/config"
	"confluence-mcp/internal/confluence"
	"confluence-mcp/internal/version"
)

// NewServer builds an MCP server exposing Confluence tools backed by client. Read
// tools are always registered; write tools are only registered when
// cfg.IsReadWrite() is true.
func NewServer(cfg *config.Config, client *confluence.Client) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "confluence-mcp",
		Version: version.Version,
	}, &mcp.ServerOptions{
		Instructions: "For article edits, first call confluence_get_page (storage by default, or atlas_doc_format). Edit raw_content and preserve existing tables, macros, links, and attachments. The content field is a lossy plain text summary. Updates replace the complete body and require the current version, not the next one. Read the page back after writing. On a conflict, retrieve the latest page and merge edits before retrying. Write tools are available only in readwrite mode.",
	})

	registerReadTools(s, client)
	if cfg.IsReadWrite() {
		registerWriteTools(s, client)
	}

	return s
}
