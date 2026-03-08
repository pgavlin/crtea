package model

import (
	"time"

	"github.com/google/uuid"
)

// FileReview tracks review status and comments for a single file.
type FileReview struct {
	Path         string            `json:"path"`
	Reviewed     bool              `json:"reviewed"`
	Status       FileStatus        `json:"status"`
	FileComments []Comment         `json:"file_comments"`
	LineComments map[int][]Comment `json:"line_comments"` // line number -> comments
}

// NewFileReview creates a new FileReview.
func NewFileReview(path string, status FileStatus) *FileReview {
	return &FileReview{
		Path:         path,
		Status:       status,
		FileComments: nil,
		LineComments: make(map[int][]Comment),
	}
}

// AddFileComment adds a file-level comment.
func (fr *FileReview) AddFileComment(c Comment) {
	fr.FileComments = append(fr.FileComments, c)
}

// AddLineComment adds a comment to a specific line.
func (fr *FileReview) AddLineComment(line int, c Comment) {
	fr.LineComments[line] = append(fr.LineComments[line], c)
}

// HasComments returns true if any comments exist.
func (fr *FileReview) HasComments() bool {
	if len(fr.FileComments) > 0 {
		return true
	}
	for _, comments := range fr.LineComments {
		if len(comments) > 0 {
			return true
		}
	}
	return false
}

// ApprovalStatus represents the overall review approval status.
type ApprovalStatus int

const (
	ApprovalNeutral ApprovalStatus = iota
	ApprovalApprove
	ApprovalRequestChanges
)

// String returns the display name of the approval status.
func (s ApprovalStatus) String() string {
	switch s {
	case ApprovalApprove:
		return "Approve"
	case ApprovalRequestChanges:
		return "Request Changes"
	default:
		return "Neutral"
	}
}

// Next returns the next approval status, cycling through all statuses.
func (s ApprovalStatus) Next() ApprovalStatus {
	return (s + 1) % 3
}

// OverallReview captures high-level review thoughts with an approval status.
type OverallReview struct {
	Body       string         `json:"body"`
	Status     ApprovalStatus `json:"status"`
	Author     string         `json:"author,omitempty"`
	ExternalID string         `json:"external_id,omitempty"`
}

// DiffSource describes what is being diffed.
type DiffSource int

const (
	DiffWorkingTree DiffSource = iota
	DiffCommitRange
	DiffPullRequest
)

// ProviderInfo identifies a remote code review provider and request.
type ProviderInfo struct {
	Name string `json:"name"`          // "github", "gitlab", etc.
	ID   string `json:"id"`            // "123" for PR #123
	URL  string `json:"url,omitempty"` // web URL
}

// ConversationComment is a general comment not tied to code.
type ConversationComment struct {
	ExternalID string    `json:"external_id,omitempty"`
	Author     string    `json:"author,omitempty"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
}

// ReviewSession persists the review state.
type ReviewSession struct {
	ID            string                 `json:"id"`
	Version       string                 `json:"version"`
	RepoPath      string                 `json:"repo_path"`
	BranchName    string                 `json:"branch_name,omitempty"`
	BaseCommit    string                 `json:"base_commit"`
	DiffSource    DiffSource             `json:"diff_source"`
	CommitRange   []string               `json:"commit_range,omitempty"`
	Description   string                 `json:"description,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
	Files         map[string]*FileReview `json:"files"`
	OverallReview *OverallReview         `json:"overall_review,omitempty"`

	// Remote provider fields
	Provider     *ProviderInfo         `json:"provider,omitempty"`
	Reviewer     string                `json:"reviewer,omitempty"`
	Reviews      []OverallReview       `json:"reviews,omitempty"`
	Conversation []ConversationComment `json:"conversation,omitempty"`
}

// NewSession creates a new review session.
func NewSession(repoPath, branchName, baseCommit string, source DiffSource) *ReviewSession {
	now := time.Now().UTC()
	return &ReviewSession{
		ID:         generateSessionID(),
		Version:    "1.0",
		RepoPath:   repoPath,
		BranchName: branchName,
		BaseCommit: baseCommit,
		DiffSource: source,
		CreatedAt:  now,
		UpdatedAt:  now,
		Files:      make(map[string]*FileReview),
	}
}

func generateSessionID() string {
	return uuid.New().String()
}

// GetOrCreateFileReview gets or creates a FileReview for the given path.
func (s *ReviewSession) GetOrCreateFileReview(path string, status FileStatus) *FileReview {
	if fr, ok := s.Files[path]; ok {
		return fr
	}
	fr := NewFileReview(path, status)
	s.Files[path] = fr
	return fr
}

// GetFileReview gets the FileReview for a path, or nil.
func (s *ReviewSession) GetFileReview(path string) *FileReview {
	return s.Files[path]
}

// ReviewedCount returns the count of reviewed files.
func (s *ReviewSession) ReviewedCount() int {
	count := 0
	for _, fr := range s.Files {
		if fr.Reviewed {
			count++
		}
	}
	return count
}

// TotalComments returns the total number of comments.
func (s *ReviewSession) TotalComments() int {
	count := 0
	for _, fr := range s.Files {
		count += len(fr.FileComments)
		for _, comments := range fr.LineComments {
			count += len(comments)
		}
	}
	return count
}
