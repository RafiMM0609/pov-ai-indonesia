package ai

import "testing"

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "pure json",
			input:    `{"foo": "bar"}`,
			expected: `{"foo": "bar"}`,
		},
		{
			name:     "markdown json block",
			input:    "```json\n{\n  \"foo\": \"bar\"\n}\n```",
			expected: "{\n  \"foo\": \"bar\"\n}",
		},
		{
			name:     "markdown standard block",
			input:    "```\n{\n  \"foo\": \"bar\"\n}\n```",
			expected: "{\n  \"foo\": \"bar\"\n}",
		},
		{
			name:     "wrapped in text",
			input:    `Here is your JSON: {"foo": "bar"} Hope you like it!`,
			expected: `{"foo": "bar"}`,
		},
		{
			name:     "no JSON",
			input:    `plain text`,
			expected: `plain text`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSON(tt.input)
			if got != tt.expected {
				t.Errorf("extractJSON() = %q, want %q", got, tt.expected)
			}
		})
	}
}
