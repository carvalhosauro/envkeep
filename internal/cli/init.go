package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/carvalhosauro/envkeep/internal/config"
)

// Init bootstraps envkeep for a repo in one obvious step (#2): it records the
// tracked env filename in the repo config and performs the first push of the
// current worktree's env file into the vault — the same union-merge +
// marker-save path as Push, so the initial 3-way base is set and
// status/pull/push behave correctly from the start. It never clobbers an
// already-initialized repo: existing config or vault state makes it a no-op
// (or an error, when --env-file contradicts what is already tracked).
func Init(w io.Writer, cwd, envFileFlag string, dry bool) error {
	ctx, err := Resolve(cwd, envFileFlag, "")
	if err != nil {
		return err
	}
	return initRepo(ctx, w, envFileFlag, dry)
}

func initRepo(ctx *Context, w io.Writer, envFileFlag string, dry bool) error {
	p := &printer{w: w}

	// Initialized == any envkeep state exists: the repo config, or anything in
	// the vault dir (a named environment's directory or the legacy flat vault,
	// whatever its filename). Either way, don't clobber.
	configured := fileExists(config.Path(ctx.CommonDir))
	vaultEntries, err := os.ReadDir(ctx.vaultDir())
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if configured || len(vaultEntries) > 0 {
		if envFileFlag != "" {
			// ctx.EnvFile already reflects the flag (Resolve), so compare
			// against what the repo actually has configured.
			cfg, err := config.Load(ctx.CommonDir)
			if err != nil {
				return err
			}
			if cfg.EnvFile != envFileFlag {
				return fmt.Errorf("already initialized tracking %q; change it with 'envkeep config set env_file %s'", cfg.EnvFile, envFileFlag)
			}
		}
		p.printf("already initialized; envkeep state lives in %s (nothing to do)\n", filepath.Dir(config.Path(ctx.CommonDir)))
		return p.err
	}

	// Validate the seed file before writing anything, so a failed init leaves
	// no partial state behind.
	if !fileExists(ctx.self.localPath) {
		return fmt.Errorf("no %s in this worktree to seed the vault from; create it, then re-run init", ctx.EnvFile)
	}

	if dry {
		p.printf("would write %s (env_file=%s)\n", config.Path(ctx.CommonDir), ctx.EnvFile)
	} else {
		if err := config.Save(ctx.CommonDir, config.Config{EnvFile: ctx.EnvFile}); err != nil {
			return err
		}
		p.printf("wrote %s (env_file=%s)\n", config.Path(ctx.CommonDir), ctx.EnvFile)
	}
	if p.err != nil {
		return p.err
	}
	if err := push(ctx, w, PushOpts{SyncOpts: SyncOpts{DryRun: dry}}); err != nil {
		return err
	}
	p.printf("initialized envkeep for this repo\n")
	return p.err
}

// fileExists reports whether path exists (any stat error counts as absent).
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
