package testutil

import (
	"fmt"
	"sync"
	"time"

	"github.com/pgavlin/crtea/provider"
)

// MockProvider implements provider.Provider with configurable behavior.
// Unlike provider/mock.Mock which has fixed demo data, this mock lets
// tests control exactly what each method returns.
type MockProvider struct {
	mu sync.Mutex

	User    string
	Request provider.ReviewRequest
	Diff    string

	// RefreshResult is returned by the next call to Refresh.
	// Reset to nil after each call.
	RefreshResult *provider.RefreshResult
	RefreshErr    error

	// Recorded calls
	SubmittedReviews []provider.SubmitReviewRequest
	Replies          []ReplyCall
	PostedComments   []string // conversation comment bodies
}

// ReplyCall records a call to ReplyToComment.
type ReplyCall struct {
	CommentID string
	Body      string
}

func NewMockProvider() *MockProvider {
	return &MockProvider{
		User: "testuser",
		Request: provider.ReviewRequest{
			ID:      "99",
			Title:   "Test PR",
			State:   "open",
			Author:  "author",
			BaseRef: "main",
			HeadRef: "feature",
			HeadSHA: "deadbeef",
		},
	}
}

func (m *MockProvider) Name() string { return "mock" }

func (m *MockProvider) GetAuthenticatedUser() (string, error) {
	return m.User, nil
}

func (m *MockProvider) GetReviewRequest(id string) (*provider.ReviewRequest, error) {
	return &m.Request, nil
}

func (m *MockProvider) GetDiff(id string) (string, error) {
	return m.Diff, nil
}

func (m *MockProvider) ListCommits(id string) ([]provider.Commit, error) {
	return nil, nil
}

func (m *MockProvider) GetCommitDiff(id string, commitID string) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (m *MockProvider) ListReviews(id string) ([]provider.Review, error) {
	return nil, nil
}

func (m *MockProvider) ListComments(id string) ([]provider.Comment, error) {
	return nil, nil
}

func (m *MockProvider) ListConversation(id string) ([]provider.ConversationComment, error) {
	return nil, nil
}

func (m *MockProvider) SubmitReview(id string, review provider.SubmitReviewRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SubmittedReviews = append(m.SubmittedReviews, review)
	return nil
}

func (m *MockProvider) ReplyToComment(id string, commentID string, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Replies = append(m.Replies, ReplyCall{CommentID: commentID, Body: body})
	return nil
}

func (m *MockProvider) PostConversationComment(id string, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PostedComments = append(m.PostedComments, body)
	return nil
}

func (m *MockProvider) Seed(rr *provider.ReviewRequest, comments []provider.Comment, reviews []provider.Review, conv []provider.ConversationComment) {
	// No-op for test mock.
}

func (m *MockProvider) Refresh(id string) (*provider.RefreshResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.RefreshErr != nil {
		return nil, m.RefreshErr
	}
	if m.RefreshResult != nil {
		r := m.RefreshResult
		m.RefreshResult = nil
		return r, nil
	}
	return &provider.RefreshResult{Request: &m.Request}, nil
}

// SetNextRefresh configures what the next Refresh call returns.
func (m *MockProvider) SetNextRefresh(result *provider.RefreshResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RefreshResult = result
}

// Helper to create a provider comment with common defaults.
func NewProviderComment(id, author, body, path string, line int, side string) provider.Comment {
	return provider.Comment{
		ExternalID: id,
		Author:     author,
		Body:       body,
		Path:       path,
		Line:       line,
		Side:       side,
		CreatedAt:  time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
	}
}

// Verify interface compliance.
var _ provider.Provider = (*MockProvider)(nil)
