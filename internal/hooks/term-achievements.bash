# term-achievements hook for bash: eval "$(term-achievements init bash)"
# Number of the last history entry seen, so each command is reported once.
__termach_hist=""
# The previous command line, passed along to detect "sudo !!".
__termach_prev=""
# Runs before each prompt (via PROMPT_COMMAND). Bash has no preexec hook, so it reads the
# newest history entry with its start time. $? is saved first and returned last.
_termach_precmd() {
  local e=$? line num ts cmd
  line=$(HISTTIMEFORMAT='%s ' builtin history 1 2>/dev/null)
  read -r num ts cmd <<< "$line"
  num=${num%\*}
  # The first run only notes the current entry, so commands from before the hook was
  # loaded are not reported.
  if [[ -n $num && $num != "$__termach_hist" ]]; then
    if [[ -n $__termach_hist ]]; then
      command term-achievements record --exit "$e" --started "$ts" --prev "$__termach_prev" -- "$cmd" 2>/dev/null
      __termach_prev=$cmd
    fi
    __termach_hist=$num
  fi
  return $e
}
# Add the hook to PROMPT_COMMAND once, even if this file is sourced again.
case ";${PROMPT_COMMAND:-};" in
  *";_termach_precmd;"*) ;;
  *) PROMPT_COMMAND="_termach_precmd${PROMPT_COMMAND:+;$PROMPT_COMMAND}" ;;
esac
