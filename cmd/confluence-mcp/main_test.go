package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestRun_MissingConfig(t *testing.T) {
	os.Clearenv()
	os.Args = []string{"confluence-mcp"}
	if err := run(); err == nil {
		t.Fatal("expected error with missing config, got nil")
	}
}

func TestRun_Version(t *testing.T) {
	os.Clearenv()
	os.Args = []string{"confluence-mcp", "--version"}
	if err := run(); err != nil {
		t.Fatalf("run with --version failed: %v", err)
	}
}

func TestOriginAllowed(t *testing.T) {
	allowed := []string{"https://mcp.example.com"}

	tests := []struct {
		name   string
		origin string
		want   bool
	}{
		{name: "configured origin", origin: "https://mcp.example.com", want: true},
		{name: "localhost", origin: "http://localhost:3000", want: true},
		{name: "loopback IPv4", origin: "http://127.0.0.1:8080", want: true},
		{name: "loopback IPv6", origin: "http://[::1]:8080", want: true},
		{name: "external origin", origin: "https://evil.example.com", want: false},
		{name: "rebinding host", origin: "http://attacker.127.0.0.1.nip.io", want: false},
		{name: "unconfigured scheme mismatch", origin: "http://mcp.example.com", want: false},
		{name: "not a URL", origin: "not a url", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := originAllowed(tt.origin, allowed); got != tt.want {
				t.Errorf("originAllowed(%q) = %v, want %v", tt.origin, got, tt.want)
			}
		})
	}
}

func TestCheckOrigin(t *testing.T) {
	tests := []struct {
		name       string
		origin     string
		wantStatus int
	}{
		{name: "no origin header", origin: "", wantStatus: http.StatusOK},
		{name: "loopback origin", origin: "http://localhost:3000", wantStatus: http.StatusOK},
		{name: "foreign origin", origin: "https://evil.example.com", wantStatus: http.StatusForbidden},
	}

	handler := checkOrigin(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if got := rec.Code; got != tt.wantStatus {
				t.Errorf("status = %d, want %d", got, tt.wantStatus)
			}
		})
	}
}
