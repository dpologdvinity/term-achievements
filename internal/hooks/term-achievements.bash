# term-achievements hook for bash: eval "$(term-achievements init bash)"
__termach_hist=""
__termach_prev=""
_termach_precmd() {
  local e=$? line num ts cmd
  line=$(HISTTIMEFORMAT='%s ' builtin history 1 2>/dev/null)
  read -r num ts cmd <<< "$line"
  num=${num%\*}
  if [[ -n $num && $num != "$__termach_hist" ]]; then
    if [[ -n $__termach_hist ]]; then
      command term-achievements record --exit "$e" --started "$ts" --prev "$__termach_prev" -- "$cmd" 2>/dev/null
      __termach_prev=$cmd
    fi
    __termach_hist=$num
  fi
  return $e
}
case ";${PROMPT_COMMAND:-};" in
  *";_termach_precmd;"*) ;;
  *) PROMPT_COMMAND="_termach_precmd${PROMPT_COMMAND:+;$PROMPT_COMMAND}" ;;
esac
