// Package config loads and validates confluence-mcp server configuration from
// environment variables and command-line flags.
package config

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
)

// ErrVersionRequested is returned by Load when --version was passed. Callers
// should print the version and exit successfully.
var ErrVersionRequested = errors.New("config: version requested")

// DefaultHTTPAddr binds to the loopback interface only. The HTTP transport has
// no built-in authentication, so it must be explicitly opted into a wider bind
// address.
const DefaultHTTPAddr = "127.0.0.1:8080"

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

	// AllowedOrigins lists browser origins permitted to call the HTTP
	// transport. Loopback origins are always allowed; requests without an
	// Origin header (i.e. non-browser clients) are not affected.
	AllowedOrigins []string
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
//   - MCP_ALLOWED_ORIGINS (comma-separated)
//
// Load returns ErrVersionRequested if --version was passed.
func Load(args []string) (*Config, error) {
	cfg := &Config{
		ConfluenceBaseURL:  os.Getenv("CONFLUENCE_BASE_URL"),
		ConfluenceEmail:    os.Getenv("CONFLUENCE_EMAIL"),
		ConfluenceAPIToken: os.Getenv("CONFLUENCE_API_TOKEN"),
		Mode:               ModeReadOnly,
		Transport:          TransportStdio,
		HTTPAddr:           DefaultHTTPAddr,
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
	allowedOrigins := fs.String("allowed-origins", os.Getenv("MCP_ALLOWED_ORIGINS"), "Comma-separated browser origins allowed to call the HTTP transport (loopback origins are always allowed)")
	showVersion := fs.Bool("version", false, "Print the confluence-mcp version and exit")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if *showVersion {
		return nil, ErrVersionRequested
	}

	cfg.ConfluenceBaseURL = *baseURL
	cfg.ConfluenceEmail = *email
	cfg.ConfluenceAPIToken = *token
	cfg.Mode = Mode(*mode)
	cfg.Transport = Transport(*transport)
	cfg.HTTPAddr = *httpAddr
	cfg.AllowedOrigins = splitOrigins(*allowedOrigins)

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// splitOrigins parses a comma-separated origin list, dropping empty entries.
func splitOrigins(raw string) []string {
	var origins []string
	for origin := range strings.SplitSeq(raw, ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}

func (c *Config) validate() error {
	var errs []string

	if c.ConfluenceBaseURL == "" {
		errs = append(errs, "CONFLUENCE_BASE_URL (or --confluence-base-url) is required")
	} else if u, err := url.Parse(c.ConfluenceBaseURL); err != nil {
		errs = append(errs, fmt.Sprintf("CONFLUENCE_BASE_URL is not a valid URL: %v", err))
	} else if u.Scheme != "https" || u.Host == "" {
		errs = append(errs, "CONFLUENCE_BASE_URL must be an absolute https URL")
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

	if c.Transport == TransportHTTP {
		if _, _, err := net.SplitHostPort(c.HTTPAddr); err != nil {
			errs = append(errs, fmt.Sprintf("MCP_HTTP_ADDR is not a valid host:port address: %v", err))
		}
		for _, origin := range c.AllowedOrigins {
			if u, err := url.Parse(origin); err != nil || u.Scheme == "" || u.Host == "" {
				errs = append(errs, fmt.Sprintf("allowed origin %q must be an absolute URL such as https://example.com", origin))
			}
		}
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
