package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"confluence-mcp/internal/config"
	"confluence-mcp/internal/confluence"
)

// This subprocess runs the real stdio transport without a second compilation.
// Exiting explicitly keeps the Go test runner's PASS message off the MCP stream.
func TestArticleStdioServerProcess(t *testing.T) {
	if os.Getenv("CONFLUENCE_TEST_STDIO") != "1" {
		t.Skip("stdio subprocess only")
	}
	client, err := confluence.NewClient(os.Getenv("CONFLUENCE_TEST_API"), "test@example.com", "test-token", nil)
	if err == nil {
		server := NewServer(&config.Config{Mode: config.Mode(os.Getenv("CONFLUENCE_TEST_MODE"))}, client)
		err = server.Run(context.Background(), &mcp.StdioTransport{})
	}
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func articleSession(t *testing.T, ctx context.Context, apiURL, transport string, mode config.Mode) *mcp.ClientSession {
	t.Helper()
	var clientTransport mcp.Transport
	if transport == "stdio" {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		//nolint:gosec // The executable is this test binary, obtained from os.Executable.
		cmd := exec.CommandContext(ctx, executable, "-test.run=^TestArticleStdioServerProcess$")
		cmd.Env = append(os.Environ(), "CONFLUENCE_TEST_STDIO=1", "CONFLUENCE_TEST_API="+apiURL, "CONFLUENCE_TEST_MODE="+string(mode))
		clientTransport = &mcp.CommandTransport{Command: cmd}
	} else {
		apiClient, err := confluence.NewClient(apiURL, "test@example.com", "test-token", nil)
		if err != nil {
			t.Fatal(err)
		}
		server := NewServer(&config.Config{Mode: mode}, apiClient)
		httpServer := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
			return server
		}, &mcp.StreamableHTTPOptions{Stateless: true}))
		t.Cleanup(httpServer.Close)
		clientTransport = &mcp.StreamableClientTransport{Endpoint: httpServer.URL}
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "article-test", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close MCP session: %v", err)
		}
	})
	return session
}

func TestArticleWriteValidationOverMCP(t *testing.T) {
	for _, transport := range []string{"stdio", "http"} {
		t.Run(transport, func(t *testing.T) {
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				t.Error("invalid or readonly write reached Confluence")
				w.WriteHeader(http.StatusInternalServerError)
			}))
			t.Cleanup(api.Close)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			t.Cleanup(cancel)
			session := articleSession(t, ctx, api.URL, transport, config.ModeReadWrite)
			for _, call := range []mcp.CallToolParams{
				{Name: "confluence_create_page", Arguments: map[string]any{"space_id": "9", "title": " "}},
				{Name: "confluence_create_page", Arguments: map[string]any{"space_id": "9", "title": "Invalid", "content": "<p>unclosed"}},
				{Name: "confluence_create_page", Arguments: map[string]any{"space_id": "9", "title": "Invalid", "body_type": "atlas_doc_format", "content": "null"}},
				{Name: "confluence_update_page", Arguments: map[string]any{"page_id": "123"}},
				{Name: "confluence_update_page", Arguments: map[string]any{"page_id": "123", "content": "<p>Body</p>"}},
				{Name: "confluence_update_page", Arguments: map[string]any{"page_id": "123", "title": "Renamed", "version_note": "Do not drop this"}},
			} {
				result, err := session.CallTool(ctx, &call)
				if err != nil || !result.IsError {
					t.Fatalf("invalid input was not reported as a tool error: %+v, %v", result, err)
				}
			}
			tools, err := session.ListTools(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, tool := range tools.Tools {
				if tool.Name == "confluence_update_page" || tool.Name == "confluence_update_footer_comment" {
					if tool.Annotations == nil || tool.Annotations.DestructiveHint == nil || !*tool.Annotations.DestructiveHint {
						t.Errorf("%s must advertise body replacement as destructive", tool.Name)
					}
				}
			}
			readonly := articleSession(t, ctx, api.URL, transport, config.ModeReadOnly)
			result, err := readonly.CallTool(ctx, &mcp.CallToolParams{Name: "confluence_create_page", Arguments: map[string]any{"space_id": "9", "title": "Forbidden"}})
			if err == nil && !result.IsError {
				t.Fatal("readonly server accepted a write")
			}
		})
	}
}

