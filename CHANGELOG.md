# Changelog

## [1.2.2]

### Added

- Original `raw_content` and `body_format` on individual page/comment reads,
  defaulting to storage format, so rich articles can be edited without losing
  tables, macros, links, or attachment references.
- Created page versions and MCP instructions for reading, editing, and verifying
  articles, with guidance for recovering from version conflicts.
- Article lifecycle regression tests over stdio and streamable HTTP.

### Fixed

- Add page labels through the supported v1 content-label endpoint.
- Preserve paragraph breaks in plain text with Windows or CR line endings.
- Validate XHTML syntax, ADF document roots, and required page write fields
  before sending requests. Reject unsupported title-only version options
  instead of silently ignoring them; explicit empty content clears the body.
- Preserve escaped URL path segments and accept base URLs ending in `/wiki`.
- Reject empty/null JSON responses and retain API error details with string
  status codes.
- Encode multipart filenames correctly and validate attachment MIME headers.
- Keep environment API tokens out of CLI help and report help as a successful
  exit. Reject unexpected positional arguments.

### Changed

- Mark page and comment body replacement as destructive in MCP annotations.
- Give default HTTP clients a 30-second timeout and isolate test environment
  changes using cleanup-aware test helpers.

## [1.2.1]

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

## [1.2.0]

### Changed

- Run the streamable HTTP transport in stateless mode.

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
