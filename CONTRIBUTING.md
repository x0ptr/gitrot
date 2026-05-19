# Contributing to gitrot

## Setup

1. Fork the repository and clone your fork.
2. Create a branch for your change.
3. Keep commits focused and scoped to one change.

## Run tests

```bash
go test ./...
```

## Build the CLI

```bash
go build -o gitrot ./cmd/gitrot
```

## Pull Request Checklist

1. `go test ./...` passes.
2. `go build -o gitrot ./cmd/gitrot` succeeds.
3. PR description includes purpose and technical summary.
