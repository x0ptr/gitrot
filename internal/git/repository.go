package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

const marker = "__GITROT_COMMIT__"
const fieldSep = "\x1f"
const trailerSep = "\x1e"

type Commit struct {
	Hash    string
	Files   []string
	Authors []string
}

type Repository struct {
	root string
}

func NewRepository(path string) (*Repository, error) {
	root, err := run(path, 10*time.Second, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}

	root = strings.TrimSpace(root)
	if root == "" {
		return nil, fmt.Errorf("unable to determine repository root")
	}

	return &Repository{root: root}, nil
}

func (r *Repository) LoadCommits(limit int) ([]Commit, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	args := []string{
		"-C", r.root,
		"log",
		fmt.Sprintf("-n%d", limit),
		"--name-only",
		"--pretty=format:" + marker + "%H" + fieldSep + "%P" + fieldSep + "%an" + fieldSep + "%(trailers:key=Co-authored-by,valueonly,separator=%x1e)" + fieldSep + "%(trailers:key=Reviewed-by,valueonly,separator=%x1e)",
		"--no-renames",
		"--",
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("git log timed out")
		}
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
		}
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	return parseCommits(stdout.Bytes(), limit)
}

func (r *Repository) Root() string {
	return r.root
}

func (r *Repository) HeadHash() (string, error) {
	hash, err := run(r.root, 10*time.Second, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}

	hash = strings.TrimSpace(hash)
	if hash == "" {
		return "", fmt.Errorf("empty HEAD hash")
	}

	return hash, nil
}

func (r *Repository) StagedFiles() ([]string, error) {
	out, err := run(r.root, 10*time.Second, "diff", "--cached", "--name-only", "--diff-filter=ACMRD", "--no-renames")
	if err != nil {
		return nil, err
	}

	seen := map[string]struct{}{}
	files := make([]string, 0)
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		files = append(files, line)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read staged files: %w", err)
	}
	sort.Strings(files)
	return files, nil
}

func parseCommits(raw []byte, capHint int) ([]Commit, error) {
	commits := make([]Commit, 0, capHint)
	sc := bufio.NewScanner(bytes.NewReader(raw))

	var currentHash string
	currentParentCount := 0
	currentAuthors := []string{}
	fileSet := map[string]struct{}{}

	flush := func() {
		if currentHash == "" || currentParentCount > 1 {
			return
		}
		files := make([]string, 0, len(fileSet))
		for f := range fileSet {
			files = append(files, f)
		}
		commits = append(commits, Commit{
			Hash:    currentHash,
			Files:   files,
			Authors: append([]string(nil), currentAuthors...),
		})
	}

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, marker) {
			flush()
			payload := strings.TrimSpace(strings.TrimPrefix(line, marker))
			if payload == "" {
				currentHash = ""
				currentParentCount = 0
				currentAuthors = nil
				fileSet = map[string]struct{}{}
				continue
			}

			fields := strings.SplitN(payload, fieldSep, 5)
			if len(fields) >= 3 {
				currentHash = strings.TrimSpace(fields[0])
				currentParentCount = countParents(fields[1])
				co := ""
				reviewed := ""
				if len(fields) > 3 {
					co = fields[3]
				}
				if len(fields) > 4 {
					reviewed = fields[4]
				}
				currentAuthors = mergeAuthors(fields[2], co, reviewed)
			} else {
				parts := strings.Fields(payload)
				if len(parts) == 0 {
					currentHash = ""
					currentParentCount = 0
					currentAuthors = nil
					fileSet = map[string]struct{}{}
					continue
				}
				currentHash = parts[0]
				currentParentCount = len(parts) - 1
				currentAuthors = nil
			}
			fileSet = map[string]struct{}{}
			continue
		}
		if currentHash != "" {
			fileSet[line] = struct{}{}
		}
	}
	flush()

	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("failed reading git log: %w", err)
	}

	return commits, nil
}

func countParents(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	return len(strings.Fields(raw))
}

func mergeAuthors(primary, coAuthored, reviewed string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 4)
	add := func(raw string) {
		name := parseAuthorName(raw)
		if name == "" {
			return
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, name)
	}
	add(primary)
	for _, part := range strings.Split(coAuthored, trailerSep) {
		add(part)
	}
	for _, part := range strings.Split(reviewed, trailerSep) {
		add(part)
	}
	return out
}

func parseAuthorName(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if idx := strings.Index(s, "<"); idx >= 0 {
		s = strings.TrimSpace(s[:idx])
	}
	s = strings.Trim(s, " ,;:")
	return s
}

func run(path string, timeout time.Duration, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmdArgs := make([]string, 0, len(args)+2)
	cmdArgs = append(cmdArgs, "-C", path)
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("git command timed out")
		}
		if stderr.Len() > 0 {
			return "", fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
		}
		return "", fmt.Errorf("git command failed: %w", err)
	}
	return stdout.String(), nil
}
