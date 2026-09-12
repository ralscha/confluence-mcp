package confluence

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func TestNewClient_RejectsInvalidBaseURL(t *testing.T) {
	for _, baseURL := range []string{"test.atlassian.net", "ftp://test.atlassian.net", "https://test.atlassian.net?tenant=other"} {
		t.Run(baseURL, func(t *testing.T) {
			if _, err := NewClient(baseURL, "test@example.com", "test-token", nil); err == nil {
				t.Fatalf("NewClient(%q) succeeded, want an error", baseURL)
			}
		})
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

	if _, err := client.GetPage(context.Background(), "123", ""); err != nil {
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

	_, err = client.GetPage(context.Background(), "12345", "")
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
		{name: "string status and detail", status: 400, body: `{"errors":[{"status":"400","title":"Invalid body","detail":"Unclosed paragraph"}]}`, message: "Invalid body: Unclosed paragraph"},
		{name: "detail only", status: 409, body: `{"errors":[{"detail":"Version conflict"}]}`, message: "Version conflict"},
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

func TestClientRequestPaths(t *testing.T) {
	for _, tt := range []struct{ base, want string }{
		{"https://example.atlassian.net", "/wiki/api/v2/pages/"},
		{"https://example.atlassian.net/wiki/", "/wiki/api/v2/pages/"},
		{"https://api.atlassian.com/ex/confluence/cloud-id/wiki", "/ex/confluence/cloud-id/wiki/api/v2/pages/"},
		{"https://example.atlassian.net/custom%20prefix", "/custom%20prefix/wiki/api/v2/pages/"},
	} {
		t.Run(tt.base, func(t *testing.T) {
			client, err := NewClient(tt.base, "email", "token", nil)
			if err != nil {
				t.Fatal(err)
			}
			id := "a/b %?世界"
			req, err := client.newRequest(context.Background(), http.MethodGet, "wiki/api/v2/pages/"+url.PathEscape(id), url.Values{"title": {"A & B"}}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := req.URL.EscapedPath(), tt.want+url.PathEscape(id); got != want {
				t.Fatalf("escaped path = %q, want %q", got, want)
			}
			if req.URL.Query().Get("title") != "A & B" {
				t.Fatalf("query corrupted: %s", req.URL.RawQuery)
			}
		})
	}
}

func TestClientRejectsEmptyJSONResponses(t *testing.T) {
	for _, body := range []string{"", " \r\n ", "null", "{}"} {
		t.Run(body, func(t *testing.T) {
			client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(w, body)
			})
			if _, err := client.CreatePage(context.Background(), CreatePageInput{SpaceID: "9", Title: "Title"}); err == nil {
				t.Fatal("empty JSON response incorrectly reported a successful create")
			}
			if _, err := client.UpdatePage(context.Background(), "123", UpdatePageInput{Title: new("Title")}); err == nil {
				t.Fatal("empty JSON response incorrectly reported a successful update")
			}
		})
	}
}

func TestClient_RejectsOversizedResponse(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id": "` + strings.Repeat("a", maxResponseBytes+1) + `"}`))
	})

	_, err := client.GetPage(context.Background(), "123", "")
	if err == nil {
		t.Fatal("expected error for oversized response, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("error = %q, want it to mention the size limit", err)
	}
}
