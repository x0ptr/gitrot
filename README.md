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

`gitrot` tells you **what** drifted. An AI agent can help decide **how** to repair it. Use one of these two modes intentionally:

### ✅ The Safe Way (Recommended)

```bash
copilot "Please fix these dissonance issues: $(gitrot status)"
```

Using command substitution (`$(...)`) passes `gitrot` output as a normal string argument to `copilot`.
Because input is passed as an argument, `stdin` stays attached to your keyboard, so the agent can pause and ask for confirmation (for example `[y/N]`) before applying patches or running `git apply`.

### ⚠️ The Unsafe / Power-User Way (Danger Zone)

```bash
gitrot status | copilot --yolo
# or
copilot -p "$(gitrot status)" --yolo
```

This is for CI/CD automation or developers who explicitly want zero prompts.
With direct piping plus `--yolo`, safety confirmations are bypassed: the agent reads the dissonance, fetches diffs, and writes changes directly to your filesystem.
Use this only on a clean working tree so you can quickly review and revert if the model hallucinates.

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
