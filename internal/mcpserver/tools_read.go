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

const (
	// defaultLimit is the page size used when a tool call omits limit.
	defaultLimit = 25
	// maxLimit is the largest page size the Confluence REST API accepts.
	maxLimit = 250
	// maxVersionBodyLimit is Atlassian's lower cap when historical page bodies
	// are requested alongside version metadata.
	maxVersionBodyLimit = 50
)

// clampLimit normalizes a caller-supplied page size into the range Confluence
// accepts, so that out-of-range values do not turn into API errors.
func clampLimit(limit int) int {
	switch {
	case limit <= 0:
		return defaultLimit
	case limit > maxLimit:
		return maxLimit
	default:
		return limit
	}
}

func clampVersionLimit(limit int, bodyFormat string) int {
	limit = clampLimit(limit)
	if bodyFormat != "" && limit > maxVersionBodyLimit {
		return maxVersionBodyLimit
	}
	return limit
}

func clampSearchLimit(limit int, expand []string) int {
	limit = clampLimit(limit)
	for _, field := range expand {
		if (field == "body.export_view" || field == "body.styled_view") && limit > defaultLimit {
			return defaultLimit
		}
	}
	return limit
}

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
	ID         string  `json:"id" jsonschema:"the page ID"`
	Title      string  `json:"title,omitempty" jsonschema:"the page title"`
	Status     string  `json:"status,omitempty" jsonschema:"the page status (current, archived)"`
	SpaceID    string  `json:"space_id,omitempty" jsonschema:"the space ID"`
	ParentID   string  `json:"parent_id,omitempty" jsonschema:"the parent page ID, if any"`
	Version    int     `json:"version,omitempty" jsonschema:"the current version number"`
	Content    string  `json:"content,omitempty" jsonschema:"the page content as plain text"`
	RawContent *string `json:"raw_content,omitempty" jsonschema:"original body returned by get_page; edit this to preserve formatting"`
	BodyFormat string  `json:"body_format,omitempty" jsonschema:"format of raw_content; use storage or atlas_doc_format as body_type when updating"`
	WebURL     string  `json:"web_url,omitempty" jsonschema:"the URL to view the page in a browser"`
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
	PageID     string `json:"page_id" jsonschema:"the Confluence page ID"`
	BodyFormat string `json:"body_format,omitempty" jsonschema:"body format: storage (default), atlas_doc_format, or view (rendered HTML; not writable)"`
}

func getPage(client *confluence.Client) mcp.ToolHandlerFor[GetPageInput, PageSummary] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in GetPageInput) (*mcp.CallToolResult, PageSummary, error) {
		if in.BodyFormat == "" {
			in.BodyFormat = "storage"
		}
		if err := validateReadBodyFormat(in.BodyFormat); err != nil {
			return nil, PageSummary{}, err
		}
		page, err := client.GetPage(ctx, in.PageID, in.BodyFormat)
		if err != nil {
			return nil, PageSummary{}, fmt.Errorf("get page %s: %w", in.PageID, err)
		}
		out := pageToSummary(page)
		out.RawContent = rawBody(page.Body, in.BodyFormat)
		if out.RawContent != nil {
			out.BodyFormat = in.BodyFormat
		}
		return nil, out, nil
	}
}

func validateReadBodyFormat(format string) error {
	switch format {
	case "storage", "atlas_doc_format", "view":
		return nil
	default:
		return fmt.Errorf("unsupported body_format %q (want storage, atlas_doc_format, or view)", format)
	}
}

func rawBody(body *confluence.PageBody, format string) *string {
	if body == nil {
		return nil
	}
	var representation *confluence.ContentRepresentation
	switch format {
	case "storage":
		representation = body.Storage
	case "atlas_doc_format":
		representation = body.AtlasDocFormat
	case "view":
		representation = body.View
	}
	if representation == nil {
		return nil
	}
	return &representation.Value
}

