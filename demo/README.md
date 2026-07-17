# demo — recording the README gifs

The gifs embedded in the top-level README are recorded with
[vhs](https://github.com/charmbracelet/vhs) from the `.tape` scripts in
[`tapes/`](tapes). Every tape runs against a throwaway repo built by
[`setup-demo-repo.sh`](setup-demo-repo.sh) (two worktrees, `dev*` + `staging`
environments), so recordings are deterministic — re-recording after a UI
change produces a comparable gif.

## Prerequisites

| Tool | Install |
|------|---------|
| `vhs` | `go install github.com/charmbracelet/vhs@latest` |
| `ttyd` | prebuilt binary from <https://github.com/tsl0922/ttyd/releases> into `~/.local/bin` |
| `ffmpeg` | distro package (`dnf install ffmpeg` / `brew install ffmpeg`) |

## Record

```sh
make demos            # all tapes -> demo/*.gif
./demo/record.sh use  # just one tape (demo/tapes/use.tape)
```

`record.sh` builds `./bin/envkeep`, puts it first on `PATH`, resets the demo
repo before each tape, and runs vhs. Gifs are written next to this file and
committed to the repo (the README references them by relative path).

## Tapes

| Tape | Shows | README section |
|------|-------|----------------|
| `demo.tape` | hero tour: status → use staging → .env rewritten | top of README |
| `use.tape` | `use <env>` switches and rewrites `.env` | Environments |
| `create.tape` | `use -c` = `git checkout -b` for envs | Environments |
| `cascade.tape` | `use --cascade` switches every worktree | Environments |
| `drift.tape` | edit → push → pull between worktrees | The problem |
| `hook.tape` | cd into a drifted worktree, hook warns | Shell integration |

## Editing a tape

Tapes are plain [vhs syntax](https://github.com/charmbracelet/vhs#vhs-command-reference).
Conventions used here:

- `Set Theme "Catppuccin Mocha"`, `FontSize 20`, `Width 1100` — keep them
  identical across tapes so the README looks uniform.
- Setup that shouldn't appear on screen goes in a `Hide` … `Show` block.
- End on a `Sleep 4s` so the last frame lingers before the gif loops.
