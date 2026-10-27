package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HumbleSoftware/toryo-spike/internal/task"
)

var now = time.Date(2026, 3, 3, 10, 0, 0, 0, time.UTC)

func tempPath(t *testing.T) string {
	return filepath.Join(t.TempDir(), "sub", "tasks.json")
}

func TestOpenMissingFile(t *testing.T) {
	s, err := Open(tempPath(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.All()) != 0 {
		t.Fatal("expected empty store")
	}
}

func TestAddSaveReopen(t *testing.T) {
	p := tempPath(t)
	s, _ := Open(p)
	for _, title := range []string{"one", "two", "three"} {
		if err := s.Add(&task.Task{Title: title}, now); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	all := s2.All()
	if len(all) != 3 || all[2].Title != "three" || all[2].ID != 3 {
		t.Fatalf("reopened = %+v", all)
	}
	if !all[0].Created.Equal(now) {
		t.Fatalf("created = %v", all[0].Created)
	}
}

func TestIDsNotReused(t *testing.T) {
	p := tempPath(t)
	s, _ := Open(p)
	s.Add(&task.Task{Title: "a"}, now)
	s.Add(&task.Task{Title: "b"}, now)
	if err := s.Delete(2); err != nil {
		t.Fatal(err)
	}
	s.Save()
	s2, _ := Open(p)
	tk := &task.Task{Title: "c"}
	s2.Add(tk, now)
	if tk.ID != 3 {
		t.Fatalf("new ID = %d, want 3", tk.ID)
	}
}

func TestAddRejectsInvalid(t *testing.T) {
	s, _ := Open(tempPath(t))
	if err := s.Add(&task.Task{Title: " "}, now); !errors.Is(err, task.ErrEmptyTitle) {
		t.Fatalf("err = %v", err)
	}
	if len(s.All()) != 0 {
		t.Fatal("invalid task was stored")
	}
}

func TestUpdateRollsBackOnInvalid(t *testing.T) {
	s, _ := Open(tempPath(t))
	s.Add(&task.Task{Title: "keep", Tags: []string{"a"}}, now)
	_, err := s.Update(1, func(t *task.Task) {
		t.Title = ""
		t.Tags = append(t.Tags, "b")
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	got, _ := s.Get(1)
	if got.Title != "keep" || strings.Join(got.Tags, ",") != "a" {
		t.Fatalf("not rolled back: %+v", got)
	}
}

func TestGetDeleteMissing(t *testing.T) {
	s, _ := Open(tempPath(t))
	if _, err := s.Get(9); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get err = %v", err)
	}
	if err := s.Delete(9); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete err = %v", err)
	}
}

func TestRefusesNewerVersion(t *testing.T) {
	p := tempPath(t)
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte(`{"version": 99, "tasks": []}`), 0o644)
	if _, err := Open(p); err == nil || !strings.Contains(err.Error(), "newer") {
		t.Fatalf("err = %v", err)
	}
}

func TestCorruptFile(t *testing.T) {
	p := tempPath(t)
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte(`{"version": 1, "tasks": [`), 0o644)
	if _, err := Open(p); err == nil {
		t.Fatal("expected error for truncated file")
	}
}

func TestSaveLeavesNoTempFiles(t *testing.T) {
	p := tempPath(t)
	s, _ := Open(p)
	s.Add(&task.Task{Title: "a"}, now)
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Dir(p))
	if len(entries) != 1 {
		t.Fatalf("dir has %d entries, want 1", len(entries))
	}
}

func TestPurge(t *testing.T) {
	s, _ := Open(tempPath(t))
	for _, title := range []string{"old", "recent", "open"} {
		s.Add(&task.Task{Title: title}, now.AddDate(0, 0, -30))
	}
	s.Update(1, func(t *task.Task) { t.Complete(now.AddDate(0, 0, -20)) })
	s.Update(2, func(t *task.Task) { t.Complete(now.AddDate(0, 0, -1)) })
	if n := s.Purge(now.AddDate(0, 0, -7)); n != 1 {
		t.Fatalf("purged %d, want 1", n)
	}
	if _, err := s.Get(1); err == nil {
		t.Fatal("old done task still present")
	}
}
