// Package mcpserver builds the confluence-mcp MCP server, registering Confluence tools
// against the official MCP Go SDK. Read tools are always registered; write
// tools are only registered when the server is running in readwrite mode.
package mcpserver

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"confluence-mcp/internal/confluence"
)

// readOnlyHint is shared by all read-only tool registrations.
var readOnlyHint = &mcp.ToolAnnotations{ReadOnlyHint: true}

func nextCursor(nextLink string) string {
	if nextLink == "" {
		return ""
	}
	u, err := url.Parse(nextLink)
	if err != nil {
		return nextLink
	}
	if cursor := u.Query().Get("cursor"); cursor != "" {
		return cursor
	}
	return nextLink
}

// PageSummary is a flattened, human-readable view of a Confluence page.
type PageSummary struct {
	ID       string `json:"id" jsonschema:"the page ID"`
	Title    string `json:"title,omitempty" jsonschema:"the page title"`
	Status   string `json:"status,omitempty" jsonschema:"the page status (current, archived)"`
	SpaceID  string `json:"space_id,omitempty" jsonschema:"the space ID"`
	ParentID string `json:"parent_id,omitempty" jsonschema:"the parent page ID, if any"`
	Version  int    `json:"version,omitempty" jsonschema:"the current version number"`
	Content  string `json:"content,omitempty" jsonschema:"the page content as plain text"`
	WebURL   string `json:"web_url,omitempty" jsonschema:"the URL to view the page in a browser"`
}

func pageToSummary(page *confluence.Page) PageSummary {
	s := PageSummary{
		ID:       page.ID,
		Title:    page.Title,
		Status:   page.Status,
		SpaceID:  page.SpaceID,
		ParentID: page.ParentID,
		Content:  page.StorageToPlainText(),
		WebURL:   page.Links.WebUI,
	}
	if page.Version != nil {
		s.Version = page.Version.Number
	}
	return s
}

// GetPageInput is the input for the confluence_get_page tool.
type GetPageInput struct {
	PageID     string   `json:"page_id" jsonschema:"the Confluence page ID"`
	BodyFormat []string `json:"body_format,omitempty" jsonschema:"optional list of body formats to include (storage, atlas_doc_format, view)"`
}

func getPage(client *confluence.Client) mcp.ToolHandlerFor[GetPageInput, PageSummary] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in GetPageInput) (*mcp.CallToolResult, PageSummary, error) {
		page, err := client.GetPage(ctx, in.PageID, in.BodyFormat)
		if err != nil {
			return nil, PageSummary{}, fmt.Errorf("get page %s: %w", in.PageID, err)
		}
		return nil, pageToSummary(page), nil
	}
}

// SearchPagesInput is the input for the confluence_search_pages tool.
type SearchPagesInput struct {
	SpaceID string `json:"space_id,omitempty" jsonschema:"filter by space ID"`
	Title   string `json:"title,omitempty" jsonschema:"filter by page title (partial match)"`
	Status  string `json:"status,omitempty" jsonschema:"filter by status (current, archived)"`
	Limit   int    `json:"limit,omitempty" jsonschema:"maximum number of results to return, defaults to 25"`
	Cursor  string `json:"cursor,omitempty" jsonschema:"pagination cursor for next page of results"`
}

// SearchPagesOutput is the output for the confluence_search_pages tool.
type SearchPagesOutput struct {
	Pages      []PageSummary `json:"pages" jsonschema:"the matching pages"`
	NextCursor string        `json:"next_cursor,omitempty" jsonschema:"cursor for the next page of results, if available"`
}

func searchPages(client *confluence.Client) mcp.ToolHandlerFor[SearchPagesInput, SearchPagesOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in SearchPagesInput) (*mcp.CallToolResult, SearchPagesOutput, error) {
		limit := in.Limit
		if limit <= 0 {
			limit = 25
		}

		result, err := client.SearchPages(ctx, confluence.SearchPagesInput{
			SpaceID: in.SpaceID,
			Title:   in.Title,
			Status:  in.Status,
			Limit:   limit,
			Cursor:  in.Cursor,
		})
		if err != nil {
			return nil, SearchPagesOutput{}, fmt.Errorf("search pages: %w", err)
		}

		out := SearchPagesOutput{
			Pages:      make([]PageSummary, len(result.Results)),
			NextCursor: nextCursor(result.Links.Next),
		}
		for i := range result.Results {
			out.Pages[i] = pageToSummary(&result.Results[i])
		}
		return nil, out, nil
	}
}

