// Package testutil provides mock implementations for testing.
package testutil

import (
	"github.com/pgavlin/crtea/model"
	"github.com/pgavlin/crtea/persistence"
	"github.com/pgavlin/crtea/vcs"
)

// MockVCS implements vcs.Backend with configurable return values.
type MockVCS struct {
	VcsInfo         vcs.VcsInfo
	WorkingTreeDiff []model.DiffFile
	CommitRangeDiff []model.DiffFile
	RevisionDiff    []model.DiffFile
	ContextLines    []model.DiffLine
	RecentCommits   []vcs.CommitInfo

	WorkingTreeDiffErr error
	CommitRangeDiffErr error
	RevisionDiffErr    error
	ContextLinesErr    error
	RecentCommitsErr   error

	// Record calls
	FetchContextCalls []FetchContextCall
}

// FetchContextCall records arguments to FetchContextLines.
type FetchContextCall struct {
	FilePath  string
	Status    model.FileStatus
	StartLine int
	EndLine   int
}

func (m *MockVCS) Info() vcs.VcsInfo {
	return m.VcsInfo
}

func (m *MockVCS) GetWorkingTreeDiff() ([]model.DiffFile, error) {
	return m.WorkingTreeDiff, m.WorkingTreeDiffErr
}

func (m *MockVCS) GetCommitRangeDiff(ids []string) ([]model.DiffFile, error) {
	return m.CommitRangeDiff, m.CommitRangeDiffErr
}

func (m *MockVCS) GetRevisionDiff(revSpec string) ([]model.DiffFile, error) {
	return m.RevisionDiff, m.RevisionDiffErr
}

func (m *MockVCS) FetchContextLines(filePath string, status model.FileStatus, startLine, endLine int) ([]model.DiffLine, error) {
	m.FetchContextCalls = append(m.FetchContextCalls, FetchContextCall{
		FilePath:  filePath,
		Status:    status,
		StartLine: startLine,
		EndLine:   endLine,
	})
	return m.ContextLines, m.ContextLinesErr
}

func (m *MockVCS) GetRecentCommits(offset, limit int) ([]vcs.CommitInfo, error) {
	return m.RecentCommits, m.RecentCommitsErr
}

func (m *MockVCS) GetCommitsInRange(revSpec string) ([]vcs.CommitInfo, error) {
	return m.RecentCommits, m.RecentCommitsErr
}

// MockStore implements persistence.Store for testing.
type MockStore struct {
	Sessions map[string]*model.ReviewSession // key: repoPath+branch+source
	SaveErr  error
	LoadErr  error
}

func NewMockStore() *MockStore {
	return &MockStore{Sessions: make(map[string]*model.ReviewSession)}
}

func (m *MockStore) Save(session *model.ReviewSession) (string, error) {
	if m.SaveErr != nil {
		return "", m.SaveErr
	}
	m.Sessions[session.RepoPath] = session
	return session.ID, nil
}

func (m *MockStore) LoadLatest(repoPath, branchName string, diffSource model.DiffSource) (*model.ReviewSession, error) {
	if m.LoadErr != nil {
		return nil, m.LoadErr
	}
	if s, ok := m.Sessions[repoPath]; ok {
		return s, nil
	}
	return nil, persistence.ErrSessionNotFound
}

// Verify interface compliance.
var (
	_ vcs.Backend       = (*MockVCS)(nil)
	_ persistence.Store = (*MockStore)(nil)
)
