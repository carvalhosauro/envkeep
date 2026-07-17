package cli

import (
	"github.com/carvalhosauro/envkeep/internal/env"
	"github.com/carvalhosauro/envkeep/internal/envfile"
	"github.com/carvalhosauro/envkeep/internal/state"
)

// printDelta renders a change set as a git-style +/~/- listing. Only key names
// are shown — values are secrets and must not reach stdout (scrollback, CI
// logs), on real runs and --dry-run alike (#22).
func printDelta(p *printer, target string, d envfile.Delta) {
	p.printf("changes to %s:\n", target)
	for _, c := range d.Added {
		p.printf("  + %s\n", c.Key)
	}
	for _, c := range d.Changed {
		p.printf("  ~ %s\n", c.Key)
	}
	for _, c := range d.Removed {
		p.printf("  - %s\n", c.Key)
	}
}

// printConflicts renders the per-key 3-way conflict listing. Key names only —
// values are secrets (#22).
func printConflicts(p *printer, cs []envfile.KeyConflict) {
	p.printf("conflicting keys:\n")
	for _, c := range cs {
		p.printf("  %s\n", c.Key)
	}
}

// saveMarker records the current sync point for the worktree: e is the active
// environment the local file now holds, base is e's vault content just synced
// to, mtimes are the files' current mtimes (the vault mtime is e's vault).
func saveMarker(ctx *Context, e env.Name, base envfile.Env) error {
	lm, _ := mtimeNanos(ctx.self.localPath)
	vm, _ := mtimeNanos(ctx.vaultPath(e))
	return state.Save(ctx.self.gitDir, state.Marker{
		Stat: state.Stat{Env: e, LocalMTime: lm, VaultMTime: vm},
		Base: base,
	})
}
