package stats

import (
	"testing"
	"time"
)

func day(y int, m time.Month, d, h int) time.Time { return time.Date(y, m, d, h, 0, 0, 0, time.UTC) }

func TestStreak(t *testing.T) {
	s := NewStats()
	for d := 1; d <= 3; d++ {
		Apply(s, ev("ls", 0, 1, day(2026, 3, d, 10), "other"))
		Apply(s, ev("ls", 0, 1, day(2026, 3, d, 11), "other"))
	}
	if s.Streak != 3 || s.Counters["best_streak"] != 3 || s.Years["2026"].ActiveDays != 3 {
		t.Fatalf("%+v", s)
	}
	Apply(s, ev("ls", 0, 1, day(2026, 3, 5, 10), "other")) // gap
	if s.Streak != 1 || s.Counters["best_streak"] != 3 {
		t.Fatal(s.Streak)
	}
}

func TestStreakAcrossNewYear(t *testing.T) {
	s := NewStats()
	Apply(s, ev("ls", 0, 1, day(2025, 12, 31, 23), "other"))
	Apply(s, ev("ls", 0, 1, day(2026, 1, 1, 0), "other"))
	if s.Streak != 2 || s.Years["2026"].BestStreak != 2 || s.Years["2025"].BestStreak != 1 {
		t.Fatalf("%+v", s)
	}
}

func TestStreakClockBackwards(t *testing.T) {
	s := NewStats()
	Apply(s, ev("ls", 0, 1, day(2026, 3, 2, 0), "other"))
	Apply(s, ev("ls", 0, 1, day(2026, 3, 1, 23), "other"))
	if s.Streak != 1 || s.LastDay != "2026-03-02" || s.Years["2026"].ActiveDays != 1 {
		t.Fatalf("%+v", s)
	}
	if s.Years["2026"].Commands != 2 {
		t.Fatal("the event itself still counts")
	}
}

func TestComeback(t *testing.T) {
	s := NewStats()
	for i := 0; i < 10; i++ {
		Apply(s, ev("make", 2, 1, t0, "other"))
		if i == 4 {
			Apply(s, ev("sleep 1", 130, 1, t0, "other"))
		}
	}
	Apply(s, ev("make", 0, 1, t0, "other"))
	if s.Counters["comebacks"] != 1 || s.FailStreak != 0 {
		t.Fatalf("%d %d", s.Counters["comebacks"], s.FailStreak)
	}
	for i := 0; i < 9; i++ {
		Apply(s, ev("make", 2, 1, t0, "other"))
	}
	Apply(s, ev("make", 0, 1, t0, "other"))
	if s.Counters["comebacks"] != 1 {
		t.Fatal("9 failures must not count")
	}
}
