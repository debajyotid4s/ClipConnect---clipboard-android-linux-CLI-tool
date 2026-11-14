package clip

import (
	"testing"
)

func TestIsEmptyOrTooLarge(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected bool // true = should be filtered out
	}{
		{"empty", "", true},
		{"whitespace_only", "   ", false},
		{"normal_text", "hello world", false},
		{"max_size", string(make([]byte, 262144)), false},
		{"over_max_size", string(make([]byte, 262145)), true},
		{"single_char", "a", false},
		{"unicode", "你好世界", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsEmptyOrTooLarge(tt.text)
			if result != tt.expected {
				t.Errorf("IsEmptyOrTooLarge(%q) = %v, want %v", tt.text, result, tt.expected)
			}
		})
	}
}

func TestIsSensitive(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected bool // true = should be filtered as sensitive
	}{
		{"password_manager_hint", "x-kde-passwordManagerHint", true},
		{"contains_kde_hint", "some text x-kde-passwordManagerHint more", true},
		{"normal_text", "hello world", false},
		{"contains_password_but_not_sensitive", "the password field is empty", false},
		{"empty_text", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsSensitive(tt.text)
			if result != tt.expected {
				t.Errorf("IsSensitive(%q) = %v, want %v", tt.text, result, tt.expected)
			}
		})
	}
}
