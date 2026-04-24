package git

import (
	"reflect"
	"testing"
)

func TestParseLogOutput(t *testing.T) {
	in := "aaa111\x1fa111\x1fAlice\x1f2026-04-01\x1fadd foo\n" +
		"bbb222\x1fb222\x1fBob\x1f2026-04-02\x1ffix bar\n"
	got := parseLogOutput(in)
	want := []Commit{
		{Hash: "aaa111", Short: "a111", Author: "Alice", Date: "2026-04-01", Subject: "add foo"},
		{Hash: "bbb222", Short: "b222", Author: "Bob", Date: "2026-04-02", Subject: "fix bar"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestParseLogOutputIgnoresBlankAndMalformed(t *testing.T) {
	in := "\n" +
		"notenough\x1fparts\n" +
		"aaa\x1fa\x1fC\x1f2026\x1fok\n"
	got := parseLogOutput(in)
	if len(got) != 1 {
		t.Fatalf("want 1 valid commit, got %d (%#v)", len(got), got)
	}
}
