package envfile

// Status is the sync relationship of a worktree's local .env and the vault,
// judged against the base (the vault content at the worktree's last sync). A
// base is required to tell "I changed it" apart from "they changed it" — see
// docs/DECISIONS.md D5.
type Status int

const (
	// Clean means neither side changed since the last sync.
	Clean Status = iota
	// Ahead means only local changed — push fast-forwards the vault.
	Ahead
	// Behind means only the vault changed — pull fast-forwards local.
	Behind
	// Diverged means both changed, on non-overlapping keys, so the two sets can
	// be merged without a decision. (When both reach the *same* value there is
	// nothing to merge — that is Clean, not Diverged.)
	Diverged
	// Conflict means both changed the same key(s) to different values.
	Conflict
)

func (s Status) String() string {
	switch s {
	case Clean:
		return "clean"
	case Ahead:
		return "ahead"
	case Behind:
		return "behind"
	case Diverged:
		return "diverged"
	case Conflict:
		return "conflict"
	default:
		return "unknown"
	}
}

// KeyConflict records one key that both sides changed differently relative to
// the base. Presence flags distinguish an absent key from one set to "".
type KeyConflict struct {
	Key                      string
	Base, Local, Vault       string
	InBase, InLocal, InVault bool
}

// Classify compares the three key/value sets and returns the overall Status
// plus, for Conflict, the per-key conflicts (sorted by key). The conflict slice
// is nil for every non-Conflict status.
func Classify(base, local, vault Env) (Status, []KeyConflict) {
	// Local and vault already agree as logical sets: there is nothing to merge,
	// so the worktree is clean no matter how stale the base is. This is what
	// keeps a stale base from producing a false "diverged" (see issue #1/#4 and
	// docs/DECISIONS.md D22).
	if local.Equal(vault) {
		return Clean, nil
	}

	localChanged := !base.Equal(local)
	vaultChanged := !base.Equal(vault)

	switch {
	case !localChanged && !vaultChanged:
		return Clean, nil
	case localChanged && !vaultChanged:
		return Ahead, nil
	case !localChanged && vaultChanged:
		return Behind, nil
	}

	// Both changed: distinguish a real conflict from a mergeable divergence.
	var conflicts []KeyConflict
	for _, k := range sortedUnionKeys(base, local, vault) {
		bv, inBase := base[k]
		lv, inLocal := local[k]
		vv, inVault := vault[k]

		// Sides agree on this key (same presence and value): not a conflict.
		if inLocal == inVault && lv == vv {
			continue
		}
		localTouched := inLocal != inBase || lv != bv
		vaultTouched := inVault != inBase || vv != bv
		if localTouched && vaultTouched {
			conflicts = append(conflicts, KeyConflict{
				Key:  k,
				Base: bv, Local: lv, Vault: vv,
				InBase: inBase, InLocal: inLocal, InVault: inVault,
			})
		}
	}

	if len(conflicts) == 0 {
		return Diverged, nil
	}
	return Conflict, conflicts
}
