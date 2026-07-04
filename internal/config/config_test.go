package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoad_ValidConfig(t *testing.T) {
	os.Clearenv()
	_ = os.Setenv("CONFLUENCE_BASE_URL", "https://test.atlassian.net")
	_ = os.Setenv("CONFLUENCE_EMAIL", "test@example.com")
	_ = os.Setenv("CONFLUENCE_API_TOKEN", "test-token")

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
}

func TestLoad_MissingRequired(t *testing.T) {
	os.Clearenv()

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
	os.Clearenv()

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
	os.Clearenv()

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
	os.Clearenv()

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
	os.Clearenv()

	_, err := Load([]string{
		"--confluence-base-url=http://test.atlassian.net",
		"--confluence-email=test@example.com",
		"--confluence-api-token=test-token",
	})
	if err == nil {
		t.Fatal("expected error with insecure base URL, got nil")
	}
}
