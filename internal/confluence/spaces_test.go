package confluence

import (
	"context"
	"net/http"
	"net/url"
	"testing"
)

func TestClient_GetSpace(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/wiki/api/v2/spaces/ENG"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		writeJSON(w, `{
			"id": "9",
			"key": "ENG",
			"name": "Engineering",
			"type": "global",
			"status": "current",
			"_links": {"webui": "/spaces/ENG"}
		}`)
	})

	space, err := client.GetSpace(context.Background(), "ENG")
	if err != nil {
		t.Fatalf("GetSpace failed: %v", err)
	}
	if got, want := space.ID, "9"; got != want {
		t.Errorf("space ID = %q, want %q", got, want)
	}
	if got, want := space.Links.WebUI, "/spaces/ENG"; got != want {
		t.Errorf("web UI link = %q, want %q", got, want)
	}
}

func TestClient_ListSpaces(t *testing.T) {
	var gotQuery url.Values
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/wiki/api/v2/spaces"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		gotQuery = r.URL.Query()
		writeJSON(w, `{
			"results": [{"id": "9", "key": "ENG", "name": "Engineering"}],
			"_links": {"next": "/wiki/api/v2/spaces?cursor=more"}
		}`)
	})

	result, err := client.ListSpaces(context.Background(), ListSpacesInput{
		Keys:   []string{"ENG", "OPS"},
		Type:   "global",
		Status: "current",
		Limit:  10,
		Cursor: "abc",
	})
	if err != nil {
		t.Fatalf("ListSpaces failed: %v", err)
	}

	if got, want := gotQuery["keys"], []string{"ENG", "OPS"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("keys = %v, want %v", got, want)
	}
	for key, want := range map[string]string{
		"type":   "global",
		"status": "current",
		"limit":  "10",
		"cursor": "abc",
	} {
		if got := gotQuery.Get(key); got != want {
			t.Errorf("query %s = %q, want %q", key, got, want)
		}
	}

	if got, want := len(result.Results), 1; got != want {
		t.Fatalf("spaces = %d, want %d", got, want)
	}
	if got, want := result.Links.Next, "/wiki/api/v2/spaces?cursor=more"; got != want {
		t.Errorf("next link = %q, want %q", got, want)
	}
}

func TestClient_GetSpaceNotFound(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		writeJSON(w, `{"errors": [{"status": 404, "title": "Space not found"}]}`)
	})

	_, err := client.GetSpace(context.Background(), "MISSING")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
