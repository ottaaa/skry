package editor

import (
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestAlignLinesPairsAdjacentDeleteInsert(t *testing.T) {
	left := "a\nb\nc\n"
	right := "a\nB\nc\n"
	rows := AlignLines(left, right)
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(rows))
	}
	if rows[0].Op != DiffEqual {
		t.Errorf("row0 op: want Equal, got %v", rows[0].Op)
	}
	if rows[1].Op != DiffChange {
		t.Errorf("row1 op: want Change, got %v", rows[1].Op)
	}
	if rows[1].Left != "b" || rows[1].Right != "B" {
		t.Errorf("row1 content: got (%q,%q)", rows[1].Left, rows[1].Right)
	}
	if rows[2].Op != DiffEqual {
		t.Errorf("row2 op: want Equal, got %v", rows[2].Op)
	}
}

func TestAlignLinesPureAddAndDelete(t *testing.T) {
	left := "a\nb\n"
	right := "a\nb\nc\n"
	rows := AlignLines(left, right)
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(rows))
	}
	if rows[2].Op != DiffAdd || rows[2].Right != "c" {
		t.Errorf("last row: got %+v", rows[2])
	}
	if rows[2].LeftNum != 0 {
		t.Errorf("added row should have left line number 0, got %d", rows[2].LeftNum)
	}
}

func TestAlignLinesDeletion(t *testing.T) {
	left := "a\nb\nc\n"
	right := "a\nc\n"
	rows := AlignLines(left, right)
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(rows))
	}
	if rows[1].Op != DiffDel || rows[1].Left != "b" {
		t.Errorf("deleted row: got %+v", rows[1])
	}
	if rows[1].RightNum != 0 {
		t.Errorf("deleted row should have right line number 0, got %d", rows[1].RightNum)
	}
}

func TestSplitKeep(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty string yields nil", "", nil},
		{"single line without newline", "abc", []string{"abc"}},
		{"single line with trailing newline", "abc\n", []string{"abc"}},
		{"multiple lines trailing newline stripped", "a\nb\nc\n", []string{"a", "b", "c"}},
		{"multiple lines no trailing newline", "a\nb\nc", []string{"a", "b", "c"}},
		{"intentional blank line preserved", "a\n\nb\n", []string{"a", "", "b"}},
		{"only newline yields single empty element", "\n", []string{""}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := splitKeep(c.in)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("splitKeep(%q) = %#v, want %#v", c.in, got, c.want)
			}
		})
	}
}

// TestRenderSplitConstantRowWidth covers a scrollbar-alignment regression:
// at odd widths the per-side integer division dropped one column on content
// rows, so the scrollbar character drifted left between content and empty
// trailing rows ("凸凹" appearance). Every emitted row, content or empty,
// must have visible width == requested width.
func TestRenderSplitConstantRowWidth(t *testing.T) {
	rows := []DiffRow{
		{Op: DiffEqual, LeftNum: 1, RightNum: 1, Left: "package main", Right: "package main"},
		{Op: DiffChange, LeftNum: 2, RightNum: 2, Left: "old line", Right: "new line"},
		{Op: DiffDel, LeftNum: 3, Left: "removed"},
		{Op: DiffAdd, RightNum: 3, Right: "added"},
		{Op: DiffEqual, LeftNum: 4, RightNum: 4, Left: "}", Right: "}"},
	}
	// Mix of even and odd widths plus a couple of small/large extremes.
	for _, w := range []int{40, 41, 60, 61, 80, 81, 100, 101, 200, 201} {
		out := RenderSplit(rows, 0, 8, w)
		for i, line := range strings.Split(out, "\n") {
			if got := ansi.StringWidth(line); got != w {
				t.Errorf("width=%d row=%d: visible width = %d, want %d", w, i, got, w)
			}
		}
	}
}
