package mg

import "testing"

func TestExtractRoomID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/r/8GJG", "8GJG"},
		{"/r/MLL6", "MLL6"},
		{"https://magicgarden.gg/r/ABC123", "ABC123"},
		{"/r/a1B2", "a1B2"},
		{"no room here", ""},
		{"", ""},
		{"/r/", ""},
	}
	for _, tt := range tests {
		got := extractRoomID(tt.input)
		if got != tt.expected {
			t.Errorf("extractRoomID(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestVersionRegex(t *testing.T) {
	tests := []struct {
		input   string
		version string
	}{
		{`<script src="/version/117/bundle.js"></script>`, "117"},
		{`href="/version/120/style.css"`, "120"},
		{"no version", ""},
	}
	for _, tt := range tests {
		m := versionRegex.FindStringSubmatch(tt.input)
		got := ""
		if m != nil {
			got = m[1]
		}
		if got != tt.version {
			t.Errorf("versionRegex on %q = %q, want %q", tt.input, got, tt.version)
		}
	}
}
