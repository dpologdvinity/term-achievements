# term-achievements hook for zsh: eval "$(term-achievements init zsh)"
# EPOCHREALTIME (microsecond clock) comes from this module.
zmodload zsh/datetime 2>/dev/null
typeset -g __termach_cmd="" __termach_prev="" __termach_t0=""
# Runs just before a command: remember the line and when it started.
_termach_preexec() {
  __termach_cmd=$1
  __termach_t0=$EPOCHREALTIME
}
# Runs before the next prompt: report the finished command. $? must be read first and
# returned last so the hook never changes the exit status the user sees.
_termach_precmd() {
  local e=$?
  if [[ -n $__termach_t0 ]]; then
    local ms=$(( (EPOCHREALTIME - __termach_t0) * 1000 ))
    command term-achievements record --exit "$e" --ms "${ms%.*}" --prev "$__termach_prev" -- "$__termach_cmd" 2>/dev/null
    __termach_prev=$__termach_cmd
    __termach_t0=""
  fi
  return $e
}
# add-zsh-hook ignores a function that is already registered, so sourcing twice is safe.
autoload -Uz add-zsh-hook
add-zsh-hook preexec _termach_preexec
add-zsh-hook precmd _termach_precmd
