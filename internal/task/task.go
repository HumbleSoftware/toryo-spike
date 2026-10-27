// Package task defines the task model shared by the store, query and render packages.
package task

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Priority orders tasks within the same due date. The zero value is PriorityNone.
type Priority int

const (
	PriorityNone Priority = iota
	PriorityLow
	PriorityMedium
	PriorityHigh
)

var priorityNames = map[Priority]string{
	PriorityNone:   "none",
	PriorityLow:    "low",
	PriorityMedium: "medium",
	PriorityHigh:   "high",
}

func (p Priority) String() string {
	if s, ok := priorityNames[p]; ok {
		return s
	}
	return fmt.Sprintf("Priority(%d)", int(p))
}

// Short is the one-character marker used in table output.
func (p Priority) Short() string {
	switch p {
	case PriorityHigh:
		return "H"
	case PriorityMedium:
		return "M"
	case PriorityLow:
		return "L"
	}
	return " "
}

// ParsePriority accepts full names, single letters, and the empty string (none).
func ParsePriority(s string) (Priority, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "none", "n":
		return PriorityNone, nil
	case "low", "l":
		return PriorityLow, nil
	case "medium", "med", "m":
		return PriorityMedium, nil
	case "high", "h":
		return PriorityHigh, nil
	}
	return PriorityNone, fmt.Errorf("unknown priority %q (want high, medium, low or none)", s)
}

func (p Priority) MarshalText() ([]byte, error) { return []byte(p.String()), nil }

func (p *Priority) UnmarshalText(b []byte) error {
	v, err := ParsePriority(string(b))
	if err != nil {
		return err
	}
	*p = v
	return nil
}

// Task is a single to-do item.
type Task struct {
	ID        int        `json:"id"`
	Title     string     `json:"title"`
	Notes     string     `json:"notes,omitempty"`
	Tags      []string   `json:"tags,omitempty"`
	Priority  Priority   `json:"priority"`
	Due       *time.Time `json:"due,omitempty"`
	Created   time.Time  `json:"created"`
	Completed *time.Time `json:"completed,omitempty"`
}

// MaxTitleLen keeps titles short enough to fit a terminal row.
const MaxTitleLen = 120

var (
	ErrEmptyTitle = errors.New("title must not be empty")
	ErrLongTitle  = fmt.Errorf("title must be at most %d characters", MaxTitleLen)
)

// Validate checks the invariants every stored task must satisfy.
func (t *Task) Validate() error {
	title := strings.TrimSpace(t.Title)
	if title == "" {
		return ErrEmptyTitle
	}
	if len([]rune(title)) > MaxTitleLen {
		return ErrLongTitle
	}
	for _, tag := range t.Tags {
		if err := ValidateTag(tag); err != nil {
			return err
		}
	}
	if t.Completed != nil && t.Completed.Before(t.Created) {
		return errors.New("completed before created")
	}
	return nil
}

// Done reports whether the task has been completed.
func (t *Task) Done() bool { return t.Completed != nil }

// Complete marks the task done at the given time. Completing twice is a no-op.
func (t *Task) Complete(at time.Time) {
	if t.Completed == nil {
		at = at.UTC()
		t.Completed = &at
	}
}

// Reopen clears the completion time.
func (t *Task) Reopen() { t.Completed = nil }

// ValidateTag rejects tags that would be ambiguous on the command line.
func ValidateTag(tag string) error {
	if tag == "" {
		return errors.New("tag must not be empty")
	}
	for _, r := range tag {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return fmt.Errorf("tag %q: only lowercase letters, digits, '-' and '_' are allowed", tag)
		}
	}
	return nil
}

// NormalizeTag lowercases a tag and strips a leading '+' or '#'.
func NormalizeTag(tag string) string {
	tag = strings.TrimSpace(tag)
	tag = strings.TrimLeft(tag, "+#")
	return strings.ToLower(tag)
}

// HasTag reports whether the task carries the given (normalized) tag.
func (t *Task) HasTag(tag string) bool {
	for _, x := range t.Tags {
		if x == tag {
			return true
		}
	}
	return false
}

// AddTags adds tags, keeping the list sorted and free of duplicates.
func (t *Task) AddTags(tags ...string) error {
	for _, tag := range tags {
		tag = NormalizeTag(tag)
		if err := ValidateTag(tag); err != nil {
			return err
		}
		if !t.HasTag(tag) {
			t.Tags = append(t.Tags, tag)
		}
	}
	sort.Strings(t.Tags)
	return nil
}

// RemoveTags removes tags; removing an absent tag is not an error.
func (t *Task) RemoveTags(tags ...string) {
	drop := make(map[string]bool, len(tags))
	for _, tag := range tags {
		drop[NormalizeTag(tag)] = true
	}
	kept := t.Tags[:0]
	for _, x := range t.Tags {
		if !drop[x] {
			kept = append(kept, x)
		}
	}
	if len(kept) == 0 {
		kept = nil
	}
	t.Tags = kept
}

// Less is the default list order: open before done, then earliest due date
// (undated last), then higher priority, then lower ID.
func Less(a, b *Task) bool {
	if a.Done() != b.Done() {
		return !a.Done()
	}
	switch {
	case a.Due != nil && b.Due == nil:
		return true
	case a.Due == nil && b.Due != nil:
		return false
	case a.Due != nil && b.Due != nil && !a.Due.Equal(*b.Due):
		return a.Due.Before(*b.Due)
	}
	if a.Priority != b.Priority {
		return a.Priority > b.Priority
	}
	return a.ID < b.ID
}

// Sort sorts tasks in place using Less.
func Sort(ts []*Task) {
	sort.SliceStable(ts, func(i, j int) bool { return Less(ts[i], ts[j]) })
}
