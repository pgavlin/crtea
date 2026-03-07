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
	Info() VcsInfo
	GetWorkingTreeDiff() ([]model.DiffFile, error)
	GetCommitRangeDiff(ids []string) ([]model.DiffFile, error)
	FetchContextLines(filePath string, status model.FileStatus, startLine, endLine int) ([]model.DiffLine, error)
	GetRecentCommits(offset, limit int) ([]CommitInfo, error)
}
