package confluence

import (
	"context"
	"net/url"
	"strconv"
)

// GetSpace fetches a single space by ID or key.
func (c *Client) GetSpace(ctx context.Context, spaceKeyOrID string) (*Space, error) {
	var space Space
	if err := c.doJSON(ctx, "GET", "wiki/api/v2/spaces/"+url.PathEscape(spaceKeyOrID), nil, nil, &space); err != nil {
		return nil, err
	}
	return &space, nil
}

// ListSpacesInput describes parameters for listing spaces.
type ListSpacesInput struct {
	Keys   []string
	Type   string // "global" or "personal"
	Status string // "current" or "archived"
	Limit  int
	Cursor string
}

// ListSpaces retrieves a list of spaces.
func (c *Client) ListSpaces(ctx context.Context, in ListSpacesInput) (*SpaceSearchResult, error) {
	query := url.Values{}
	if len(in.Keys) > 0 {
		for _, key := range in.Keys {
			query.Add("keys", key)
		}
	}
	if in.Type != "" {
		query.Set("type", in.Type)
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

	var result SpaceSearchResult
	if err := c.doJSON(ctx, "GET", "wiki/api/v2/spaces", query, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
