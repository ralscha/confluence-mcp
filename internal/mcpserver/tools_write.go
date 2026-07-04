package mcpserver

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"confluence-mcp/internal/confluence"
)

// nonDestructiveHint is shared by write tools that never delete data.
var nonDestructiveHint = &mcp.ToolAnnotations{DestructiveHint: new(false)}

// CreatePageInput is the input for the confluence_create_page tool.
type CreatePageInput struct {
	SpaceID  string `json:"space_id" jsonschema:"the space ID to create the page in"`
	Title    string `json:"title" jsonschema:"the page title"`
	ParentID string `json:"parent_id,omitempty" jsonschema:"optional parent page ID"`
	Content  string `json:"content,omitempty" jsonschema:"the page content as plain text or storage format"`
	BodyType string `json:"body_type,omitempty" jsonschema:"content format type: storage (default) or atlas_doc_format"`
}

// CreatedPage describes a newly created page.
type CreatedPage struct {
	PageID string `json:"page_id" jsonschema:"the ID of the created page"`
	Title  string `json:"title,omitempty" jsonschema:"the page title"`
}

func createPage(client *confluence.Client) mcp.ToolHandlerFor[CreatePageInput, CreatedPage] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in CreatePageInput) (*mcp.CallToolResult, CreatedPage, error) {
		page, err := client.CreatePage(ctx, confluence.CreatePageInput{
			SpaceID:  in.SpaceID,
			Title:    in.Title,
			ParentID: in.ParentID,
			Body:     in.Content,
			BodyType: in.BodyType,
		})
		if err != nil {
			return nil, CreatedPage{}, fmt.Errorf("create page in space %s: %w", in.SpaceID, err)
		}
		return nil, CreatedPage{PageID: page.ID, Title: page.Title}, nil
	}
}

// UpdatePageInput is the input for the confluence_update_page tool. Title and
// Content are optional; only non-empty fields are updated. At least one must
// be provided. Version must be the current version number of the page.
type UpdatePageInput struct {
	PageID      string  `json:"page_id" jsonschema:"the Confluence page ID to update"`
	Title       *string `json:"title,omitempty" jsonschema:"new title for the page"`
	Content     *string `json:"content,omitempty" jsonschema:"new content for the page, as plain text or storage format"`
	BodyType    string  `json:"body_type,omitempty" jsonschema:"content format type: storage (default) or atlas_doc_format"`
	Version     int     `json:"version" jsonschema:"the current version number of the page (required)"`
	VersionNote string  `json:"version_note,omitempty" jsonschema:"optional version message describing the change"`
}

// UpdatePageOutput confirms a page update.
type UpdatePageOutput struct {
	PageID  string `json:"page_id" jsonschema:"the ID of the updated page"`
	Updated bool   `json:"updated" jsonschema:"whether the update succeeded"`
	Version int    `json:"version,omitempty" jsonschema:"the new version number"`
}

func updatePage(client *confluence.Client) mcp.ToolHandlerFor[UpdatePageInput, UpdatePageOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in UpdatePageInput) (*mcp.CallToolResult, UpdatePageOutput, error) {
		page, err := client.UpdatePage(ctx, in.PageID, confluence.UpdatePageInput{
			Title:       in.Title,
			Body:        in.Content,
			BodyType:    in.BodyType,
			Version:     in.Version,
			VersionNote: in.VersionNote,
		})
		if err != nil {
			return nil, UpdatePageOutput{}, fmt.Errorf("update page %s: %w", in.PageID, err)
		}
		version := 0
		if page.Version != nil {
			version = page.Version.Number
		}
		return nil, UpdatePageOutput{PageID: in.PageID, Updated: true, Version: version}, nil
	}
}

// DeletePageInput is the input for the confluence_delete_page tool.
type DeletePageInput struct {
	PageID string `json:"page_id" jsonschema:"the Confluence page ID to delete"`
}

// DeletePageOutput confirms a page deletion.
type DeletePageOutput struct {
	PageID  string `json:"page_id" jsonschema:"the ID of the deleted page"`
	Deleted bool   `json:"deleted" jsonschema:"whether the deletion succeeded"`
}

func deletePage(client *confluence.Client) mcp.ToolHandlerFor[DeletePageInput, DeletePageOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in DeletePageInput) (*mcp.CallToolResult, DeletePageOutput, error) {
		err := client.DeletePage(ctx, in.PageID)
		if err != nil {
			return nil, DeletePageOutput{}, fmt.Errorf("delete page %s: %w", in.PageID, err)
		}
		return nil, DeletePageOutput{PageID: in.PageID, Deleted: true}, nil
	}
}