// SearchCQLInput is the input for the confluence_search_cql tool.
type SearchCQLInput struct {
	CQL                   string   `json:"cql" jsonschema:"the Confluence Query Language expression, e.g. type=page AND space=ENG"`
	CQLContext            string   `json:"cql_context,omitempty" jsonschema:"optional CQL context JSON for app-aware searches"`
	Expand                []string `json:"expand,omitempty" jsonschema:"optional expand fields, e.g. body.storage, version, space"`
	Excerpt               string   `json:"excerpt,omitempty" jsonschema:"excerpt mode to request from Confluence"`
	IncludeArchivedSpaces bool     `json:"include_archived_spaces,omitempty" jsonschema:"include results from archived spaces"`
	ExcludeCurrentSpaces  bool     `json:"exclude_current_spaces,omitempty" jsonschema:"exclude results from current spaces"`
	Limit                 int      `json:"limit,omitempty" jsonschema:"maximum number of results to return, defaults to 25"`
	Start                 int      `json:"start,omitempty" jsonschema:"offset for APIs that still support start pagination"`
	Cursor                string   `json:"cursor,omitempty" jsonschema:"pagination cursor for next page of results"`
}

// SearchCQLResultSummary is a compact summary of a CQL search hit.
type SearchCQLResultSummary struct {
	ID                   string  `json:"id,omitempty" jsonschema:"the content ID"`
	Type                 string  `json:"type,omitempty" jsonschema:"the content type, e.g. page, blogpost, comment"`
	Status               string  `json:"status,omitempty" jsonschema:"the content status"`
	Title                string  `json:"title,omitempty" jsonschema:"the result title"`
	SpaceKey             string  `json:"space_key,omitempty" jsonschema:"the space key"`
	SpaceName            string  `json:"space_name,omitempty" jsonschema:"the space name"`
	Excerpt              string  `json:"excerpt,omitempty" jsonschema:"the result excerpt returned by Confluence"`
	Content              string  `json:"content,omitempty" jsonschema:"plain text content if body expansion was requested"`
	URL                  string  `json:"url,omitempty" jsonschema:"the result URL returned by Confluence"`
	EntityType           string  `json:"entity_type,omitempty" jsonschema:"the search entity type"`
	LastModified         string  `json:"last_modified,omitempty" jsonschema:"when the result was last modified"`
	FriendlyLastModified string  `json:"friendly_last_modified,omitempty" jsonschema:"human-readable last modified text"`
	Score                float64 `json:"score,omitempty" jsonschema:"search relevance score"`
}

// SearchCQLOutput is the output for the confluence_search_cql tool.
type SearchCQLOutput struct {
	Results             []SearchCQLResultSummary `json:"results" jsonschema:"the matching search results"`
	NextCursor          string                   `json:"next_cursor,omitempty" jsonschema:"cursor for the next page of results, if available"`
	PreviousCursor      string                   `json:"previous_cursor,omitempty" jsonschema:"cursor for the previous page of results, if available"`
	Size                int                      `json:"size,omitempty" jsonschema:"number of results in this response"`
	TotalSize           int                      `json:"total_size,omitempty" jsonschema:"total number of results, if returned by Confluence"`
	CQLQuery            string                   `json:"cql_query,omitempty" jsonschema:"the CQL query executed by Confluence"`
	SearchDuration      int                      `json:"search_duration_ms,omitempty" jsonschema:"search duration reported by Confluence, in milliseconds"`
	ArchivedResultCount int                      `json:"archived_result_count,omitempty" jsonschema:"number of archived results reported by Confluence"`
}

