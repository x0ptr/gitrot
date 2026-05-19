# gitrot

`gitrot` is a CLI tool that detects semantic decay in a Git repository by analyzing historical file coupling and current drift.

## The Problem

Code evolves with implicit dependencies:

- Source code is coupled to its documentation.
- Feature implementations are coupled to their unit tests.

When one file changes repeatedly while historically coupled files stop changing, dissonance increases. Static analysis and linters typically do not detect this.

## Installation

```bash
go install github.com/x0ptr/gitrot/cmd/gitrot@latest
```

## Quick Start

1. Open a Git repository.
2. Initialize config:
   ```bash
   gitrot init
   ```
3. Run analysis:
   ```bash
   gitrot status
   ```

## 🤖 Auto-Healing with AI Agents (The Unix Way)

`gitrot` is built to be piped. It tells you exactly *what* drifted; an AI agent can take that output and decide *how* to fix it.

```bash
gitrot status | copilot
```

That is modern Unix philosophy in action: one tool reports semantic dissonance, another tool repairs it.

Example workflow:

1. `gitrot` reports: "File A drifted 3 commits away from File B."
2. The AI agent reads the report and runs `git diff` on File A.
3. The agent writes the missing implementation/tests/docs for File B to realign both files.

## Configuration

`gitrot init` creates `.gitrot.toml` with these thresholds:

- `history`
- `max_files`
- `min_coupling`
- `min_shared`
- `min_drift`

## Acknowledge Current State

```bash
gitrot ack <file_path>
```

This writes the current `HEAD` hash for the target file into `.gitrot-state.json`.

Commit both `.gitrot.toml` and `.gitrot-state.json` so the repository uses shared thresholds and acknowledgements.
