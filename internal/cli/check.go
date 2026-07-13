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
	ctx, err := Resolve(cwd, "")
	if err != nil {
		return "", "", false
	}
	localMTime, localExists := mtimeNanos(ctx.LocalPath)
	if !localExists {
		return "", "", false
	}
	vaultMTime, vaultExists := mtimeNanos(ctx.VaultPath)
	if !vaultExists {
		return "", "", false
	}
	marker, hasMarker, err := state.Load(ctx.GitDir)
	if err != nil {
		return "", "", false
	}
	if !hasMarker {
		return stateUnsynced, ctx.EnvFile, true
	}

	// mtime fast path: nothing moved since the last sync → definitely clean.
	// This is the path the prompt hook hits on every render, so it must parse
	// nothing — not even the vault (D7). The vault read is deferred to the miss
	// case below, so its cost scales with edits, not with prompt renders (#6).
	if localMTime == marker.LocalMTime && vaultMTime == marker.VaultMTime {
		return "", "", false
	}

	// mtime miss: now the actual content must be read and compared.
	vaultEnv, vaultOK, err := readVault(ctx)
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
		// with the vault from a stale base). Refresh the marker to retire a stale
		// base and restore the mtime fast path. Ignore write errors — check must
		// never fail or break the prompt.
		_ = saveMarker(ctx, vaultEnv)
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
