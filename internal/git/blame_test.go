package git

import "testing"

func TestParseBlameOutputMinimal(t *testing.T) {
	// Minimal synthetic porcelain output: one line, one commit.
	out := "0000000000000000000000000000000000000000 1 1 1\n" +
		"author Alice\n" +
		"author-time 1735689600\n" + // 2025-01-01
		"summary initial\n" +
		"\tpackage foo\n"
	got := parseBlameOutput(out)
	if len(got) != 1 {
		t.Fatalf("want 1 line, got %d", len(got))
	}
	l := got[0]
	if l.Author != "Alice" {
		t.Errorf("author: got %q", l.Author)
	}
	if l.Summary != "initial" {
		t.Errorf("summary: got %q", l.Summary)
	}
	if l.Text != "package foo" {
		t.Errorf("text: got %q", l.Text)
	}
	if l.Line != 1 {
		t.Errorf("line: got %d", l.Line)
	}
	if l.Date == "" {
		t.Errorf("date should be filled from author-time")
	}
}

func TestParseBlameOutputReusesCommitMetadata(t *testing.T) {
	// Second line from same commit: only the header + \t content repeats; the
	// author/summary metadata is NOT repeated by git in porcelain format.
	out := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa 1 1 2\n" +
		"author Alice\n" +
		"author-time 1735689600\n" +
		"summary initial\n" +
		"\tline one\n" +
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa 2 2\n" +
		"\tline two\n"
	got := parseBlameOutput(out)
	if len(got) != 2 {
		t.Fatalf("want 2 lines, got %d", len(got))
	}
	if got[0].Author != "Alice" || got[1].Author != "Alice" {
		t.Errorf("author should be inherited across lines of same commit: got %q / %q", got[0].Author, got[1].Author)
	}
	if got[1].Line != 2 || got[1].Text != "line two" {
		t.Errorf("second line: got %+v", got[1])
	}
}