// SearchPagesInput is the input for the confluence_search_pages tool.
type SearchPagesInput struct {
	SpaceID    string `json:"space_id,omitempty" jsonschema:"filter by space ID"`
	Title      string `json:"title,omitempty" jsonschema:"filter by exact page title"`
	Status     string `json:"status,omitempty" jsonschema:"filter by status (current, archived)"`
	Sort       string `json:"sort,omitempty" jsonschema:"optional Confluence page sort order"`
	BodyFormat string `json:"body_format,omitempty" jsonschema:"optional body format to include (storage or atlas_doc_format)"`
	Limit      int    `json:"limit,omitempty" jsonschema:"maximum number of results to return, defaults to 25, maximum 250"`
	Cursor     string `json:"cursor,omitempty" jsonschema:"pagination cursor for next page of results"`
}

// PageVersionSummary is a flattened entry in a page's version history.
type PageVersionSummary struct {
	Number    int    `json:"number" jsonschema:"the version number"`
	Message   string `json:"message,omitempty" jsonschema:"the version message"`
	CreatedAt string `json:"created_at,omitempty" jsonschema:"when the version was created"`
	AuthorID  string `json:"author_id,omitempty" jsonschema:"the author's account ID"`
	MinorEdit bool   `json:"minor_edit,omitempty" jsonschema:"whether this was marked as a minor edit"`
	Title     string `json:"title,omitempty" jsonschema:"the page title at this version"`
	Content   string `json:"content,omitempty" jsonschema:"plain text content when body_format was requested"`
}

// ListPageVersionsInput is the input for confluence_list_page_versions.
type ListPageVersionsInput struct {
	PageID     string `json:"page_id" jsonschema:"the Confluence page ID"`
	BodyFormat string `json:"body_format,omitempty" jsonschema:"optional historical body format to include (storage or atlas_doc_format)"`
	Sort       string `json:"sort,omitempty" jsonschema:"optional version sort order, such as -modified-date"`
	Limit      int    `json:"limit,omitempty" jsonschema:"maximum versions to return, defaults to 25, maximum 250 (50 when body_format is set)"`
	Cursor     string `json:"cursor,omitempty" jsonschema:"pagination cursor for the next page of results"`
}

// ListPageVersionsOutput is the output for confluence_list_page_versions.
type ListPageVersionsOutput struct {
	Versions   []PageVersionSummary `json:"versions" jsonschema:"the page version history"`
	NextCursor string               `json:"next_cursor,omitempty" jsonschema:"cursor for the next page of results, if available"`
}

func listPageVersions(client *confluence.Client) mcp.ToolHandlerFor[ListPageVersionsInput, ListPageVersionsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListPageVersionsInput) (*mcp.CallToolResult, ListPageVersionsOutput, error) {
		result, err := client.ListPageVersions(ctx, confluence.ListPageVersionsInput{
			PageID:     in.PageID,
			BodyFormat: in.BodyFormat,
			Sort:       in.Sort,
			Limit:      clampVersionLimit(in.Limit, in.BodyFormat),
			Cursor:     in.Cursor,
		})
		if err != nil {
			return nil, ListPageVersionsOutput{}, fmt.Errorf("list versions of page %s: %w", in.PageID, err)
		}

		out := ListPageVersionsOutput{
			Versions:   make([]PageVersionSummary, len(result.Results)),
			NextCursor: nextCursor(result.Links.Next),
		}
		for i, version := range result.Results {
			createdAt := version.CreatedAt
			if createdAt == "" {
				createdAt = version.When
			}
			out.Versions[i] = PageVersionSummary{
				Number:    version.Number,
				Message:   version.Message,
				CreatedAt: createdAt,
				AuthorID:  version.AuthorID,
				MinorEdit: version.MinorEdit,
			}
			if version.Page != nil {
				out.Versions[i].Title = version.Page.Title
				out.Versions[i].Content = version.Page.Body.PlainText()
			}
		}
		return nil, out, nil
	}
}

// SearchPagesOutput is the output for the confluence_search_pages tool.
type SearchPagesOutput struct {
	Pages      []PageSummary `json:"pages" jsonschema:"the matching pages"`
	NextCursor string        `json:"next_cursor,omitempty" jsonschema:"cursor for the next page of results, if available"`
}

