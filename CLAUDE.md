# monvif — project conventions

## Build

```
go build -o monvif .
```

Requires Go 1.22+. The binary is a single static Linux CLI.

## Module path

`github.com/arturbaumann/monvif`

## Package layout

| Path | Purpose |
|---|---|
| `main.go` | Entry point — calls `cmd.Execute()` |
| `cmd/` | Cobra CLI commands, one file per subcommand |
| `internal/discovery/` | WS-Discovery logic (no CLI dependencies) |
| `internal/camera/` | ONVIF device client (wraps use-go/onvif) |

## Key dependencies

- `github.com/use-go/onvif` — ONVIF + WS-Discovery
- `github.com/spf13/cobra` — CLI
- `golang.org/x/term` — interactive password prompt

## Conventions

- No passwords in logs. `resolvePassword()` in `cmd/auth.go` handles the
  flag / env / interactive flow.
- `MONVIF_PASSWORD` env var is the preferred non-interactive secret path.
- Camera query functions live in `internal/camera` and accept `context.Context`
  as first argument so callers can set timeouts.
- Discovery is in `internal/discovery`; the library's `SendProbe` has a 1 s
  read deadline, so `Discover()` loops until the user timeout.
- Default ONVIF port is 80 (set in each command's flag definition).

## Running tests

```
go test ./...
```

Unit tests live alongside the code (`_test.go`). Do not add integration tests
that dial real cameras to the test suite.
