package editor

import (
	"strings"
	"testing"
)

func countThumb(cells []string) int {
	n := 0
	for _, c := range cells {
		if strings.Contains(c, "█") {
			n++
		}
	}
	return n
}

func thumbRange(cells []string) (int, int) {
	start, end := -1, -1
	for i, c := range cells {
		if strings.Contains(c, "█") {
			if start < 0 {
				start = i
			}
			end = i
		}
	}
	return start, end
}

func TestScrollbarAllTrackWhenFits(t *testing.T) {
	cells := scrollbarChars(0, 10, 5)
	if len(cells) != 10 {
		t.Fatalf("want 10 cells, got %d", len(cells))
	}
	if countThumb(cells) != 0 {
		t.Errorf("expected no thumb when content fits, got %d thumb cells", countThumb(cells))
	}
}

func TestScrollbarThumbAtTop(t *testing.T) {
	cells := scrollbarChars(0, 10, 100)
	start, _ := thumbRange(cells)
	if start != 0 {
		t.Errorf("thumb should start at top when top=0, started at %d", start)
	}
}

func TestScrollbarThumbAtBottom(t *testing.T) {
	cells := scrollbarChars(90, 10, 100)
	_, end := thumbRange(cells)
	if end != 9 {
		t.Errorf("thumb should end at last row when scrolled to bottom, ended at %d", end)
	}
}

func TestScrollbarThumbSizeProportional(t *testing.T) {
	// content is 4x the viewport → thumb ~1/4 of height
	cells := scrollbarChars(0, 20, 80)
	count := countThumb(cells)
	if count < 4 || count > 6 {
		t.Errorf("thumb size: got %d cells, expected ~5", count)
	}
}
