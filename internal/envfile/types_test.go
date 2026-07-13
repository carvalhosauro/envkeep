package envfile

import "testing"

func TestCloneIsIndependent(t *testing.T) {
	orig := Env{"A": "1"}
	cp := orig.Clone()
	cp["A"] = "2"
	cp["B"] = "3"
	if orig["A"] != "1" || len(orig) != 1 {
		t.Errorf("Clone not independent: orig mutated to %v", orig)
	}
	// Clone of nil is usable and empty.
	if got := Env(nil).Clone(); got == nil || len(got) != 0 {
		t.Errorf("Env(nil).Clone() = %v, want empty non-nil", got)
	}
}

func TestEqual(t *testing.T) {
	if !(Env{"A": "1"}).Equal(Env{"A": "1"}) {
		t.Error("Equal = false for identical sets")
	}
	if (Env{"A": "1"}).Equal(Env{"A": "2"}) {
		t.Error("Equal = true for differing values")
	}
	if (Env{"A": "1"}).Equal(Env{"A": "1", "B": "2"}) {
		t.Error("Equal = true for differing sizes")
	}
}
