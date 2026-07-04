package confluence

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	client, err := NewClient("https://test.atlassian.net", "test@example.com", "test-token", nil)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if client.email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", client.email)
	}
}

func TestClient_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message": "Page not found"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test@example.com", "test-token", nil)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	_, err = client.GetPage(context.Background(), "12345", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != 404 {
		t.Errorf("expected status 404, got %d", apiErr.StatusCode)
	}
}
