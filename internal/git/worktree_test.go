package git

import (
	"reflect"
	"testing"
)

func TestWorktreesParse(t *testing.T) {
	// Mimic output of `git worktree list --porcelain`.
	out := "worktree /a\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree /b\nHEAD def\nbranch refs/heads/feat\n\n" +
		"worktree /c\nHEAD ghi\ndetached\n\n"
	res := parseWorktreesOutput(out)
	want := []Worktree{
		{Path: "/a", Head: "abc", Branch: "main"},
		{Path: "/b", Head: "def", Branch: "feat"},
		{Path: "/c", Head: "ghi", Detached: true},
	}
	if !reflect.DeepEqual(res, want) {
		t.Fatalf("got %#v, want %#v", res, want)
	}
}
