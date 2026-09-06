package confluence

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// GetPageAttachmentsInput describes filters and pagination for a page's
// attachments.
type GetPageAttachmentsInput struct {
	PageID    string
	Sort      string
	Status    []string
	MediaType string
	Filename  string
	Limit     int
	Cursor    string
}

// GetPageAttachments retrieves attachments for a page.
func (c *Client) GetPageAttachments(ctx context.Context, in GetPageAttachmentsInput) (*AttachmentSearchResult, error) {
	query := url.Values{}
	if in.Sort != "" {
		query.Set("sort", in.Sort)
	}
	for _, status := range in.Status {
		query.Add("status", status)
	}
	if in.MediaType != "" {
		query.Set("mediaType", in.MediaType)
	}
	if in.Filename != "" {
		query.Set("filename", in.Filename)
	}
	if in.Limit > 0 {
		query.Set("limit", strconv.Itoa(in.Limit))
	}
	if in.Cursor != "" {
		query.Set("cursor", in.Cursor)
	}

	var result AttachmentSearchResult
	if err := c.doJSON(ctx, "GET", "wiki/api/v2/pages/"+url.PathEscape(in.PageID)+"/attachments", query, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetAttachment retrieves attachment metadata by ID.
func (c *Client) GetAttachment(ctx context.Context, attachmentID string) (*Attachment, error) {
	var attachment Attachment
	if err := c.doJSON(ctx, "GET", "wiki/api/v2/attachments/"+url.PathEscape(attachmentID), nil, nil, &attachment); err != nil {
		return nil, err
	}
	return &attachment, nil
}

// UploadAttachment uploads a file as an attachment to a page.
func (c *Client) UploadAttachment(ctx context.Context, pageID, filename, mimeType string, data []byte) (*Attachment, error) {
	var result struct {
		Results []Attachment `json:"results"`
	}

	// Attachment creation remains a v1 operation; the v2 attachment API only
	// exposes reads and deletion.
	if err := c.doMultipart(ctx, "POST", "wiki/rest/api/content/"+url.PathEscape(pageID)+"/child/attachment", nil, filename, mimeType, data, &result); err != nil {
		return nil, err
	}

	if len(result.Results) == 0 {
		return nil, fmt.Errorf("confluence: no attachment returned after upload")
	}

	return &result.Results[0], nil
}

// DownloadAttachment downloads the content of an attachment by ID.
func (c *Client) DownloadAttachment(ctx context.Context, attachmentID string) ([]byte, error) {
	attachment, err := c.GetAttachment(ctx, attachmentID)
	if err != nil {
		return nil, fmt.Errorf("confluence: retrieving attachment metadata: %w", err)
	}
	if attachment.PageID == "" {
		return nil, fmt.Errorf("confluence: attachment %s is not attached to a page", attachmentID)
	}

	// The supported download endpoint is in v1 and requires both the page and
	// attachment IDs. It redirects to the binary data; http.Client follows the
	// redirect while applying its normal credential-forwarding safeguards.
	path := "wiki/rest/api/content/" + url.PathEscape(attachment.PageID) + "/child/attachment/" + url.PathEscape(attachmentID) + "/download"
	req, err := c.newRequest(ctx, "GET", path, nil, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("confluence: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := readLimited(resp.Body, maxResponseBytes)
		return nil, parseAPIError(resp.StatusCode, body)
	}

	if resp.ContentLength > maxAttachmentBytes {
		return nil, fmt.Errorf("confluence: attachment %s is %d bytes, exceeding the %d byte download limit", attachmentID, resp.ContentLength, maxAttachmentBytes)
	}

	data, err := readLimited(resp.Body, maxAttachmentBytes)
	if err != nil {
		return nil, fmt.Errorf("confluence: reading attachment data: %w", err)
	}

	return data, nil
}

// DeleteAttachment deletes an attachment by ID.
func (c *Client) DeleteAttachment(ctx context.Context, attachmentID string) error {
	return c.doJSON(ctx, "DELETE", "wiki/api/v2/attachments/"+url.PathEscape(attachmentID), nil, nil, nil)
}
