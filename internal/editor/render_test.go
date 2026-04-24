package editor

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestExpandTabsColumnAware(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"\tfoo", "    foo"},
		{"a\tb", "a   b"},   // tab stops at col 4
		{"ab\tc", "ab  c"},  // tab stops at col 4
		{"abc\td", "abc d"}, // tab stops at col 4
		{"abcd\te", "abcd    e"},
		{"no tabs here", "no tabs here"},
	}
	for _, c := range cases {
		if got := expandTabs(c.in); got != c.want {
			t.Errorf("expandTabs(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFitANSIPadsAndTruncates(t *testing.T) {
	// ASCII: pads to exact width.
	got := fitANSI("abc", 6)
	if ansi.StringWidth(got) != 6 {
		t.Errorf("pad visible width: got %d want 6 (%q)", ansi.StringWidth(got), got)
	}
	// Truncate ascii.
	got = fitANSI("abcdef", 3)
	if ansi.StringWidth(got) != 3 {
		t.Errorf("truncate visible width: got %d want 3 (%q)", ansi.StringWidth(got), got)
	}
}

func TestFitANSIWithEscapeSequences(t *testing.T) {
	// Simulate a chroma-style colored word.
	colored := "\x1b[31mhello\x1b[0m world"
	got := fitANSI(colored, 8)
	if ansi.StringWidth(got) != 8 {
		t.Errorf("visible width with ANSI: got %d want 8 (%q)", ansi.StringWidth(got), got)
	}
	if !strings.Contains(got, "\x1b[31m") {
		t.Errorf("want to keep color escape, got %q", got)
	}
}

func TestClipANSIRespectsDisplayWidth(t *testing.T) {
	got := clipANSI("日本語テスト", 4) // each CJK glyph = width 2
	if ansi.StringWidth(got) > 4 {
		t.Errorf("clipANSI overflowed: got width %d for %q", ansi.StringWidth(got), got)
	}
}
