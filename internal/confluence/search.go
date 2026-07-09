package confluence

import (
	"context"
	"net/url"
	"strconv"
)

// SearchContentInput describes parameters for a CQL content search.
type SearchContentInput struct {
	CQL                   string
	CQLContext            string
	Expand                []string
	Cursor                string
	Limit                 int
	Start                 int
	IncludeArchivedSpaces bool
	ExcludeCurrentSpaces  bool
	Excerpt               string
}

// SearchContent searches Confluence content using CQL.
func (c *Client) SearchContent(ctx context.Context, in SearchContentInput) (*ContentSearchResult, error) {
	query := url.Values{}
	query.Set("cql", in.CQL)
	if in.CQLContext != "" {
		query.Set("cqlcontext", in.CQLContext)
	}
	for _, expand := range in.Expand {
		query.Add("expand", expand)
	}
	if in.Cursor != "" {
		query.Set("cursor", in.Cursor)
	}
	if in.Limit > 0 {
		query.Set("limit", strconv.Itoa(in.Limit))
	}
	if in.Start > 0 {
		query.Set("start", strconv.Itoa(in.Start))
	}
	if in.IncludeArchivedSpaces {
		query.Set("includeArchivedSpaces", "true")
	}
	if in.ExcludeCurrentSpaces {
		query.Set("excludeCurrentSpaces", "true")
	}
	if in.Excerpt != "" {
		query.Set("excerpt", in.Excerpt)
	}

	var result ContentSearchResult
	if err := c.doJSON(ctx, "GET", "wiki/rest/api/search", query, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
