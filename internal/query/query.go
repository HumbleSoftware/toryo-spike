// Package query implements the filter language used by "taskq list".
//
// A query is a whitespace-separated list of terms; a task matches when every term
// matches (AND). Terms:
//
//	tag:work   +work      has tag "work"
//	-tag:work  -work      does not have tag "work"
//	pri:high   pri:>=med  priority equal to / at least
//	due:today  due:<fri   due on / before / after a date (any form dates.Parse accepts)
//	overdue               open and due before today
//	nodue                 has no due date
//	done  open            completion state (default: open)
//	all                   include both open and done tasks
//	anything-else         case-insensitive substring of the title
package query

import (
	"fmt"
	"strings"
	"time"

	"github.com/HumbleSoftware/toryo-spike/internal/dates"
	"github.com/HumbleSoftware/toryo-spike/internal/task"
)

// State selects tasks by completion.
type State int

const (
	StateOpen State = iota
	StateDone
	StateAll
)

// Query is a compiled filter.
type Query struct {
	State   State
	matches []func(*task.Task) bool
}

// Parse compiles the terms of a query relative to now.
func Parse(terms []string, now time.Time) (*Query, error) {
	q := &Query{State: StateOpen}
	today := dates.Midnight(now)
	for _, raw := range terms {
		term := strings.TrimSpace(raw)
		if term == "" {
			continue
		}
		lower := strings.ToLower(term)
		switch {
		case lower == "done":
			q.State = StateDone
		case lower == "open":
			q.State = StateOpen
		case lower == "all":
			q.State = StateAll
		case lower == "overdue":
			q.add(func(t *task.Task) bool { return !t.Done() && t.Due != nil && t.Due.Before(today) })
		case lower == "nodue":
			q.add(func(t *task.Task) bool { return t.Due == nil })
		case strings.HasPrefix(lower, "tag:"), strings.HasPrefix(lower, "+"):
			tag := task.NormalizeTag(strings.TrimPrefix(lower, "tag:"))
			if err := task.ValidateTag(tag); err != nil {
				return nil, err
			}
			q.add(func(t *task.Task) bool { return t.HasTag(tag) })
		case strings.HasPrefix(lower, "-tag:"), len(lower) > 1 && lower[0] == '-' && !strings.Contains(lower, ":"):
			tag := task.NormalizeTag(strings.TrimPrefix(strings.TrimPrefix(lower, "-tag:"), "-"))
			if err := task.ValidateTag(tag); err != nil {
				return nil, err
			}
			q.add(func(t *task.Task) bool { return !t.HasTag(tag) })
		case strings.HasPrefix(lower, "pri:"):
			m, err := parsePriority(strings.TrimPrefix(lower, "pri:"))
			if err != nil {
				return nil, err
			}
			q.add(m)
		case strings.HasPrefix(lower, "due:"):
			m, err := parseDue(strings.TrimPrefix(lower, "due:"), now)
			if err != nil {
				return nil, err
			}
			q.add(m)
		default:
			needle := lower
			q.add(func(t *task.Task) bool { return strings.Contains(strings.ToLower(t.Title), needle) })
		}
	}
	return q, nil
}

func (q *Query) add(m func(*task.Task) bool) { q.matches = append(q.matches, m) }

// Match reports whether t satisfies the query.
func (q *Query) Match(t *task.Task) bool {
	switch q.State {
	case StateOpen:
		if t.Done() {
			return false
		}
	case StateDone:
		if !t.Done() {
			return false
		}
	}
	for _, m := range q.matches {
		if !m(t) {
			return false
		}
	}
	return true
}

// Filter returns the tasks matching q, preserving order.
func (q *Query) Filter(ts []*task.Task) []*task.Task {
	var out []*task.Task
	for _, t := range ts {
		if q.Match(t) {
			out = append(out, t)
		}
	}
	return out
}

// splitOp separates a leading comparison operator from its operand.
func splitOp(s string) (op, rest string) {
	for _, o := range []string{"<=", ">=", "<", ">", "="} {
		if strings.HasPrefix(s, o) {
			return o, s[len(o):]
		}
	}
	return "=", s
}

func parsePriority(s string) (func(*task.Task) bool, error) {
	op, rest := splitOp(s)
	p, err := task.ParsePriority(rest)
	if err != nil {
		return nil, err
	}
	return func(t *task.Task) bool { return compare(op, int(t.Priority), int(p)) }, nil
}

func parseDue(s string, now time.Time) (func(*task.Task) bool, error) {
	op, rest := splitOp(s)
	d, err := dates.Parse(rest, now)
	if err != nil {
		return nil, fmt.Errorf("due: %w", err)
	}
	return func(t *task.Task) bool {
		if t.Due == nil {
			return false
		}
		return compare(op, dates.Days(d, *t.Due), 0)
	}, nil
}

func compare(op string, a, b int) bool {
	switch op {
	case "<":
		return a < b
	case "<=":
		return a <= b
	case ">":
		return a > b
	case ">=":
		return a >= b
	}
	return a == b
}
