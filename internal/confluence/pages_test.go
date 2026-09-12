package confluence

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"
)

func TestClient_SearchPages(t *testing.T) {
	var gotQuery url.Values
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/wiki/api/v2/pages"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		gotQuery = r.URL.Query()
		writeJSON(w, `{
			"results": [
				{
					"id": "123",
					"status": "current",
					"title": "Roadmap",
					"spaceId": "9",
					"version": {"number": 3},
					"_links": {"webui": "/spaces/ENG/pages/123"}
				}
			],
			"_links": {"next": "/wiki/api/v2/pages?cursor=next-token"}
		}`)
	})

	result, err := client.SearchPages(context.Background(), SearchPagesInput{
		SpaceID:    "9",
		Title:      "Road",
		Status:     "current",
		Sort:       "-modified-date",
		BodyFormat: "storage",
		Limit:      10,
		Cursor:     "abc",
	})
	if err != nil {
		t.Fatalf("SearchPages failed: %v", err)
	}

	for key, want := range map[string]string{
		"space-id":    "9",
		"title":       "Road",
		"status":      "current",
		"sort":        "-modified-date",
		"body-format": "storage",
		"limit":       "10",
		"cursor":      "abc",
	} {
		if got := gotQuery.Get(key); got != want {
			t.Errorf("query %s = %q, want %q", key, got, want)
		}
	}

	if got, want := len(result.Results), 1; got != want {
		t.Fatalf("results = %d, want %d", got, want)
	}
	if got, want := result.Results[0].ID, "123"; got != want {
		t.Errorf("page ID = %q, want %q", got, want)
	}
	if got, want := result.Links.Next, "/wiki/api/v2/pages?cursor=next-token"; got != want {
		t.Errorf("next link = %q, want %q", got, want)
	}
}

func TestClient_CreatePage(t *testing.T) {
	var gotBody map[string]any
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Method, http.MethodPost; got != want {
			t.Errorf("method = %q, want %q", got, want)
		}
		if got, want := r.URL.Path, "/wiki/api/v2/pages"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		decodeJSONBody(t, r, &gotBody)
		writeJSON(w, `{"id": "456", "title": "New page"}`)
	})

	page, err := client.CreatePage(context.Background(), CreatePageInput{
		SpaceID:  "9",
		Title:    "New page",
		ParentID: "123",
		Body:     "<p>hello</p>",
	})
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	if got, want := page.ID, "456"; got != want {
		t.Errorf("page ID = %q, want %q", got, want)
	}
	if got, want := gotBody["spaceId"], "9"; got != want {
		t.Errorf("spaceId = %v, want %v", got, want)
	}
	if got, want := gotBody["parentId"], "123"; got != want {
		t.Errorf("parentId = %v, want %v", got, want)
	}

	storage, ok := gotBody["body"].(map[string]any)
	if !ok {
		t.Fatalf("body.storage missing in %v", gotBody)
	}
	if got, want := storage["value"], "<p>hello</p>"; got != want {
		t.Errorf("body value = %v, want %v", got, want)
	}
	if got, want := storage["representation"], "storage"; got != want {
		t.Errorf("representation = %v, want %v", got, want)
	}
}

func TestClient_UpdatePage(t *testing.T) {
	var gotBody map[string]any
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Method, http.MethodPut; got != want {
			t.Errorf("method = %q, want %q", got, want)
		}
		if got, want := r.URL.Path, "/wiki/api/v2/pages/123"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		decodeJSONBody(t, r, &gotBody)
		writeJSON(w, `{"id": "123", "version": {"number": 4}}`)
	})

	title := "Updated"
	content := "<p>Updated body</p>"
	page, err := client.UpdatePage(context.Background(), "123", UpdatePageInput{
		Title:       &title,
		Body:        &content,
		Version:     3,
		VersionNote: "tidy up",
	})
	if err != nil {
		t.Fatalf("UpdatePage failed: %v", err)
	}

	if page.Version == nil || page.Version.Number != 4 {
		t.Fatalf("version = %v, want 4", page.Version)
	}
	if got, want := gotBody["title"], "Updated"; got != want {
		t.Errorf("title = %v, want %v", got, want)
	}

	version, ok := gotBody["version"].(map[string]any)
	if !ok {
		t.Fatalf("version missing in %v", gotBody)
	}
	if got, want := version["number"], float64(4); got != want {
		t.Errorf("version number = %v, want %v", got, want)
	}
	if got, want := version["message"], "tidy up"; got != want {
		t.Errorf("version message = %v, want %v", got, want)
	}
}

func TestClient_UpdatePageRequiresTitleOrBody(t *testing.T) {
	client := newTestClient(t, func(http.ResponseWriter, *http.Request) {
		t.Error("expected no request to be sent")
	})

	if _, err := client.UpdatePage(context.Background(), "123", UpdatePageInput{Version: 3}); err == nil {
		t.Fatal("expected error when neither title nor body is set, got nil")
	}
}

