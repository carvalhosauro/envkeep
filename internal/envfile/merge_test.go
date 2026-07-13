package envfile

import (
	"reflect"
	"testing"
)

func TestUnionIncomingWinsBaseKept(t *testing.T) {
	base := Env{"A": "1", "B": "2"}
	incoming := Env{"B": "9", "C": "3"}
	got := base.Union(incoming)
	want := Env{"A": "1", "B": "9", "C": "3"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Union = %v, want %v", got, want)
	}
	// Inputs must not be mutated.
	if base["B"] != "2" || len(incoming) != 2 {
		t.Error("Union mutated an input")
	}
}

func TestWithout(t *testing.T) {
	m := Env{"A": "1", "PORT": "3000", "B": "2"}
	override := Env{"PORT": "3001"}
	got := m.Without(override)
	want := Env{"A": "1", "B": "2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Without = %v, want %v", got, want)
	}
}
