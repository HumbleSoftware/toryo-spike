package dates

import (
	"testing"
	"time"
)

// Tuesday 2026-03-03, mid-morning.
var now = time.Date(2026, 3, 3, 10, 30, 0, 0, time.UTC)

func TestParse(t *testing.T) {
	cases := []struct{ in, want string }{
		{"today", "2026-03-03"},
		{"Tomorrow", "2026-03-04"},
		{"yesterday", "2026-03-02"},
		{"+3d", "2026-03-06"},
		{"-1d", "2026-03-02"},
		{"+2w", "2026-03-17"},
		{"+1m", "2026-04-03"},
		{"fri", "2026-03-06"},
		{"friday", "2026-03-06"},
		{"mon", "2026-03-09"},
		{"sun", "2026-03-08"},
		{"eow", "2026-03-08"},
		{"eom", "2026-03-31"},
		{"2026-12-25", "2026-12-25"},
	}
	for _, c := range cases {
		got, err := Parse(c.in, now)
		if err != nil {
			t.Errorf("Parse(%q): %v", c.in, err)
			continue
		}
		if got.Format(Layout) != c.want {
			t.Errorf("Parse(%q) = %s, want %s", c.in, got.Format(Layout), c.want)
		}
		if got.Hour() != 0 || got.Minute() != 0 {
			t.Errorf("Parse(%q) not midnight: %v", c.in, got)
		}
	}
}

func TestParseErrors(t *testing.T) {
	for _, in := range []string{"", "+", "+d", "+3y", "soon", "2026-13-01", "next week"} {
		if _, err := Parse(in, now); err == nil {
			t.Errorf("Parse(%q): expected error", in)
		}
	}
}

func TestEOWOnSunday(t *testing.T) {
	sunday := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	got, _ := Parse("eow", sunday)
	if got.Format(Layout) != "2026-03-08" {
		t.Fatalf("eow on Sunday = %s", got.Format(Layout))
	}
}

func TestDaysAcrossDST(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("tzdata unavailable")
	}
	a := time.Date(2026, 3, 7, 12, 0, 0, 0, loc) // day before DST starts
	b := time.Date(2026, 3, 9, 12, 0, 0, 0, loc)
	if d := Days(a, b); d != 2 {
		t.Fatalf("Days across DST = %d, want 2", d)
	}
}

func TestRelative(t *testing.T) {
	cases := []struct {
		due  time.Time
		want string
	}{
		{now, "today"},
		{now.AddDate(0, 0, 1), "tomorrow"},
		{now.AddDate(0, 0, -1), "yesterday"},
		{now.AddDate(0, 0, 5), "in 5d"},
		{now.AddDate(0, 0, -3), "3d ago"},
		{now.AddDate(0, 0, 40), "2026-04-12"},
	}
	for _, c := range cases {
		if got := Relative(c.due, now); got != c.want {
			t.Errorf("Relative(%s) = %q, want %q", c.due.Format(Layout), got, c.want)
		}
	}
}
