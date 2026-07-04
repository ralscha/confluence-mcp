// Package config loads and validates confluence-mcp server configuration from
// environment variables and command-line flags.
package config

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// Mode controls whether write tools are registered on the MCP server.
type Mode string

const (
	ModeReadOnly  Mode = "readonly"
	ModeReadWrite Mode = "readwrite"
)

// Transport selects how the MCP server communicates with clients.
type Transport string

const (
	TransportStdio Transport = "stdio"
	TransportHTTP  Transport = "http"
)

// Config holds all settings needed to run the confluence-mcp server.
type Config struct {
	ConfluenceBaseURL  string
	ConfluenceEmail    string
	ConfluenceAPIToken string

	Mode      Mode
	Transport Transport
	HTTPAddr  string
}

// Load builds a Config from environment variables, then applies overrides
// from the given command-line arguments (excluding the program name).
//
// Environment variables:
//   - CONFLUENCE_BASE_URL
//   - CONFLUENCE_EMAIL
//   - CONFLUENCE_API_TOKEN
//   - CONFLUENCE_MODE (readonly|readwrite)
//   - MCP_TRANSPORT (stdio|http)
//   - MCP_HTTP_ADDR
func Load(args []string) (*Config, error) {
	cfg := &Config{
		ConfluenceBaseURL:  os.Getenv("CONFLUENCE_BASE_URL"),
		ConfluenceEmail:    os.Getenv("CONFLUENCE_EMAIL"),
		ConfluenceAPIToken: os.Getenv("CONFLUENCE_API_TOKEN"),
		Mode:               ModeReadOnly,
		Transport:          TransportStdio,
		HTTPAddr:           ":8080",
	}

	if v := os.Getenv("CONFLUENCE_MODE"); v != "" {
		cfg.Mode = Mode(v)
	}
	if v := os.Getenv("MCP_TRANSPORT"); v != "" {
		cfg.Transport = Transport(v)
	}
	if v := os.Getenv("MCP_HTTP_ADDR"); v != "" {
		cfg.HTTPAddr = v
	}

	fs := flag.NewFlagSet("confluence-mcp", flag.ContinueOnError)
	baseURL := fs.String("confluence-base-url", cfg.ConfluenceBaseURL, "Confluence Cloud base URL, e.g. https://your-domain.atlassian.net")
	email := fs.String("confluence-email", cfg.ConfluenceEmail, "Confluence account email used for API token authentication")
	token := fs.String("confluence-api-token", cfg.ConfluenceAPIToken, "Confluence API token")
	mode := fs.String("mode", string(cfg.Mode), "Server mode: readonly or readwrite")
	transport := fs.String("transport", string(cfg.Transport), "Transport: stdio or http")
	httpAddr := fs.String("http-addr", cfg.HTTPAddr, "Address to listen on when --transport=http")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	cfg.ConfluenceBaseURL = *baseURL
	cfg.ConfluenceEmail = *email
	cfg.ConfluenceAPIToken = *token
	cfg.Mode = Mode(*mode)
	cfg.Transport = Transport(*transport)
	cfg.HTTPAddr = *httpAddr

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	var errs []string

	if c.ConfluenceBaseURL == "" {
		errs = append(errs, "CONFLUENCE_BASE_URL (or --confluence-base-url) is required")
	} else if _, err := url.Parse(c.ConfluenceBaseURL); err != nil {
		errs = append(errs, fmt.Sprintf("CONFLUENCE_BASE_URL is not a valid URL: %v", err))
	}

	if c.ConfluenceEmail == "" {
		errs = append(errs, "CONFLUENCE_EMAIL (or --confluence-email) is required")
	}

	if c.ConfluenceAPIToken == "" {
		errs = append(errs, "CONFLUENCE_API_TOKEN (or --confluence-api-token) is required")
	}

	if c.Mode != ModeReadOnly && c.Mode != ModeReadWrite {
		errs = append(errs, fmt.Sprintf("mode must be 'readonly' or 'readwrite', got '%s'", c.Mode))
	}

	if c.Transport != TransportStdio && c.Transport != TransportHTTP {
		errs = append(errs, fmt.Sprintf("transport must be 'stdio' or 'http', got '%s'", c.Transport))
	}

	if len(errs) > 0 {
		return fmt.Errorf("configuration errors:\n  %s", strings.Join(errs, "\n  "))
	}

	return nil
}

// IsReadWrite returns true if write tools should be registered.
func (c *Config) IsReadWrite() bool {
	return c.Mode == ModeReadWrite
}
