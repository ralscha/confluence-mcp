package confluence

import (
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strings"
)

const (
	bodyTypeStorage        = "storage"
	bodyTypeAtlasDocFormat = "atlas_doc_format"
	bodyTypePlainText      = "plain_text"
)

var (
	storageTagPattern = regexp.MustCompile(`<[^>]*>`)
	whitespacePattern = regexp.MustCompile(`\s+`)
)

// storageToPlainText performs a basic extraction of plain text from Confluence
// storage format (XHTML). This is a simplified conversion that strips HTML tags.
func storageToPlainText(storage string) string {
	text := storageTagPattern.ReplaceAllString(storage, " ")
	text = html.UnescapeString(text)
	text = whitespacePattern.ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}

// adfNode contains the small subset of Atlas Document Format needed for a
// best-effort text rendering. Unknown node types are traversed, so newer ADF
// nodes still expose any text they contain.
type adfNode struct {
	Type    string    `json:"type"`
	Text    string    `json:"text"`
	Content []adfNode `json:"content"`
}

func adfToPlainText(adf string) string {
	var root adfNode
	if err := json.Unmarshal([]byte(adf), &root); err != nil {
		// Preserve unexpected content instead of silently returning an empty
		// tool result when Confluence introduces a new representation.
		return strings.TrimSpace(adf)
	}

	var text strings.Builder
	writeADFText(&text, root)
	return normalizePlainText(text.String())
}

func writeADFText(out *strings.Builder, node adfNode) {
	switch node.Type {
	case "text":
		out.WriteString(node.Text)
		return
	case "hardBreak":
		out.WriteByte('\n')
		return
	}

	for _, child := range node.Content {
		writeADFText(out, child)
	}

	switch node.Type {
	case "paragraph", "heading", "blockquote", "codeBlock", "listItem":
		out.WriteByte('\n')
	}
}

func normalizePlainText(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line != "" || (len(normalized) > 0 && normalized[len(normalized)-1] != "") {
			normalized = append(normalized, line)
		}
	}
	return strings.TrimSpace(strings.Join(normalized, "\n"))
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

type bodyWrite struct {
	Representation string `json:"representation"`
	Value          string `json:"value"`
}

// bodyForWrite validates a public body type and converts explicit plain text
// to Confluence storage format. An omitted type remains storage for backward
// compatibility with earlier versions of this server.
func bodyForWrite(content, bodyType string) (bodyWrite, error) {
	switch bodyType {
	case "", bodyTypeStorage:
		return bodyWrite{Representation: bodyTypeStorage, Value: content}, nil
	case bodyTypePlainText:
		return bodyWrite{Representation: bodyTypeStorage, Value: plainTextToStorage(content)}, nil
	case bodyTypeAtlasDocFormat:
		if !json.Valid([]byte(content)) {
			return bodyWrite{}, fmt.Errorf("confluence: atlas_doc_format content must be valid JSON")
		}
		return bodyWrite{Representation: bodyTypeAtlasDocFormat, Value: content}, nil
	default:
		return bodyWrite{}, fmt.Errorf("confluence: unsupported body type %q (want storage, atlas_doc_format, or plain_text)", bodyType)
	}
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
		return adfToPlainText(body.AtlasDocFormat.Value)
	default:
		return ""
	}
}

// PlainText extracts a best-effort plain text version of the body.
func (b *PageBody) PlainText() string {
	return bodyToPlainText(b)
}
