package cmd

import (
	"fmt"
	"os/exec"
	"strings"
)

type GitInfo struct {
	Dirty      bool
	CommitHash string
	Tag        []string
}

func getGitInfo(path string) (*GitInfo, error) {
	output, err := run(path, "git", "rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD sha: %w", err)
	}
	sha := strings.TrimSpace(string(output))

	output, err = run(path, "git", "status", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("failed to check if git is dirty: %w", err)
	}
	dirty := len(strings.TrimSpace(string(output))) > 0

	output, err = run(path, "git", "tag", "--points-at", sha)
	if err != nil {
		return nil, fmt.Errorf("failed to get tags: %w", err)
	}
	tags := strings.Split(string(output), "\n")
	for i := range tags {
		tags[i] = strings.TrimSpace(tags[i])
	}

	// trim empty string at the end
	if tags[len(tags)-1] == "" {
		tags = tags[:len(tags)-1]
	}

	return &GitInfo{
		Dirty:      dirty,
		CommitHash: sha,
		Tag:        tags,
	}, nil
}

func run(path string, args ...string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = path

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to run %s: %w", strings.Join(args, " "), err)
	}
	return string(output), nil
}