func TestClient_UpdatePageTitleOnly(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/wiki/api/v2/pages/123/title"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		var body map[string]any
		decodeJSONBody(t, r, &body)
		if got, want := body["title"], "Renamed"; got != want {
			t.Errorf("title = %v, want %v", got, want)
		}
		if _, exists := body["version"]; exists {
			t.Error("title-only update unexpectedly sent a version")
		}
		writeJSON(w, `{"id":"123","title":"Renamed","version":{"number":4}}`)
	})

	title := "Renamed"
	if _, err := client.UpdatePage(context.Background(), "123", UpdatePageInput{Title: &title}); err != nil {
		t.Fatalf("UpdatePage failed: %v", err)
	}
}

func TestClient_UpdatePageContentOnlyFetchesTitle(t *testing.T) {
	requestCount := 0
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/wiki/api/v2/pages/123":
			writeJSON(w, `{"id":"123","title":"Existing title","version":{"number":3}}`)
		case r.Method == http.MethodPut && r.URL.Path == "/wiki/api/v2/pages/123":
			var body map[string]any
			decodeJSONBody(t, r, &body)
			if got, want := body["title"], "Existing title"; got != want {
				t.Errorf("title = %v, want %v", got, want)
			}
			version := body["version"].(map[string]any)
			if got, want := version["number"], float64(4); got != want {
				t.Errorf("version = %v, want %v", got, want)
			}
			writeJSON(w, `{"id":"123","title":"Existing title","version":{"number":4}}`)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})

	content := "<p>New content</p>"
	if _, err := client.UpdatePage(context.Background(), "123", UpdatePageInput{Body: &content, Version: 3}); err != nil {
		t.Fatalf("UpdatePage failed: %v", err)
	}
	if got, want := requestCount, 2; got != want {
		t.Errorf("request count = %d, want %d", got, want)
	}
}

func TestClient_ListPageVersions(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/wiki/api/v2/pages/123/versions"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		for key, want := range map[string]string{
			"body-format": "storage",
			"sort":        "-modified-date",
			"limit":       "20",
			"cursor":      "next",
		} {
			if got := r.URL.Query().Get(key); got != want {
				t.Errorf("query %s = %q, want %q", key, got, want)
			}
		}
		writeJSON(w, `{
			"results":[{"number":3,"message":"release","createdAt":"2026-01-02T03:04:05Z","authorId":"acct","page":{"id":"123","title":"Roadmap","body":{"storage":{"value":"<p>Plan</p>"}}}}],
			"_links":{"next":"/wiki/api/v2/pages/123/versions?cursor=more"}
		}`)
	})

	result, err := client.ListPageVersions(context.Background(), ListPageVersionsInput{
		PageID: "123", BodyFormat: "storage", Sort: "-modified-date", Limit: 20, Cursor: "next",
	})
	if err != nil {
		t.Fatalf("ListPageVersions failed: %v", err)
	}
	if got, want := result.Results[0].Page.Body.PlainText(), "Plan"; got != want {
		t.Errorf("content = %q, want %q", got, want)
	}
}

func TestClient_DeletePage(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Method, http.MethodDelete; got != want {
			t.Errorf("method = %q, want %q", got, want)
		}
		if got, want := r.URL.Path, "/wiki/api/v2/pages/123"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	if err := client.DeletePage(context.Background(), "123"); err != nil {
		t.Fatalf("DeletePage failed: %v", err)
	}
}

func TestClient_GetPageLabels(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/wiki/api/v2/pages/123/labels"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if got, want := r.URL.Query().Get("limit"), "50"; got != want {
			t.Errorf("limit = %q, want %q", got, want)
		}
		if got, want := r.URL.Query().Get("cursor"), "next"; got != want {
			t.Errorf("cursor = %q, want %q", got, want)
		}
		writeJSON(w, `{"results": [{"id": "1", "name": "release"}, {"id": "2", "name": "draft"}], "_links":{"next":"/wiki/api/v2/pages/123/labels?cursor=more"}}`)
	})

	result, err := client.GetPageLabels(context.Background(), GetPageLabelsInput{PageID: "123", Limit: 50, Cursor: "next"})
	if err != nil {
		t.Fatalf("GetPageLabels failed: %v", err)
	}
	if got, want := len(result.Results), 2; got != want {
		t.Fatalf("labels = %d, want %d", got, want)
	}
	if got, want := result.Results[0].Name, "release"; got != want {
		t.Errorf("label name = %q, want %q", got, want)
	}
}

