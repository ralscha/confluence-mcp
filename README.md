# confluence-mcp

A [Model Context Protocol](https://modelcontextprotocol.io/) server that exposes
[Confluence Cloud](https://www.atlassian.com/software/confluence) tools to MCP clients (such
as Claude, VS Code, or any MCP-compatible host). Built with the official
[`github.com/modelcontextprotocol/go-sdk`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp).

Supports **stdio** and **streamable HTTP** transports, and a **readonly** /
**readwrite** mode switch so you can control whether write operations are
exposed.

## Installation

Download the latest release for your platform from the [Releases](https://github.com/ralscha/confluence-mcp/releases) page.

## Quick start

```bash
# Set required environment variables
export CONFLUENCE_BASE_URL=https://your-domain.atlassian.net
export CONFLUENCE_EMAIL=you@example.com
export CONFLUENCE_API_TOKEN=your-api-token

# Run in readonly mode over stdio (safe for exploration)
go run ./cmd/confluence-mcp --mode=readonly --transport=stdio
```

Generate an API token at [Atlassian account settings](https://id.atlassian.com/manage-profile/security/api-tokens).

## Configuration

All settings can be provided via **environment variables** or **CLI flags**.
Flags take precedence over environment variables.

| Environment variable       | CLI flag                   | Default      | Description                                          |
| -------------------------- | -------------------------- | ------------ | ---------------------------------------------------- |
| `CONFLUENCE_BASE_URL`      | `--confluence-base-url`    | *(required)* | Confluence Cloud base URL, e.g. `https://your-domain.atlassian.net` |
| `CONFLUENCE_EMAIL`         | `--confluence-email`       | *(required)* | Confluence account email (used for Basic auth)       |
| `CONFLUENCE_API_TOKEN`     | `--confluence-api-token`   | *(required)* | Confluence API token                                 |
| `CONFLUENCE_MODE`          | `--mode`                   | `readonly`   | `readonly` or `readwrite`                            |
| `MCP_TRANSPORT`            | `--transport`              | `stdio`      | `stdio` or `http`                                    |
| `MCP_HTTP_ADDR`            | `--http-addr`              | `:8080`      | Listen address when `--transport=http`               |

## Tools

### Read-only tools (always available)

| Tool                                | Description                                                |
| ----------------------------------- | ---------------------------------------------------------- |
| `confluence_get_page`               | Get a single Confluence page by ID                         |
| `confluence_search_pages`           | Search Confluence pages with filters and pagination        |
| `confluence_get_space`              | Get a single Confluence space by key or ID                 |
| `confluence_list_spaces`            | List Confluence spaces with filters and pagination         |
| `confluence_get_page_labels`        | Get labels attached to a Confluence page                   |
| `confluence_get_page_attachments`   | Get attachments on a Confluence page                       |
| `confluence_download_attachment`    | Download a Confluence attachment's content (base64-encoded)|

### Write tools (only in `readwrite` mode)

| Tool                            | Description                                    |
| ------------------------------- | ---------------------------------------------- |
| `confluence_create_page`        | Create a new Confluence page                   |
| `confluence_update_page`        | Update title and/or content of a page          |
| `confluence_delete_page`        | Delete a Confluence page                       |
| `confluence_add_page_label`     | Add a label to a Confluence page               |
| `confluence_upload_attachment`  | Upload a file attachment to a page             |
| `confluence_delete_attachment`  | Delete a Confluence attachment                 |

The default mode is `readonly`. Set `CONFLUENCE_MODE=readwrite` (or
`--mode=readwrite`) explicitly to enable write tools.

Page content uses [Confluence storage format](https://confluence.atlassian.com/doc/confluence-storage-format-790796544.html)
(XHTML) or Atlas Document Format (ADF). The server provides basic plain text
conversion helpers, so you can work with plain strings without handling markup
directly. Rich formatting (tables, macros, etc.) is not preserved through these
conversions.

## Transports

### stdio

The default transport. The server communicates over stdin/stdout using
newline-delimited JSON (the standard MCP transport for subprocess-based tools).

```bash
confluence-mcp --transport=stdio
```

### HTTP (streamable)

The server exposes a [streamable HTTP](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports)
endpoint on the configured address.

```bash
confluence-mcp --transport=http --http-addr=:8080
```

The HTTP transport has **no built-in authentication**. When running in
`readwrite` mode, secure it at the network or deployment layer (reverse proxy,
firewall, loopback-only binding) to prevent unauthorized page modifications.

## Authentication

The server authenticates to Confluence Cloud using [HTTP Basic auth](https://developer.atlassian.com/cloud/confluence/basic-auth-for-rest-apis/)
with your Confluence account email as the username and an [API token](https://id.atlassian.com/manage-profile/security/api-tokens)
as the password.

### Required token permissions

Atlassian API tokens do not grant more access than the Atlassian account has.
Use a dedicated account with the smallest Confluence space permissions needed
for the tools you expose.

You can use either:

- A classic/unscoped API token with `CONFLUENCE_BASE_URL` set to your site URL,
  e.g. `https://your-domain.atlassian.net`.
- A scoped API token. Scoped tokens must call the Atlassian API gateway, e.g.
  `CONFLUENCE_BASE_URL=https://api.atlassian.com/ex/confluence/{cloudId}`.

For scoped tokens, grant these Confluence scopes:

| Mode | Token scopes | Confluence permissions the account still needs |
| ---- | ------------ | ---------------------------------------------- |
| `readonly` | `read:page:confluence`, `read:space:confluence`, `read:attachment:confluence` | Confluence product access (`Can use`) and view permission for the spaces/pages/attachments to read. Page restrictions still apply. |
| `readwrite` | `read:page:confluence`, `read:space:confluence`, `read:attachment:confluence`, `write:page:confluence`, `write:label:confluence`, `write:attachment:confluence`, `delete:page:confluence`, `delete:attachment:confluence` | The readonly permissions, plus only the space permissions required by the write tools you use: add/update/delete pages, add labels, add attachments, and/or delete attachments. |

`confluence-mcp` does not need Confluence admin scopes or space-management
scopes because it does not create spaces or change space settings.

## Development

### Requirements

- Go 1.26+

### Build

```bash
go build ./...
```

### Test

```bash
go test ./...
```

Tests cover:

- Config loading, validation, and flag/env precedence
- Confluence REST API client against `httptest.Server` mocks for every endpoint
  (success, error mapping, multipart attachment uploads)
- Mode-gated tool registration (`readonly` excludes write tools)
- End-to-end smoke tests that spawn the real binary and drive it via the
  official SDK client over both stdio and HTTP transports

## MCP client configuration

### Claude Desktop (stdio)

```json
{
  "mcpServers": {
    "confluence": {
      "command": "confluence-mcp",
      "args": [],
      "env": {
        "CONFLUENCE_BASE_URL": "https://your-domain.atlassian.net",
        "CONFLUENCE_EMAIL": "you@example.com",
        "CONFLUENCE_API_TOKEN": "your-api-token",
        "CONFLUENCE_MODE": "readonly"
      }
    }
  }
}
```

### VS Code / GitHub Copilot (stdio)

Add to `.vscode/mcp.json` (or your user-level `mcp.json`):

```json
{
  "servers": {
    "confluence": {
      "command": "confluence-mcp",
      "args": [],
      "env": {
        "CONFLUENCE_BASE_URL": "https://your-domain.atlassian.net",
        "CONFLUENCE_EMAIL": "you@example.com",
        "CONFLUENCE_API_TOKEN": "your-api-token",
        "CONFLUENCE_MODE": "readonly"
      }
    }
  }
}
```

## API Coverage

This server uses the [Confluence Cloud REST API v2](https://developer.atlassian.com/cloud/confluence/rest/v2/intro/).
It covers a core subset of functionality focused on pages, spaces, labels, and
attachments.

Not currently supported:
- Blog posts
- Comments
- Content properties
- Advanced search with CQL (Confluence Query Language)
- Space permissions
- User management

## License

MIT

## Related Projects

- [jira-mcp](../jira-mcp) - MCP server for Jira Cloud
- [Model Context Protocol](https://modelcontextprotocol.io/)
- [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)
