package confluence

import (
	"testing"
)

func TestStorageToPlainText(t *testing.T) {
	tests := []struct {
		name     string
		storage  string
		expected string
	}{
		{
			name:     "simple paragraph",
			storage:  "<p>Hello world</p>",
			expected: "Hello world",
		},
		{
			name:     "multiple paragraphs",
			storage:  "<p>First paragraph</p><p>Second paragraph</p>",
			expected: "First paragraph Second paragraph",
		},
		{
			name:     "with entities",
			storage:  "<p>Test &amp; example &lt;tag&gt;</p>",
			expected: "Test & example <tag>",
		},
		{
			name:     "with formatting",
			storage:  "<p><strong>Bold</strong> and <em>italic</em></p>",
			expected: "Bold and italic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := storageToPlainText(tt.storage)
			if got != tt.expected {
				t.Errorf("storageToPlainText() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestPlainTextToStorage(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected string
	}{
		{
			name:     "simple text",
			text:     "Hello world",
			expected: "<p>Hello world</p>",
		},
		{
			name:     "multiple paragraphs",
			text:     "First paragraph\n\nSecond paragraph",
			expected: "<p>First paragraph</p><p>Second paragraph</p>",
		},
		{
			name:     "line breaks",
			text:     "Line one\nLine two",
			expected: "<p>Line one<br/>Line two</p>",
		},
		{
			name:     "special characters",
			text:     "Test & <tag>",
			expected: "<p>Test &amp; &lt;tag&gt;</p>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := plainTextToStorage(tt.text)
			if got != tt.expected {
				t.Errorf("plainTextToStorage() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestADFToPlainText(t *testing.T) {
	adf := `{
		"type":"doc",
		"version":1,
		"content":[
			{"type":"heading","content":[{"type":"text","text":"Roadmap"}]},
			{"type":"paragraph","content":[{"type":"text","text":"First line"},{"type":"hardBreak"},{"type":"text","text":"second line"}]}
		]
	}`

	if got, want := adfToPlainText(adf), "Roadmap\nFirst line\nsecond line"; got != want {
		t.Errorf("adfToPlainText() = %q, want %q", got, want)
	}
}

func TestBodyForWrite(t *testing.T) {
	body, err := bodyForWrite("A & B\nnext", "plain_text")
	if err != nil {
		t.Fatalf("bodyForWrite failed: %v", err)
	}
	if got, want := body.Representation, "storage"; got != want {
		t.Errorf("representation = %q, want %q", got, want)
	}
	if got, want := body.Value, "<p>A &amp; B<br/>next</p>"; got != want {
		t.Errorf("value = %q, want %q", got, want)
	}

	if _, err := bodyForWrite("not-json", "atlas_doc_format"); err == nil {
		t.Fatal("expected invalid ADF to fail")
	}
	if _, err := bodyForWrite("text", "unknown"); err == nil {
		t.Fatal("expected unsupported body type to fail")
	}
}