func cqlResultToSummary(result *confluence.ContentSearchItem) SearchCQLResultSummary {
	s := SearchCQLResultSummary{
		Title:                result.Title,
		Excerpt:              result.Excerpt,
		URL:                  result.URL,
		EntityType:           result.EntityType,
		LastModified:         result.LastModified,
		FriendlyLastModified: result.FriendlyLastModified,
		Score:                result.Score,
	}
	if result.Content != nil {
		s.ID = result.Content.ID
		s.Type = result.Content.Type
		s.Status = result.Content.Status
		if s.Title == "" {
			s.Title = result.Content.Title
		}
		if s.URL == "" {
			s.URL = result.Content.Links.WebUI
		}
		if result.Content.Body != nil {
			s.Content = result.Content.Body.PlainText()
		}
		if result.Content.Space != nil {
			s.SpaceKey = result.Content.Space.Key
			s.SpaceName = result.Content.Space.Name
		}
	}
	return s
}

func searchCQL(client *confluence.Client) mcp.ToolHandlerFor[SearchCQLInput, SearchCQLOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in SearchCQLInput) (*mcp.CallToolResult, SearchCQLOutput, error) {
		limit := in.Limit
		if limit <= 0 {
			limit = 25
		}

		result, err := client.SearchContent(ctx, confluence.SearchContentInput{
			CQL:                   in.CQL,
			CQLContext:            in.CQLContext,
			Expand:                in.Expand,
			Cursor:                in.Cursor,
			Limit:                 limit,
			Start:                 in.Start,
			IncludeArchivedSpaces: in.IncludeArchivedSpaces,
			ExcludeCurrentSpaces:  in.ExcludeCurrentSpaces,
			Excerpt:               in.Excerpt,
		})
		if err != nil {
			return nil, SearchCQLOutput{}, fmt.Errorf("search CQL: %w", err)
		}

		out := SearchCQLOutput{
			Results:             make([]SearchCQLResultSummary, len(result.Results)),
			NextCursor:          nextCursor(result.Links.Next),
			PreviousCursor:      nextCursor(result.Links.Prev),
			Size:                result.Size,
			TotalSize:           result.TotalSize,
			CQLQuery:            result.CQLQuery,
			SearchDuration:      result.SearchDuration,
			ArchivedResultCount: result.ArchivedResultCount,
		}
		for i := range result.Results {
			out.Results[i] = cqlResultToSummary(&result.Results[i])
		}
		return nil, out, nil
	}
}

// SpaceSummary is a flattened view of a Confluence space.
type SpaceSummary struct {
	ID     string `json:"id" jsonschema:"the space ID"`
	Key    string `json:"key" jsonschema:"the space key"`
	Name   string `json:"name,omitempty" jsonschema:"the space name"`
	Type   string `json:"type,omitempty" jsonschema:"the space type (global, personal)"`
	Status string `json:"status,omitempty" jsonschema:"the space status (current, archived)"`
	WebURL string `json:"web_url,omitempty" jsonschema:"the URL to view the space in a browser"`
}

func spaceToSummary(s *confluence.Space) SpaceSummary {
	return SpaceSummary{
		ID:     s.ID,
		Key:    s.Key,
		Name:   s.Name,
		Type:   s.Type,
		Status: s.Status,
		WebURL: s.Links.WebUI,
	}
}

// GetSpaceInput is the input for the confluence_get_space tool.
type GetSpaceInput struct {
	SpaceKeyOrID string `json:"space_key_or_id" jsonschema:"the Confluence space key or ID"`
}

func getSpace(client *confluence.Client) mcp.ToolHandlerFor[GetSpaceInput, SpaceSummary] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in GetSpaceInput) (*mcp.CallToolResult, SpaceSummary, error) {
		space, err := client.GetSpace(ctx, in.SpaceKeyOrID)
		if err != nil {
			return nil, SpaceSummary{}, fmt.Errorf("get space %s: %w", in.SpaceKeyOrID, err)
		}
		return nil, spaceToSummary(space), nil
	}
}

