# Changelog

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
