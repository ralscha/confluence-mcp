// Package confluence provides a minimal client for the Confluence Cloud REST API v2,
// covering the subset of endpoints needed by the confluence-mcp server.
package confluence

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
)

// Client is a Confluence Cloud REST API v2 client authenticated via Basic auth
// using an account email and API token.
type Client struct {
	httpClient *http.Client
	baseURL    *url.URL
	email      string
	token      string
}

// NewClient creates a Client for the given Confluence Cloud base URL (e.g.
// "https://your-domain.atlassian.net"). If httpClient is nil,
// http.DefaultClient is used.
func NewClient(baseURL, email, token string, httpClient *http.Client) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("confluence: invalid base URL: %w", err)
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		httpClient: httpClient,
		baseURL:    u,
		email:      email,
		token:      token,
	}, nil
}

// APIError represents a non-2xx response from the Confluence REST API.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("confluence: request failed with status %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("confluence: request failed with status %d", e.StatusCode)
}

// doJSON sends a request with an optional JSON-encoded body and decodes a
// JSON response into out (if non-nil). path is resolved relative to the
// client's base URL.
func (c *Client) doJSON(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("confluence: encoding request body: %w", err)
		}
		reader = bytes.NewReader(b)
	}

	req, err := c.newRequest(ctx, method, path, query, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("confluence: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("confluence: reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parseAPIError(resp.StatusCode, respBody)
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("confluence: decoding response body: %w", err)
		}
	}

	return nil
}

// newRequest builds an HTTP request with Basic auth.
func (c *Client) newRequest(ctx context.Context, method, path string, query url.Values, body io.Reader) (*http.Request, error) {
	u := *c.baseURL
	u.Path = strings.TrimSuffix(u.Path, "/") + "/" + strings.TrimPrefix(path, "/")
	if query != nil {
		u.RawQuery = query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("confluence: creating request: %w", err)
	}

	req.SetBasicAuth(c.email, c.token)
	return req, nil
}

// parseAPIError attempts to extract an error message from an API response body.
func parseAPIError(statusCode int, body []byte) error {
	var errResp struct {
		Message string `json:"message"`
		Errors  []struct {
			Status int    `json:"status"`
			Title  string `json:"title"`
		} `json:"errors"`
	}

	if len(body) > 0 {
		if err := json.Unmarshal(body, &errResp); err == nil {
			if errResp.Message != "" {
				return &APIError{StatusCode: statusCode, Message: errResp.Message}
			}
			if len(errResp.Errors) > 0 {
				return &APIError{StatusCode: statusCode, Message: errResp.Errors[0].Title}
			}
		}
	}

	return &APIError{StatusCode: statusCode}
}

// doMultipart sends a multipart/form-data request with file data.
func (c *Client) doMultipart(ctx context.Context, method, path string, query url.Values, filename, mimeType string, data []byte, out any) error {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, filename))
	if mimeType != "" {
		h.Set("Content-Type", mimeType)
	}
	part, err := writer.CreatePart(h)
	if err != nil {
		return fmt.Errorf("confluence: creating multipart part: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return fmt.Errorf("confluence: writing multipart data: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("confluence: closing multipart writer: %w", err)
	}

	req, err := c.newRequest(ctx, method, path, query, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Atlassian-Token", "no-check")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("confluence: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("confluence: reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parseAPIError(resp.StatusCode, respBody)
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("confluence: decoding response body: %w", err)
		}
	}

	return nil
}
