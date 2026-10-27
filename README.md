# taskq

A small command-line task tracker backed by a single JSON file.

```sh
go install ./cmd/taskq

taskq add -due fri -pri high -tag work Write the quarterly report
taskq add Buy milk
taskq list                 # open tasks, soonest due first
taskq list +work pri:>=med # filter with the query language (taskq help query)
taskq done 1
taskq list done
```

The task file is `$TASKQ_FILE`, or `~/.local/share/taskq/tasks.json`. Every command accepts
`-file` to override it. Writes are atomic (temp file + rename).

## Layout

| Package | Purpose |
|---|---|
| `cmd/taskq` | CLI entry point and command handling |
| `internal/task` | Task model, priorities, tags, sort order |
| `internal/dates` | Human-friendly date parsing (`tomorrow`, `+2w`, `fri`, `eom`, …) |
| `internal/query` | Filter language for `list` |
| `internal/store` | JSON file storage |
| `internal/render` | Table, detail and JSON output |

## Development

```sh
go vet ./...
go test ./...
```

Go 1.22 or newer. No dependencies outside the standard library.
