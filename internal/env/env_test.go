package env

import "testing"

func TestIsUnnamedAndString(t *testing.T) {
	if !Unnamed.IsUnnamed() {
		t.Error("Unnamed.IsUnnamed() = false, want true")
	}
	if Name("prod").IsUnnamed() {
		t.Error(`Name("prod").IsUnnamed() = true, want false`)
	}
	if Unnamed.String() != "" {
		t.Errorf("Unnamed.String() = %q, want empty", Unnamed.String())
	}
	if Name("prod").String() != "prod" {
		t.Errorf("String() = %q, want prod", Name("prod").String())
	}
}

func TestValidate(t *testing.T) {
	for _, ok := range []Name{"prod", "homo-1", "a.b_c", "Dev2"} {
		if err := ok.Validate(); err != nil {
			t.Errorf("%q.Validate() = %v, want nil", ok, err)
		}
	}
	for _, bad := range []Name{"", ".", "..", "shared", "_base", "a/b", "a b", "x*", "up/../x"} {
		if err := bad.Validate(); err == nil {
			t.Errorf("%q.Validate() = nil, want error", bad)
		}
	}
}
