package confluence

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_SearchContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wiki/rest/api/search" {
			t.Fatalf("path = %q, want /wiki/rest/api/search", r.URL.Path)
		}
		query := r.URL.Query()
		if got, want := query.Get("cql"), `type=page AND space="ENG"`; got != want {
			t.Fatalf("cql = %q, want %q", got, want)
		}
		if got, want := query["expand"], []string{"body.storage", "version"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
			t.Fatalf("expand = %v, want %v", got, want)
		}
		if got, want := query.Get("limit"), "10"; got != want {
			t.Fatalf("limit = %q, want %q", got, want)
		}
		if got, want := query.Get("cursor"), "next-token"; got != want {
			t.Fatalf("cursor = %q, want %q", got, want)
		}
		if got, want := query.Get("includeArchivedSpaces"), "true"; got != want {
			t.Fatalf("includeArchivedSpaces = %q, want %q", got, want)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"results": [
				{
					"content": {
						"id": "123",
						"type": "page",
						"status": "current",
						"title": "Roadmap",
						"space": {"key": "ENG", "name": "Engineering"},
						"body": {"storage": {"representation": "storage", "value": "<p>Hello &amp; welcome</p>"}},
						"_links": {"webui": "/spaces/ENG/pages/123/Roadmap"}
					},
					"title": "Roadmap",
					"excerpt": "Hello",
					"url": "/wiki/spaces/ENG/pages/123/Roadmap",
					"entityType": "content",
					"score": 42
				}
			],
			"size": 1,
			"totalSize": 3,
			"cqlQuery": "type=page",
			"_links": {"next": "/wiki/rest/api/search?cursor=another-token"}
		}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test@example.com", "test-token", server.Client())
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	result, err := client.SearchContent(context.Background(), SearchContentInput{
		CQL:                   `type=page AND space="ENG"`,
		Expand:                []string{"body.storage", "version"},
		Limit:                 10,
		Cursor:                "next-token",
		IncludeArchivedSpaces: true,
	})
	if err != nil {
		t.Fatalf("SearchContent failed: %v", err)
	}

	if got, want := result.Size, 1; got != want {
		t.Fatalf("size = %d, want %d", got, want)
	}
	if got, want := result.Results[0].Content.ID, "123"; got != want {
		t.Fatalf("content ID = %q, want %q", got, want)
	}
	if got, want := result.Results[0].Content.Body.PlainText(), "Hello & welcome"; got != want {
		t.Fatalf("plain text = %q, want %q", got, want)
	}
}

func TestClient_ListPageCommentsInline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wiki/api/v2/pages/123/inline-comments" {
			t.Fatalf("path = %q, want /wiki/api/v2/pages/123/inline-comments", r.URL.Path)
		}
		query := r.URL.Query()
		if got, want := query.Get("body-format"), "storage"; got != want {
			t.Fatalf("body-format = %q, want %q", got, want)
		}
		if got, want := query.Get("status"), "current"; got != want {
			t.Fatalf("status = %q, want %q", got, want)
		}
		if got, want := query.Get("resolution-status"), "open"; got != want {
			t.Fatalf("resolution-status = %q, want %q", got, want)
		}
		if got, want := query.Get("cursor"), "abc"; got != want {
			t.Fatalf("cursor = %q, want %q", got, want)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"results": [
				{
					"id": "c1",
					"pageId": "123",
					"status": "current",
					"resolutionStatus": "open",
					"properties": {
						"inlineMarkerRef": "marker-1",
						"inlineOriginalSelection": "selected text"
					},
					"body": {"storage": {"representation": "storage", "value": "<p>Inline note</p>"}},
					"version": {"number": 4, "createdAt": "2026-07-09T12:00:00Z", "authorId": "acct-1"},
					"_links": {"webui": "/comment/c1"}
				}
			],
			"_links": {"next": "/wiki/api/v2/pages/123/inline-comments?cursor=def"}
		}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test@example.com", "test-token", server.Client())
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	result, err := client.ListPageComments(context.Background(), ListPageCommentsInput{
		PageID:           "123",
		CommentType:      CommentTypeInline,
		BodyFormat:       "storage",
		Status:           []string{"current"},
		ResolutionStatus: []string{"open"},
		Limit:            25,
		Cursor:           "abc",
	})
	if err != nil {
		t.Fatalf("ListPageComments failed: %v", err)
	}

	if got, want := result.Results[0].PlainText(), "Inline note"; got != want {
		t.Fatalf("plain text = %q, want %q", got, want)
	}
	if got, want := result.Results[0].InlineCommentProperties.InlineMarkerRef, "marker-1"; got != want {
		t.Fatalf("inline marker = %q, want %q", got, want)
	}
}

func TestClient_FooterCommentWrites(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/wiki/api/v2/footer-comments":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode create body: %v", err)
			}
			if got, want := body["pageId"], "123"; got != want {
				t.Fatalf("pageId = %v, want %v", got, want)
			}
			commentBody := body["body"].(map[string]any)
			if got, want := commentBody["representation"], "storage"; got != want {
				t.Fatalf("representation = %v, want %v", got, want)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"c1","pageId":"123","version":{"number":1}}`))
		case r.Method == http.MethodPut && r.URL.Path == "/wiki/api/v2/footer-comments/c1":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode update body: %v", err)
			}
			version := body["version"].(map[string]any)
			if got, want := version["number"], float64(1); got != want {
				t.Fatalf("version number = %v, want %v", got, want)
			}
			if got, want := version["message"], "clarify"; got != want {
				t.Fatalf("version message = %v, want %v", got, want)
			}
			_, _ = w.Write([]byte(`{"id":"c1","pageId":"123","version":{"number":2}}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/wiki/api/v2/footer-comments/c1":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test@example.com", "test-token", server.Client())
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	created, err := client.CreateFooterComment(context.Background(), CreateFooterCommentInput{
		PageID: "123",
		Body:   "<p>Hello</p>",
	})
	if err != nil {
		t.Fatalf("CreateFooterComment failed: %v", err)
	}
	if got, want := created.ID, "c1"; got != want {
		t.Fatalf("created ID = %q, want %q", got, want)
	}

	updated, err := client.UpdateFooterComment(context.Background(), "c1", UpdateFooterCommentInput{
		Body:        "<p>Updated</p>",
		Version:     1,
		VersionNote: "clarify",
	})
	if err != nil {
		t.Fatalf("UpdateFooterComment failed: %v", err)
	}
	if got, want := updated.Version.Number, 2; got != want {
		t.Fatalf("updated version = %d, want %d", got, want)
	}

	if err := client.DeleteFooterComment(context.Background(), "c1"); err != nil {
		t.Fatalf("DeleteFooterComment failed: %v", err)
	}
}
