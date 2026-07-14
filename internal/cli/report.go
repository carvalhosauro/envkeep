package cli

import (
	"github.com/carvalhosauro/envkeep/internal/envfile"
	"github.com/carvalhosauro/envkeep/internal/state"
)

// printDelta renders a change set as a git-style +/~/- listing.
func printDelta(p *printer, target string, d envfile.Delta) {
	p.printf("changes to %s:\n", target)
	for _, c := range d.Added {
		p.printf("  + %s=%s\n", c.Key, c.New)
	}
	for _, c := range d.Changed {
		p.printf("  ~ %s: %s -> %s\n", c.Key, c.Old, c.New)
	}
	for _, c := range d.Removed {
		p.printf("  - %s (was %s)\n", c.Key, c.Old)
	}
}

// printConflicts renders the per-key 3-way conflict detail.
func printConflicts(p *printer, cs []envfile.KeyConflict) {
	p.printf("conflicting keys:\n")
	for _, c := range cs {
		p.printf("  %s: base=%q local=%q vault=%q\n", c.Key, c.Base, c.Local, c.Vault)
	}
}

// saveMarker records the current sync point for the worktree: env is the active
// environment the local file now holds, base is env's vault content just synced
// to, mtimes are the files' current mtimes (the vault mtime is env's vault).
func saveMarker(ctx *Context, env string, base envfile.Env) error {
	lm, _ := mtimeNanos(ctx.LocalPath)
	vm, _ := mtimeNanos(ctx.vaultPath(env))
	return state.Save(ctx.GitDir, state.Marker{Env: env, Base: base, LocalMTime: lm, VaultMTime: vm})
}
