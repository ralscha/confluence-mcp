package confluence

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// GetPage fetches a single page by ID. If bodyFormat is non-empty, the page
// body is included in the specified format (e.g. "storage", "atlas_doc_format").
func (c *Client) GetPage(ctx context.Context, pageID string, bodyFormat []string) (*Page, error) {
	query := url.Values{}
	if len(bodyFormat) > 0 {
		query.Set("body-format", strings.Join(bodyFormat, ","))
	}

	var page Page
	if err := c.doJSON(ctx, "GET", "wiki/api/v2/pages/"+url.PathEscape(pageID), query, nil, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// SearchPagesInput describes parameters for searching pages.
type SearchPagesInput struct {
	SpaceID string
	Title   string
	Status  string
	Limit   int
	Cursor  string
}

// SearchPages searches for pages matching the given criteria.
func (c *Client) SearchPages(ctx context.Context, in SearchPagesInput) (*PageSearchResult, error) {
	query := url.Values{}
	if in.SpaceID != "" {
		query.Set("space-id", in.SpaceID)
	}
	if in.Title != "" {
		query.Set("title", in.Title)
	}
	if in.Status != "" {
		query.Set("status", in.Status)
	}
	if in.Limit > 0 {
		query.Set("limit", strconv.Itoa(in.Limit))
	}
	if in.Cursor != "" {
		query.Set("cursor", in.Cursor)
	}

	var result PageSearchResult
	if err := c.doJSON(ctx, "GET", "wiki/api/v2/pages", query, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreatePageInput describes the fields for a new page.
type CreatePageInput struct {
	SpaceID  string
	Title    string
	ParentID string
	Body     string // content in storage format or plain text
	BodyType string // "storage" or "atlas_doc_format"
}

// CreatePage creates a new page and returns the created page.
func (c *Client) CreatePage(ctx context.Context, in CreatePageInput) (*Page, error) {
	body := map[string]any{
		"spaceId": in.SpaceID,
		"status":  "current",
		"title":   in.Title,
	}

	if in.ParentID != "" {
		body["parentId"] = in.ParentID
	}

	if in.Body != "" {
		bodyType := in.BodyType
		if bodyType == "" {
			bodyType = "storage"
		}
		body["body"] = map[string]any{
			bodyType: map[string]any{
				"representation": bodyType,
				"value":          in.Body,
			},
		}
	}

	var created Page
	if err := c.doJSON(ctx, "POST", "wiki/api/v2/pages", nil, body, &created); err != nil {
		return nil, err
	}
	return &created, nil
}

// UpdatePageInput describes the fields to change on an existing page.
type UpdatePageInput struct {
	Title       *string
	Body        *string // content in storage format or plain text
	BodyType    string  // "storage" or "atlas_doc_format"
	Version     int     // current version number (required for updates)
	VersionNote string  // optional version message
}

// UpdatePage updates the given fields on an existing page.
func (c *Client) UpdatePage(ctx context.Context, pageID string, in UpdatePageInput) (*Page, error) {
	body := map[string]any{
		"id":      pageID,
		"status":  "current",
		"version": map[string]any{"number": in.Version},
	}

	if in.VersionNote != "" {
		body["version"].(map[string]any)["message"] = in.VersionNote
	}

	if in.Title != nil {
		body["title"] = *in.Title
	}

	if in.Body != nil {
		bodyType := in.BodyType
		if bodyType == "" {
			bodyType = "storage"
		}
		body["body"] = map[string]any{
			bodyType: map[string]any{
				"representation": bodyType,
				"value":          *in.Body,
			},
		}
	}

	if in.Title == nil && in.Body == nil {
		return nil, fmt.Errorf("confluence: UpdatePage requires at least title or body to update")
	}

	var updated Page
	if err := c.doJSON(ctx, "PUT", "wiki/api/v2/pages/"+url.PathEscape(pageID), nil, body, &updated); err != nil {
		return nil, err
	}
	return &updated, nil
}

// DeletePage deletes a page by ID.
func (c *Client) DeletePage(ctx context.Context, pageID string) error {
	return c.doJSON(ctx, "DELETE", "wiki/api/v2/pages/"+url.PathEscape(pageID), nil, nil, nil)
}

// GetPageLabels retrieves labels for a page.
func (c *Client) GetPageLabels(ctx context.Context, pageID string, limit int) (*LabelSearchResult, error) {
	query := url.Values{}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}

	var result LabelSearchResult
	if err := c.doJSON(ctx, "GET", "wiki/api/v2/pages/"+url.PathEscape(pageID)+"/labels", query, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// AddPageLabel adds a label to a page.
func (c *Client) AddPageLabel(ctx context.Context, pageID, labelName string) error {
	body := []map[string]any{
		{
			"prefix": "global",
			"name":   labelName,
		},
	}
	return c.doJSON(ctx, "POST", "wiki/api/v2/pages/"+url.PathEscape(pageID)+"/labels", nil, body, nil)
}

// StorageToPlainText extracts plain text from a storage format body.
func (p *Page) StorageToPlainText() string {
	return bodyToPlainText(p.Body)
}
