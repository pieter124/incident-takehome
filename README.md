# render-schedule

Computes the final on-call schedule for a given time window, combining a
base rotation with any overrides. See [CHALLENGE.md](./CHALLENGE.md) for the
original take-home brief, and [notes.md](./notes.md) for the design notes.

## Requirements

Go 1.21 or later. No external dependencies, standard library only.

## Build

```
go build -o render-schedule .
```

## Run

```
./render-schedule \
  --schedule=schedule.json \
  --overrides=overrides.json \
  --from='2026-11-06T17:00:00Z' \
  --until='2026-11-20T17:00:00Z'
```

- `--schedule` path to a schedule JSON file (required)
- `--overrides` path to an overrides JSON file (optional, omit if there are none)
- `--from` RFC3339 timestamp, start of the window (required)
- `--until` RFC3339 timestamp, end of the window (required)

The final schedule is printed to stdout as a JSON array. Errors go to
stderr with a non-zero exit code, for example when `--from` is not before
`--until`, or when a provided `--overrides` file can't be read or parsed.

## Test

```
go test ./...
```
