package search

import "testing"

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
