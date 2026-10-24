# term-achievements hook for fish: term-achievements init fish | source
if not functions -q __termach_postexec
    set -g __termach_prev ""
    function __termach_postexec --on-event fish_postexec
        set -l e $status
        set -l d $CMD_DURATION
        test -n "$argv[1]"; or return
        command term-achievements record --exit $e --ms $d --prev "$__termach_prev" -- "$argv[1]" 2>/dev/null
        set -g __termach_prev $argv[1]
    end
end
