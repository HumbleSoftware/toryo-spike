// Package render formats tasks for the terminal and for machine consumption.
package render

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/HumbleSoftware/toryo-spike/internal/dates"
	"github.com/HumbleSoftware/toryo-spike/internal/task"
)

// Options controls table output.
type Options struct {
	Now      time.Time
	Width    int  // max line width; 0 means 80
	ShowDone bool // include the completion column
}

// Table writes tasks as aligned columns: ID, priority, due, title, tags.
func Table(w io.Writer, ts []*task.Task, o Options) error {
	if len(ts) == 0 {
		_, err := fmt.Fprintln(w, "No tasks.")
		return err
	}
	width := o.Width
	if width <= 0 {
		width = 80
	}

	type row struct{ id, pri, due, done, title, tags string }
	rows := make([]row, len(ts))
	idW, dueW := len("ID"), len("Due")
	for i, t := range ts {
		r := row{
			id:  fmt.Sprint(t.ID),
			pri: t.Priority.Short(),
		}
		if t.Due != nil {
			r.due = dates.Relative(*t.Due, o.Now)
			if !t.Done() && dates.Days(o.Now, *t.Due) < 0 {
				r.due = "!" + r.due
			}
		}
		if t.Done() {
			r.done = "x"
		}
		r.title = t.Title
		if len(t.Tags) > 0 {
			r.tags = "+" + strings.Join(t.Tags, " +")
		}
		idW = max(idW, len(r.id))
		dueW = max(dueW, len(r.due))
		rows[i] = r
	}

	prefix := func(id, pri, due, done string) string {
		s := fmt.Sprintf("%*s %s %-*s ", idW, id, pri, dueW, due)
		if o.ShowDone {
			s += fmt.Sprintf("%-1s ", done)
		}
		return s
	}

	fmt.Fprintln(w, strings.TrimRight(prefix("ID", "P", "Due", " ")+"Title", " "))
	for _, r := range rows {
		line := prefix(r.id, r.pri, r.due, r.done)
		rest := r.title
		if r.tags != "" {
			rest += "  " + r.tags
		}
		avail := width - utf8.RuneCountInString(line)
		line += Truncate(rest, avail)
		if _, err := fmt.Fprintln(w, strings.TrimRight(line, " ")); err != nil {
			return err
		}
	}
	return nil
}

// Truncate shortens s to at most n runes, marking the cut with "…".
func Truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	if n == 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
}

// Detail writes every field of a single task.
func Detail(w io.Writer, t *task.Task, now time.Time) error {
	var b strings.Builder
	fmt.Fprintf(&b, "ID:        %d\n", t.ID)
	fmt.Fprintf(&b, "Title:     %s\n", t.Title)
	fmt.Fprintf(&b, "Priority:  %s\n", t.Priority)
	if t.Due != nil {
		fmt.Fprintf(&b, "Due:       %s (%s)\n", t.Due.Format(dates.Layout), dates.Relative(*t.Due, now))
	}
	if len(t.Tags) > 0 {
		fmt.Fprintf(&b, "Tags:      %s\n", strings.Join(t.Tags, ", "))
	}
	fmt.Fprintf(&b, "Created:   %s\n", t.Created.Format(time.RFC3339))
	if t.Completed != nil {
		fmt.Fprintf(&b, "Completed: %s\n", t.Completed.Format(time.RFC3339))
	}
	if t.Notes != "" {
		fmt.Fprintf(&b, "\n%s\n", t.Notes)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// JSON writes tasks as an indented JSON array.
func JSON(w io.Writer, ts []*task.Task) error {
	if ts == nil {
		ts = []*task.Task{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(ts)
}
