// Package hook emits the shell snippet that warns, on directory change, when
// the current worktree's env file has drifted from the vault. It calls
// `envkeep check`, which is silent unless there is drift and cheap thanks to the
// binary-side mtime fast path (D7 layer 2).
//
// The emitted snippet also carries the shell-side guard (D7 layer 1): it stats
// the local `.env` and skips spawning the binary entirely when that file has not
// moved since the last check of this directory and that check was clean. So a
// quiescent worktree costs a stat (a zsh builtin — zero spawn) instead of a
// process launch on every `cd`.
package hook

import "fmt"

// Snippet returns the shell integration code for the given shell ("zsh" or
// "bash"), suitable for sourcing from .zshrc / .bashrc.
func Snippet(shell string) (string, error) {
	switch shell {
	case "zsh":
		return zshSnippet, nil
	case "bash":
		return bashSnippet, nil
	default:
		return "", fmt.Errorf("unsupported shell %q (want zsh or bash)", shell)
	}
}

// zshSnippet uses the native chpwd hook, so the check runs only when the current
// directory changes — exactly when you enter another worktree. The shell-side
// mtime guard (D7 layer 1) reads the local .env mtime with the zsh/stat builtin
// (no external process) and returns without spawning envkeep when nothing moved
// since a clean check of this directory.
const zshSnippet = `# envkeep shell integration (zsh). Add to ~/.zshrc:
#   eval "$(envkeep hook zsh)"
typeset -gA _envkeep_mtime _envkeep_clean
_envkeep_check() {
  command -v envkeep >/dev/null 2>&1 || return
  local envf="$PWD/.env"
  # Shell-side mtime guard (D7 layer 1): if this directory's .env has not moved
  # since our last check *and* that check was clean, skip the binary entirely.
  # Absent .env (a custom env_file repo the shell can't cheaply resolve) falls
  # through to the binary, which reads the repo config and decides.
  if [[ -f "$envf" ]]; then
    zmodload -F zsh/stat b:zstat 2>/dev/null
    local -a _st
    zstat -A _st +mtime -- "$envf" 2>/dev/null
    local m="$_st[1]"
    if [[ -n "$m" && "$m" == "$_envkeep_mtime[$PWD]" && "$_envkeep_clean[$PWD]" == 1 ]]; then
      return
    fi
    local out; out="$(envkeep check 2>/dev/null)"
    _envkeep_mtime[$PWD]="$m"
    if [[ -n "$out" ]]; then
      print -r -- "$out"
      _envkeep_clean[$PWD]=0
    else
      _envkeep_clean[$PWD]=1
    fi
    return
  fi
  envkeep check 2>/dev/null
}
autoload -Uz add-zsh-hook
add-zsh-hook chpwd _envkeep_check
_envkeep_check
`

// bashSnippet drives the check from PROMPT_COMMAND but acts only on a $PWD change
// (bash has no native chpwd), so sitting in one directory costs nothing. On a
// directory change, the shell-side mtime guard (D7 layer 1) skips the binary when
// returning to a directory whose .env is unchanged since a clean check. The
// per-directory memory needs associative arrays (bash 4+); older bash (e.g. the
// macOS system bash 3.2) transparently falls back to a plain per-$PWD guard.
const bashSnippet = `# envkeep shell integration (bash). Add to ~/.bashrc:
#   eval "$(envkeep hook bash)"
_envkeep_mtime() { stat -c %Y "$1" 2>/dev/null || stat -f %m "$1" 2>/dev/null; }
if ((BASH_VERSINFO[0] >= 4)); then declare -A _ENVKEEP_MT _ENVKEEP_CLEAN; fi
_envkeep_check() {
  command -v envkeep >/dev/null 2>&1 || return
  [ "$PWD" = "$_ENVKEEP_LAST_PWD" ] && return
  _ENVKEEP_LAST_PWD="$PWD"
  local envf="$PWD/.env"
  if ((BASH_VERSINFO[0] >= 4)) && [ -f "$envf" ]; then
    local m; m="$(_envkeep_mtime "$envf")"
    if [ -n "$m" ] && [ "$m" = "${_ENVKEEP_MT[$PWD]}" ] && [ "${_ENVKEEP_CLEAN[$PWD]}" = 1 ]; then
      return
    fi
    local out; out="$(envkeep check 2>/dev/null)"
    _ENVKEEP_MT[$PWD]="$m"
    if [ -n "$out" ]; then printf '%s\n' "$out"; _ENVKEEP_CLEAN[$PWD]=0; else _ENVKEEP_CLEAN[$PWD]=1; fi
    return
  fi
  envkeep check 2>/dev/null
}
case "$PROMPT_COMMAND" in
  *_envkeep_check*) ;;
  *) PROMPT_COMMAND="_envkeep_check${PROMPT_COMMAND:+; $PROMPT_COMMAND}" ;;
esac
`
