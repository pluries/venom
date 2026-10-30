package interpolate

import "testing"

// TestTitle pins the "title" template helper against strings.Title's known output (deprecated
// since Go 1.18, SA1019 — #2028) for the shapes that exercise its word-boundary rule: ASCII words,
// punctuation, underscores, leading space, digits, mixed case, whitespace other than the plain
// space, and non-ASCII letters. Parity was also checked by hand against strings.Title directly
// before this replacement landed; this test is what keeps that guarantee from silently drifting.
func TestTitle(t *testing.T) {
	tests := []struct{ in, want string }{
		{"hello world", "Hello World"},
		{"  leading spaces", "  Leading Spaces"},
		{"under_score word", "Under_score Word"},
		{"foo-bar_baz", "Foo-Bar_baz"},
		{"éclair du jour", "Éclair Du Jour"},
		{"123abc def456", "123abc Def456"},
		{"", ""},
		{"single", "Single"},
		{"MiXeD CaSe", "MiXeD CaSe"},
		{"tab\tseparated\nlines", "Tab\tSeparated\nLines"},
		{"日本語 test", "日本語 Test"},
		{"foo.bar", "Foo.Bar"},
	}
	for _, tt := range tests {
		if got := title(tt.in); got != tt.want {
			t.Errorf("title(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
