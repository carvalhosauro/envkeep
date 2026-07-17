package cli

import (
	"io"

	"github.com/carvalhosauro/envkeep/internal/env"
	"github.com/carvalhosauro/envkeep/internal/envfile"
)

// Check is the lightweight per-worktree drift check invoked by the shell hook
// and by prompt integrations. With porcelain=false it prints a human sentence
// (or nothing when in sync); with porcelain=true it prints a bare state token
// (ahead/behind/diverged/conflict/unsynced) or nothing — for scripting and
// prompt segments. It stays silent on any error so it never breaks the prompt,
// and the mtime fast path keeps it cheap (D7).
func Check(w io.Writer, cwd string, porcelain bool) error {
	stateStr, envFile, report := assess(cwd)
	if !report {
		return nil
	}
	p := &printer{w: w}
	if porcelain {
		p.printf("%s\n", stateStr)
		return p.err
	}
	if stateStr == stateUnsynced {
		p.printf("envkeep: %s is not yet synced with the vault — run 'envkeep status'\n", envFile)
	} else {
		p.printf("envkeep: %s is %s vs the vault — %s\n", envFile, stateStr, suggestion(stateStr))
	}
	return p.err
}

// stateUnsynced is the pseudo-state for a worktree that has a local env and a
// vault but has never synced (no marker), so a 3-way comparison isn't possible.
const stateUnsynced = "unsynced"

// assess resolves the current worktree's drift via the shared state machine
// (assessWorktree). report is false (with empty strings) when there is nothing
// to say: in sync, nothing to compare, or any error — check must never be noisy
// or fail the prompt. On a slow-path clean with a stale base it retires the
// marker so the fast path is restored next time.
func assess(cwd string) (stateStr, envFile string, report bool) {
	ctx, err := Resolve(cwd, "", "") // the hook passes no flags
	if err != nil {
		return "", "", false
	}
	loadVault := func(e env.Name) (envfile.Env, error) {
		v, _, err := ctx.readVault(e)
		return v, err
	}
	d, err := assessWorktree(ctx.Repo, ctx.worktree(), loadVault)
	if err != nil {
		return "", "", false
	}
	switch {
	case !d.localOK, !d.vaultOK:
		return "", "", false // nothing to compare — stay silent
	case !d.hasMark:
		return stateUnsynced, ctx.EnvFile, true
	case d.clean:
		if d.stale {
			// Content agrees but the base/env is stale; retire it so the mtime
			// fast path is restored. Ignore write errors — check must never fail
			// or break the prompt.
			_ = saveMarker(ctx, d.env, d.vaultEnv)
		}
		return "", "", false
	default:
		return d.status.String(), ctx.EnvFile, true
	}
}

// suggestion maps a drift state to the command that resolves it.
func suggestion(stateStr string) string {
	switch stateStr {
	case envfile.Ahead.String():
		return "run 'envkeep push'"
	case envfile.Behind.String():
		return "run 'envkeep pull'"
	default: // diverged, conflict
		return "run 'envkeep status'"
	}
}