func TestClient_RemovePageLabel(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Method, http.MethodDelete; got != want {
			t.Errorf("method = %q, want %q", got, want)
		}
		if got, want := r.URL.Path, "/wiki/rest/api/content/123/label"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if got, want := r.URL.Query().Get("name"), "team/roadmap"; got != want {
			t.Errorf("label = %q, want %q", got, want)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	if err := client.RemovePageLabel(context.Background(), "123", "team/roadmap"); err != nil {
		t.Fatalf("RemovePageLabel failed: %v", err)
	}
}

func TestClient_AddPageLabel(t *testing.T) {
	var gotBody []map[string]any
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Method, http.MethodPost; got != want {
			t.Errorf("method = %q, want %q", got, want)
		}
		if got, want := r.URL.Path, "/wiki/rest/api/content/123/label"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		decodeJSONBody(t, r, &gotBody)
		w.WriteHeader(http.StatusNoContent)
	})

	if err := client.AddPageLabel(context.Background(), "123", "release"); err != nil {
		t.Fatalf("AddPageLabel failed: %v", err)
	}
	if got, want := len(gotBody), 1; got != want {
		t.Fatalf("labels sent = %d, want %d", got, want)
	}
	if got, want := gotBody[0]["name"], "release"; got != want {
		t.Errorf("label name = %v, want %v", got, want)
	}
	if got, want := gotBody[0]["prefix"], "global"; got != want {
		t.Errorf("label prefix = %v, want %v", got, want)
	}
}

func TestClient_GetPageChildren(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/wiki/api/v2/pages/123/direct-children"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		query := r.URL.Query()
		if got, want := query.Get("sort"), "child-position"; got != want {
			t.Errorf("sort = %q, want %q", got, want)
		}
		if got, want := query.Get("limit"), "25"; got != want {
			t.Errorf("limit = %q, want %q", got, want)
		}
		writeJSON(w, `{
			"results": [
				{"id": "456", "type": "page", "status": "current", "title": "Child", "spaceId": "9", "childPosition": 1}
			],
			"_links": {"next": "/wiki/api/v2/pages/123/direct-children?cursor=more"}
		}`)
	})

	result, err := client.GetPageChildren(context.Background(), GetPageChildrenInput{
		PageID: "123",
		Sort:   "child-position",
		Limit:  25,
	})
	if err != nil {
		t.Fatalf("GetPageChildren failed: %v", err)
	}
	if got, want := len(result.Results), 1; got != want {
		t.Fatalf("children = %d, want %d", got, want)
	}
	if got, want := result.Results[0].Title, "Child"; got != want {
		t.Errorf("child title = %q, want %q", got, want)
	}
	if got, want := result.Results[0].ChildPosition, 1; got != want {
		t.Errorf("child position = %d, want %d", got, want)
	}
	if got, want := result.Results[0].Type, "page"; got != want {
		t.Errorf("child type = %q, want %q", got, want)
	}
}

func TestClient_GetPageAncestors(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/wiki/api/v2/pages/123/ancestors"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if got, want := r.URL.Query().Get("limit"), "5"; got != want {
			t.Errorf("limit = %q, want %q", got, want)
		}
		writeJSON(w, `{"results": [{"id": "1", "type": "page", "title": "Root"}, {"id": "2", "type": "page", "title": "Parent"}]}`)
	})

	result, err := client.GetPageAncestors(context.Background(), "123", 5)
	if err != nil {
		t.Fatalf("GetPageAncestors failed: %v", err)
	}
	if got, want := len(result.Results), 2; got != want {
		t.Fatalf("ancestors = %d, want %d", got, want)
	}
	if got, want := result.Results[1].Title, "Parent"; got != want {
		t.Errorf("ancestor title = %q, want %q", got, want)
	}
}

func TestClient_GetSpacePages(t *testing.T) {
	var gotQuery url.Values
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/wiki/api/v2/spaces/9/pages"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		gotQuery = r.URL.Query()
		writeJSON(w, `{"results": [{"id": "123", "title": "Roadmap"}]}`)
	})

	result, err := client.GetSpacePages(context.Background(), GetSpacePagesInput{
		SpaceID:    "9",
		Title:      "Road",
		Status:     []string{"current", "archived"},
		Sort:       "title",
		BodyFormat: "storage",
		Limit:      25,
		Cursor:     "abc",
	})
	if err != nil {
		t.Fatalf("GetSpacePages failed: %v", err)
	}
	if got, want := len(result.Results), 1; got != want {
		t.Fatalf("pages = %d, want %d", got, want)
	}
	if got, want := gotQuery["status"], []string{"current", "archived"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("status = %v, want %v", got, want)
	}
	if got, want := gotQuery.Get("sort"), "title"; got != want {
		t.Errorf("sort = %q, want %q", got, want)
	}
	if got, want := gotQuery.Get("body-format"), "storage"; got != want {
		t.Errorf("body-format = %q, want %q", got, want)
	}
}

func TestPage_StorageToPlainText(t *testing.T) {
	page := &Page{Body: &PageBody{Storage: &ContentRepresentation{Value: "<p>Hello &amp; welcome</p>"}}}
	if got, want := page.StorageToPlainText(), "Hello & welcome"; got != want {
		t.Errorf("plain text = %q, want %q", got, want)
	}

	empty := &Page{}
	if got := empty.StorageToPlainText(); got != "" {
		t.Errorf("plain text for empty body = %q, want empty", got)
	}
}

// decodeJSONBody decodes the request body into out, failing the test on error.
func decodeJSONBody(t *testing.T, r *http.Request, out any) {
	t.Helper()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("reading request body: %v", err)
	}
	if err := json.Unmarshal(body, out); err != nil {
		t.Fatalf("decoding request body %q: %v", body, err)
	}
}
