// Package provider defines the interface and types for remote code review providers.
package provider

import "time"

// ReviewRequest holds metadata for a PR / MR / etc.
type ReviewRequest struct {
	ID      string // provider-specific identifier
	Title   string
	Body    string
	State   string // "open", "closed", "merged"
	IsDraft bool
	Author  string
	URL     string // web URL
	BaseRef string
	HeadRef string
	HeadSHA string
}

// ReviewState represents the state of a top-level review.
type ReviewState int

const (
	ReviewComment ReviewState = iota
	ReviewApprove
	ReviewRequestChanges
)

// Review is a top-level review (body + status).
type Review struct {
	ExternalID string
	Author     string
	Body       string
	State      ReviewState
	CreatedAt  time.Time
}

// Comment is an inline comment on a specific file/line.
type Comment struct {
	ExternalID string
	Author     string
	Body       string
	Path       string
	Line       int
	StartLine  int    // 0 for single-line
	Side       string // "old" or "new"
	StartSide  string
	ReplyToID  string // empty if top-level
	CreatedAt  time.Time
	IsOutdated bool
	ThreadID   string // GraphQL node ID of the review thread
	IsResolved bool   // whether the thread is resolved
}

// ConversationComment is a general comment not tied to code.
type ConversationComment struct {
	ExternalID string
	Author     string
	Body       string
	CreatedAt  time.Time
}

// SubmitReviewRequest is the payload for submitting a review.
type SubmitReviewRequest struct {
	Body     string
	State    ReviewState
	CommitID string // SHA of the commit to review against
	Comments []CommentDraft
}

// CommentDraft is a new inline comment to submit.
type CommentDraft struct {
	Path      string
	Line      int
	StartLine int
	Side      string
	StartSide string
	Body      string
}

// Commit holds metadata for a single commit in a review request.
type Commit struct {
	ID      string
	ShortID string
	Summary string
	Author  string
	Time    time.Time
}

// RefreshResult describes what changed on refresh.
type RefreshResult struct {
	Request         *ReviewRequest
	NewComments     []Comment
	AllComments     []Comment // full current comment list (for updating outdated status)
	NewReviews      []Review
	NewConversation []ConversationComment
	DiffChanged     bool   // true if the diff changed (new commits pushed)
	Diff            string // new diff content, if DiffChanged
}
