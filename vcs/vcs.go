// Package vcs defines the version control backend interface and implementations.
package vcs

import (
	"time"

	"github.com/pgavlin/crtea/model"
)

// VcsInfo holds repository metadata.
type VcsInfo struct {
	RootPath   string
	HeadCommit string
	BranchName string
	VcsType    string // "git"
}

// CommitInfo holds information about a single commit.
type CommitInfo struct {
	ID         string
	ShortID    string
	BranchName string
	Summary    string
	Author     string
	Time       time.Time
}

// Backend defines the interface for version control operations.
type Backend interface {
	// Info returns metadata about the repository.
	Info() VcsInfo
	// GetWorkingTreeDiff returns the diff of uncommitted changes.
	GetWorkingTreeDiff() ([]model.DiffFile, error)
	// GetCommitRangeDiff returns the combined diff for a set of commit IDs.
	GetCommitRangeDiff(ids []string) ([]model.DiffFile, error)
	// GetRevisionDiff returns the diff for a revision specification (e.g. "main~5..HEAD").
	GetRevisionDiff(revSpec string) ([]model.DiffFile, error)
	// FetchContextLines retrieves source lines surrounding a diff region for context expansion.
	FetchContextLines(filePath string, status model.FileStatus, startLine, endLine int) ([]model.DiffLine, error)
	// GetRecentCommits returns recent commits starting from offset, up to limit.
	GetRecentCommits(offset, limit int) ([]CommitInfo, error)
}
