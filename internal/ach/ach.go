// Package ach holds the declarative achievement table and the unlock check.
package ach

import (
	"time"

	"github.com/dpologdvinity/termachievements/internal/stats"
)

// Achievement unlocks when Counter reaches Goal.
type Achievement struct {
	ID, Title, Desc, Counter string
	Goal                     int64
}

// All is the achievement table, in display order.
var All = []Achievement{
	{"first_steps", "First Steps", "run your first command", "commands", 1},
	{"warming_up", "Warming Up", "run 100 commands", "commands", 100},
	{"keyboard_warrior", "Keyboard Warrior", "run 1,000 commands", "commands", 1000},
	{"shell_dweller", "Shell Dweller", "run 10,000 commands", "commands", 10000},
	{"scratchpad", "Scratchpad", "run 100 commands in a temp dir", "cwd_tmp", 100},
	{"escaped_vim", "Escaped Vim", "exit vim 10 times", "vim", 10},
	{"vim_is_home", "Vim Is Home", "exit vim 500 times", "vim", 500},
	{"church_of_emacs", "Church of Emacs", "open emacs 10 times", "emacs", 10},
	{"nano_second", "Nano Second", "open nano 10 times", "nano", 10},
	{"rm_rf_survivor", "rm -rf Survivor", "run rm -rf and live to tell", "rm_rf", 1},
	{"vroom_vroom", "Vroom Vroom", "type gti 50 times", "gti", 50},
	{"fat_fingers", "Fat Fingers", "make 100 typos", "typos", 100},
	{"choo_choo", "Choo Choo", "run sl", "sl", 1},
	{"great_power", "With Great Power", "use sudo for the first time", "sudo", 1},
	{"pretty_please", "Pretty Please", "retry a command with sudo !!", "sudo_retry", 1},
	{"rtfm", "RTFM", "read a man page", "man", 1},
	{"repo_rat", "Repo Rat", "run 1,000 commands inside git repos", "cwd_repo", 1000},
	{"snapshot", "Snapshot", "make your first git commit", "commits", 1},
	{"commit_machine", "Commit Machine", "make 1,000 git commits", "commits", 1000},
	{"before_the_ci", "Up Before the CI", "make 100 commits before 9am", "commits_before_9", 100},
	{"the_force", "May the Force Be With You", "force-push a branch", "force_push", 1},
	{"rewriting_history", "Rewriting History", "run git rebase 10 times", "rebase", 10},
	{"detective", "Detective", "start a git bisect", "bisect", 1},
	{"time_traveler", "Time Traveler", "open the git reflog", "reflog", 1},
	{"squirrel", "Squirrel", "run git stash 25 times", "stash", 25},
	{"night_owl", "Night Owl", "run 100 commands between midnight and 4am", "night", 100},
	{"early_bird", "Early Bird", "run 100 commands between 5am and 7am", "early", 100},
	{"weekend_warrior", "Weekend Warrior", "run 500 commands on weekends", "weekend", 500},
	{"on_a_roll", "On a Roll", "use the terminal 10 days in a row", "best_streak", 10},
	{"habit_formed", "Habit Formed", "use the terminal 30 days in a row", "best_streak", 30},
	{"patience", "Patience of a Saint", "wait out a 10-minute build", "long_build", 1},
	{"marathon", "Marathon", "run a command for over an hour", "marathon", 1},
	{"command_not_found", "Command Not Found", "hit \"command not found\" 100 times", "not_found", 100},
	{"abort_mission", "Abort Mission", "press Ctrl-C 100 times", "interrupts", 100},
	{"never_give_up", "Never Give Up", "succeed right after 10 failures in a row", "comebacks", 1},
	{"wide_open", "Wide Open", "chmod 777 something", "chmod777", 1},
	{"no_mercy", "No Mercy", "kill -9 a process", "kill9", 1},
	{"easily_amused", "Easily Amused", "run cowsay, fortune and friends 10 times", "toys", 10},
	{"clean_slate", "Clean Slate", "clear the screen 500 times", "clear", 500},
	{"toolbox", "Toolbox", "use 30 different tracked commands", "distinct", 30},
}

// Evaluate unlocks every achievement whose goal is reached and returns the new ones in table order.
func Evaluate(s *stats.Stats, now time.Time) []Achievement {
	var got []Achievement
	for _, a := range All {
		if _, done := s.Unlocked[a.ID]; !done && s.Counters[a.Counter] >= a.Goal {
			s.Unlocked[a.ID] = now
			got = append(got, a)
		}
	}
	return got
}
