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
	}, nil)

	registerReadTools(s, client)
	if cfg.IsReadWrite() {
		registerWriteTools(s, client)
	}

	return s
}
