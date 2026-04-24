package search

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGrepLine(t *testing.T) {
	cases := []struct {
		in      string
		want    GrepHit
		wantOK  bool
	}{
		{"README.md:3:hello world", GrepHit{Path: "README.md", Line: 3, Text: "hello world"}, true},
		{"a/b/c.go:42:func foo() {", GrepHit{Path: "a/b/c.go", Line: 42, Text: "func foo() {"}, true},
		{"noline", GrepHit{}, false},
		{"path:notanumber:text", GrepHit{}, false},
		{"path:10:", GrepHit{Path: "path", Line: 10, Text: ""}, true},
	}
	for _, c := range cases {
		got, ok := parseGrepLine(c.in)
		if ok != c.wantOK {
			t.Errorf("parseGrepLine(%q) ok = %v, want %v", c.in, ok, c.wantOK)
			continue
		}
		if ok && got != c.want {
			t.Errorf("parseGrepLine(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestRunFallback(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "a.txt", "hello world\nHELLO again\nno match here\n")
	writeFile(t, root, "sub/b.txt", "another hello\ngoodbye\n")
	writeFile(t, root, "empty.txt", "")

	files := []string{"a.txt", "sub/b.txt", "empty.txt", "does-not-exist.txt"}

	t.Run("case-insensitive matches across files", func(t *testing.T) {
		hits, err := runFallback(root, "hello", files)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(hits) != 3 {
			t.Fatalf("want 3 hits, got %d: %+v", len(hits), hits)
		}
		// Hits should preserve file order and carry 1-based line numbers.
		want := []GrepHit{
			{Path: "a.txt", Line: 1, Text: "hello world"},
			{Path: "a.txt", Line: 2, Text: "HELLO again"},
			{Path: "sub/b.txt", Line: 1, Text: "another hello"},
		}
		for i, w := range want {
			if hits[i] != w {
				t.Errorf("hit[%d] = %+v, want %+v", i, hits[i], w)
			}
		}
	})

	t.Run("no matches returns empty", func(t *testing.T) {
		hits, err := runFallback(root, "zzz-not-present", files)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(hits) != 0 {
			t.Errorf("want 0 hits, got %d", len(hits))
		}
	})

	t.Run("caps at 500+ hits", func(t *testing.T) {
		big := strings.Repeat("x hit line\n", 600)
		writeFile(t, root, "big.txt", big)
		hits, err := runFallback(root, "hit", []string{"big.txt"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(hits) < 500 || len(hits) > 501 {
			t.Errorf("expected ~500 hits (cap is 500 exclusive), got %d", len(hits))
		}
	})
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
