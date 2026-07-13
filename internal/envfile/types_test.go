package envfile

import "testing"

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
