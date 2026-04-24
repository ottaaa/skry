package git

import "testing"

func TestDecode(t *testing.T) {
	cases := []struct {
		x, y byte
		want Status
	}{
		{'?', '?', StatusUntracked},
		{' ', 'M', StatusModified},
		{'M', ' ', StatusModified},
		{'M', 'M', StatusModified},
		{'A', ' ', StatusAdded},
		{' ', 'A', StatusAdded},
		{' ', 'D', StatusDeleted},
		{'D', ' ', StatusDeleted},
		{'R', ' ', StatusRenamed},
		{'C', ' ', StatusNone}, // copy not explicitly modeled; falls through
		{' ', ' ', StatusNone},
	}
	for _, c := range cases {
		if got := decode(c.x, c.y); got != c.want {
			t.Errorf("decode(%q,%q) = %q, want %q", c.x, c.y, got, c.want)
		}
	}
}
