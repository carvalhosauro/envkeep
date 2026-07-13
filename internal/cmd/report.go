package cmd

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

// saveMarker records the current sync point for the worktree: base is the shared
// content just synced to, mtimes are the files' current mtimes.
func saveMarker(ctx *Context, base envfile.Env) error {
	lm, _ := mtimeNanos(ctx.LocalPath)
	vm, _ := mtimeNanos(ctx.VaultPath)
	return state.Save(ctx.GitDir, state.Marker{Base: base, LocalMTime: lm, VaultMTime: vm})
}
