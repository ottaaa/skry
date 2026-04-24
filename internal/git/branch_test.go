package git

import (
	"reflect"
	"testing"
)

func TestParseBranchesOutput(t *testing.T) {
	in := "*\x00main\n" +
		" \x00feat/git-ui\n" +
		" \x00release\n"
	got := parseBranchesOutput(in)
	want := []Branch{
		{Name: "main", Current: true},
		{Name: "feat/git-ui", Current: false},
		{Name: "release", Current: false},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
