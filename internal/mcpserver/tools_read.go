// Package mcpserver builds the confluence-mcp MCP server, registering Confluence tools
// against the official MCP Go SDK. Read tools are always registered; write
// tools are only registered when the server is running in readwrite mode.
package mcpserver

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"confluence-mcp/internal/confluence"
)

// readOnlyHint is shared by all read-only tool registrations.
var readOnlyHint = &mcp.ToolAnnotations{ReadOnlyHint: true}

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
			NextCursor: result.Links.Next,
		}
		for i := range result.Results {
			out.Pages[i] = pageToSummary(&result.Results[i])
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
			NextCursor: result.Links.Next,
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
