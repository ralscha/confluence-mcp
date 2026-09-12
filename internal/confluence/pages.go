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
func (c *Client) GetPage(ctx context.Context, pageID, bodyFormat string) (*Page, error) {
	query := url.Values{}
	if bodyFormat != "" {
		query.Set("body-format", bodyFormat)
	}

	var page Page
	if err := c.doJSON(ctx, "GET", "wiki/api/v2/pages/"+url.PathEscape(pageID), query, nil, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// SearchPagesInput describes parameters for searching pages.
type SearchPagesInput struct {
	SpaceID    string
	Title      string
	Status     string
	Sort       string
	BodyFormat string
	Limit      int
	Cursor     string
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
	if in.Sort != "" {
		query.Set("sort", in.Sort)
	}
	if in.BodyFormat != "" {
		query.Set("body-format", in.BodyFormat)
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
	BodyType string // "storage", "atlas_doc_format", or "plain_text"
}

// CreatePage creates a new page and returns the created page.
func (c *Client) CreatePage(ctx context.Context, in CreatePageInput) (*Page, error) {
	if strings.TrimSpace(in.SpaceID) == "" || strings.TrimSpace(in.Title) == "" {
		return nil, fmt.Errorf("confluence: CreatePage requires a space ID and a non-blank title")
	}
	pageBody, err := bodyForWrite(in.Body, in.BodyType)
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"spaceId": in.SpaceID,
		"status":  "current",
		"title":   in.Title,
		"body":    pageBody,
	}

	if in.ParentID != "" {
		body["parentId"] = in.ParentID
	}

	var created Page
	if err := c.doJSON(ctx, "POST", "wiki/api/v2/pages", nil, body, &created); err != nil {
		return nil, err
	}
	if created.ID == "" {
		return nil, fmt.Errorf("confluence: create response has no page ID; verify whether the page was created before retrying")
	}
	return &created, nil
}

// UpdatePageInput describes the fields to change on an existing page.
type UpdatePageInput struct {
	Title       *string
	Body        *string // content in storage format or plain text
	BodyType    string  // "storage", "atlas_doc_format", or "plain_text"
	Version     int     // current version number (required for updates)
	VersionNote string  // optional version message
}

// UpdatePage updates the given fields on an existing page. Version is the
// current version number; Confluence expects the request to contain the next
// version number, so it is incremented here.
func (c *Client) UpdatePage(ctx context.Context, pageID string, in UpdatePageInput) (*Page, error) {
	if strings.TrimSpace(pageID) == "" {
		return nil, fmt.Errorf("confluence: UpdatePage requires a page ID")
	}
	if in.Title != nil && strings.TrimSpace(*in.Title) == "" {
		return nil, fmt.Errorf("confluence: page title must not be blank")
	}
	if in.Title == nil && in.Body == nil {
		return nil, fmt.Errorf("confluence: UpdatePage requires at least title or body to update")
	}
	if in.Body == nil {
		if in.Version != 0 || in.VersionNote != "" || in.BodyType != "" {
			return nil, fmt.Errorf("confluence: title-only updates do not support version, version_note, or body_type; provide content for a versioned update")
		}
		var updated Page
		body := map[string]any{"status": "current", "title": *in.Title}
		if err := c.doJSON(ctx, "PUT", "wiki/api/v2/pages/"+url.PathEscape(pageID)+"/title", nil, body, &updated); err != nil {
			return nil, err
		}
		if updated.ID != pageID {
			return nil, fmt.Errorf("confluence: update response has a missing or unexpected page ID; read the page before retrying")
		}
		return &updated, nil
	}
	if in.Version < 1 || in.Version == int(^uint(0)>>1) {
		return nil, fmt.Errorf("confluence: UpdatePage requires the current version number when updating content")
	}
	pageBody, err := bodyForWrite(*in.Body, in.BodyType)
	if err != nil {
		return nil, err
	}

	title := in.Title
	if title == nil {
		current, err := c.GetPage(ctx, pageID, "")
		if err != nil {
			return nil, fmt.Errorf("confluence: retrieving current page title: %w", err)
		}
		title = &current.Title
	}

	version := map[string]any{"number": in.Version + 1}
	if in.VersionNote != "" {
		version["message"] = in.VersionNote
	}
	body := map[string]any{
		"id":      pageID,
		"status":  "current",
		"title":   *title,
		"body":    pageBody,
		"version": version,
	}

	var updated Page
	if err := c.doJSON(ctx, "PUT", "wiki/api/v2/pages/"+url.PathEscape(pageID), nil, body, &updated); err != nil {
		return nil, err
	}
	if updated.ID != pageID {
		return nil, fmt.Errorf("confluence: update response has a missing or unexpected page ID; read the page before retrying")
	}
	return &updated, nil
}

// ListPageVersionsInput describes parameters for listing a page's history.
type ListPageVersionsInput struct {
	PageID     string
	BodyFormat string
	Sort       string
	Limit      int
	Cursor     string
}

// ListPageVersions returns the version history of a page.
func (c *Client) ListPageVersions(ctx context.Context, in ListPageVersionsInput) (*PageVersionSearchResult, error) {
	query := url.Values{}
	if in.BodyFormat != "" {
		query.Set("body-format", in.BodyFormat)
	}
	if in.Sort != "" {
		query.Set("sort", in.Sort)
	}
	if in.Limit > 0 {
		query.Set("limit", strconv.Itoa(in.Limit))
	}
	if in.Cursor != "" {
		query.Set("cursor", in.Cursor)
	}

	var result PageVersionSearchResult
	if err := c.doJSON(ctx, "GET", "wiki/api/v2/pages/"+url.PathEscape(in.PageID)+"/versions", query, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeletePage deletes a page by ID.
func (c *Client) DeletePage(ctx context.Context, pageID string) error {
	return c.doJSON(ctx, "DELETE", "wiki/api/v2/pages/"+url.PathEscape(pageID), nil, nil, nil)
}

// GetPageChildrenInput describes parameters for listing direct child content.
type GetPageChildrenInput struct {
	PageID string
	Sort   string
	Limit  int
	Cursor string
}

// GetPageChildren lists direct child content of a page.
func (c *Client) GetPageChildren(ctx context.Context, in GetPageChildrenInput) (*ChildPageSearchResult, error) {
	query := url.Values{}
	if in.Sort != "" {
		query.Set("sort", in.Sort)
	}
	if in.Limit > 0 {
		query.Set("limit", strconv.Itoa(in.Limit))
	}
	if in.Cursor != "" {
		query.Set("cursor", in.Cursor)
	}

	var result ChildPageSearchResult
	if err := c.doJSON(ctx, "GET", "wiki/api/v2/pages/"+url.PathEscape(in.PageID)+"/direct-children", query, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetPageAncestors lists a page's ancestors, ordered from the root downwards.
func (c *Client) GetPageAncestors(ctx context.Context, pageID string, limit int) (*AncestorSearchResult, error) {
	query := url.Values{}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}

	var result AncestorSearchResult
	if err := c.doJSON(ctx, "GET", "wiki/api/v2/pages/"+url.PathEscape(pageID)+"/ancestors", query, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetSpacePagesInput describes parameters for listing the pages in a space.
type GetSpacePagesInput struct {
	SpaceID    string
	Title      string
	Status     []string
	Sort       string
	BodyFormat string
	Limit      int
	Cursor     string
}

// GetSpacePages lists the pages in a space.
func (c *Client) GetSpacePages(ctx context.Context, in GetSpacePagesInput) (*PageSearchResult, error) {
	query := url.Values{}
	if in.Title != "" {
		query.Set("title", in.Title)
	}
	for _, status := range in.Status {
		query.Add("status", status)
	}
	if in.Sort != "" {
		query.Set("sort", in.Sort)
	}
	if in.BodyFormat != "" {
		query.Set("body-format", in.BodyFormat)
	}
	if in.Limit > 0 {
		query.Set("limit", strconv.Itoa(in.Limit))
	}
	if in.Cursor != "" {
		query.Set("cursor", in.Cursor)
	}

	var result PageSearchResult
	if err := c.doJSON(ctx, "GET", "wiki/api/v2/spaces/"+url.PathEscape(in.SpaceID)+"/pages", query, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetPageLabelsInput describes parameters for listing a page's labels.
type GetPageLabelsInput struct {
	PageID string
	Prefix string
	Sort   string
	Limit  int
	Cursor string
}

// GetPageLabels retrieves labels for a page.
func (c *Client) GetPageLabels(ctx context.Context, in GetPageLabelsInput) (*LabelSearchResult, error) {
	query := url.Values{}
	if in.Prefix != "" {
		query.Set("prefix", in.Prefix)
	}
	if in.Sort != "" {
		query.Set("sort", in.Sort)
	}
	if in.Limit > 0 {
		query.Set("limit", strconv.Itoa(in.Limit))
	}
	if in.Cursor != "" {
		query.Set("cursor", in.Cursor)
	}

	var result LabelSearchResult
	if err := c.doJSON(ctx, "GET", "wiki/api/v2/pages/"+url.PathEscape(in.PageID)+"/labels", query, nil, &result); err != nil {
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
	return c.doJSON(ctx, "POST", "wiki/rest/api/content/"+url.PathEscape(pageID)+"/label", nil, body, nil)
}

// RemovePageLabel removes a label from a page. Label removal is only exposed
// by the v1 content-label endpoint.
func (c *Client) RemovePageLabel(ctx context.Context, pageID, labelName string) error {
	query := url.Values{"name": {labelName}}
	return c.doJSON(ctx, "DELETE", "wiki/rest/api/content/"+url.PathEscape(pageID)+"/label", query, nil, nil)
}

// StorageToPlainText extracts plain text from a storage format body.
func (p *Page) StorageToPlainText() string {
	return bodyToPlainText(p.Body)
}
