package cli

import (
	"github.com/carvalhosauro/envkeep/internal/env"
	"github.com/carvalhosauro/envkeep/internal/envfile"
	"github.com/carvalhosauro/envkeep/internal/state"
)

// drift is a worktree's computed sync state for its active environment. It is
// the single result type behind both `status` (which labels every worktree) and
// `check` (the quiet hook), so the state machine lives in one place.
type drift struct {
	env      env.Name       // the worktree's resolved active environment
	localOK  bool           // local env file exists
	vaultOK  bool           // the active env's vault exists
	hasMark  bool           // a usable sync marker exists
	clean    bool           // in sync (mtime fast path, or 3-way Classify == Clean)
	stale    bool           // clean via the slow path with a stale base/env → retire-able
	status   envfile.Status // the drift, valid only when localOK && vaultOK && hasMark && !clean
	vaultEnv envfile.Env    // the parsed active-env vault (nil on the fast path)
}

// assessWorktree computes the drift for one worktree (its paths in wt) against
// the vault of the environment it resolves to (--env > marker > default_env,
// D25). It runs the D5/D7 mtime fast path and, on a miss, the 3-way Classify.
// loadVault parses an environment's vault lazily — called at most once, and only
// on the slow path — so a quiescent repo never pays for the parse.
func assessWorktree(r *Repo, wt worktreePaths, loadVault func(env.Name) (envfile.Env, error)) (drift, error) {
	stat, hasMark, err := state.LoadStat(wt.gitDir)
	if err != nil {
		return drift{}, err
	}
	e := r.resolveEnv(stat.Env)
	d := drift{env: e, hasMark: hasMark}

	localMTime, localOK := mtimeNanos(wt.localPath)
	d.localOK = localOK
	vaultMTime, vaultOK := mtimeNanos(r.vaultPath(e))
	d.vaultOK = vaultOK
	if !localOK || !vaultOK || !hasMark {
		return d, nil
	}

	// mtime fast path: same active env AND neither file moved since the last
	// sync → clean without loading the base or parsing the vault. A differing
	// env means a re-point is pending, so it falls through to the slow path.
	if stat.Env == e && localMTime == stat.LocalMTime && vaultMTime == stat.VaultMTime {
		d.clean = true
		return d, nil
	}

	// mtime miss (or re-point): the base drives the 3-way compare.
	marker, ok, err := state.Load(wt.gitDir)
	if err != nil {
		return d, err
	}
	if !ok {
		d.hasMark = false
		return d, nil
	}
	vaultEnv, err := loadVault(e)
	if err != nil {
		return d, err
	}
	d.vaultEnv = vaultEnv
	shared, _, err := readShared(wt.localPath, wt.overridePath)
	if err != nil {
		return d, err
	}
	st, _ := envfile.Classify(marker.Base, shared, vaultEnv)
	if st == envfile.Clean {
		// Content agrees though the mtime moved (an edit-and-revert, a reconverge
		// from a stale base, or a re-point that already matches): clean, and the
		// base/env is retire-able so the fast path can be restored.
		d.clean = true
		d.stale = true
		return d, nil
	}
	d.status = st
	return d, nil
}
