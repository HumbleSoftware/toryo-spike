// Command taskq is a small command-line task tracker backed by a JSON file.
//
// The task file defaults to $TASKQ_FILE, then ~/.local/share/taskq/tasks.json.
// Every command also accepts -file to override it.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/HumbleSoftware/toryo-spike/internal/dates"
	"github.com/HumbleSoftware/toryo-spike/internal/query"
	"github.com/HumbleSoftware/toryo-spike/internal/render"
	"github.com/HumbleSoftware/toryo-spike/internal/store"
	"github.com/HumbleSoftware/toryo-spike/internal/task"
)

const usage = `usage: taskq <command> [flags] [args]

commands:
  add <title...>        add a task (-due, -pri, -tag, -notes)
  list [query...]       list tasks (-json, -width); see "taskq help query"
  show <id>             show every field of a task
  done <id...>          mark tasks done
  undo <id...>          reopen tasks
  edit <id>             change a task (-title, -due, -pri, -notes)
  tag <id> <+t|-t...>   add or remove tags
  rm <id...>            delete tasks
  purge                 delete done tasks older than -days (default 30)
  help [query]          show this help, or the query language
`

const queryHelp = `query terms (all must match):
  tag:work  +work      has tag            -tag:work  -work   lacks tag
  pri:high  pri:>=med  priority           due:today  due:<fri  due date
  overdue   nodue      open|done|all      any other word: title contains it
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, time.Now()))
}

func defaultFile() string {
	if f := os.Getenv("TASKQ_FILE"); f != "" {
		return f
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "tasks.json"
	}
	return filepath.Join(home, ".local", "share", "taskq", "tasks.json")
}

func run(args []string, stdout, stderr io.Writer, now time.Time) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	cmd, args := args[0], args[1:]

	switch cmd {
	case "add":
		fs := flag.NewFlagSet("add", flag.ContinueOnError)
		fs.SetOutput(stderr)
		file := fs.String("file", defaultFile(), "task file")
		due := fs.String("due", "", "due date")
		pri := fs.String("pri", "", "priority")
		tags := fs.String("tag", "", "comma-separated tags")
		notes := fs.String("notes", "", "notes")
		if err := fs.Parse(args); err != nil {
			return 2
		}
		if fs.NArg() == 0 {
			fmt.Fprintln(stderr, "taskq add: missing title")
			return 2
		}
		s, err := store.Open(*file)
		if err != nil {
			fmt.Fprintln(stderr, "taskq:", err)
			return 1
		}
		t := &task.Task{Title: strings.Join(fs.Args(), " "), Notes: *notes}
		if *due != "" {
			d, err := dates.Parse(*due, now)
			if err != nil {
				fmt.Fprintln(stderr, "taskq add:", err)
				return 2
			}
			t.Due = &d
		}
		p, err := task.ParsePriority(*pri)
		if err != nil {
			fmt.Fprintln(stderr, "taskq add:", err)
			return 2
		}
		t.Priority = p
		if *tags != "" {
			if err := t.AddTags(strings.Split(*tags, ",")...); err != nil {
				fmt.Fprintln(stderr, "taskq add:", err)
				return 2
			}
		}
		if err := s.Add(t, now); err != nil {
			fmt.Fprintln(stderr, "taskq add:", err)
			return 2
		}
		if err := s.Save(); err != nil {
			fmt.Fprintln(stderr, "taskq:", err)
			return 1
		}
		fmt.Fprintf(stdout, "Added task %d.\n", t.ID)
		return 0

	case "list", "ls":
		fs := flag.NewFlagSet("list", flag.ContinueOnError)
		fs.SetOutput(stderr)
		file := fs.String("file", defaultFile(), "task file")
		asJSON := fs.Bool("json", false, "print JSON")
		width := fs.Int("width", 0, "max line width (default 80)")
		if err := fs.Parse(args); err != nil {
			return 2
		}
		s, err := store.Open(*file)
		if err != nil {
			fmt.Fprintln(stderr, "taskq:", err)
			return 1
		}
		q, err := query.Parse(fs.Args(), now)
		if err != nil {
			fmt.Fprintln(stderr, "taskq list:", err)
			return 2
		}
		ts := q.Filter(s.All())
		task.Sort(ts)
		if *asJSON {
			err = render.JSON(stdout, ts)
		} else {
			err = render.Table(stdout, ts, render.Options{Now: now, Width: *width, ShowDone: q.State != query.StateOpen})
		}
		if err != nil {
			fmt.Fprintln(stderr, "taskq:", err)
			return 1
		}
		return 0

	case "show":
		fs := flag.NewFlagSet("show", flag.ContinueOnError)
		fs.SetOutput(stderr)
		file := fs.String("file", defaultFile(), "task file")
		if err := fs.Parse(args); err != nil {
			return 2
		}
		if fs.NArg() != 1 {
			fmt.Fprintln(stderr, "taskq show: need exactly one task ID")
			return 2
		}
		id, err := strconv.Atoi(fs.Arg(0))
		if err != nil {
			fmt.Fprintf(stderr, "taskq show: bad task ID %q\n", fs.Arg(0))
			return 2
		}
		s, err := store.Open(*file)
		if err != nil {
			fmt.Fprintln(stderr, "taskq:", err)
			return 1
		}
		t, err := s.Get(id)
		if err != nil {
			fmt.Fprintln(stderr, "taskq show:", err)
			return 1
		}
		render.Detail(stdout, t, now)
		return 0

	case "done", "undo":
		fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
		fs.SetOutput(stderr)
		file := fs.String("file", defaultFile(), "task file")
		if err := fs.Parse(args); err != nil {
			return 2
		}
		if fs.NArg() == 0 {
			fmt.Fprintf(stderr, "taskq %s: need at least one task ID\n", cmd)
			return 2
		}
		var ids []int
		for _, a := range fs.Args() {
			id, err := strconv.Atoi(a)
			if err != nil {
				fmt.Fprintf(stderr, "taskq %s: bad task ID %q\n", cmd, a)
				return 2
			}
			ids = append(ids, id)
		}
		s, err := store.Open(*file)
		if err != nil {
			fmt.Fprintln(stderr, "taskq:", err)
			return 1
		}
		for _, id := range ids {
			_, err := s.Update(id, func(t *task.Task) {
				if cmd == "done" {
					t.Complete(now)
				} else {
					t.Reopen()
				}
			})
			if err != nil {
				fmt.Fprintf(stderr, "taskq %s: %v\n", cmd, err)
				return 1
			}
		}
		if err := s.Save(); err != nil {
			fmt.Fprintln(stderr, "taskq:", err)
			return 1
		}
		verb := "Completed"
		if cmd == "undo" {
			verb = "Reopened"
		}
		fmt.Fprintf(stdout, "%s %d task(s).\n", verb, len(ids))
		return 0

	case "edit":
		fs := flag.NewFlagSet("edit", flag.ContinueOnError)
		fs.SetOutput(stderr)
		file := fs.String("file", defaultFile(), "task file")
		title := fs.String("title", "", "new title")
		due := fs.String("due", "", `new due date ("none" clears it)`)
		pri := fs.String("pri", "", "new priority")
		notes := fs.String("notes", "", "new notes")
		if err := fs.Parse(args); err != nil {
			return 2
		}
		if fs.NArg() != 1 {
			fmt.Fprintln(stderr, "taskq edit: need exactly one task ID")
			return 2
		}
		id, err := strconv.Atoi(fs.Arg(0))
		if err != nil {
			fmt.Fprintf(stderr, "taskq edit: bad task ID %q\n", fs.Arg(0))
			return 2
		}
		set := map[string]bool{}
		fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
		var newDue *time.Time
		if set["due"] && *due != "none" {
			d, err := dates.Parse(*due, now)
			if err != nil {
				fmt.Fprintln(stderr, "taskq edit:", err)
				return 2
			}
			newDue = &d
		}
		var newPri task.Priority
		if set["pri"] {
			p, err := task.ParsePriority(*pri)
			if err != nil {
				fmt.Fprintln(stderr, "taskq edit:", err)
				return 2
			}
			newPri = p
		}
		s, err := store.Open(*file)
		if err != nil {
			fmt.Fprintln(stderr, "taskq:", err)
			return 1
		}
		_, err = s.Update(id, func(t *task.Task) {
			if set["title"] {
				t.Title = *title
			}
			if set["due"] {
				t.Due = newDue
			}
			if set["pri"] {
				t.Priority = newPri
			}
			if set["notes"] {
				t.Notes = *notes
			}
		})
		if errors.Is(err, store.ErrNotFound) {
			fmt.Fprintln(stderr, "taskq edit:", err)
			return 1
		}
		if err != nil {
			fmt.Fprintln(stderr, "taskq edit:", err)
			return 2
		}
		if err := s.Save(); err != nil {
			fmt.Fprintln(stderr, "taskq:", err)
			return 1
		}
		fmt.Fprintf(stdout, "Updated task %d.\n", id)
		return 0

	case "tag":
		fs := flag.NewFlagSet("tag", flag.ContinueOnError)
		fs.SetOutput(stderr)
		file := fs.String("file", defaultFile(), "task file")
		if err := fs.Parse(args); err != nil {
			return 2
		}
		if fs.NArg() < 2 {
			fmt.Fprintln(stderr, "taskq tag: usage: taskq tag <id> +tag -tag ...")
			return 2
		}
		id, err := strconv.Atoi(fs.Arg(0))
		if err != nil {
			fmt.Fprintf(stderr, "taskq tag: bad task ID %q\n", fs.Arg(0))
			return 2
		}
		var add, remove []string
		for _, a := range fs.Args()[1:] {
			if strings.HasPrefix(a, "-") {
				remove = append(remove, a[1:])
			} else {
				add = append(add, a)
			}
		}
		s, err := store.Open(*file)
		if err != nil {
			fmt.Fprintln(stderr, "taskq:", err)
			return 1
		}
		var tagErr error
		_, err = s.Update(id, func(t *task.Task) {
			tagErr = t.AddTags(add...)
			t.RemoveTags(remove...)
		})
		if err == nil {
			err = tagErr
		}
		if err != nil {
			fmt.Fprintln(stderr, "taskq tag:", err)
			return 1
		}
		if err := s.Save(); err != nil {
			fmt.Fprintln(stderr, "taskq:", err)
			return 1
		}
		fmt.Fprintf(stdout, "Updated task %d.\n", id)
		return 0

	case "rm":
		fs := flag.NewFlagSet("rm", flag.ContinueOnError)
		fs.SetOutput(stderr)
		file := fs.String("file", defaultFile(), "task file")
		if err := fs.Parse(args); err != nil {
			return 2
		}
		if fs.NArg() == 0 {
			fmt.Fprintln(stderr, "taskq rm: need at least one task ID")
			return 2
		}
		var ids []int
		for _, a := range fs.Args() {
			id, err := strconv.Atoi(a)
			if err != nil {
				fmt.Fprintf(stderr, "taskq rm: bad task ID %q\n", a)
				return 2
			}
			ids = append(ids, id)
		}
		s, err := store.Open(*file)
		if err != nil {
			fmt.Fprintln(stderr, "taskq:", err)
			return 1
		}
		for _, id := range ids {
			if err := s.Delete(id); err != nil {
				fmt.Fprintln(stderr, "taskq rm:", err)
				return 1
			}
		}
		if err := s.Save(); err != nil {
			fmt.Fprintln(stderr, "taskq:", err)
			return 1
		}
		fmt.Fprintf(stdout, "Deleted %d task(s).\n", len(ids))
		return 0

	case "purge":
		fs := flag.NewFlagSet("purge", flag.ContinueOnError)
		fs.SetOutput(stderr)
		file := fs.String("file", defaultFile(), "task file")
		days := fs.Int("days", 30, "delete done tasks completed more than this many days ago")
		if err := fs.Parse(args); err != nil {
			return 2
		}
		if *days < 0 {
			fmt.Fprintln(stderr, "taskq purge: -days must not be negative")
			return 2
		}
		s, err := store.Open(*file)
		if err != nil {
			fmt.Fprintln(stderr, "taskq:", err)
			return 1
		}
		n := s.Purge(dates.Midnight(now).AddDate(0, 0, -*days))
		if err := s.Save(); err != nil {
			fmt.Fprintln(stderr, "taskq:", err)
			return 1
		}
		fmt.Fprintf(stdout, "Purged %d task(s).\n", n)
		return 0

	case "help", "-h", "-help", "--help":
		if len(args) > 0 && args[0] == "query" {
			fmt.Fprint(stdout, queryHelp)
			return 0
		}
		fmt.Fprint(stdout, usage)
		return 0
	}

	fmt.Fprintf(stderr, "taskq: unknown command %q\n\n%s", cmd, usage)
	return 2
}
