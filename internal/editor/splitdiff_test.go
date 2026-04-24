package editor

import "testing"

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
