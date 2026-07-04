package mcpserver

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"confluence-mcp/internal/config"
	"confluence-mcp/internal/confluence"
)

// serverVersion is the reported version of this MCP server implementation.
const serverVersion = "0.1.0"

// NewServer builds an MCP server exposing Confluence tools backed by client. Read
// tools are always registered; write tools are only registered when
// cfg.IsReadWrite() is true.
func NewServer(cfg *config.Config, client *confluence.Client) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "confluence-mcp",
		Version: serverVersion,
	}, nil)

	registerReadTools(s, client)
	if cfg.IsReadWrite() {
		registerWriteTools(s, client)
	}

	return s
}
