// Package hook emits the shell snippet that warns, on directory change, when
// the current worktree's env file has drifted from the vault. It calls
// `envkeep check`, which is silent unless there is drift and cheap thanks to the
// mtime fast path (D7).
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
// directory changes — exactly when you enter another worktree.
const zshSnippet = `# envkeep shell integration (zsh). Add to ~/.zshrc:
#   eval "$(envkeep hook zsh)"
_envkeep_check() {
  command -v envkeep >/dev/null 2>&1 || return
  envkeep check 2>/dev/null
}
autoload -Uz add-zsh-hook
add-zsh-hook chpwd _envkeep_check
_envkeep_check
`

// bashSnippet drives the check from PROMPT_COMMAND but guards on $PWD, so it only
// actually runs on a directory change (bash has no native chpwd).
const bashSnippet = `# envkeep shell integration (bash). Add to ~/.bashrc:
#   eval "$(envkeep hook bash)"
_envkeep_check() {
  command -v envkeep >/dev/null 2>&1 || return
  if [ "$PWD" != "$_ENVKEEP_LAST_PWD" ]; then
    _ENVKEEP_LAST_PWD="$PWD"
    envkeep check 2>/dev/null
  fi
}
case "$PROMPT_COMMAND" in
  *_envkeep_check*) ;;
  *) PROMPT_COMMAND="_envkeep_check${PROMPT_COMMAND:+; $PROMPT_COMMAND}" ;;
esac
`
