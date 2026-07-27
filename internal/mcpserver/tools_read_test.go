package mcpserver

import (
	"testing"

	"confluence-mcp/internal/confluence"
)

func TestClampLimit(t *testing.T) {
	tests := []struct {
		name  string
		limit int
		want  int
	}{
		{name: "zero uses default", limit: 0, want: defaultLimit},
		{name: "negative uses default", limit: -5, want: defaultLimit},
		{name: "within range is kept", limit: 42, want: 42},
		{name: "at maximum is kept", limit: maxLimit, want: maxLimit},
		{name: "above maximum is capped", limit: 10000, want: maxLimit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clampLimit(tt.limit); got != tt.want {
				t.Errorf("clampLimit(%d) = %d, want %d", tt.limit, got, tt.want)
			}
		})
	}
}

func TestNextCursor(t *testing.T) {
	tests := []struct {
		name string
		next string
		want string
	}{
		{name: "empty", next: "", want: ""},
		{name: "extracts cursor", next: "/wiki/api/v2/pages?limit=25&cursor=abc123", want: "abc123"},
		{name: "no cursor param falls back to link", next: "/wiki/api/v2/pages?limit=25", want: "/wiki/api/v2/pages?limit=25"},
		{name: "unparseable falls back to link", next: "://bad", want: "://bad"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nextCursor(tt.next); got != tt.want {
				t.Errorf("nextCursor(%q) = %q, want %q", tt.next, got, tt.want)
			}
		})
	}
}

func TestPageToSummary(t *testing.T) {
	page := &confluence.Page{
		ID:       "123",
		Title:    "Roadmap",
		Status:   "current",
		SpaceID:  "9",
		ParentID: "1",
		Version:  &confluence.Version{Number: 4},
		Body:     &confluence.PageBody{Storage: &confluence.ContentRepresentation{Value: "<p>Hello &amp; welcome</p>"}},
		Links:    confluence.PageLinks{WebUI: "/spaces/ENG/pages/123"},
	}

	got := pageToSummary(page)
	want := PageSummary{
		ID:       "123",
		Title:    "Roadmap",
		Status:   "current",
		SpaceID:  "9",
		ParentID: "1",
		Version:  4,
		Content:  "Hello & welcome",
		WebURL:   "/spaces/ENG/pages/123",
	}
	if got != want {
		t.Errorf("pageToSummary() = %+v, want %+v", got, want)
	}
}

func TestPageToSummaryWithoutVersionOrBody(t *testing.T) {
	got := pageToSummary(&confluence.Page{ID: "123"})
	if got.Version != 0 {
		t.Errorf("version = %d, want 0", got.Version)
	}
	if got.Content != "" {
		t.Errorf("content = %q, want empty", got.Content)
	}
}

func TestSpaceToSummary(t *testing.T) {
	got := spaceToSummary(&confluence.Space{
		ID:     "9",
		Key:    "ENG",
		Name:   "Engineering",
		Type:   "global",
		Status: "current",
		Links:  confluence.SpaceLinks{WebUI: "/spaces/ENG"},
	})
	want := SpaceSummary{
		ID:     "9",
		Key:    "ENG",
		Name:   "Engineering",
		Type:   "global",
		Status: "current",
		WebURL: "/spaces/ENG",
	}
	if got != want {
		t.Errorf("spaceToSummary() = %+v, want %+v", got, want)
	}
}

func TestCommentToSummary(t *testing.T) {
	comment := &confluence.Comment{
		ID:               "c1",
		Status:           "current",
		Title:            "Re: Roadmap",
		PageID:           "123",
		ParentCommentID:  "c0",
		ResolutionStatus: "open",
		Version:          &confluence.Version{Number: 2, CreatedAt: "2026-07-09T12:00:00Z", AuthorID: "acct-1"},
		Body:             &confluence.PageBody{Storage: &confluence.ContentRepresentation{Value: "<p>Looks good</p>"}},
		InlineCommentProperties: &confluence.InlineCommentProperties{
			InlineMarkerRef:         "marker-1",
			InlineOriginalSelection: "selected text",
		},
		Links: confluence.PageLinks{WebUI: "/spaces/ENG/pages/123?focusedCommentId=c1"},
	}

	got := commentToSummary(comment, confluence.CommentTypeInline)

	if got.Type != confluence.CommentTypeInline {
		t.Errorf("type = %q, want %q", got.Type, confluence.CommentTypeInline)
	}
	if got.Content != "Looks good" {
		t.Errorf("content = %q, want %q", got.Content, "Looks good")
	}
	if got.Version != 2 {
		t.Errorf("version = %d, want 2", got.Version)
	}
	if got.CreatedAt != "2026-07-09T12:00:00Z" {
		t.Errorf("created at = %q, want %q", got.CreatedAt, "2026-07-09T12:00:00Z")
	}
	if got.AuthorID != "acct-1" {
		t.Errorf("author ID = %q, want %q", got.AuthorID, "acct-1")
	}
	if got.InlineOriginalSelection != "selected text" {
		t.Errorf("inline selection = %q, want %q", got.InlineOriginalSelection, "selected text")
	}
}

func TestCommentToSummaryVersionFallbacks(t *testing.T) {
	comment := &confluence.Comment{
		ID: "c1",
		Version: &confluence.Version{
			Number: 1,
			When:   "2026-07-09T12:00:00Z",
			By:     &confluence.User{AccountID: "acct-2"},
		},
	}

	got := commentToSummary(comment, confluence.CommentTypeFooter)

	if got.CreatedAt != "2026-07-09T12:00:00Z" {
		t.Errorf("created at = %q, want the 'when' value", got.CreatedAt)
	}
	if got.AuthorID != "acct-2" {
		t.Errorf("author ID = %q, want the 'by' account ID", got.AuthorID)
	}
}

func TestCQLResultToSummary(t *testing.T) {
	item := &confluence.ContentSearchItem{
		Title:      "Roadmap",
		Excerpt:    "Hello",
		EntityType: "content",
		Score:      42,
		Content: &confluence.SearchContent{
			ID:     "123",
			Type:   "page",
			Status: "current",
			Title:  "Roadmap (content)",
			Space:  &confluence.SearchSpace{Key: "ENG", Name: "Engineering"},
			Body:   &confluence.PageBody{Storage: &confluence.ContentRepresentation{Value: "<p>Body text</p>"}},
			Links:  confluence.PageLinks{WebUI: "/spaces/ENG/pages/123"},
		},
	}

	got := cqlResultToSummary(item)

	if got.ID != "123" {
		t.Errorf("ID = %q, want %q", got.ID, "123")
	}
	if got.Title != "Roadmap" {
		t.Errorf("title = %q, want the search hit title", got.Title)
	}
	if got.URL != "/spaces/ENG/pages/123" {
		t.Errorf("URL = %q, want the content web UI link", got.URL)
	}
	if got.SpaceKey != "ENG" || got.SpaceName != "Engineering" {
		t.Errorf("space = %q/%q, want ENG/Engineering", got.SpaceKey, got.SpaceName)
	}
	if got.Content != "Body text" {
		t.Errorf("content = %q, want %q", got.Content, "Body text")
	}
}

func TestCQLResultToSummaryWithoutContent(t *testing.T) {
	got := cqlResultToSummary(&confluence.ContentSearchItem{Title: "Standalone", URL: "/x"})

	if got.Title != "Standalone" {
		t.Errorf("title = %q, want %q", got.Title, "Standalone")
	}
	if got.ID != "" {
		t.Errorf("ID = %q, want empty", got.ID)
	}
}
