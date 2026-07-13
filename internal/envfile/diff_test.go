package envfile

import (
	"reflect"
	"testing"
)

func TestDiff(t *testing.T) {
	from := Env{"KEEP": "x", "CHANGE": "old", "GONE": "z"}
	to := Env{"KEEP": "x", "CHANGE": "new", "ADD": "y"}
	got := from.Diff(to)

	want := Delta{
		Added:   []Change{{Key: "ADD", New: "y"}},
		Changed: []Change{{Key: "CHANGE", Old: "old", New: "new"}},
		Removed: []Change{{Key: "GONE", Old: "z"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Diff = %+v, want %+v", got, want)
	}
	if got.Empty() {
		t.Error("Empty() = true, want false")
	}
}

func TestDiffEmpty(t *testing.T) {
	m := Env{"A": "1"}
	if !m.Diff(Env{"A": "1"}).Empty() {
		t.Error("Empty() = false for identical sets")
	}
}
