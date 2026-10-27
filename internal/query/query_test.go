package query

import (
	"strings"
	"testing"
	"time"

	"github.com/HumbleSoftware/toryo-spike/internal/task"
)

var now = time.Date(2026, 3, 3, 10, 0, 0, 0, time.UTC) // Tuesday

func day(n int) *time.Time {
	d := time.Date(2026, 3, 3+n, 0, 0, 0, 0, time.UTC)
	return &d
}

func fixture() []*task.Task {
	done := now.Add(-time.Hour)
	return []*task.Task{
		{ID: 1, Title: "Write report", Tags: []string{"work"}, Priority: task.PriorityHigh, Due: day(0)},
		{ID: 2, Title: "Buy milk", Tags: []string{"home"}, Due: day(-2)},
		{ID: 3, Title: "Review PR", Tags: []string{"work", "code"}, Priority: task.PriorityMedium, Due: day(3)},
		{ID: 4, Title: "Call plumber", Tags: []string{"home"}, Priority: task.PriorityLow},
		{ID: 5, Title: "File taxes", Priority: task.PriorityHigh, Due: day(-5), Completed: &done},
	}
}

func ids(t *testing.T, terms string) string {
	t.Helper()
	q, err := Parse(strings.Fields(terms), now)
	if err != nil {
		t.Fatalf("Parse(%q): %v", terms, err)
	}
	var out []string
	for _, x := range q.Filter(fixture()) {
		out = append(out, string(rune('0'+x.ID)))
	}
	return strings.Join(out, ",")
}

func TestFilters(t *testing.T) {
	cases := []struct{ terms, want string }{
		{"", "1,2,3,4"},
		{"all", "1,2,3,4,5"},
		{"done", "5"},
		{"tag:work", "1,3"},
		{"+home", "2,4"},
		{"-work", "2,4"},
		{"-tag:home", "1,3"},
		{"pri:high", "1"},
		{"pri:>=med", "1,3"},
		{"pri:<med all", "2,4"},
		{"due:today", "1"},
		{"due:<today", "2"},
		{"due:<=fri", "1,2,3"},
		{"due:>today", "3"},
		{"overdue", "2"},
		{"overdue all", "2"},
		{"nodue", "4"},
		{"REPORT", "1"},
		{"tag:work review", "3"},
	}
	for _, c := range cases {
		if got := ids(t, c.terms); got != c.want {
			t.Errorf("%q => %s, want %s", c.terms, got, c.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	for _, terms := range []string{"pri:urgent", "due:someday", "tag:Bad!"} {
		if _, err := Parse(strings.Fields(terms), now); err == nil {
			t.Errorf("Parse(%q): expected error", terms)
		}
	}
}
