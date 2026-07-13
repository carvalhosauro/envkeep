package cli

import (
	"io"

	"github.com/carvalhosauro/envkeep/internal/envfile"
	"github.com/carvalhosauro/envkeep/internal/state"
)

// Check is the lightweight per-worktree drift check invoked by the shell hook.
// It prints nothing when the local env is in sync (or when there is nothing to
// check) and a single discreet line when it has drifted. It stays silent on any
// error — outside a repo, corrupt state, etc. — so it never breaks the prompt;
// `status` is the place errors surface. The mtime fast path keeps it cheap (D7).
func Check(w io.Writer, cwd string) error {
	ctx, err := Resolve(cwd, "")
	if err != nil {
		return nil // not a resolvable repo — say nothing
	}
	vaultEnv, vaultExists, err := readVault(ctx)
	if err != nil || !vaultExists {
		return nil // no vault yet, or unreadable — nothing to nag about
	}
	localMTime, localExists := mtimeNanos(ctx.LocalPath)
	if !localExists {
		return nil
	}
	marker, hasMarker, err := state.Load(ctx.GitDir)
	if err != nil {
		return nil
	}

	p := &printer{w: w}
	if !hasMarker {
		p.printf("envkeep: %s is not yet synced with the vault — run 'envkeep status'\n", ctx.EnvFile)
		return p.err
	}

	// mtime fast path: nothing moved since the last sync → definitely clean.
	vaultMTime, _ := mtimeNanos(ctx.VaultPath)
	if localMTime == marker.LocalMTime && vaultMTime == marker.VaultMTime {
		return nil
	}

	localEnv, _, err := readEnv(ctx.LocalPath)
	if err != nil {
		return nil
	}
	overrideEnv, err := readEnvOrEmpty(ctx.OverridePath)
	if err != nil {
		return nil
	}
	st, _ := envfile.Classify(marker.Base, localEnv.Without(overrideEnv), vaultEnv)
	if st == envfile.Clean {
		return nil
	}
	p.printf("envkeep: %s is %s vs the vault — %s\n", ctx.EnvFile, st, suggestion(st))
	return p.err
}

// suggestion maps a drift state to the command that resolves it.
func suggestion(st envfile.Status) string {
	switch st {
	case envfile.Ahead:
		return "run 'envkeep push'"
	case envfile.Behind:
		return "run 'envkeep pull'"
	default: // Diverged, Conflict
		return "run 'envkeep status'"
	}
}
