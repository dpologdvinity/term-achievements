# termachievements hook for zsh: eval "$(termachievements init zsh)"
zmodload zsh/datetime 2>/dev/null
typeset -g __termach_cmd="" __termach_prev="" __termach_t0=""
_termach_preexec() {
  __termach_cmd=$1
  __termach_t0=$EPOCHREALTIME
}
_termach_precmd() {
  local e=$?
  if [[ -n $__termach_t0 ]]; then
    local ms=$(( (EPOCHREALTIME - __termach_t0) * 1000 ))
    command termachievements record --exit "$e" --ms "${ms%.*}" --prev "$__termach_prev" -- "$__termach_cmd" 2>/dev/null
    __termach_prev=$__termach_cmd
    __termach_t0=""
  fi
  return $e
}
autoload -Uz add-zsh-hook
add-zsh-hook preexec _termach_preexec
add-zsh-hook precmd _termach_precmd