// AddPageLabelInput is the input for the confluence_add_page_label tool.
type AddPageLabelInput struct {
	PageID    string `json:"page_id" jsonschema:"the Confluence page ID"`
	LabelName string `json:"label_name" jsonschema:"the label name to add"`
}

// AddPageLabelOutput confirms adding a label to a page.
type AddPageLabelOutput struct {
	PageID    string `json:"page_id" jsonschema:"the page ID"`
	LabelName string `json:"label_name" jsonschema:"the label name added"`
	Added     bool   `json:"added" jsonschema:"whether the label was added successfully"`
}

func addPageLabel(client *confluence.Client) mcp.ToolHandlerFor[AddPageLabelInput, AddPageLabelOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in AddPageLabelInput) (*mcp.CallToolResult, AddPageLabelOutput, error) {
		err := client.AddPageLabel(ctx, in.PageID, in.LabelName)
		if err != nil {
			return nil, AddPageLabelOutput{}, fmt.Errorf("add label %s to page %s: %w", in.LabelName, in.PageID, err)
		}
		return nil, AddPageLabelOutput{PageID: in.PageID, LabelName: in.LabelName, Added: true}, nil
	}
}

// UploadAttachmentInput is the input for the confluence_upload_attachment tool.
type UploadAttachmentInput struct {
	PageID     string `json:"page_id" jsonschema:"the Confluence page ID to attach the file to"`
	Filename   string `json:"filename" jsonschema:"the filename to store the attachment as"`
	MimeType   string `json:"mime_type,omitempty" jsonschema:"the attachment's MIME type, e.g. image/png"`
	DataBase64 string `json:"data_base64" jsonschema:"the file content, base64-encoded"`
}

// UploadedAttachment describes a newly uploaded attachment.
type UploadedAttachment struct {
	AttachmentID string `json:"attachment_id" jsonschema:"the ID of the created attachment"`
	Filename     string `json:"filename,omitempty" jsonschema:"the stored filename"`
}

func uploadAttachment(client *confluence.Client) mcp.ToolHandlerFor[UploadAttachmentInput, UploadedAttachment] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in UploadAttachmentInput) (*mcp.CallToolResult, UploadedAttachment, error) {
		data, err := base64.StdEncoding.DecodeString(in.DataBase64)
		if err != nil {
			return nil, UploadedAttachment{}, fmt.Errorf("decode data_base64: %w", err)
		}
		attachment, err := client.UploadAttachment(ctx, in.PageID, in.Filename, in.MimeType, data)
		if err != nil {
			return nil, UploadedAttachment{}, fmt.Errorf("upload attachment to page %s: %w", in.PageID, err)
		}
		return nil, UploadedAttachment{AttachmentID: attachment.ID, Filename: attachment.Title}, nil
	}
}

// DeleteAttachmentInput is the input for the confluence_delete_attachment tool.
type DeleteAttachmentInput struct {
	AttachmentID string `json:"attachment_id" jsonschema:"the Confluence attachment ID to delete"`
}

// DeleteAttachmentOutput confirms an attachment deletion.
type DeleteAttachmentOutput struct {
	AttachmentID string `json:"attachment_id" jsonschema:"the ID of the deleted attachment"`
	Deleted      bool   `json:"deleted" jsonschema:"whether the deletion succeeded"`
}

func deleteAttachment(client *confluence.Client) mcp.ToolHandlerFor[DeleteAttachmentInput, DeleteAttachmentOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in DeleteAttachmentInput) (*mcp.CallToolResult, DeleteAttachmentOutput, error) {
		err := client.DeleteAttachment(ctx, in.AttachmentID)
		if err != nil {
			return nil, DeleteAttachmentOutput{}, fmt.Errorf("delete attachment %s: %w", in.AttachmentID, err)
		}
		return nil, DeleteAttachmentOutput{AttachmentID: in.AttachmentID, Deleted: true}, nil
	}
}

// registerWriteTools registers all write Confluence tools on the server.
func registerWriteTools(s *mcp.Server, client *confluence.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "confluence_create_page",
		Description: "Create a new Confluence page",
		Annotations: nonDestructiveHint,
	}, createPage(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "confluence_update_page",
		Description: "Update title and/or content of an existing Confluence page",
		Annotations: nonDestructiveHint,
	}, updatePage(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "confluence_delete_page",
		Description: "Delete a Confluence page",
	}, deletePage(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "confluence_add_page_label",
		Description: "Add a label to a Confluence page",
		Annotations: nonDestructiveHint,
	}, addPageLabel(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "confluence_upload_attachment",
		Description: "Upload a file attachment to a Confluence page",
		Annotations: nonDestructiveHint,
	}, uploadAttachment(client))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "confluence_delete_attachment",
		Description: "Delete a Confluence attachment",
	}, deleteAttachment(client))
}