// ListSpacesInput is the input for the confluence_list_spaces tool.
type ListSpacesInput struct {
	Keys   []string `json:"keys,omitempty" jsonschema:"filter by space keys"`
	Type   string   `json:"type,omitempty" jsonschema:"filter by type (global, personal)"`
	Status string   `json:"status,omitempty" jsonschema:"filter by status (current, archived)"`
	Limit  int      `json:"limit,omitempty" jsonschema:"maximum number of results to return, defaults to 25"`
	Cursor string   `json:"cursor,omitempty" jsonschema:"pagination cursor for next page of results"`
}

// ListSpacesOutput is the output for the confluence_list_spaces tool.
type ListSpacesOutput struct {
	Spaces     []SpaceSummary `json:"spaces" jsonschema:"the matching spaces"`
	NextCursor string         `json:"next_cursor,omitempty" jsonschema:"cursor for the next page of results, if available"`
}

func listSpaces(client *confluence.Client) mcp.ToolHandlerFor[ListSpacesInput, ListSpacesOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListSpacesInput) (*mcp.CallToolResult, ListSpacesOutput, error) {
		limit := in.Limit
		if limit <= 0 {
			limit = 25
		}

		result, err := client.ListSpaces(ctx, confluence.ListSpacesInput{
			Keys:   in.Keys,
			Type:   in.Type,
			Status: in.Status,
			Limit:  limit,
			Cursor: in.Cursor,
		})
		if err != nil {
			return nil, ListSpacesOutput{}, fmt.Errorf("list spaces: %w", err)
		}

		out := ListSpacesOutput{
			Spaces:     make([]SpaceSummary, len(result.Results)),
			NextCursor: nextCursor(result.Links.Next),
		}
		for i := range result.Results {
			out.Spaces[i] = spaceToSummary(&result.Results[i])
		}
		return nil, out, nil
	}
}

// GetPageLabelsInput is the input for the confluence_get_page_labels tool.
type GetPageLabelsInput struct {
	PageID string `json:"page_id" jsonschema:"the Confluence page ID"`
	Limit  int    `json:"limit,omitempty" jsonschema:"maximum number of labels to return, defaults to 25"`
}

// GetPageLabelsOutput is the output for the confluence_get_page_labels tool.
type GetPageLabelsOutput struct {
	Labels []string `json:"labels" jsonschema:"the label names attached to the page"`
}

func getPageLabels(client *confluence.Client) mcp.ToolHandlerFor[GetPageLabelsInput, GetPageLabelsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in GetPageLabelsInput) (*mcp.CallToolResult, GetPageLabelsOutput, error) {
		limit := in.Limit
		if limit <= 0 {
			limit = 25
		}

		result, err := client.GetPageLabels(ctx, in.PageID, limit)
		if err != nil {
			return nil, GetPageLabelsOutput{}, fmt.Errorf("get page labels %s: %w", in.PageID, err)
		}

		labels := make([]string, len(result.Results))
		for i, label := range result.Results {
			labels[i] = label.Name
		}
		return nil, GetPageLabelsOutput{Labels: labels}, nil
	}
}

// CommentSummary is a flattened view of a Confluence comment.
type CommentSummary struct {
	ID                      string `json:"id" jsonschema:"the comment ID"`
	Type                    string `json:"type,omitempty" jsonschema:"the comment type: footer or inline"`
	Status                  string `json:"status,omitempty" jsonschema:"the comment status"`
	Title                   string `json:"title,omitempty" jsonschema:"the comment title"`
	PageID                  string `json:"page_id,omitempty" jsonschema:"the page ID the comment belongs to"`
	ParentCommentID         string `json:"parent_comment_id,omitempty" jsonschema:"the parent comment ID, if this is a reply"`
	Version                 int    `json:"version,omitempty" jsonschema:"the current comment version number"`
	CreatedAt               string `json:"created_at,omitempty" jsonschema:"when this comment version was created"`
	AuthorID                string `json:"author_id,omitempty" jsonschema:"the author account ID"`
	Content                 string `json:"content,omitempty" jsonschema:"the comment body as plain text"`
	ResolutionStatus        string `json:"resolution_status,omitempty" jsonschema:"inline comment resolution status"`
	InlineMarkerRef         string `json:"inline_marker_ref,omitempty" jsonschema:"inline comment marker reference"`
	InlineOriginalSelection string `json:"inline_original_selection,omitempty" jsonschema:"the originally selected text for an inline comment"`
	WebURL                  string `json:"web_url,omitempty" jsonschema:"the URL to view the comment in a browser"`
}

