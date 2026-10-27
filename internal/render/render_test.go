package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/HumbleSoftware/toryo-spike/internal/task"
)

var now = time.Date(2026, 3, 3, 10, 0, 0, 0, time.UTC)

func at(days int) *time.Time {
	d := time.Date(2026, 3, 3+days, 0, 0, 0, 0, time.UTC)
	return &d
}

func TestTable(t *testing.T) {
	ts := []*task.Task{
		{ID: 1, Title: "Write report", Priority: task.PriorityHigh, Due: at(0), Tags: []string{"work"}},
		{ID: 12, Title: "Buy milk", Due: at(-2)},
		{ID: 3, Title: "Someday"},
	}
	var buf bytes.Buffer
	if err := Table(&buf, ts, Options{Now: now}); err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		"ID P Due     Title",
		" 1 H today   Write report  +work",
		"12   !2d ago Buy milk",
		" 3           Someday",
		"",
	}, "\n")
	if buf.String() != want {
		t.Fatalf("got:\n%s\nwant:\n%s", buf.String(), want)
	}
}

func TestTableTruncatesToWidth(t *testing.T) {
	ts := []*task.Task{{ID: 1, Title: strings.Repeat("long ", 30), Tags: []string{"x"}}}
	var buf bytes.Buffer
	Table(&buf, ts, Options{Now: now, Width: 40})
	for _, line := range strings.Split(strings.TrimRight(buf.String(), "\n"), "\n") {
		if n := len([]rune(line)); n > 40 {
			t.Fatalf("line is %d runes: %q", n, line)
		}
	}
	if !strings.Contains(buf.String(), "…") {
		t.Fatal("expected ellipsis")
	}
}

func TestTableEmpty(t *testing.T) {
	var buf bytes.Buffer
	Table(&buf, nil, Options{Now: now})
	if buf.String() != "No tasks.\n" {
		t.Fatalf("got %q", buf.String())
	}
}

func TestTableShowDone(t *testing.T) {
	done := now
	ts := []*task.Task{{ID: 1, Title: "a", Completed: &done}, {ID: 2, Title: "b"}}
	var buf bytes.Buffer
	Table(&buf, ts, Options{Now: now, ShowDone: true})
	lines := strings.Split(buf.String(), "\n")
	if !strings.HasPrefix(lines[1], " 1       x a") || !strings.HasPrefix(lines[2], " 2         b") {
		t.Fatalf("got:\n%s", buf.String())
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello", 4, "hel…"},
		{"héllo", 3, "hé…"},
		{"hello", 1, "…"},
		{"hello", 0, ""},
	}
	for _, c := range cases {
		if got := Truncate(c.in, c.n); got != c.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
		}
	}
}

func TestDetail(t *testing.T) {
	tk := &task.Task{ID: 7, Title: "Plan trip", Priority: task.PriorityMedium, Due: at(3),
		Tags: []string{"home", "travel"}, Created: now.Add(-24 * time.Hour), Notes: "book flights"}
	var buf bytes.Buffer
	Detail(&buf, tk, now)
	for _, want := range []string{"ID:        7", "Due:       2026-03-06 (in 3d)", "Tags:      home, travel", "\nbook flights\n"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("missing %q in:\n%s", want, buf.String())
		}
	}
}

func TestJSON(t *testing.T) {
	var buf bytes.Buffer
	JSON(&buf, nil)
	var out []task.Task
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil || len(out) != 0 {
		t.Fatalf("JSON(nil) = %q, %v", buf.String(), err)
	}
}
