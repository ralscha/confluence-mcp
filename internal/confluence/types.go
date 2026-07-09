package confluence

// User is a minimal Confluence Cloud user reference.
type User struct {
	AccountID   string `json:"accountId,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	Email       string `json:"email,omitempty"`
}

// Space represents a Confluence space.
type Space struct {
	ID     string     `json:"id,omitempty"`
	Key    string     `json:"key,omitempty"`
	Name   string     `json:"name,omitempty"`
	Type   string     `json:"type,omitempty"`
	Status string     `json:"status,omitempty"`
	Links  SpaceLinks `json:"_links"`
}

// SpaceLinks contains URLs for a space.
type SpaceLinks struct {
	WebUI string `json:"webui,omitempty"`
}

// SpaceSearchResult is the response body of GET /wiki/api/v2/spaces.
type SpaceSearchResult struct {
	Results []Space         `json:"results"`
	Links   PaginationLinks `json:"_links"`
}

// Page represents a Confluence page.
type Page struct {
	ID       string    `json:"id,omitempty"`
	Status   string    `json:"status,omitempty"`
	Title    string    `json:"title,omitempty"`
	SpaceID  string    `json:"spaceId,omitempty"`
	ParentID string    `json:"parentId,omitempty"`
	Version  *Version  `json:"version,omitempty"`
	Body     *PageBody `json:"body,omitempty"`
	Links    PageLinks `json:"_links"`
}

// Version represents the version information of a page.
type Version struct {
	Number    int    `json:"number,omitempty"`
	Message   string `json:"message,omitempty"`
	When      string `json:"when,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
	MinorEdit bool   `json:"minorEdit,omitempty"`
	AuthorID  string `json:"authorId,omitempty"`
	By        *User  `json:"by,omitempty"`
}

// PageBody holds the content of a page in various formats.
type PageBody struct {
	Storage        *ContentRepresentation `json:"storage,omitempty"`
	AtlasDocFormat *ContentRepresentation `json:"atlas_doc_format,omitempty"`
	View           *ContentRepresentation `json:"view,omitempty"`
}

// ContentRepresentation holds content in a specific format.
type ContentRepresentation struct {
	Representation string `json:"representation,omitempty"`
	Value          string `json:"value,omitempty"`
}

// PageLinks contains URLs for a page.
type PageLinks struct {
	WebUI string `json:"webui,omitempty"`
}

// PageSearchResult is the response body of GET /wiki/api/v2/pages.
type PageSearchResult struct {
	Results []Page          `json:"results"`
	Links   PaginationLinks `json:"_links"`
}

// PaginationLinks contains next/prev URLs for paginated results.
type PaginationLinks struct {
	Next string `json:"next,omitempty"`
}

// Comment represents a Confluence footer or inline comment.
type Comment struct {
	ID                      string                   `json:"id,omitempty"`
	Status                  string                   `json:"status,omitempty"`
	Title                   string                   `json:"title,omitempty"`
	PageID                  string                   `json:"pageId,omitempty"`
	BlogPostID              string                   `json:"blogPostId,omitempty"`
	AttachmentID            string                   `json:"attachmentId,omitempty"`
	CustomContentID         string                   `json:"customContentId,omitempty"`
	ParentCommentID         string                   `json:"parentCommentId,omitempty"`
	Version                 *Version                 `json:"version,omitempty"`
	Body                    *PageBody                `json:"body,omitempty"`
	ResolutionStatus        string                   `json:"resolutionStatus,omitempty"`
	InlineCommentProperties *InlineCommentProperties `json:"properties,omitempty"`
	Links                   PageLinks                `json:"_links"`
}

// InlineCommentProperties describes page anchor metadata for inline comments.
type InlineCommentProperties struct {
	InlineMarkerRef         string `json:"inlineMarkerRef,omitempty"`
	InlineOriginalSelection string `json:"inlineOriginalSelection,omitempty"`
}

// CommentSearchResult is the response body of comment list endpoints.
type CommentSearchResult struct {
	Results []Comment       `json:"results"`
	Links   PaginationLinks `json:"_links"`
}

// SearchLinks contains pagination and location links returned by CQL search.
type SearchLinks struct {
	Next    string `json:"next,omitempty"`
	Prev    string `json:"prev,omitempty"`
	Self    string `json:"self,omitempty"`
	Base    string `json:"base,omitempty"`
	Context string `json:"context,omitempty"`
}

// SearchSpace is the subset of a v1 search space result used by the MCP tools.
type SearchSpace struct {
	Key    string     `json:"key,omitempty"`
	Name   string     `json:"name,omitempty"`
	Type   string     `json:"type,omitempty"`
	Status string     `json:"status,omitempty"`
	Links  SpaceLinks `json:"_links"`
}

// SearchContent is the subset of a v1 search content result used by the MCP tools.
type SearchContent struct {
	ID      string       `json:"id,omitempty"`
	Type    string       `json:"type,omitempty"`
	Status  string       `json:"status,omitempty"`
	Title   string       `json:"title,omitempty"`
	Space   *SearchSpace `json:"space,omitempty"`
	Version *Version     `json:"version,omitempty"`
	Body    *PageBody    `json:"body,omitempty"`
	Links   PageLinks    `json:"_links"`
}

// ContentSearchItem represents a single CQL search hit.
type ContentSearchItem struct {
	Content              *SearchContent `json:"content,omitempty"`
	Title                string         `json:"title,omitempty"`
	Excerpt              string         `json:"excerpt,omitempty"`
	URL                  string         `json:"url,omitempty"`
	EntityType           string         `json:"entityType,omitempty"`
	LastModified         string         `json:"lastModified,omitempty"`
	FriendlyLastModified string         `json:"friendlyLastModified,omitempty"`
	Score                float64        `json:"score,omitempty"`
}

// ContentSearchResult is the response body of GET /wiki/rest/api/search.
type ContentSearchResult struct {
	Results             []ContentSearchItem `json:"results"`
	Start               int                 `json:"start,omitempty"`
	Limit               int                 `json:"limit,omitempty"`
	Size                int                 `json:"size,omitempty"`
	TotalSize           int                 `json:"totalSize,omitempty"`
	CQLQuery            string              `json:"cqlQuery,omitempty"`
	SearchDuration      int                 `json:"searchDuration,omitempty"`
	ArchivedResultCount int                 `json:"archivedResultCount,omitempty"`
	Links               SearchLinks         `json:"_links"`
}

// Label represents a Confluence label.
type Label struct {
	ID     string `json:"id,omitempty"`
	Name   string `json:"name,omitempty"`
	Prefix string `json:"prefix,omitempty"`
}

// LabelSearchResult is the response body of GET /wiki/api/v2/pages/{id}/labels.
type LabelSearchResult struct {
	Results []Label         `json:"results"`
	Links   PaginationLinks `json:"_links"`
}

// Attachment represents a Confluence attachment.
type Attachment struct {
	ID          string   `json:"id,omitempty"`
	Status      string   `json:"status,omitempty"`
	Title       string   `json:"title,omitempty"`
	MediaType   string   `json:"mediaType,omitempty"`
	FileSize    int64    `json:"fileSize,omitempty"`
	Comment     string   `json:"comment,omitempty"`
	Version     *Version `json:"version,omitempty"`
	DownloadURL string   `json:"downloadLink,omitempty"`
}

// AttachmentSearchResult is the response body of GET /wiki/api/v2/pages/{id}/attachments.
type AttachmentSearchResult struct {
	Results []Attachment    `json:"results"`
	Links   PaginationLinks `json:"_links"`
}