func commentToSummary(comment *confluence.Comment, commentType string) CommentSummary {
	s := CommentSummary{
		ID:               comment.ID,
		Type:             commentType,
		Status:           comment.Status,
		Title:            comment.Title,
		PageID:           comment.PageID,
		ParentCommentID:  comment.ParentCommentID,
		Content:          comment.PlainText(),
		ResolutionStatus: comment.ResolutionStatus,
		WebURL:           comment.Links.WebUI,
	}
	if comment.Version != nil {
		s.Version = comment.Version.Number
		s.CreatedAt = comment.Version.CreatedAt
		if s.CreatedAt == "" {
			s.CreatedAt = comment.Version.When
		}
		s.AuthorID = comment.Version.AuthorID
		if s.AuthorID == "" && comment.Version.By != nil {
			s.AuthorID = comment.Version.By.AccountID
		}
	}
	if comment.InlineCommentProperties != nil {
		s.InlineMarkerRef = comment.InlineCommentProperties.InlineMarkerRef
		s.InlineOriginalSelection = comment.InlineCommentProperties.InlineOriginalSelection
	}
	return s
}

// ListPageCommentsInput is the input for the confluence_list_page_comments tool.
type ListPageCommentsInput struct {
	PageID           string   `json:"page_id" jsonschema:"the Confluence page ID"`
	CommentType      string   `json:"comment_type,omitempty" jsonschema:"comment type to list: footer (default) or inline"`
	BodyFormat       string   `json:"body_format,omitempty" jsonschema:"optional body format to include (storage, atlas_doc_format, view)"`
	Status           []string `json:"status,omitempty" jsonschema:"optional comment statuses to include"`
	ResolutionStatus []string `json:"resolution_status,omitempty" jsonschema:"optional inline resolution statuses to include"`
	Sort             string   `json:"sort,omitempty" jsonschema:"optional Confluence comment sort order"`
	Limit            int      `json:"limit,omitempty" jsonschema:"maximum number of comments to return, defaults to 25"`
	Cursor           string   `json:"cursor,omitempty" jsonschema:"pagination cursor for next page of results"`
}

// ListPageCommentsOutput is the output for the confluence_list_page_comments tool.
type ListPageCommentsOutput struct {
	Comments   []CommentSummary `json:"comments" jsonschema:"the matching comments"`
	NextCursor string           `json:"next_cursor,omitempty" jsonschema:"cursor for the next page of results, if available"`
}

func listPageComments(client *confluence.Client) mcp.ToolHandlerFor[ListPageCommentsInput, ListPageCommentsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListPageCommentsInput) (*mcp.CallToolResult, ListPageCommentsOutput, error) {
		limit := in.Limit
		if limit <= 0 {
			limit = 25
		}
		commentType := in.CommentType
		if commentType == "" {
			commentType = confluence.CommentTypeFooter
		}

		result, err := client.ListPageComments(ctx, confluence.ListPageCommentsInput{
			PageID:           in.PageID,
			CommentType:      commentType,
			BodyFormat:       in.BodyFormat,
			Status:           in.Status,
			ResolutionStatus: in.ResolutionStatus,
			Sort:             in.Sort,
			Limit:            limit,
			Cursor:           in.Cursor,
		})
		if err != nil {
			return nil, ListPageCommentsOutput{}, fmt.Errorf("list %s comments for page %s: %w", commentType, in.PageID, err)
		}

		out := ListPageCommentsOutput{
			Comments:   make([]CommentSummary, len(result.Results)),
			NextCursor: nextCursor(result.Links.Next),
		}
		for i := range result.Results {
			out.Comments[i] = commentToSummary(&result.Results[i], commentType)
		}
		return nil, out, nil
	}
}

// GetCommentInput is the input for the confluence_get_comment tool.
type GetCommentInput struct {
	CommentID   string `json:"comment_id" jsonschema:"the Confluence comment ID"`
	CommentType string `json:"comment_type,omitempty" jsonschema:"comment type: footer (default) or inline"`
	BodyFormat  string `json:"body_format,omitempty" jsonschema:"optional body format to include (storage, atlas_doc_format, view)"`
}

