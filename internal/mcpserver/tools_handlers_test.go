package mcpserver

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"confluence-mcp/internal/confluence"
)

// newTestClient starts an httptest server running handler and returns a
// Confluence client pointed at it.
func newTestClient(t *testing.T, handler http.HandlerFunc) *confluence.Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := confluence.NewClient(server.URL, "test@example.com", "test-token", server.Client())
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	return client
}

func TestGetPageHandler(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Query().Get("body-format"), "storage,view"; got != want {
			t.Errorf("body-format = %q, want %q", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "123",
			"title": "Roadmap",
			"version": {"number": 3},
			"body": {"storage": {"value": "<p>Plan</p>"}}
		}`))
	})

	_, out, err := getPage(client)(context.Background(), nil, GetPageInput{
		PageID:     "123",
		BodyFormat: []string{"storage", "view"},
	})
	if err != nil {
		t.Fatalf("getPage failed: %v", err)
	}
	if got, want := out.ID, "123"; got != want {
		t.Errorf("ID = %q, want %q", got, want)
	}
	if got, want := out.Content, "Plan"; got != want {
		t.Errorf("content = %q, want %q", got, want)
	}
}

func TestGetPageHandlerError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message": "Page not found"}`))
	})

	_, _, err := getPage(client)(context.Background(), nil, GetPageInput{PageID: "123"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "get page 123") {
		t.Errorf("error = %q, want it to name the page", err)
	}
}

func TestSearchPagesHandlerDefaultsAndCursor(t *testing.T) {
	var gotLimit string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotLimit = r.URL.Query().Get("limit")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"results": [{"id": "123", "title": "Roadmap"}],
			"_links": {"next": "/wiki/api/v2/pages?cursor=next-token"}
		}`))
	})

	_, out, err := searchPages(client)(context.Background(), nil, SearchPagesInput{})
	if err != nil {
		t.Fatalf("searchPages failed: %v", err)
	}
	if got, want := gotLimit, "25"; got != want {
		t.Errorf("limit = %q, want the default %q", got, want)
	}
	if got, want := out.NextCursor, "next-token"; got != want {
		t.Errorf("next cursor = %q, want %q", got, want)
	}
	if got, want := len(out.Pages), 1; got != want {
		t.Fatalf("pages = %d, want %d", got, want)
	}
}

func TestSearchPagesHandlerClampsLimit(t *testing.T) {
	var gotLimit string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotLimit = r.URL.Query().Get("limit")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results": []}`))
	})

	if _, _, err := searchPages(client)(context.Background(), nil, SearchPagesInput{Limit: 100000}); err != nil {
		t.Fatalf("searchPages failed: %v", err)
	}
	if got, want := gotLimit, "250"; got != want {
		t.Errorf("limit = %q, want it clamped to %q", got, want)
	}
}

func TestGetPageChildrenHandler(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/wiki/api/v2/pages/123/children"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"results": [{"id": "456", "title": "Child", "status": "current", "spaceId": "9", "childPosition": 2}],
			"_links": {"next": "/wiki/api/v2/pages/123/children?cursor=more"}
		}`))
	})

	_, out, err := getPageChildren(client)(context.Background(), nil, GetPageChildrenInput{PageID: "123"})
	if err != nil {
		t.Fatalf("getPageChildren failed: %v", err)
	}
	if got, want := len(out.Children), 1; got != want {
		t.Fatalf("children = %d, want %d", got, want)
	}
	want := ChildPageSummary{ID: "456", Title: "Child", Status: "current", SpaceID: "9", ChildPosition: 2}
	if out.Children[0] != want {
		t.Errorf("child = %+v, want %+v", out.Children[0], want)
	}
	if got, want := out.NextCursor, "more"; got != want {
		t.Errorf("next cursor = %q, want %q", got, want)
	}
}

func TestGetPageAncestorsHandler(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/wiki/api/v2/pages/123/ancestors"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results": [{"id": "1", "type": "page", "title": "Root", "status": "current"}]}`))
	})

	_, out, err := getPageAncestors(client)(context.Background(), nil, GetPageAncestorsInput{PageID: "123"})
	if err != nil {
		t.Fatalf("getPageAncestors failed: %v", err)
	}
	want := AncestorSummary{ID: "1", Title: "Root", Type: "page", Status: "current"}
	if got, wantLen := len(out.Ancestors), 1; got != wantLen {
		t.Fatalf("ancestors = %d, want %d", got, wantLen)
	}
	if out.Ancestors[0] != want {
		t.Errorf("ancestor = %+v, want %+v", out.Ancestors[0], want)
	}
}

func TestGetSpacePagesHandler(t *testing.T) {
	var gotQuery url.Values
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/wiki/api/v2/spaces/9/pages"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results": [{"id": "123", "title": "Roadmap"}]}`))
	})

	_, out, err := getSpacePages(client)(context.Background(), nil, GetSpacePagesInput{
		SpaceID: "9",
		Sort:    "title",
	})
	if err != nil {
		t.Fatalf("getSpacePages failed: %v", err)
	}
	if got, want := gotQuery.Get("sort"), "title"; got != want {
		t.Errorf("sort = %q, want %q", got, want)
	}
	if got, want := len(out.Pages), 1; got != want {
		t.Fatalf("pages = %d, want %d", got, want)
	}
}

func TestListPageCommentsHandlerDefaultsToFooter(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/wiki/api/v2/pages/123/footer-comments"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results": [{"id": "c1", "pageId": "123"}]}`))
	})

	_, out, err := listPageComments(client)(context.Background(), nil, ListPageCommentsInput{PageID: "123"})
	if err != nil {
		t.Fatalf("listPageComments failed: %v", err)
	}
	if got, want := len(out.Comments), 1; got != want {
		t.Fatalf("comments = %d, want %d", got, want)
	}
	if got, want := out.Comments[0].Type, confluence.CommentTypeFooter; got != want {
		t.Errorf("comment type = %q, want %q", got, want)
	}
}

func TestDownloadAttachmentHandler(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("file-contents"))
	})

	_, out, err := downloadAttachment(client)(context.Background(), nil, DownloadAttachmentInput{AttachmentID: "att1"})
	if err != nil {
		t.Fatalf("downloadAttachment failed: %v", err)
	}

	data, err := base64.StdEncoding.DecodeString(out.DataBase64)
	if err != nil {
		t.Fatalf("decoding result: %v", err)
	}
	if got, want := string(data), "file-contents"; got != want {
		t.Errorf("data = %q, want %q", got, want)
	}
}

func TestUploadAttachmentHandlerRejectsBadBase64(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("expected no request to be sent")
	})

	_, _, err := uploadAttachment(client)(context.Background(), nil, UploadAttachmentInput{
		PageID:     "123",
		Filename:   "a.txt",
		DataBase64: "not-base64!!",
	})
	if err == nil {
		t.Fatal("expected error for invalid base64, got nil")
	}
}

func TestCreatePageHandler(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id": "456", "title": "New page"}`))
	})

	_, out, err := createPage(client)(context.Background(), nil, CreatePageInput{
		SpaceID: "9",
		Title:   "New page",
		Content: "hello",
	})
	if err != nil {
		t.Fatalf("createPage failed: %v", err)
	}
	if got, want := out.PageID, "456"; got != want {
		t.Errorf("page ID = %q, want %q", got, want)
	}
}

func TestDeletePageHandler(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	_, out, err := deletePage(client)(context.Background(), nil, DeletePageInput{PageID: "123"})
	if err != nil {
		t.Fatalf("deletePage failed: %v", err)
	}
	if !out.Deleted {
		t.Error("deleted = false, want true")
	}
}
