package config

import (
	"errors"
	"flag"
	"io"
	"os"
	"strings"
	"testing"
)

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{"CONFLUENCE_BASE_URL", "CONFLUENCE_EMAIL", "CONFLUENCE_API_TOKEN", "CONFLUENCE_MODE", "MCP_TRANSPORT", "MCP_HTTP_ADDR", "MCP_ALLOWED_ORIGINS"} {
		t.Setenv(name, "")
	}
}

func TestLoadHelpDoesNotLeakToken(t *testing.T) {
	clearConfigEnv(t)
	const secret = "secret-must-never-appear-in-usage"
	t.Setenv("CONFLUENCE_API_TOKEN", secret)
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stderr
	os.Stderr = writer
	t.Cleanup(func() {
		os.Stderr = previous
		_ = reader.Close()
		_ = writer.Close()
	})
	_, err = Load([]string{"--help"})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("help error = %v", err)
	}
	_ = writer.Close()
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(output), secret) {
		t.Fatal("help exposed the API token")
	}
}

func TestLoadTokenFlagPrecedence(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("CONFLUENCE_BASE_URL", "https://test.atlassian.net")
	t.Setenv("CONFLUENCE_EMAIL", "test@example.com")
	t.Setenv("CONFLUENCE_API_TOKEN", "environment-token")
	cfg, err := Load([]string{"--confluence-api-token=flag-token"})
	if err != nil || cfg.ConfluenceAPIToken != "flag-token" {
		t.Fatalf("flag token override failed: %v", err)
	}
	if _, err := Load([]string{"--confluence-api-token="}); err == nil {
		t.Fatal("explicit empty token flag must override the environment and fail validation")
	}
	if _, err := Load([]string{"unexpected-argument"}); err == nil {
		t.Fatal("positional argument silently ignored")
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("CONFLUENCE_BASE_URL", "https://test.atlassian.net")
	t.Setenv("CONFLUENCE_EMAIL", "test@example.com")
	t.Setenv("CONFLUENCE_API_TOKEN", "test-token")

	cfg, err := Load([]string{})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.ConfluenceBaseURL != "https://test.atlassian.net" {
		t.Errorf("expected base URL https://test.atlassian.net, got %s", cfg.ConfluenceBaseURL)
	}
	if cfg.Mode != ModeReadOnly {
		t.Errorf("expected default mode readonly, got %s", cfg.Mode)
	}
	if cfg.Transport != TransportStdio {
		t.Errorf("expected default transport stdio, got %s", cfg.Transport)
	}
	if cfg.HTTPAddr != DefaultHTTPAddr {
		t.Errorf("expected default HTTP address %s, got %s", DefaultHTTPAddr, cfg.HTTPAddr)
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	clearConfigEnv(t)

	_, err := Load([]string{})
	if err == nil {
		t.Fatal("expected error with missing required config, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "CONFLUENCE_BASE_URL") {
		t.Errorf("error should mention CONFLUENCE_BASE_URL: %v", err)
	}
}

func TestLoad_FlagOverride(t *testing.T) {
	clearConfigEnv(t)

	cfg, err := Load([]string{
		"--confluence-base-url=https://override.atlassian.net",
		"--confluence-email=override@example.com",
		"--confluence-api-token=override-token",
		"--mode=readwrite",
	})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.ConfluenceBaseURL != "https://override.atlassian.net" {
		t.Errorf("flag override failed for base URL")
	}
	if cfg.Mode != ModeReadWrite {
		t.Errorf("flag override failed for mode")
	}
	if !cfg.IsReadWrite() {
		t.Errorf("IsReadWrite should return true when mode is readwrite")
	}
}

func TestLoad_InvalidMode(t *testing.T) {
	clearConfigEnv(t)

	_, err := Load([]string{
		"--confluence-base-url=https://test.atlassian.net",
		"--confluence-email=test@example.com",
		"--confluence-api-token=test-token",
		"--mode=invalid",
	})
	if err == nil {
		t.Fatal("expected error with invalid mode, got nil")
	}
}

func TestLoad_InvalidBaseURL(t *testing.T) {
	clearConfigEnv(t)

	_, err := Load([]string{
		"--confluence-base-url=not-a-url",
		"--confluence-email=test@example.com",
		"--confluence-api-token=test-token",
	})
	if err == nil {
		t.Fatal("expected error with invalid base URL, got nil")
	}
}

func TestLoad_InsecureBaseURL(t *testing.T) {
	clearConfigEnv(t)

	_, err := Load([]string{
		"--confluence-base-url=http://test.atlassian.net",
		"--confluence-email=test@example.com",
		"--confluence-api-token=test-token",
	})
	if err == nil {
		t.Fatal("expected error with insecure base URL, got nil")
	}
}

func TestLoad_RejectsBaseURLQuery(t *testing.T) {
	clearConfigEnv(t)

	_, err := Load([]string{
		"--confluence-base-url=https://test.atlassian.net?tenant=other",
		"--confluence-email=test@example.com",
		"--confluence-api-token=test-token",
	})
	if err == nil {
		t.Fatal("expected error for a base URL with a query, got nil")
	}
}

func TestLoad_VersionRequested(t *testing.T) {
	clearConfigEnv(t)

	_, err := Load([]string{"--version"})
	if !errors.Is(err, ErrVersionRequested) {
		t.Fatalf("expected ErrVersionRequested, got %v", err)
	}
}

func TestLoad_AllowedOrigins(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("MCP_ALLOWED_ORIGINS", "https://a.example.com, ,https://b.example.com")

	cfg, err := Load([]string{
		"--confluence-base-url=https://test.atlassian.net",
		"--confluence-email=test@example.com",
		"--confluence-api-token=test-token",
		"--transport=http",
	})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	want := []string{"https://a.example.com", "https://b.example.com"}
	if len(cfg.AllowedOrigins) != len(want) {
		t.Fatalf("allowed origins = %v, want %v", cfg.AllowedOrigins, want)
	}
	for i, origin := range want {
		if cfg.AllowedOrigins[i] != origin {
			t.Errorf("allowed origin %d = %q, want %q", i, cfg.AllowedOrigins[i], origin)
		}
	}
}

func TestLoad_InvalidAllowedOrigin(t *testing.T) {
	clearConfigEnv(t)

	_, err := Load([]string{
		"--confluence-base-url=https://test.atlassian.net",
		"--confluence-email=test@example.com",
		"--confluence-api-token=test-token",
		"--transport=http",
		"--allowed-origins=example.com",
	})
	if err == nil {
		t.Fatal("expected error with a non-absolute allowed origin, got nil")
	}
	if !strings.Contains(err.Error(), "allowed origin") {
		t.Errorf("error should mention the allowed origin: %v", err)
	}
}

func TestLoad_RejectsNonOriginURL(t *testing.T) {
	clearConfigEnv(t)

	_, err := Load([]string{
		"--confluence-base-url=https://test.atlassian.net",
		"--confluence-email=test@example.com",
		"--confluence-api-token=test-token",
		"--transport=http",
		"--allowed-origins=https://example.com/path",
	})
	if err == nil {
		t.Fatal("expected error for an allowed origin with a path, got nil")
	}
}

func TestLoad_InvalidHTTPAddr(t *testing.T) {
	clearConfigEnv(t)

	_, err := Load([]string{
		"--confluence-base-url=https://test.atlassian.net",
		"--confluence-email=test@example.com",
		"--confluence-api-token=test-token",
		"--transport=http",
		"--http-addr=8080",
	})
	if err == nil {
		t.Fatal("expected error with an address that has no port, got nil")
	}
}
