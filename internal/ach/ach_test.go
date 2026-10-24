package ach

import (
	"regexp"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/dpologdvinity/termachievements/internal/stats"
)

var t0 = time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)

func TestTableInvariants(t *testing.T) {
	if len(All) != 40 {
		t.Fatal(len(All))
	}
	keys := map[string]bool{}
	for _, k := range stats.CounterKeys {
		keys[k] = true
	}
	seen := map[string]bool{}
	idRe := regexp.MustCompile(`^[a-z0-9_]+$`)
	for _, a := range All {
		if seen[a.ID] || !idRe.MatchString(a.ID) {
			t.Errorf("bad id %q", a.ID)
		}
		seen[a.ID] = true
		if !keys[a.Counter] {
			t.Errorf("%s: unknown counter %q", a.ID, a.Counter)
		}
		if a.Goal < 1 || utf8.RuneCountInString(a.Title) > 26 || utf8.RuneCountInString(a.Desc) > 44 {
			t.Errorf("%s: bad goal or length", a.ID)
		}
	}
	if All[5] != (Achievement{"escaped_vim", "Escaped Vim", "exit vim 10 times", "vim", 10}) {
		t.Fatalf("%+v", All[5])
	}
	if All[39].ID != "toolbox" {
		t.Fatal(All[39])
	}
}

func TestEvaluate(t *testing.T) {
	s := stats.NewStats()
	s.Counters["commands"] = 100
	s.Counters["vim"] = 9
	got := Evaluate(s, t0)
	if len(got) != 2 || got[0].ID != "first_steps" || got[1].ID != "warming_up" || !s.Unlocked["warming_up"].Equal(t0) {
		t.Fatal(got)
	}
	if again := Evaluate(s, t0.Add(time.Hour)); len(again) != 0 {
		t.Fatal("unlocked twice")
	}
	s.Counters["vim"] = 10
	if got := Evaluate(s, t0); len(got) != 1 || got[0].Title != "Escaped Vim" {
		t.Fatal(got)
	}
}
