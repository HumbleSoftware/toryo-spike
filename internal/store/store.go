// Package store persists tasks in a single JSON file.
//
// Writes go to a temporary file in the same directory which is then renamed over the
// original, so a crash mid-write never leaves a truncated file behind.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/HumbleSoftware/toryo-spike/internal/task"
)

// Version is the on-disk format version. Files with a newer version are refused.
const Version = 1

// ErrNotFound is returned when a task ID does not exist.
var ErrNotFound = errors.New("task not found")

type file struct {
	Version int          `json:"version"`
	NextID  int          `json:"next_id"`
	Tasks   []*task.Task `json:"tasks"`
}

// Store is an in-memory view of a task file. It is not safe for concurrent use.
type Store struct {
	path   string
	nextID int
	tasks  []*task.Task
}

// Open loads the file at path. A missing file yields an empty store; the file is
// created on the first Save.
func Open(path string) (*Store, error) {
	s := &Store{path: path, nextID: 1}
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	var f file
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if f.Version > Version {
		return nil, fmt.Errorf("%s: format version %d is newer than this build supports (%d)", path, f.Version, Version)
	}
	s.tasks = f.Tasks
	s.nextID = f.NextID
	for _, t := range s.tasks {
		if t.ID >= s.nextID {
			s.nextID = t.ID + 1
		}
	}
	if s.nextID < 1 {
		s.nextID = 1
	}
	return s, nil
}

// Path returns the file backing the store.
func (s *Store) Path() string { return s.path }

// Save writes the store back to disk atomically.
func (s *Store) Save() error {
	f := file{Version: Version, NextID: s.nextID, Tasks: s.tasks}
	if f.Tasks == nil {
		f.Tasks = []*task.Task{}
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".taskq-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.path)
}

// Add validates t, assigns it an ID and creation time, and appends it.
func (s *Store) Add(t *task.Task, now time.Time) error {
	if t.Created.IsZero() {
		t.Created = now.UTC()
	}
	if err := t.Validate(); err != nil {
		return err
	}
	t.ID = s.nextID
	s.nextID++
	s.tasks = append(s.tasks, t)
	return nil
}

// Get returns the task with the given ID.
func (s *Store) Get(id int) (*task.Task, error) {
	for _, t := range s.tasks {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, fmt.Errorf("%w: %d", ErrNotFound, id)
}

// Update applies fn to the task with the given ID and re-validates it. If validation
// fails the task is restored to its previous state.
func (s *Store) Update(id int, fn func(*task.Task)) (*task.Task, error) {
	t, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	before := *t
	before.Tags = append([]string(nil), t.Tags...)
	fn(t)
	if err := t.Validate(); err != nil {
		*t = before
		return nil, err
	}
	return t, nil
}

// Delete removes the task with the given ID. IDs are never reused.
func (s *Store) Delete(id int) error {
	for i, t := range s.tasks {
		if t.ID == id {
			s.tasks = append(s.tasks[:i], s.tasks[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("%w: %d", ErrNotFound, id)
}

// All returns every task in storage order. The slice is a copy; the tasks are not.
func (s *Store) All() []*task.Task {
	return append([]*task.Task(nil), s.tasks...)
}

// Purge removes completed tasks finished before cutoff and returns how many were removed.
func (s *Store) Purge(cutoff time.Time) int {
	kept := s.tasks[:0]
	n := 0
	for _, t := range s.tasks {
		if t.Completed != nil && t.Completed.Before(cutoff) {
			n++
			continue
		}
		kept = append(kept, t)
	}
	s.tasks = kept
	return n
}
