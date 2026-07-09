package confluence

import (
	"regexp"
	"strings"
)

// storageToPlainText performs a basic extraction of plain text from Confluence
// storage format (XHTML). This is a simplified conversion that strips HTML tags.
func storageToPlainText(storage string) string {
	// Remove HTML tags
	re := regexp.MustCompile(`<[^>]*>`)
	text := re.ReplaceAllString(storage, " ")

	// Decode common HTML entities
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&quot;", "\"")

	// Collapse multiple spaces
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}

// plainTextToStorage wraps plain text in minimal storage format markup.
func plainTextToStorage(text string) string {
	// Escape HTML special characters
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")
	text = strings.ReplaceAll(text, "\"", "&quot;")

	// Convert line breaks to <p> tags
	paragraphs := strings.Split(text, "\n\n")
	var result []string
	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p != "" {
			// Replace single line breaks with <br/>
			p = strings.ReplaceAll(p, "\n", "<br/>")
			result = append(result, "<p>"+p+"</p>")
		}
	}

	if len(result) == 0 {
		return "<p></p>"
	}

	return strings.Join(result, "")
}

func bodyToPlainText(body *PageBody) string {
	if body == nil {
		return ""
	}
	switch {
	case body.Storage != nil:
		return storageToPlainText(body.Storage.Value)
	case body.View != nil:
		return storageToPlainText(body.View.Value)
	case body.AtlasDocFormat != nil:
		return body.AtlasDocFormat.Value
	default:
		return ""
	}
}

// PlainText extracts a best-effort plain text version of the body.
func (b *PageBody) PlainText() string {
	return bodyToPlainText(b)
}
