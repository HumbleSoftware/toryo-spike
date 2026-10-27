// Package dates parses the human-friendly due dates accepted on the command line.
//
// Supported forms (case-insensitive):
//
//	today, tomorrow, yesterday
//	+3d, -1d, +2w, +1m        relative days, weeks or months
//	mon ... sun, monday ...    the next occurrence of that weekday, never today
//	eow                        end of week (the coming Sunday; today if it is Sunday)
//	eom                        last day of the current month
//	2026-03-14                 ISO date
//
// All results are midnight in the location of now.
package dates

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Layout is the ISO date layout used for parsing and display.
const Layout = "2006-01-02"

var weekdays = map[string]time.Weekday{
	"sun": time.Sunday, "sunday": time.Sunday,
	"mon": time.Monday, "monday": time.Monday,
	"tue": time.Tuesday, "tues": time.Tuesday, "tuesday": time.Tuesday,
	"wed": time.Wednesday, "wednesday": time.Wednesday,
	"thu": time.Thursday, "thur": time.Thursday, "thurs": time.Thursday, "thursday": time.Thursday,
	"fri": time.Friday, "friday": time.Friday,
	"sat": time.Saturday, "saturday": time.Saturday,
}

// Midnight truncates t to the start of its day in t's location.
func Midnight(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// Parse interprets s relative to now.
func Parse(s string, now time.Time) (time.Time, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	today := Midnight(now)

	switch s {
	case "":
		return time.Time{}, fmt.Errorf("empty date")
	case "today":
		return today, nil
	case "tomorrow":
		return today.AddDate(0, 0, 1), nil
	case "yesterday":
		return today.AddDate(0, 0, -1), nil
	case "eow":
		return today.AddDate(0, 0, (7-int(today.Weekday()))%7), nil
	case "eom":
		return time.Date(today.Year(), today.Month()+1, 0, 0, 0, 0, 0, today.Location()), nil
	}

	if wd, ok := weekdays[s]; ok {
		delta := (int(wd) - int(today.Weekday()) + 7) % 7
		if delta == 0 {
			delta = 7
		}
		return today.AddDate(0, 0, delta), nil
	}

	if s[0] == '+' || s[0] == '-' {
		return parseRelative(s, today)
	}

	t, err := time.ParseInLocation(Layout, s, now.Location())
	if err != nil {
		return time.Time{}, fmt.Errorf("unrecognized date %q", s)
	}
	return t, nil
}

func parseRelative(s string, today time.Time) (time.Time, error) {
	if len(s) < 3 {
		return time.Time{}, fmt.Errorf("bad relative date %q (try +3d, +2w, +1m)", s)
	}
	unit := s[len(s)-1]
	n, err := strconv.Atoi(s[:len(s)-1])
	if err != nil {
		return time.Time{}, fmt.Errorf("bad relative date %q (try +3d, +2w, +1m)", s)
	}
	switch unit {
	case 'd':
		return today.AddDate(0, 0, n), nil
	case 'w':
		return today.AddDate(0, 0, 7*n), nil
	case 'm':
		return today.AddDate(0, n, 0), nil
	}
	return time.Time{}, fmt.Errorf("bad relative date unit %q in %q (use d, w or m)", unit, s)
}

// Days returns the number of whole calendar days from a to b (negative if b is earlier).
func Days(a, b time.Time) int {
	a, b = Midnight(a), Midnight(b.In(a.Location()))
	// Go through UTC dates to stay correct across DST changes.
	ua := time.Date(a.Year(), a.Month(), a.Day(), 0, 0, 0, 0, time.UTC)
	ub := time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, time.UTC)
	return int(ub.Sub(ua).Hours() / 24)
}

// Relative describes due relative to now in a few characters: "today", "tomorrow",
// "in 3d", "2d ago", or the ISO date when it is more than 4 weeks away.
func Relative(due, now time.Time) string {
	n := Days(now, due)
	switch {
	case n == 0:
		return "today"
	case n == 1:
		return "tomorrow"
	case n == -1:
		return "yesterday"
	case n > 28 || n < -28:
		return due.Format(Layout)
	case n > 0:
		return fmt.Sprintf("in %dd", n)
	default:
		return fmt.Sprintf("%dd ago", -n)
	}
}
