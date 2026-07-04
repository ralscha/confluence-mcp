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
