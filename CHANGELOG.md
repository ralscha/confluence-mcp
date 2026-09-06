# Changelog

## Unreleased

### Added

- Page-version history and attachment-metadata tools.
- Attachment and label pagination/filter parameters, plus page-list body and
  sort options.
- Page-label removal and explicit `plain_text` write support.

### Fixed

- Use the documented v2 page write-body shape and increment content versions
  correctly.
- Use supported v1 routes for attachment upload/download.
- Resolve space keys through the v2 space listing endpoint instead of passing
  them to the ID-only endpoint.
- Extract readable text from Atlas Document Format content.
- Replace the deprecated page-only children endpoint with the direct-children
  endpoint, which also returns databases, whiteboards, embeds, and folders.
- Clamp CQL export-view searches to Atlassian's lower result limit.
- Reject malformed base URLs and browser-origin allow-list entries early.

### Changed

- Mark destructive tools explicitly for MCP hosts and document API-specific
  result limits.

## [1.1.0]

### Added

- `confluence_get_page_children`, `confluence_get_page_ancestors`, and
  `confluence_get_space_pages` tools for navigating the page hierarchy.
- `--version` flag. The version is injected at build time and is also reported
  as the MCP server implementation version.
- `MCP_ALLOWED_ORIGINS` / `--allowed-origins` for the HTTP transport.

### Changed

- The HTTP transport now binds to `127.0.0.1:8080` by default instead of all
  interfaces.
- Tool `limit` parameters are clamped to the range Confluence accepts
  (1 to 250) instead of being passed through unchecked.

### Security

- Browser requests to the HTTP transport from non-loopback origins are rejected
  unless the origin is listed in `MCP_ALLOWED_ORIGINS`, guarding against DNS
  rebinding.
- API responses are capped at 4 MiB and attachment downloads are rejected above
  8 MiB, so an oversized response cannot exhaust memory.

## [1.0.1]

Initial public releases.
