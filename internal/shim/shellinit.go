package shim

import (
	"fmt"
	"strings"

	"github.com/naveedcs/aip/internal/paths"
)

const shellInitTemplate = `# aip shell integration
case ":$PATH:" in
  *:%[1]s:*) ;;
  *) PATH=%[1]s${PATH:+":$PATH"} ;;
esac
export PATH
export AIP_SHELL_INIT=1

aip() {
  case "$1" in
    use|deactivate)
      _aip_out="$(command aip "$@")"
      _aip_status=$?
      if [ "$_aip_status" -ne 0 ]; then
        return "$_aip_status"
      fi
      eval "$_aip_out"
      ;;
    *)
      command aip "$@"
      ;;
  esac
}

%[2]s
`

const promptSnippetZsh = `setopt PROMPT_SUBST 2>/dev/null
_aip_prompt_segment() {
  if [ -n "${AIP_PROFILE:-}" ]; then
    printf '(aip:%s) ' "$AIP_PROFILE"
  fi
}
case "$PROMPT" in
  *'$(_aip_prompt_segment)'*) ;;
  *) PROMPT='$(_aip_prompt_segment)'"$PROMPT" ;;
esac`

const promptSnippetBash = `_aip_prompt_segment() {
  if [ -n "${AIP_PROFILE:-}" ]; then
    printf '(aip:%s) ' "$AIP_PROFILE"
  fi
}
case "$PS1" in
  *'$(_aip_prompt_segment)'*) ;;
  *) PS1='$(_aip_prompt_segment)'"$PS1" ;;
esac`

func ShellInit(shell string, p paths.Paths) (string, error) {
	var promptSnippet string
	switch shell {
	case "zsh":
		promptSnippet = promptSnippetZsh
	case "bash":
		promptSnippet = promptSnippetBash
	default:
		return "", fmt.Errorf("unsupported shell %q", shell)
	}

	return fmt.Sprintf(shellInitTemplate, shellQuote(p.ShimsDir), promptSnippet), nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
