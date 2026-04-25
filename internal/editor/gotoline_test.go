package editor

import (
	"strings"
	"testing"
)

func TestFindRightLineRow(t *testing.T) {
	rows := []DiffRow{
		{Op: DiffEqual, LeftNum: 1, RightNum: 1},
		{Op: DiffDel, LeftNum: 2}, // RightNum == 0
		{Op: DiffAdd, RightNum: 2},
		{Op: DiffEqual, LeftNum: 3, RightNum: 3},
		{Op: DiffChange, LeftNum: 4, RightNum: 4},
	}
	cases := []struct {
		name string
		line int
		want int
	}{
		{"line 1 maps to row 0", 1, 0},
		{"line 2 maps to row 2 (skipping deleted line)", 2, 2},
		{"line 3 maps to row 3", 3, 3},
		{"line 4 maps to row 4", 4, 4},
		{"line not present returns -1", 99, -1},
		{"line 0 (no right side, deleted only) returns -1", 0, -1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := findRightLineRow(rows, c.line); got != c.want {
				t.Errorf("findRightLineRow(line=%d) = %d, want %d", c.line, got, c.want)
			}
		})
	}
}

func TestViewModeGoToLineCenters(t *testing.T) {
	v := newViewMode()
	v.SetSize(40, 20)
	var lines []string
	for i := 1; i <= 100; i++ {
		lines = append(lines, "row")
	}
	v.Load("x.txt", join(lines))
	v.GoToLine(50)
	// height=20, target = 50 - 1 - 10 = 39 → viewport shows lines 40-59 (1-based)
	if got := v.vp.YOffset; got != 39 {
		t.Errorf("GoToLine(50) yoffset = %d, want 39", got)
	}
}

func TestViewModeGoToLineClampsTop(t *testing.T) {
	v := newViewMode()
	v.SetSize(40, 20)
	var lines []string
	for i := 1; i <= 100; i++ {
		lines = append(lines, "row")
	}
	v.Load("x.txt", join(lines))
	v.GoToLine(3) // target would be -8, clamp to 0
	if got := v.vp.YOffset; got != 0 {
		t.Errorf("GoToLine(3) yoffset = %d, want 0 (clamped)", got)
	}
}

func TestViewModeGoToLineNoOp(t *testing.T) {
	v := newViewMode()
	v.SetSize(40, 10)
	var lines []string
	for i := 1; i <= 100; i++ {
		lines = append(lines, "row")
	}
	v.Load("x.txt", join(lines))
	v.vp.SetYOffset(20)
	before := v.vp.YOffset
	v.GoToLine(0) // no-op
	if got := v.vp.YOffset; got != before {
		t.Errorf("GoToLine(0) should not change yoffset, got %d, want %d", got, before)
	}
}

func join(lines []string) string { return strings.Join(lines, "\n") }
