package envfile

import "sort"

// Change describes one key's transition between two key/value sets.
type Change struct {
	Key string
	Old string // previous value ("" if newly added)
	New string // next value ("" if removed)
}

// Delta is the ordered difference between two key/value sets. Each slice is
// sorted by key for deterministic output.
type Delta struct {
	Added   []Change // in "to" but not "from"
	Changed []Change // in both, different value
	Removed []Change // in "from" but not "to"
}

// Empty reports whether the two sets were identical.
func (d Delta) Empty() bool {
	return len(d.Added) == 0 && len(d.Changed) == 0 && len(d.Removed) == 0
}

// Diff computes what it takes to turn the receiver into to.
func (e Env) Diff(to Env) Delta {
	var d Delta
	for _, k := range sortedUnionKeys(e, to) {
		fv, inFrom := e[k]
		tv, inTo := to[k]
		switch {
		case inFrom && !inTo:
			d.Removed = append(d.Removed, Change{Key: k, Old: fv})
		case !inFrom && inTo:
			d.Added = append(d.Added, Change{Key: k, New: tv})
		case fv != tv:
			d.Changed = append(d.Changed, Change{Key: k, Old: fv, New: tv})
		}
	}
	return d
}

func sortedUnionKeys(ms ...Env) []string {
	set := map[string]struct{}{}
	for _, m := range ms {
		for k := range m {
			set[k] = struct{}{}
		}
	}
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
