package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const marker = "__GITROT_COMMIT__"

type Commit struct {
	Hash  string
	Files []string
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
		"--pretty=format:" + marker + "%H %P",
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

func parseCommits(raw []byte, capHint int) ([]Commit, error) {
	commits := make([]Commit, 0, capHint)
	sc := bufio.NewScanner(bytes.NewReader(raw))

	var currentHash string
	currentParentCount := 0
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
			Hash:  currentHash,
			Files: files,
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
			parts := strings.Fields(payload)
			if len(parts) == 0 {
				currentHash = ""
				currentParentCount = 0
				fileSet = map[string]struct{}{}
				continue
			}

			currentHash = parts[0]
			currentParentCount = len(parts) - 1
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
