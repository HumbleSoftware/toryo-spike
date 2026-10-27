package task

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestParsePriority(t *testing.T) {
	cases := map[string]Priority{
		"": PriorityNone, "none": PriorityNone, "l": PriorityLow, "LOW": PriorityLow,
		"med": PriorityMedium, "m": PriorityMedium, "high": PriorityHigh, " H ": PriorityHigh,
	}
	for in, want := range cases {
		got, err := ParsePriority(in)
		if err != nil || got != want {
			t.Errorf("ParsePriority(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	if _, err := ParsePriority("urgent"); err == nil {
		t.Error("ParsePriority(urgent): expected error")
	}
}

func TestPriorityJSONRoundTrip(t *testing.T) {
	in := Task{ID: 1, Title: "x", Priority: PriorityHigh}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"priority":"high"`) {
		t.Fatalf("priority not encoded as text: %s", b)
	}
	var out Task
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Priority != PriorityHigh {
		t.Fatalf("got %v", out.Priority)
	}
}

func TestValidate(t *testing.T) {
	now := time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC)
	ok := Task{Title: "write report", Created: now, Tags: []string{"work"}}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid task rejected: %v", err)
	}
	bad := []Task{
		{Title: "   ", Created: now},
		{Title: strings.Repeat("x", MaxTitleLen+1), Created: now},
		{Title: "t", Created: now, Tags: []string{"Work"}},
		{Title: "t", Created: now, Completed: ptr(now.Add(-time.Hour))},
	}
	for i, b := range bad {
		if err := b.Validate(); err == nil {
			t.Errorf("case %d: expected error", i)
		}
	}
}

func TestCompleteAndReopen(t *testing.T) {
	now := time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC)
	tk := Task{Title: "t", Created: now}
	tk.Complete(now.Add(time.Hour))
	first := *tk.Completed
	tk.Complete(now.Add(2 * time.Hour))
	if !tk.Completed.Equal(first) {
		t.Fatal("second Complete changed the completion time")
	}
	tk.Reopen()
	if tk.Done() {
		t.Fatal("still done after Reopen")
	}
}

func TestTags(t *testing.T) {
	tk := Task{Title: "t"}
	if err := tk.AddTags("+Work", "#home", "work"); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(tk.Tags, ","); got != "home,work" {
		t.Fatalf("tags = %q", got)
	}
	if err := tk.AddTags("bad tag"); err == nil {
		t.Fatal("expected error for tag with space")
	}
	tk.RemoveTags("home", "missing")
	if got := strings.Join(tk.Tags, ","); got != "work" {
		t.Fatalf("tags after remove = %q", got)
	}
	tk.RemoveTags("work")
	if tk.Tags != nil {
		t.Fatalf("expected nil tags, got %#v", tk.Tags)
	}
}

func TestSortOrder(t *testing.T) {
	d1 := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	done := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	ts := []*Task{
		{ID: 1, Title: "done", Due: &d1, Completed: &done},
		{ID: 2, Title: "undated high", Priority: PriorityHigh},
		{ID: 3, Title: "later", Due: &d2},
		{ID: 4, Title: "sooner low", Due: &d1, Priority: PriorityLow},
		{ID: 5, Title: "sooner high", Due: &d1, Priority: PriorityHigh},
		{ID: 6, Title: "undated none"},
	}
	Sort(ts)
	var ids []int
	for _, x := range ts {
		ids = append(ids, x.ID)
	}
	want := []int{5, 4, 3, 2, 6, 1}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("order = %v, want %v", ids, want)
		}
	}
}

func ptr(t time.Time) *time.Time { return &t }