func searchPages(client *confluence.Client) mcp.ToolHandlerFor[SearchPagesInput, SearchPagesOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in SearchPagesInput) (*mcp.CallToolResult, SearchPagesOutput, error) {
		result, err := client.SearchPages(ctx, confluence.SearchPagesInput{
			SpaceID:    in.SpaceID,
			Title:      in.Title,
			Status:     in.Status,
			Sort:       in.Sort,
			BodyFormat: in.BodyFormat,
			Limit:      clampLimit(in.Limit),
			Cursor:     in.Cursor,
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

// ChildPageSummary is a compact view of child content in a page hierarchy.
type ChildPageSummary struct {
	ID            string `json:"id" jsonschema:"the page ID"`
	Type          string `json:"type,omitempty" jsonschema:"the child content type, such as page, database, whiteboard, embed, or folder"`
	Title         string `json:"title,omitempty" jsonschema:"the page title"`
	Status        string `json:"status,omitempty" jsonschema:"the page status (current, archived)"`
	SpaceID       string `json:"space_id,omitempty" jsonschema:"the space ID"`
	ChildPosition int    `json:"child_position,omitempty" jsonschema:"the position of this page among its siblings"`
}

// GetPageChildrenInput is the input for the confluence_get_page_children tool.
type GetPageChildrenInput struct {
	PageID string `json:"page_id" jsonschema:"the Confluence page ID whose children to list"`
	Sort   string `json:"sort,omitempty" jsonschema:"optional Confluence sort order, e.g. child-position or -created-date"`
	Limit  int    `json:"limit,omitempty" jsonschema:"maximum number of children to return, defaults to 25, maximum 250"`
	Cursor string `json:"cursor,omitempty" jsonschema:"pagination cursor for next page of results"`
}

// GetPageChildrenOutput is the output for the confluence_get_page_children tool.
type GetPageChildrenOutput struct {
	Children   []ChildPageSummary `json:"children" jsonschema:"the direct child content"`
	NextCursor string             `json:"next_cursor,omitempty" jsonschema:"cursor for the next page of results, if available"`
}

func getPageChildren(client *confluence.Client) mcp.ToolHandlerFor[GetPageChildrenInput, GetPageChildrenOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in GetPageChildrenInput) (*mcp.CallToolResult, GetPageChildrenOutput, error) {
		result, err := client.GetPageChildren(ctx, confluence.GetPageChildrenInput{
			PageID: in.PageID,
			Sort:   in.Sort,
			Limit:  clampLimit(in.Limit),
			Cursor: in.Cursor,
		})
		if err != nil {
			return nil, GetPageChildrenOutput{}, fmt.Errorf("get children of page %s: %w", in.PageID, err)
		}

		out := GetPageChildrenOutput{
			Children:   make([]ChildPageSummary, len(result.Results)),
			NextCursor: nextCursor(result.Links.Next),
		}
		for i, child := range result.Results {
			out.Children[i] = ChildPageSummary{
				ID:            child.ID,
				Type:          child.Type,
				Title:         child.Title,
				Status:        child.Status,
				SpaceID:       child.SpaceID,
				ChildPosition: child.ChildPosition,
			}
		}
		return nil, out, nil
	}
}

// AncestorSummary is a compact view of an ancestor of a page.
type AncestorSummary struct {
	ID     string `json:"id" jsonschema:"the ancestor content ID"`
	Title  string `json:"title,omitempty" jsonschema:"the ancestor title"`
	Type   string `json:"type,omitempty" jsonschema:"the ancestor content type, e.g. page"`
	Status string `json:"status,omitempty" jsonschema:"the ancestor status"`
}

// GetPageAncestorsInput is the input for the confluence_get_page_ancestors tool.
type GetPageAncestorsInput struct {
	PageID string `json:"page_id" jsonschema:"the Confluence page ID whose ancestors to list"`
	Limit  int    `json:"limit,omitempty" jsonschema:"maximum number of ancestors to return, defaults to 25, maximum 250"`
}

// GetPageAncestorsOutput is the output for the confluence_get_page_ancestors tool.
type GetPageAncestorsOutput struct {
	Ancestors []AncestorSummary `json:"ancestors" jsonschema:"the ancestors of the page, ordered from the root downwards"`
}

func getPageAncestors(client *confluence.Client) mcp.ToolHandlerFor[GetPageAncestorsInput, GetPageAncestorsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in GetPageAncestorsInput) (*mcp.CallToolResult, GetPageAncestorsOutput, error) {
		result, err := client.GetPageAncestors(ctx, in.PageID, clampLimit(in.Limit))
		if err != nil {
			return nil, GetPageAncestorsOutput{}, fmt.Errorf("get ancestors of page %s: %w", in.PageID, err)
		}

		ancestors := make([]AncestorSummary, len(result.Results))
		for i, ancestor := range result.Results {
			ancestors[i] = AncestorSummary{
				ID:     ancestor.ID,
				Title:  ancestor.Title,
				Type:   ancestor.Type,
				Status: ancestor.Status,
			}
		}
		return nil, GetPageAncestorsOutput{Ancestors: ancestors}, nil
	}
}

// GetSpacePagesInput is the input for the confluence_get_space_pages tool.
type GetSpacePagesInput struct {
	SpaceID    string   `json:"space_id" jsonschema:"the Confluence space ID whose pages to list"`
	Title      string   `json:"title,omitempty" jsonschema:"filter by page title"`
	Status     []string `json:"status,omitempty" jsonschema:"filter by statuses (current, archived, trashed, deleted)"`
	Sort       string   `json:"sort,omitempty" jsonschema:"optional Confluence sort order, e.g. title or -modified-date"`
	BodyFormat string   `json:"body_format,omitempty" jsonschema:"optional body format to include (storage or atlas_doc_format)"`
	Limit      int      `json:"limit,omitempty" jsonschema:"maximum number of pages to return, defaults to 25, maximum 250"`
	Cursor     string   `json:"cursor,omitempty" jsonschema:"pagination cursor for next page of results"`
}

// GetSpacePagesOutput is the output for the confluence_get_space_pages tool.
type GetSpacePagesOutput struct {
	Pages      []PageSummary `json:"pages" jsonschema:"the pages in the space"`
	NextCursor string        `json:"next_cursor,omitempty" jsonschema:"cursor for the next page of results, if available"`
}

func getSpacePages(client *confluence.Client) mcp.ToolHandlerFor[GetSpacePagesInput, GetSpacePagesOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in GetSpacePagesInput) (*mcp.CallToolResult, GetSpacePagesOutput, error) {
		result, err := client.GetSpacePages(ctx, confluence.GetSpacePagesInput{
			SpaceID:    in.SpaceID,
			Title:      in.Title,
			Status:     in.Status,
			Sort:       in.Sort,
			BodyFormat: in.BodyFormat,
			Limit:      clampLimit(in.Limit),
			Cursor:     in.Cursor,
		})
		if err != nil {
			return nil, GetSpacePagesOutput{}, fmt.Errorf("get pages in space %s: %w", in.SpaceID, err)
		}

		out := GetSpacePagesOutput{
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
	Limit                 int      `json:"limit,omitempty" jsonschema:"maximum number of results to return, defaults to 25, maximum 250"`
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
		result, err := client.SearchContent(ctx, confluence.SearchContentInput{
			CQL:                   in.CQL,
			CQLContext:            in.CQLContext,
			Expand:                in.Expand,
			Cursor:                in.Cursor,
			Limit:                 clampSearchLimit(in.Limit, in.Expand),
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
	Limit  int      `json:"limit,omitempty" jsonschema:"maximum number of results to return, defaults to 25, maximum 250"`
	Cursor string   `json:"cursor,omitempty" jsonschema:"pagination cursor for next page of results"`
}

// ListSpacesOutput is the output for the confluence_list_spaces tool.
type ListSpacesOutput struct {
	Spaces     []SpaceSummary `json:"spaces" jsonschema:"the matching spaces"`
	NextCursor string         `json:"next_cursor,omitempty" jsonschema:"cursor for the next page of results, if available"`
}

func listSpaces(client *confluence.Client) mcp.ToolHandlerFor[ListSpacesInput, ListSpacesOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListSpacesInput) (*mcp.CallToolResult, ListSpacesOutput, error) {
		result, err := client.ListSpaces(ctx, confluence.ListSpacesInput{
			Keys:   in.Keys,
			Type:   in.Type,
			Status: in.Status,
			Limit:  clampLimit(in.Limit),
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
	Prefix string `json:"prefix,omitempty" jsonschema:"optional label prefix filter"`
	Sort   string `json:"sort,omitempty" jsonschema:"optional label sort order"`
	Limit  int    `json:"limit,omitempty" jsonschema:"maximum number of labels to return, defaults to 25, maximum 250"`
	Cursor string `json:"cursor,omitempty" jsonschema:"pagination cursor for the next page of results"`
}

// GetPageLabelsOutput is the output for the confluence_get_page_labels tool.
type GetPageLabelsOutput struct {
	Labels     []string `json:"labels" jsonschema:"the label names attached to the page"`
	NextCursor string   `json:"next_cursor,omitempty" jsonschema:"cursor for the next page of results, if available"`
}

func getPageLabels(client *confluence.Client) mcp.ToolHandlerFor[GetPageLabelsInput, GetPageLabelsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in GetPageLabelsInput) (*mcp.CallToolResult, GetPageLabelsOutput, error) {
		result, err := client.GetPageLabels(ctx, confluence.GetPageLabelsInput{
			PageID: in.PageID,
			Prefix: in.Prefix,
			Sort:   in.Sort,
			Limit:  clampLimit(in.Limit),
			Cursor: in.Cursor,
		})
		if err != nil {
			return nil, GetPageLabelsOutput{}, fmt.Errorf("get page labels %s: %w", in.PageID, err)
		}

		labels := make([]string, len(result.Results))
		for i, label := range result.Results {
			labels[i] = label.Name
		}
		return nil, GetPageLabelsOutput{Labels: labels, NextCursor: nextCursor(result.Links.Next)}, nil
	}
}

// CommentSummary is a flattened view of a Confluence comment.
type CommentSummary struct {
	ID                      string  `json:"id" jsonschema:"the comment ID"`
	Type                    string  `json:"type,omitempty" jsonschema:"the comment type: footer or inline"`
	Status                  string  `json:"status,omitempty" jsonschema:"the comment status"`
	Title                   string  `json:"title,omitempty" jsonschema:"the comment title"`
	PageID                  string  `json:"page_id,omitempty" jsonschema:"the page ID the comment belongs to"`
	ParentCommentID         string  `json:"parent_comment_id,omitempty" jsonschema:"the parent comment ID, if this is a reply"`
	Version                 int     `json:"version,omitempty" jsonschema:"the current comment version number"`
	CreatedAt               string  `json:"created_at,omitempty" jsonschema:"when this comment version was created"`
	AuthorID                string  `json:"author_id,omitempty" jsonschema:"the author account ID"`
	Content                 string  `json:"content,omitempty" jsonschema:"the comment body as plain text"`
	RawContent              *string `json:"raw_content,omitempty" jsonschema:"original body returned by get_comment; edit this to preserve formatting"`
	BodyFormat              string  `json:"body_format,omitempty" jsonschema:"format of raw_content"`
	ResolutionStatus        string  `json:"resolution_status,omitempty" jsonschema:"inline comment resolution status"`
	InlineMarkerRef         string  `json:"inline_marker_ref,omitempty" jsonschema:"inline comment marker reference"`
	InlineOriginalSelection string  `json:"inline_original_selection,omitempty" jsonschema:"the originally selected text for an inline comment"`
	WebURL                  string  `json:"web_url,omitempty" jsonschema:"the URL to view the comment in a browser"`
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
	Limit            int      `json:"limit,omitempty" jsonschema:"maximum number of comments to return, defaults to 25, maximum 250"`
	Cursor           string   `json:"cursor,omitempty" jsonschema:"pagination cursor for next page of results"`
}

// ListPageCommentsOutput is the output for the confluence_list_page_comments tool.
type ListPageCommentsOutput struct {
	Comments   []CommentSummary `json:"comments" jsonschema:"the matching comments"`
	NextCursor string           `json:"next_cursor,omitempty" jsonschema:"cursor for the next page of results, if available"`
}

func listPageComments(client *confluence.Client) mcp.ToolHandlerFor[ListPageCommentsInput, ListPageCommentsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListPageCommentsInput) (*mcp.CallToolResult, ListPageCommentsOutput, error) {
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
			Limit:            clampLimit(in.Limit),
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
	BodyFormat  string `json:"body_format,omitempty" jsonschema:"body format: storage (default), atlas_doc_format, or view (rendered HTML; not writable)"`
}

func getComment(client *confluence.Client) mcp.ToolHandlerFor[GetCommentInput, CommentSummary] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in GetCommentInput) (*mcp.CallToolResult, CommentSummary, error) {
		if in.BodyFormat == "" {
			in.BodyFormat = "storage"
		}
		if err := validateReadBodyFormat(in.BodyFormat); err != nil {
			return nil, CommentSummary{}, err
		}
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
		out := commentToSummary(comment, commentType)
		out.RawContent = rawBody(comment.Body, in.BodyFormat)
		if out.RawContent != nil {
			out.BodyFormat = in.BodyFormat
		}
		return nil, out, nil
	}
}

// ListCommentChildrenInput is the input for the confluence_list_comment_children tool.
type ListCommentChildrenInput struct {
	CommentID   string `json:"comment_id" jsonschema:"the Confluence parent comment ID"`
	CommentType string `json:"comment_type,omitempty" jsonschema:"comment type: footer (default) or inline"`
	BodyFormat  string `json:"body_format,omitempty" jsonschema:"optional body format to include (storage, atlas_doc_format, view)"`
	Sort        string `json:"sort,omitempty" jsonschema:"optional Confluence comment sort order"`
	Limit       int    `json:"limit,omitempty" jsonschema:"maximum number of replies to return, defaults to 25, maximum 250"`
	Cursor      string `json:"cursor,omitempty" jsonschema:"pagination cursor for next page of results"`
}

// ListCommentChildrenOutput is the output for the confluence_list_comment_children tool.
type ListCommentChildrenOutput struct {
	Comments   []CommentSummary `json:"comments" jsonschema:"the child comments"`
	NextCursor string           `json:"next_cursor,omitempty" jsonschema:"cursor for the next page of results, if available"`
}

func listCommentChildren(client *confluence.Client) mcp.ToolHandlerFor[ListCommentChildrenInput, ListCommentChildrenOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListCommentChildrenInput) (*mcp.CallToolResult, ListCommentChildrenOutput, error) {
		commentType := in.CommentType
		if commentType == "" {
			commentType = confluence.CommentTypeFooter
		}

		result, err := client.ListCommentChildren(ctx, confluence.ListCommentChildrenInput{
			CommentID:   in.CommentID,
			CommentType: commentType,
			BodyFormat:  in.BodyFormat,
			Sort:        in.Sort,
			Limit:       clampLimit(in.Limit),
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
	PageID    string   `json:"page_id" jsonschema:"the Confluence page ID"`
	Sort      string   `json:"sort,omitempty" jsonschema:"optional attachment sort order"`
	Status    []string `json:"status,omitempty" jsonschema:"optional attachment statuses to include"`
	MediaType string   `json:"media_type,omitempty" jsonschema:"filter by MIME type"`
	Filename  string   `json:"filename,omitempty" jsonschema:"filter by filename"`
	Limit     int      `json:"limit,omitempty" jsonschema:"maximum number of attachments to return, defaults to 25, maximum 250"`
	Cursor    string   `json:"cursor,omitempty" jsonschema:"pagination cursor for the next page of results"`
}

// AttachmentSummary is a summary of a Confluence attachment.
type AttachmentSummary struct {
	ID          string `json:"id" jsonschema:"the attachment ID"`
	PageID      string `json:"page_id,omitempty" jsonschema:"the page containing the attachment"`
	Status      string `json:"status,omitempty" jsonschema:"the attachment status"`
	Title       string `json:"title,omitempty" jsonschema:"the attachment filename"`
	MediaType   string `json:"media_type,omitempty" jsonschema:"the attachment MIME type"`
	FileSize    int64  `json:"file_size,omitempty" jsonschema:"the attachment file size in bytes"`
	Comment     string `json:"comment,omitempty" jsonschema:"the attachment comment"`
	Version     int    `json:"version,omitempty" jsonschema:"the attachment version number"`
	WebURL      string `json:"web_url,omitempty" jsonschema:"the URL to view the attachment"`
	DownloadURL string `json:"download_url,omitempty" jsonschema:"the URL to download the attachment"`
}

func attachmentToSummary(att *confluence.Attachment) AttachmentSummary {
	downloadURL := att.DownloadURL
	if downloadURL == "" {
		downloadURL = att.Links.Download
	}
	summary := AttachmentSummary{
		ID:          att.ID,
		PageID:      att.PageID,
		Status:      att.Status,
		Title:       att.Title,
		MediaType:   att.MediaType,
		FileSize:    att.FileSize,
		Comment:     att.Comment,
		WebURL:      att.Links.WebUI,
		DownloadURL: downloadURL,
	}
	if att.Version != nil {
		summary.Version = att.Version.Number
	}
	return summary
}

// GetPageAttachmentsOutput is the output for the confluence_get_page_attachments tool.
type GetPageAttachmentsOutput struct {
	Attachments []AttachmentSummary `json:"attachments" jsonschema:"the attachments on the page"`
	NextCursor  string              `json:"next_cursor,omitempty" jsonschema:"cursor for the next page of results, if available"`
}

func getPageAttachments(client *confluence.Client) mcp.ToolHandlerFor[GetPageAttachmentsInput, GetPageAttachmentsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in GetPageAttachmentsInput) (*mcp.CallToolResult, GetPageAttachmentsOutput, error) {
		result, err := client.GetPageAttachments(ctx, confluence.GetPageAttachmentsInput{
			PageID:    in.PageID,
			Sort:      in.Sort,
			Status:    in.Status,
			MediaType: in.MediaType,
			Filename:  in.Filename,
			Limit:     clampLimit(in.Limit),
			Cursor:    in.Cursor,
		})
		if err != nil {
			return nil, GetPageAttachmentsOutput{}, fmt.Errorf("get page attachments %s: %w", in.PageID, err)
		}

		attachments := make([]AttachmentSummary, len(result.Results))
		for i := range result.Results {
			attachments[i] = attachmentToSummary(&result.Results[i])
		}
		return nil, GetPageAttachmentsOutput{
			Attachments: attachments,
			NextCursor:  nextCursor(result.Links.Next),
		}, nil
	}
}

// GetAttachmentInput is the input for confluence_get_attachment.
type GetAttachmentInput struct {
	AttachmentID string `json:"attachment_id" jsonschema:"the Confluence attachment ID"`
}

func getAttachment(client *confluence.Client) mcp.ToolHandlerFor[GetAttachmentInput, AttachmentSummary] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in GetAttachmentInput) (*mcp.CallToolResult, AttachmentSummary, error) {
		attachment, err := client.GetAttachment(ctx, in.AttachmentID)
		if err != nil {
			return nil, AttachmentSummary{}, fmt.Errorf("get attachment %s: %w", in.AttachmentID, err)
		}
		return nil, attachmentToSummary(attachment), nil
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
		Description: "Get a Confluence page with its current version, readable text, and original raw_content for editing. Defaults to storage format. Preserve raw_content when updating rich articles.",
		Annotations: readOnlyHint,
	}, getPage(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "confluence_search_pages",
		Description: "Search Confluence pages with filters and pagination",
		Annotations: readOnlyHint,
	}, searchPages(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "confluence_get_page_children",
		Description: "List direct child content of a Confluence page",
		Annotations: readOnlyHint,
	}, getPageChildren(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "confluence_get_page_ancestors",
		Description: "List the ancestors of a Confluence page, from the root downwards",
		Annotations: readOnlyHint,
	}, getPageAncestors(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "confluence_list_page_versions",
		Description: "List the version history of a Confluence page",
		Annotations: readOnlyHint,
	}, listPageVersions(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "confluence_get_space_pages",
		Description: "List the pages in a Confluence space",
		Annotations: readOnlyHint,
	}, getSpacePages(client))

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
		Name:        "confluence_get_attachment",
		Description: "Get metadata for a Confluence attachment",
		Annotations: readOnlyHint,
	}, getAttachment(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "confluence_download_attachment",
		Description: "Download a Confluence attachment's content (base64-encoded)",
		Annotations: readOnlyHint,
	}, downloadAttachment(client))
}
