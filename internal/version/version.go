// Package version exposes the build version of the confluence-mcp binary.
package version

// Version is the confluence-mcp release version. The default value is used for
// local builds; release builds override it at link time via
// -ldflags "-X confluence-mcp/internal/version.Version=<version>".
var Version = "dev"
