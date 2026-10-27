package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 3, 3, 10, 0, 0, 0, time.UTC) // Tuesday

type cli struct {
	t    *testing.T
	file string
}

func newCLI(t *testing.T) *cli {
	return &cli{t: t, file: filepath.Join(t.TempDir(), "tasks.json")}
}

// run executes a command against the test file and returns exit code, stdout, stderr.
func (c *cli) run(args ...string) (int, string, string) {
	c.t.Helper()
	var out, errb bytes.Buffer
	full := append([]string{args[0], "-file", c.file}, args[1:]...)
	code := run(full, &out, &errb, now)
	return code, out.String(), errb.String()
}

// ok runs a command that must succeed and returns its stdout.
func (c *cli) ok(args ...string) string {
	c.t.Helper()
	code, out, errs := c.run(args...)
	if code != 0 {
		c.t.Fatalf("%v: exit %d, stderr: %s", args, code, errs)
	}
	return out
}

func TestAddAndList(t *testing.T) {
	c := newCLI(t)
	if got := c.ok("add", "-due", "tomorrow", "-pri", "h", "-tag", "work,+Q1", "Write", "report"); got != "Added task 1.\n" {
		t.Fatalf("add output = %q", got)
	}
	c.ok("add", "Buy milk")
	want := "ID P Due      Title\n" +
		" 1 H tomorrow Write report  +q1 +work\n" +
		" 2            Buy milk\n"
	if got := c.ok("list"); got != want {
		t.Fatalf("list:\n%s\nwant:\n%s", got, want)
	}
}

func TestListQueryAndJSON(t *testing.T) {
	c := newCLI(t)
	c.ok("add", "-tag", "work", "Review PR")
	c.ok("add", "-tag", "home", "Call plumber")
	got := c.ok("list", "-json", "+home")
	if !strings.Contains(got, `"title": "Call plumber"`) || strings.Contains(got, "Review PR") {
		t.Fatalf("json list = %s", got)
	}
	if got := c.ok("list", "nothing-matches"); got != "No tasks.\n" {
		t.Fatalf("empty list = %q", got)
	}
}

func TestDoneUndo(t *testing.T) {
	c := newCLI(t)
	c.ok("add", "a")
	c.ok("add", "b")
	if got := c.ok("done", "1", "2"); got != "Completed 2 task(s).\n" {
		t.Fatalf("done = %q", got)
	}
	if got := c.ok("list"); got != "No tasks.\n" {
		t.Fatalf("list after done = %q", got)
	}
	if !strings.Contains(c.ok("list", "done"), " x a") {
		t.Fatal("done list missing completion marker")
	}
	c.ok("undo", "2")
	if got := c.ok("list"); !strings.Contains(got, "b") || strings.Contains(got, " a\n") {
		t.Fatalf("list after undo = %q", got)
	}
}

func TestEdit(t *testing.T) {
	c := newCLI(t)
	c.ok("add", "-due", "fri", "old title")
	c.ok("edit", "-title", "new title", "-pri", "med", "1")
	out := c.ok("show", "1")
	for _, want := range []string{"Title:     new title", "Priority:  medium", "Due:       2026-03-06"} {
		if !strings.Contains(out, want) {
			t.Errorf("show missing %q:\n%s", want, out)
		}
	}
	c.ok("edit", "-due", "none", "1")
	if strings.Contains(c.ok("show", "1"), "Due:") {
		t.Fatal("due not cleared")
	}
	if code, _, _ := c.run("edit", "-title", "", "1"); code != 2 {
		t.Fatalf("empty title exit = %d, want 2", code)
	}
}

func TestTag(t *testing.T) {
	c := newCLI(t)
	c.ok("add", "-tag", "a,b", "t")
	c.ok("tag", "1", "+c", "-a")
	if !strings.Contains(c.ok("show", "1"), "Tags:      b, c") {
		t.Fatal("tags not updated")
	}
}

func TestRmAndPurge(t *testing.T) {
	c := newCLI(t)
	c.ok("add", "a")
	c.ok("add", "b")
	c.ok("rm", "1")
	if code, _, _ := c.run("show", "1"); code != 1 {
		t.Fatalf("show deleted task exit = %d", code)
	}
	c.ok("done", "2")
	if got := c.ok("purge", "-days", "0"); got != "Purged 0 task(s).\n" {
		t.Fatalf("purge = %q", got)
	}
}

func TestErrors(t *testing.T) {
	c := newCLI(t)
	cases := []struct {
		args []string
		code int
		msg  string
	}{
		{[]string{"add"}, 2, "missing title"},
		{[]string{"add", "-due", "someday", "x"}, 2, "unrecognized date"},
		{[]string{"add", "-pri", "urgent", "x"}, 2, "unknown priority"},
		{[]string{"done"}, 2, "need at least one"},
		{[]string{"done", "abc"}, 2, "bad task ID"},
		{[]string{"done", "42"}, 1, "not found"},
		{[]string{"list", "pri:urgent"}, 2, "unknown priority"},
		{[]string{"bogus"}, 2, "unknown command"},
	}
	for _, tc := range cases {
		code, _, errs := c.run(tc.args...)
		if code != tc.code || !strings.Contains(errs, tc.msg) {
			t.Errorf("%v: exit %d stderr %q; want %d containing %q", tc.args, code, errs, tc.code, tc.msg)
		}
	}
}

func TestHelp(t *testing.T) {
	var out bytes.Buffer
	if code := run([]string{"help", "query"}, &out, &out, now); code != 0 || !strings.Contains(out.String(), "overdue") {
		t.Fatalf("help query: %d %q", code, out.String())
	}
	if code := run(nil, &out, &out, now); code != 2 {
		t.Fatalf("no args exit = %d", code)
	}
}
