package confluence

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestClient starts an httptest server running handler and returns a Client
// pointed at it.
func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := NewClient(server.URL, "test@example.com", "test-token", server.Client())
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	return client
}

// writeJSON writes a JSON response body with the appropriate content type.
func writeJSON(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(body))
}

func TestNewClient(t *testing.T) {
	client, err := NewClient("https://test.atlassian.net", "test@example.com", "test-token", nil)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if client.email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", client.email)
	}
}

func TestClient_BasicAuth(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		email, token, ok := r.BasicAuth()
		if !ok {
			t.Error("expected basic auth credentials")
		}
		if got, want := email, "test@example.com"; got != want {
			t.Errorf("email = %q, want %q", got, want)
		}
		if got, want := token, "test-token"; got != want {
			t.Errorf("token = %q, want %q", got, want)
		}
		writeJSON(w, `{"id": "123"}`)
	})

	if _, err := client.GetPage(context.Background(), "123", nil); err != nil {
		t.Fatalf("GetPage failed: %v", err)
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

func TestParseAPIError(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		message string
	}{
		{name: "message field", status: 404, body: `{"message": "Page not found"}`, message: "Page not found"},
		{name: "errors array", status: 403, body: `{"errors": [{"status": 403, "title": "Not permitted"}]}`, message: "Not permitted"},
		{name: "empty body", status: 500, body: ``, message: ""},
		{name: "unparseable body", status: 502, body: `<html>bad gateway</html>`, message: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parseAPIError(tt.status, []byte(tt.body))

			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected APIError, got %T", err)
			}
			if got, want := apiErr.StatusCode, tt.status; got != want {
				t.Errorf("status = %d, want %d", got, want)
			}
			if got, want := apiErr.Message, tt.message; got != want {
				t.Errorf("message = %q, want %q", got, want)
			}
			if !strings.Contains(apiErr.Error(), "confluence: request failed") {
				t.Errorf("Error() = %q, want it to describe the failure", apiErr.Error())
			}
		})
	}
}

func TestClient_RejectsOversizedResponse(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id": "` + strings.Repeat("a", maxResponseBytes+1) + `"}`))
	})

	_, err := client.GetPage(context.Background(), "123", nil)
	if err == nil {
		t.Fatal("expected error for oversized response, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("error = %q, want it to mention the size limit", err)
	}
}
