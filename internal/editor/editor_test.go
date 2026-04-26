package editor

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ottaaa/skry/internal/git"
)

func TestIsBinary(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want bool
	}{
		{"nil", nil, false},
		{"empty", []byte{}, false},
		{"plain ascii", []byte("hello world\n"), false},
		{"utf8 japanese", []byte("こんにちは"), false},
		{"leading nul", []byte{0x00, 'a', 'b'}, true},
		{"trailing nul in first 8kb", append(bytes.Repeat([]byte("a"), 100), 0x00), true},
		{"nul at boundary (byte 7999)", makeBytesWithNULAt(7999), true},
		{"nul just past boundary (byte 8000)", makeBytesWithNULAt(8000), false},
		{"large pure text (16 KB)", bytes.Repeat([]byte("abcd"), 4096), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isBinary(c.in); got != c.want {
				t.Errorf("isBinary(%s) = %v, want %v", c.name, got, c.want)
			}
		})
	}
}

func makeBytesWithNULAt(idx int) []byte {
	b := bytes.Repeat([]byte("a"), idx+10)
	b[idx] = 0
	return b
}

func TestHumanSize(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024*1024*1024 + 1024*1024*512, "1.5 GB"},
		{1024 * 1024 * 1024 * 1024, "1.0 TB"},
	}
	for _, c := range cases {
		if got := humanSize(c.in); got != c.want {
			t.Errorf("humanSize(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIsChanged(t *testing.T) {
	changed := []git.Status{
		git.StatusModified,
		git.StatusAdded,
		git.StatusDeleted,
		git.StatusRenamed,
		git.StatusUntracked,
	}
	for _, s := range changed {
		if !isChanged(s) {
			t.Errorf("isChanged(%q) = false, want true", s)
		}
	}
	notChanged := []git.Status{"", git.Status("X"), git.Status(" ")}
	for _, s := range notChanged {
		if isChanged(s) {
			t.Errorf("isChanged(%q) = true, want false", s)
		}
	}
}

func TestIsNewFile(t *testing.T) {
	if !isNewFile(git.StatusAdded) {
		t.Error("StatusAdded should be new")
	}
	if !isNewFile(git.StatusUntracked) {
		t.Error("StatusUntracked should be new")
	}
	for _, s := range []git.Status{git.StatusModified, git.StatusDeleted, git.StatusRenamed, ""} {
		if isNewFile(s) {
			t.Errorf("isNewFile(%q) = true, want false", s)
		}
	}
}

func TestRenderBinaryContainsMessage(t *testing.T) {
	out := renderBinary(2048, 10)
	if !strings.Contains(out, "Binary file") {
		t.Errorf("renderBinary output missing label: %q", out)
	}
	if !strings.Contains(out, "2.0 KB") {
		t.Errorf("renderBinary output missing size: %q", out)
	}
}