func getComment(client *confluence.Client) mcp.ToolHandlerFor[GetCommentInput, CommentSummary] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in GetCommentInput) (*mcp.CallToolResult, CommentSummary, error) {
		commentType := in.CommentType
		if commentType == "" {
			commentType = confluence.CommentTypeFooter
		}

		comment, err := client.GetComment(ctx, confluence.GetCommentInput{
			CommentID:   in.CommentID,
			CommentType: commentType,
			BodyFormat:  in.BodyFormat,
		})
		if err != nil {
			return nil, CommentSummary{}, fmt.Errorf("get %s comment %s: %w", commentType, in.CommentID, err)
		}
		return nil, commentToSummary(comment, commentType), nil
	}
}

// ListCommentChildrenInput is the input for the confluence_list_comment_children tool.
type ListCommentChildrenInput struct {
	CommentID   string `json:"comment_id" jsonschema:"the Confluence parent comment ID"`
	CommentType string `json:"comment_type,omitempty" jsonschema:"comment type: footer (default) or inline"`
	BodyFormat  string `json:"body_format,omitempty" jsonschema:"optional body format to include (storage, atlas_doc_format, view)"`
	Sort        string `json:"sort,omitempty" jsonschema:"optional Confluence comment sort order"`
	Limit       int    `json:"limit,omitempty" jsonschema:"maximum number of replies to return, defaults to 25"`
	Cursor      string `json:"cursor,omitempty" jsonschema:"pagination cursor for next page of results"`
}

// ListCommentChildrenOutput is the output for the confluence_list_comment_children tool.
type ListCommentChildrenOutput struct {
	Comments   []CommentSummary `json:"comments" jsonschema:"the child comments"`
	NextCursor string           `json:"next_cursor,omitempty" jsonschema:"cursor for the next page of results, if available"`
}

func listCommentChildren(client *confluence.Client) mcp.ToolHandlerFor[ListCommentChildrenInput, ListCommentChildrenOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListCommentChildrenInput) (*mcp.CallToolResult, ListCommentChildrenOutput, error) {
		limit := in.Limit
		if limit <= 0 {
			limit = 25
		}
		commentType := in.CommentType
		if commentType == "" {
			commentType = confluence.CommentTypeFooter
		}

		result, err := client.ListCommentChildren(ctx, confluence.ListCommentChildrenInput{
			CommentID:   in.CommentID,
			CommentType: commentType,
			BodyFormat:  in.BodyFormat,
			Sort:        in.Sort,
			Limit:       limit,
			Cursor:      in.Cursor,
		})
		if err != nil {
			return nil, ListCommentChildrenOutput{}, fmt.Errorf("list %s comment children for %s: %w", commentType, in.CommentID, err)
		}

		out := ListCommentChildrenOutput{
			Comments:   make([]CommentSummary, len(result.Results)),
			NextCursor: nextCursor(result.Links.Next),
		}
		for i := range result.Results {
			out.Comments[i] = commentToSummary(&result.Results[i], commentType)
		}
		return nil, out, nil
	}
}

// GetPageAttachmentsInput is the input for the confluence_get_page_attachments tool.
type GetPageAttachmentsInput struct {
	PageID string `json:"page_id" jsonschema:"the Confluence page ID"`
	Limit  int    `json:"limit,omitempty" jsonschema:"maximum number of attachments to return, defaults to 25"`
}

// AttachmentSummary is a summary of a Confluence attachment.
type AttachmentSummary struct {
	ID          string `json:"id" jsonschema:"the attachment ID"`
	Title       string `json:"title,omitempty" jsonschema:"the attachment filename"`
	MediaType   string `json:"media_type,omitempty" jsonschema:"the attachment MIME type"`
	FileSize    int64  `json:"file_size,omitempty" jsonschema:"the attachment file size in bytes"`
	DownloadURL string `json:"download_url,omitempty" jsonschema:"the URL to download the attachment"`
}

