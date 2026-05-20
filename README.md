# gitrot

`gitrot` is a local CLI for detecting semantic decay from Git history.

It currently covers:
- **Dissonance Drift**: a file keeps changing while historically coupled files are left behind.
- **Tangled Commit Detection**: staged files in one commit have low historical cohesion.
- **Context Loss (Knowledge Silo)**: drift is driven by authors with no historical overlap on the coupled file pair.
- **Knowledge Map**: quick coupling and ownership discovery for a specific file.

## Installation

```bash
go install github.com/x0ptr/gitrot/cmd/gitrot@latest
```

## Commands

```bash
gitrot init
gitrot status [--history 2000] [--min-coupling 60] [--min-cohesion 30] [--min-shared 3] [--min-drift 2] [--max-files 30] [--ignore-tangled] [--ignore-silo]
gitrot staged [--history 2000] [--min-coupling 60] [--min-cohesion 30] [--max-files 30] [--ignore-tangled] [--ignore-silo]
gitrot map [--hide-name] <file_path>
gitrot hotspot [--history 2000] [--min-coupling 60] [--max-files 30] [path]
gitrot ack <file_path>
```

## `gitrot status`

`status` analyzes commit history and prints drift findings.

When enabled (default), it also evaluates staged cohesion at the end and prints a warning if the staged set looks tangled.
Unlike `staged`, `status` does not fail the process for tangled commits.

### Context Loss

For each drift finding `(A -> B)`:
- `HistoricalAuthors`: authors who historically committed `A` and `B` together.
- `DriftAuthors`: authors from drift commits on `A` (including `Co-authored-by` and `Reviewed-by` trailers).

A Context Loss warning is shown only when there is no intersection between these sets.

Disable with:

```bash
gitrot status --ignore-silo
```

## `gitrot staged` (pre-commit guard)

Use in a Git pre-commit hook to block tangled commits.

Behavior:
1. If `--ignore-tangled=true`, exits `0` immediately.
2. Reads staged files from `git diff --cached --name-only`.
3. Ignores staged files with no Git history.
4. If historical staged file count `< 2`, exits `0`.
5. Computes cohesion from pair coupling against `min_coupling`.
6. If `N >= 3` and cohesion `< min_cohesion`, prints a warning to `stderr` and exits `1`.
7. Otherwise exits `0` silently.

Bypass once:

```bash
gitrot staged --ignore-tangled
```

## `gitrot map <file>`

Prints a discovery view for one file:
- top historically coupled files (discovery threshold: `> 20%`)
- top knowledge holders by commit participation count (full `git user.name`)

Use `--hide-name` (or `[features].hide_name = true`) to obfuscate author names as deterministic IDs (`auth-<8hex>`).

If no historical data is available for the target, it prints:

```text
Error: No historical data found for <file>
```

## `gitrot hotspot`

Prints top refactoring hotspots using only Git metadata:
- **Churn**: number of commits per file
- **Coupling Degree**: number of unique files coupled above `min_coupling`
- **Score**: `churn * coupling_degree`

Files with fewer than 5 commits are ignored as low-churn noise.
Sorted by score (desc), limited to top 10.
Optionally pass a path prefix (for example `src/api`) to restrict which hotspot files are shown.

If no file exceeds the current thresholds:

```text
No critical hotspots detected based on current thresholds.
```

## Configuration

`gitrot init` creates `.gitrot.toml`:

```toml
# .gitrot.toml - Configuration for gitrot

[thresholds]
history = 2000       # Number of past commits to analyze
max_files = 30       # Ignore merge commits or mass refactors touching >30 files
min_coupling = 60    # Files must have been committed together >= 60% of the time
min_shared = 3       # Files must have at least 3 shared commits to be considered coupled
min_drift = 2        # Warn if a file is left behind by >= 2 commits
min_cohesion = 30    # Minimum cohesion percentage (0-100) for staged commits

[features]
ignore_tangled = false  # Set to true to disable Tangled Commit detection (`gitrot staged`)
ignore_silo = false     # Set to true to disable Context Loss/Silo detection
hide_name = false       # Set to true to obfuscate author names in `gitrot map`
```

## Precedence

Configuration precedence is:
1. CLI flags provided explicitly
2. `.gitrot.toml`
3. Built-in defaults

## Acknowledging a file

```bash
gitrot ack <file_path>
```

This writes the current `HEAD` hash for the file into `.gitrot-state.json`.
