package cli

import (
	"io"

	"github.com/carvalhosauro/envkeep/internal/envfile"
	"github.com/carvalhosauro/envkeep/internal/state"
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

// assess resolves the current worktree's drift. report is false (with empty
// strings) when there is nothing to say: in sync, nothing to compare, or any
// error — check must never be noisy or fail the prompt.
func assess(cwd string) (stateStr, envFile string, report bool) {
	ctx, err := Resolve(cwd, "", "") // the hook passes no flags
	if err != nil {
		return "", "", false
	}
	localMTime, localExists := mtimeNanos(ctx.LocalPath)
	if !localExists {
		return "", "", false
	}

	// Read the marker's active env (and cached mtimes) cheaply; the active env
	// selects which vault to compare against (D25). No marker env / legacy →
	// resolveEnv falls back to default_env or the unnamed vault.
	markerEnv, markerLocal, markerVault, hasMarker, err := state.LoadStat(ctx.GitDir)
	if err != nil {
		return "", "", false
	}
	activeEnv := ctx.resolveEnv(markerEnv)

	vaultMTime, vaultExists := mtimeNanos(ctx.vaultPath(activeEnv))
	if !vaultExists {
		return "", "", false
	}
	if !hasMarker {
		return stateUnsynced, ctx.EnvFile, true
	}

	// mtime fast path: same active env AND neither file moved since the last
	// sync → definitely clean. Reads only the marker's cached env + mtimes
	// (state.LoadStat) — never the base snapshot, nor the vault (#6) — so the
	// prompt hook this backs stays cheap on every render (D7, #8). A differing
	// env means a re-point is pending, so it falls through to the slow path.
	if markerEnv == activeEnv && localMTime == markerLocal && vaultMTime == markerVault {
		return "", "", false
	}

	// mtime miss (or re-point): load the full marker (its base drives the 3-way
	// compare) and read the actual content of the active env.
	marker, ok, err := state.Load(ctx.GitDir)
	if err != nil || !ok {
		return "", "", false
	}
	vaultEnv, vaultOK, err := readVaultFor(ctx, activeEnv)
	if err != nil || !vaultOK {
		return "", "", false
	}
	localEnv, _, err := readEnv(ctx.LocalPath)
	if err != nil {
		return "", "", false
	}
	overrideEnv, err := readEnvOrEmpty(ctx.OverridePath)
	if err != nil {
		return "", "", false
	}
	st, _ := envfile.Classify(marker.Base, localEnv.Without(overrideEnv), vaultEnv)
	if st == envfile.Clean {
		// The mtime moved but the content is clean (unchanged, or reconverged
		// with the active env from a stale base — including the first check after
		// a legacy→env migration, where marker.Env is "" but content agrees).
		// Refresh the marker to retire the stale base / stale env and restore the
		// mtime fast path. Ignore write errors — check must never break the prompt.
		_ = saveMarker(ctx, activeEnv, vaultEnv)
		return "", "", false
	}
	return st.String(), ctx.EnvFile, true
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
