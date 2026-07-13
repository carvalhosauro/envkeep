package envfile

import "testing"

func m(pairs ...string) map[string]string {
	out := map[string]string{}
	for i := 0; i+1 < len(pairs); i += 2 {
		out[pairs[i]] = pairs[i+1]
	}
	return out
}

func TestClassifyStates(t *testing.T) {
	tests := []struct {
		name              string
		base, local, vlt  map[string]string
		want              Status
		wantConflictCount int
	}{
		{"clean", m("A", "1"), m("A", "1"), m("A", "1"), Clean, 0},
		{"ahead", m("A", "1"), m("A", "2"), m("A", "1"), Ahead, 0},
		{"behind", m("A", "1"), m("A", "1"), m("A", "2"), Behind, 0},
		{
			"diverged non-overlapping",
			m("A", "1", "B", "1"),
			m("A", "2", "B", "1"), // local changed A
			m("A", "1", "B", "2"), // vault changed B
			Diverged, 0,
		},
		{
			// Both sides moved A to the SAME value from a stale base: nothing to
			// merge, so it is clean regardless of base age (issue #1/#4).
			"reconverged to same value is clean",
			m("A", "1"),
			m("A", "2"),
			m("A", "2"),
			Clean, 0,
		},
		{
			// Agreement holds across multiple keys too, even with an empty base.
			"reconverged multi-key is clean",
			m(),
			m("A", "2", "B", "3"),
			m("A", "2", "B", "3"),
			Clean, 0,
		},
		{
			"conflict same key",
			m("A", "1"),
			m("A", "2"),
			m("A", "3"),
			Conflict, 1,
		},
		{
			"conflict add vs add",
			m(),
			m("NEW", "local"),
			m("NEW", "vault"),
			Conflict, 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, conflicts := Classify(tt.base, tt.local, tt.vlt)
			if got != tt.want {
				t.Errorf("Status = %v, want %v", got, tt.want)
			}
			if len(conflicts) != tt.wantConflictCount {
				t.Errorf("conflicts = %d, want %d (%+v)", len(conflicts), tt.wantConflictCount, conflicts)
			}
		})
	}
}

func TestClassifyConflictDetail(t *testing.T) {
	base := m("A", "1")
	local := m("A", "2")
	vault := m() // vault deleted A, local edited it
	status, conflicts := Classify(base, local, vault)
	if status != Conflict || len(conflicts) != 1 {
		t.Fatalf("got status=%v conflicts=%+v", status, conflicts)
	}
	c := conflicts[0]
	if c.Key != "A" || !c.InBase || !c.InLocal || c.InVault {
		t.Errorf("unexpected conflict detail: %+v", c)
	}
}

func TestStatusString(t *testing.T) {
	for s, want := range map[Status]string{
		Clean: "clean", Ahead: "ahead", Behind: "behind",
		Diverged: "diverged", Conflict: "conflict", Status(99): "unknown",
	} {
		if got := s.String(); got != want {
			t.Errorf("Status(%d).String() = %q, want %q", s, got, want)
		}
	}
}
