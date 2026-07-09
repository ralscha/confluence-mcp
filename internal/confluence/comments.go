package confluence

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

const (
	// CommentTypeFooter is a footer comment.
	CommentTypeFooter = "footer"
	// CommentTypeInline is an inline comment.
	CommentTypeInline = "inline"
)

// ListPageCommentsInput describes parameters for listing page comments.
type ListPageCommentsInput struct {
	PageID           string
	CommentType      string
	BodyFormat       string
	Status           []string
	ResolutionStatus []string
	Sort             string
	Limit            int
	Cursor           string
}

// ListPageComments lists root footer or inline comments for a page.
func (c *Client) ListPageComments(ctx context.Context, in ListPageCommentsInput) (*CommentSearchResult, error) {
	commentType := normalizeCommentType(in.CommentType)
	pathSuffix, err := pageCommentPathSuffix(commentType)
	if err != nil {
		return nil, err
	}
	if commentType != CommentTypeInline && len(in.ResolutionStatus) > 0 {
		return nil, fmt.Errorf("confluence: resolution status filters are only valid for inline comments")
	}

	query := commentListQuery(in.BodyFormat, in.Status, in.Sort, in.Limit, in.Cursor)
	if commentType == CommentTypeInline {
		for _, status := range in.ResolutionStatus {
			query.Add("resolution-status", status)
		}
	}

	var result CommentSearchResult
	if err := c.doJSON(ctx, "GET", "wiki/api/v2/pages/"+url.PathEscape(in.PageID)+"/"+pathSuffix, query, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetCommentInput describes parameters for retrieving a comment by ID.
type GetCommentInput struct {
	CommentID   string
	CommentType string
	BodyFormat  string
}

// GetComment fetches a footer or inline comment by ID.
func (c *Client) GetComment(ctx context.Context, in GetCommentInput) (*Comment, error) {
	commentType := normalizeCommentType(in.CommentType)
	basePath, err := commentBasePath(commentType)
	if err != nil {
		return nil, err
	}

	query := url.Values{}
	if in.BodyFormat != "" {
		query.Set("body-format", in.BodyFormat)
	}

	var comment Comment
	if err := c.doJSON(ctx, "GET", basePath+"/"+url.PathEscape(in.CommentID), query, nil, &comment); err != nil {
		return nil, err
	}
	return &comment, nil
}

// ListCommentChildrenInput describes parameters for listing comment replies.
type ListCommentChildrenInput struct {
	CommentID   string
	CommentType string
	BodyFormat  string
	Sort        string
	Limit       int
	Cursor      string
}

// ListCommentChildren lists replies to a footer or inline comment.
func (c *Client) ListCommentChildren(ctx context.Context, in ListCommentChildrenInput) (*CommentSearchResult, error) {
	commentType := normalizeCommentType(in.CommentType)
	basePath, err := commentBasePath(commentType)
	if err != nil {
		return nil, err
	}

	query := commentListQuery(in.BodyFormat, nil, in.Sort, in.Limit, in.Cursor)

	var result CommentSearchResult
	if err := c.doJSON(ctx, "GET", basePath+"/"+url.PathEscape(in.CommentID)+"/children", query, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateFooterCommentInput describes fields for a new footer comment.
type CreateFooterCommentInput struct {
	PageID          string
	ParentCommentID string
	Body            string
	BodyType        string
}

// CreateFooterComment creates a top-level page footer comment or a reply.
func (c *Client) CreateFooterComment(ctx context.Context, in CreateFooterCommentInput) (*Comment, error) {
	if in.PageID == "" && in.ParentCommentID == "" {
		return nil, fmt.Errorf("confluence: CreateFooterComment requires page ID or parent comment ID")
	}

	bodyType := in.BodyType
	if bodyType == "" {
		bodyType = "storage"
	}
	body := map[string]any{
		"body": map[string]any{
			"representation": bodyType,
			"value":          in.Body,
		},
	}
	if in.PageID != "" {
		body["pageId"] = in.PageID
	}
	if in.ParentCommentID != "" {
		body["parentCommentId"] = in.ParentCommentID
	}

	var created Comment
	if err := c.doJSON(ctx, "POST", "wiki/api/v2/footer-comments", nil, body, &created); err != nil {
		return nil, err
	}
	return &created, nil
}

// UpdateFooterCommentInput describes fields for updating a footer comment.
type UpdateFooterCommentInput struct {
	Body        string
	BodyType    string
	Version     int
	VersionNote string
}

// UpdateFooterComment updates the body of an existing footer comment.
func (c *Client) UpdateFooterComment(ctx context.Context, commentID string, in UpdateFooterCommentInput) (*Comment, error) {
	bodyType := in.BodyType
	if bodyType == "" {
		bodyType = "storage"
	}
	body := map[string]any{
		"version": map[string]any{"number": in.Version},
		"body": map[string]any{
			"representation": bodyType,
			"value":          in.Body,
		},
	}
	if in.VersionNote != "" {
		body["version"].(map[string]any)["message"] = in.VersionNote
	}

	var updated Comment
	if err := c.doJSON(ctx, "PUT", "wiki/api/v2/footer-comments/"+url.PathEscape(commentID), nil, body, &updated); err != nil {
		return nil, err
	}
	return &updated, nil
}

// DeleteFooterComment permanently deletes a footer comment.
func (c *Client) DeleteFooterComment(ctx context.Context, commentID string) error {
	return c.doJSON(ctx, "DELETE", "wiki/api/v2/footer-comments/"+url.PathEscape(commentID), nil, nil, nil)
}

// PlainText extracts a best-effort plain text version of the comment body.
func (c *Comment) PlainText() string {
	return bodyToPlainText(c.Body)
}

func normalizeCommentType(commentType string) string {
	if commentType == "" {
		return CommentTypeFooter
	}
	return commentType
}

func pageCommentPathSuffix(commentType string) (string, error) {
	switch commentType {
	case CommentTypeFooter:
		return "footer-comments", nil
	case CommentTypeInline:
		return "inline-comments", nil
	default:
		return "", fmt.Errorf("confluence: unsupported comment type %q", commentType)
	}
}

func commentBasePath(commentType string) (string, error) {
	switch commentType {
	case CommentTypeFooter:
		return "wiki/api/v2/footer-comments", nil
	case CommentTypeInline:
		return "wiki/api/v2/inline-comments", nil
	default:
		return "", fmt.Errorf("confluence: unsupported comment type %q", commentType)
	}
}

func commentListQuery(bodyFormat string, status []string, sort string, limit int, cursor string) url.Values {
	query := url.Values{}
	if bodyFormat != "" {
		query.Set("body-format", bodyFormat)
	}
	for _, s := range status {
		query.Add("status", s)
	}
	if sort != "" {
		query.Set("sort", sort)
	}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	return query
}
