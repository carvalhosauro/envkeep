package envfile

import "maps"

// Union overlays incoming onto the receiver: incoming wins on shared keys, and
// keys present only in the receiver are preserved. Neither input is mutated.
//
// This is the primitive behind both sync directions:
//   - push:  vault.Union(localWithoutOverrides) — local wins, vault-only kept,
//     so a worktree that lacks a key can never delete it from the vault (D8).
//   - pull:  vault.Union(override) — the worktree override wins (D9).
func (e Env) Union(incoming Env) Env {
	out := make(Env, len(e)+len(incoming))
	maps.Copy(out, e)
	maps.Copy(out, incoming)
	return out
}

// Without returns a copy of the receiver with every key present in exclude
// removed. Used to strip worktree-local override keys before pushing to the
// shared vault (D9).
func (e Env) Without(exclude Env) Env {
	out := make(Env, len(e))
	for k, v := range e {
		if _, skip := exclude[k]; skip {
			continue
		}
		out[k] = v
	}
	return out
}
