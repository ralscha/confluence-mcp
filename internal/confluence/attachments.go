package confluence

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strconv"
)

// GetPageAttachments retrieves attachments for a page.
func (c *Client) GetPageAttachments(ctx context.Context, pageID string, limit int) (*AttachmentSearchResult, error) {
	query := url.Values{}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}

	var result AttachmentSearchResult
	if err := c.doJSON(ctx, "GET", "wiki/api/v2/pages/"+url.PathEscape(pageID)+"/attachments", query, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UploadAttachment uploads a file as an attachment to a page.
func (c *Client) UploadAttachment(ctx context.Context, pageID, filename, mimeType string, data []byte) (*Attachment, error) {
	var result struct {
		Results []Attachment `json:"results"`
	}

	if err := c.doMultipart(ctx, "POST", "wiki/api/v2/pages/"+url.PathEscape(pageID)+"/attachments", nil, filename, mimeType, data, &result); err != nil {
		return nil, err
	}

	if len(result.Results) == 0 {
		return nil, fmt.Errorf("confluence: no attachment returned after upload")
	}

	return &result.Results[0], nil
}

// DownloadAttachment downloads the content of an attachment by ID.
func (c *Client) DownloadAttachment(ctx context.Context, attachmentID string) ([]byte, error) {
	req, err := c.newRequest(ctx, "GET", "wiki/api/v2/attachments/"+url.PathEscape(attachmentID)+"/data", nil, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("confluence: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, parseAPIError(resp.StatusCode, body)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("confluence: reading attachment data: %w", err)
	}

	return data, nil
}

// DeleteAttachment deletes an attachment by ID.
func (c *Client) DeleteAttachment(ctx context.Context, attachmentID string) error {
	return c.doJSON(ctx, "DELETE", "wiki/api/v2/attachments/"+url.PathEscape(attachmentID), nil, nil, nil)
}
