package app

import "testing"

func TestClampTreeOuter(t *testing.T) {
	cases := []struct {
		name     string
		v, total int
		want     int
	}{
		{"below min clamps up", 10, 200, treeWidthMin},
		{"above max clamps down", 500, 200, 140}, // 200 * 7/10
		{"inside range kept", 60, 200, 60},
		{"min == clamp floor", treeWidthMin, 200, treeWidthMin},
		{"narrow terminal: max is min+8", 100, 30, treeWidthMin + 8},
		{"narrow terminal: value respects widened max", treeWidthMin + 4, 30, treeWidthMin + 4},
		{"zero total: max falls back to min+8", 50, 0, treeWidthMin + 8},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := clampTreeOuter(c.v, c.total); got != c.want {
				t.Errorf("clampTreeOuter(%d, %d) = %d, want %d", c.v, c.total, got, c.want)
			}
		})
	}
}
