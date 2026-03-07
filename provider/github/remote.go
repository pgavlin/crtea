// Package github implements provider.Provider for GitHub using the gh CLI.
package github

import (
	"fmt"
	"os/exec"
	"strings"
)

// DetectRemote extracts owner/repo from the git remote URL.
// It checks the "origin" remote and handles SSH, HTTPS, and gh-style formats.
func DetectRemote(repoPath string) (owner, repo string, err error) {
	cmd := exec.Command("git", "-C", repoPath, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("getting remote URL: %w", err)
	}
	return ParseRemoteURL(strings.TrimSpace(string(out)))
}

// ParseRemoteURL extracts owner/repo from a GitHub remote URL.
// Supported formats:
//   - git@github.com:owner/repo.git
//   - https://github.com/owner/repo.git
//   - https://github.com/owner/repo
//   - ssh://git@github.com/owner/repo.git
//   - github.com/owner/repo
func ParseRemoteURL(rawURL string) (owner, repo string, err error) {
	s := rawURL

	// SSH format: git@github.com:owner/repo.git
	if strings.HasPrefix(s, "git@github.com:") {
		s = strings.TrimPrefix(s, "git@github.com:")
		return splitOwnerRepo(s)
	}

	// ssh:// format: ssh://git@github.com/owner/repo.git
	if strings.HasPrefix(s, "ssh://git@github.com/") {
		s = strings.TrimPrefix(s, "ssh://git@github.com/")
		return splitOwnerRepo(s)
	}

	// HTTPS format: https://github.com/owner/repo.git
	if strings.HasPrefix(s, "https://github.com/") {
		s = strings.TrimPrefix(s, "https://github.com/")
		return splitOwnerRepo(s)
	}

	// Bare format: github.com/owner/repo
	if strings.HasPrefix(s, "github.com/") {
		s = strings.TrimPrefix(s, "github.com/")
		return splitOwnerRepo(s)
	}

	return "", "", fmt.Errorf("not a GitHub remote: %s", rawURL)
}

func splitOwnerRepo(path string) (string, string, error) {
	path = strings.TrimSuffix(path, ".git")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("cannot parse owner/repo from %q", path)
	}
	return parts[0], parts[1], nil
}
