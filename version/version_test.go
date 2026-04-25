package version

import "testing"

func TestStringWithoutRevision(t *testing.T) {
	saved := Revision
	defer func() { Revision = saved }()
	Revision = ""
	if got := String(); got != Version {
		t.Errorf("String() = %q, want %q", got, Version)
	}
}

func TestStringWithRevision(t *testing.T) {
	saved := Revision
	defer func() { Revision = saved }()
	Revision = "abc1234"
	want := Version + " (abc1234)"
	if got := String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