// GetPageAttachmentsOutput is the output for the confluence_get_page_attachments tool.
type GetPageAttachmentsOutput struct {
	Attachments []AttachmentSummary `json:"attachments" jsonschema:"the attachments on the page"`
}

func getPageAttachments(client *confluence.Client) mcp.ToolHandlerFor[GetPageAttachmentsInput, GetPageAttachmentsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in GetPageAttachmentsInput) (*mcp.CallToolResult, GetPageAttachmentsOutput, error) {
		limit := in.Limit
		if limit <= 0 {
			limit = 25
		}

		result, err := client.GetPageAttachments(ctx, in.PageID, limit)
		if err != nil {
			return nil, GetPageAttachmentsOutput{}, fmt.Errorf("get page attachments %s: %w", in.PageID, err)
		}

		attachments := make([]AttachmentSummary, len(result.Results))
		for i, att := range result.Results {
			attachments[i] = AttachmentSummary{
				ID:          att.ID,
				Title:       att.Title,
				MediaType:   att.MediaType,
				FileSize:    att.FileSize,
				DownloadURL: att.DownloadURL,
			}
		}
		return nil, GetPageAttachmentsOutput{Attachments: attachments}, nil
	}
}

// DownloadAttachmentInput is the input for the confluence_download_attachment tool.
type DownloadAttachmentInput struct {
	AttachmentID string `json:"attachment_id" jsonschema:"the Confluence attachment ID"`
}

// DownloadAttachmentOutput is the output for the confluence_download_attachment tool.
type DownloadAttachmentOutput struct {
	AttachmentID string `json:"attachment_id" jsonschema:"the attachment ID"`
	DataBase64   string `json:"data_base64" jsonschema:"the attachment content, base64-encoded"`
}

func downloadAttachment(client *confluence.Client) mcp.ToolHandlerFor[DownloadAttachmentInput, DownloadAttachmentOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in DownloadAttachmentInput) (*mcp.CallToolResult, DownloadAttachmentOutput, error) {
		data, err := client.DownloadAttachment(ctx, in.AttachmentID)
		if err != nil {
			return nil, DownloadAttachmentOutput{}, fmt.Errorf("download attachment %s: %w", in.AttachmentID, err)
		}
		return nil, DownloadAttachmentOutput{
			AttachmentID: in.AttachmentID,
			DataBase64:   base64.StdEncoding.EncodeToString(data),
		}, nil
	}
}

// registerReadTools registers all read-only Confluence tools on the server.
func registerReadTools(s *mcp.Server, client *confluence.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "confluence_get_page",
		Description: "Get a single Confluence page by ID",
		Annotations: readOnlyHint,
	}, getPage(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "confluence_search_pages",
		Description: "Search Confluence pages with filters and pagination",
		Annotations: readOnlyHint,
	}, searchPages(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "confluence_search_cql",
		Description: "Search Confluence content with CQL (Confluence Query Language)",
		Annotations: readOnlyHint,
	}, searchCQL(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "confluence_get_space",
		Description: "Get a single Confluence space by key or ID",
		Annotations: readOnlyHint,
	}, getSpace(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "confluence_list_spaces",
		Description: "List Confluence spaces with filters and pagination",
		Annotations: readOnlyHint,
	}, listSpaces(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "confluence_get_page_labels",
		Description: "Get labels attached to a Confluence page",
		Annotations: readOnlyHint,
	}, getPageLabels(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "confluence_list_page_comments",
		Description: "List footer or inline comments on a Confluence page",
		Annotations: readOnlyHint,
	}, listPageComments(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "confluence_get_comment",
		Description: "Get a footer or inline Confluence comment by ID",
		Annotations: readOnlyHint,
	}, getComment(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "confluence_list_comment_children",
		Description: "List replies to a footer or inline Confluence comment",
		Annotations: readOnlyHint,
	}, listCommentChildren(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "confluence_get_page_attachments",
		Description: "Get attachments on a Confluence page",
		Annotations: readOnlyHint,
	}, getPageAttachments(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "confluence_download_attachment",
		Description: "Download a Confluence attachment's content (base64-encoded)",
		Annotations: readOnlyHint,
	}, downloadAttachment(client))
}
