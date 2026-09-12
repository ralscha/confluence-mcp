package confluence

import (
	"context"
	"net/http"
	"testing"
)

func TestPageWritesRejectInvalidInputBeforeRequests(t *testing.T) {
	client := newTestClient(t, func(http.ResponseWriter, *http.Request) {
		t.Error("invalid input reached Confluence")
	})
	ctx := context.Background()
	for _, in := range []CreatePageInput{
		{Title: "Title"},
		{SpaceID: "9", Title: " \n "},
		{SpaceID: "9", Title: "Title", BodyType: "invalid"},
		{SpaceID: "9", Title: "Title", BodyType: "atlas_doc_format"},
		{SpaceID: "9", Title: "Title", Body: "<p>invalid"},
	} {
		if _, err := client.CreatePage(ctx, in); err == nil {
			t.Errorf("CreatePage accepted %+v", in)
		}
	}
	for _, in := range []UpdatePageInput{
		{Title: new("")},
		{Title: new("Title"), VersionNote: "lost note"},
		{Title: new("Title"), Version: 3},
		{Body: new("<p>hello</p>")},
		{Body: new("<p>hello</p>"), Version: -1},
		{Body: new("<p>hello</p>"), Version: int(^uint(0) >> 1)},
		{Body: new("<p>invalid"), Version: 3},
		{Body: new("null"), BodyType: "atlas_doc_format", Version: 3},
	} {
		if _, err := client.UpdatePage(ctx, "123", in); err == nil {
			t.Errorf("UpdatePage accepted %+v", in)
		}
	}
	if _, err := client.UpdatePage(ctx, "", UpdatePageInput{Title: new("Title")}); err == nil {
		t.Error("UpdatePage accepted an empty page ID")
	}
}

func TestCreateEmptyPageSendsStorageBody(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Body bodyWrite `json:"body"`
		}
		decodeJSONBody(t, r, &body)
		if body.Body.Representation != "storage" || body.Body.Value != "<p></p>" {
			t.Errorf("empty page body = %+v", body.Body)
		}
		writeJSON(w, `{"id":"123","version":{"number":1}}`)
	})
	if _, err := client.CreatePage(context.Background(), CreatePageInput{SpaceID: "9", Title: "Empty"}); err != nil {
		t.Fatal(err)
	}
}

func TestCommentWritesUseArticleValidation(t *testing.T) {
	client := newTestClient(t, func(http.ResponseWriter, *http.Request) {
		t.Error("malformed comment reached Confluence")
	})
	if _, err := client.CreateFooterComment(context.Background(), CreateFooterCommentInput{PageID: "123", Body: "<p>invalid"}); err == nil {
		t.Error("create accepted malformed XHTML")
	}
	if _, err := client.UpdateFooterComment(context.Background(), "123", UpdateFooterCommentInput{Body: "null", BodyType: "atlas_doc_format", Version: 1}); err == nil {
		t.Error("update accepted a non-document ADF body")
	}
}

func TestUpdateRejectsMissingOrWrongPageResponse(t *testing.T) {
	for _, response := range []string{"{}", `{"id":"other-page"}`} {
		t.Run(response, func(t *testing.T) {
			client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(w, response)
			})
			for _, body := range []*string{nil, new("<p>Body</p>")} {
				in := UpdatePageInput{Title: new("Title"), Body: body}
				if body != nil {
					in.Version = 1
				}
				if _, err := client.UpdatePage(context.Background(), "123", in); err == nil {
					t.Error("invalid response reported a successful update")
				}
			}
		})
	}
}