func articleTool[T any](t *testing.T, ctx context.Context, session *mcp.ClientSession, name string, args any) T {
	t.Helper()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if result.IsError {
		t.Fatalf("%s returned a tool error: %+v", name, result.Content)
	}
	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var out T
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode %s result: %v", name, err)
	}
	return out
}

func TestArticleLifecycleOverMCP(t *testing.T) {
	richStorage := `<h1>Guide</h1><p>Grüezi 世界 👋 &amp; welcome</p><table><tbody><tr><th>Key</th><td>Value</td></tr></tbody></table><ac:structured-macro ac:name="code"><ac:plain-text-body><![CDATA[if (a < b) { return "<&>"; }]]></ac:plain-text-body></ac:structured-macro><p><a href="https://example.com/?a=1&amp;b=2">Link</a></p><ac:image><ri:attachment ri:filename="diagram.png" /></ac:image>`
	richADF := `{"version":1,"type":"doc","content":[{"type":"heading","attrs":{"level":1},"content":[{"type":"text","text":"Guide"}]},{"type":"paragraph","content":[{"type":"text","text":"Grüezi 世界 👋","marks":[{"type":"strong"}]}]}]}`
	for _, transport := range []string{"stdio", "http"} {
		for _, tt := range []struct{ name, format, content, stored string }{
			{"storage", "storage", richStorage, richStorage},
			{"ADF", "atlas_doc_format", richADF, richADF},
			{"plain text", "plain_text", "First & <second>\r\nline\r\n\r\nGrüezi 世界 👋", "<p>First &amp; &lt;second&gt;<br/>line</p><p>Grüezi 世界 👋</p>"},
		} {
			t.Run(transport+"/"+tt.name, func(t *testing.T) {
				var mu sync.Mutex
				var page confluence.Page
				var value, format string
				writes := 0
				api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					mu.Lock()
					defer mu.Unlock()
					w.Header().Set("Content-Type", "application/json")
					if email, token, ok := r.BasicAuth(); !ok || email != "test@example.com" || token != "test-token" {
						t.Error("missing Confluence authentication")
					}
					if r.Method == http.MethodPost && r.URL.Path == "/wiki/rest/api/content/123/label" {
						_, _ = w.Write([]byte(`{"results":[{"name":"guide"}]}`))
						return
					}
					switch r.Method + " " + r.URL.Path {
					case "POST /wiki/api/v2/pages", "PUT /wiki/api/v2/pages/123", "PUT /wiki/api/v2/pages/123/title":
						var in struct {
							ID, Title, Status, SpaceID, ParentID string
							Body                                 *struct{ Representation, Value string }
							Version                              *confluence.Version
						}
						if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
							t.Error(err)
							http.Error(w, "invalid JSON", http.StatusBadRequest)
							return
						}
						writes++
						if in.Title == "" || in.Status != "current" {
							t.Errorf("missing required title/status: %+v", in)
						}
						if r.Method == http.MethodPost {
							if in.SpaceID != "9" || in.ParentID != "10" || in.Body == nil {
								t.Errorf("invalid create payload: %+v", in)
							}
							page = confluence.Page{ID: "123", SpaceID: in.SpaceID, ParentID: in.ParentID, Status: "current", Version: &confluence.Version{Number: 1}}
						} else if !strings.HasSuffix(r.URL.Path, "/title") {
							if in.Version == nil || in.Version.Number != page.Version.Number+1 {
								w.WriteHeader(http.StatusConflict)
								_, _ = w.Write([]byte(`{"errors":[{"status":"409","title":"Conflict","detail":"Stale page version"}]}`))
								return
							}
							if in.ID != "123" || in.Body == nil {
								t.Errorf("invalid update payload: %+v", in)
							}
							page.Version = in.Version
						}
						page.Title = in.Title
						if in.Body != nil {
							format, value = in.Body.Representation, in.Body.Value
						}
					case "GET /wiki/api/v2/pages/123":
						requested := r.URL.Query().Get("body-format")
						if requested != "" && requested != format {
							t.Errorf("body-format = %q, want %q", requested, format)
						}
					default:
						t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
						http.NotFound(w, r)
						return
					}
					page.Body = &confluence.PageBody{}
					representation := &confluence.ContentRepresentation{Representation: format, Value: value}
					if format == "atlas_doc_format" {
						page.Body.AtlasDocFormat = representation
					} else {
						page.Body.Storage = representation
					}
					if err := json.NewEncoder(w).Encode(page); err != nil {
						t.Error(err)
					}
				}))
				t.Cleanup(api.Close)
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				t.Cleanup(cancel)
				session := articleSession(t, ctx, api.URL, transport, config.ModeReadWrite)
				created := articleTool[CreatedPage](t, ctx, session, "confluence_create_page", map[string]any{
					"space_id": "9", "parent_id": "10", "title": "Guide", "content": tt.content, "body_type": tt.format,
				})
				if created.PageID != "123" || created.Version != 1 {
					t.Fatalf("create result = %+v", created)
				}
				readArgs := map[string]any{"page_id": created.PageID}
				if tt.format == "atlas_doc_format" {
					readArgs["body_format"] = tt.format
				}
				read := articleTool[PageSummary](t, ctx, session, "confluence_get_page", readArgs)
				if read.RawContent == nil || *read.RawContent != tt.stored || read.Version != 1 {
					t.Fatalf("article did not round-trip: %+v", read)
				}
				edited := strings.Replace(*read.RawContent, "Grüezi", "Bonjour", 1)
				updated := articleTool[UpdatePageOutput](t, ctx, session, "confluence_update_page", map[string]any{
					"page_id": "123", "content": edited, "body_type": read.BodyFormat, "version": read.Version, "version_note": "Translate greeting",
				})
				if !updated.Updated || updated.Version != 2 {
					t.Fatalf("update result = %+v", updated)
				}
				read = articleTool[PageSummary](t, ctx, session, "confluence_get_page", readArgs)
				if read.RawContent == nil || *read.RawContent != edited || read.Title != "Guide" {
					t.Fatal("content-only update lost formatting or changed title")
				}
				articleTool[UpdatePageOutput](t, ctx, session, "confluence_update_page", map[string]any{"page_id": "123", "title": "Renamed"})
				read = articleTool[PageSummary](t, ctx, session, "confluence_get_page", readArgs)
				if read.Title != "Renamed" || read.RawContent == nil || *read.RawContent != edited {
					t.Fatal("title-only rename changed the body")
				}
				articleTool[AddPageLabelOutput](t, ctx, session, "confluence_add_page_label", map[string]any{"page_id": "123", "label_name": "guide"})
				conflict, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "confluence_update_page", Arguments: map[string]any{
					"page_id": "123", "title": "Stale", "content": edited, "body_type": read.BodyFormat, "version": 1,
				}})
				if err != nil || !conflict.IsError {
					t.Fatalf("conflict was not a tool error: %+v, %v", conflict, err)
				}
				if len(conflict.Content) == 0 || !strings.Contains(conflict.Content[0].(*mcp.TextContent).Text, "merge") {
					t.Fatal("conflict error must explain how to recover")
				}
				mu.Lock()
				if value != edited || page.Title != "Renamed" || writes != 4 || page.Version.Message != "Translate greeting" {
					t.Errorf("stale update changed state, was retried, or lost version note: writes=%d page=%+v", writes, page)
				}
				mu.Unlock()
				articleTool[UpdatePageOutput](t, ctx, session, "confluence_update_page", map[string]any{
					"page_id": "123", "title": "Empty", "content": "", "body_type": "storage", "version": 2,
				})
				read = articleTool[PageSummary](t, ctx, session, "confluence_get_page", map[string]any{"page_id": "123"})
				if read.RawContent == nil || *read.RawContent != "<p></p>" || read.Version != 3 {
					t.Fatalf("explicit empty content did not clear article: %+v", read)
				}
			})
		}
	}
}
