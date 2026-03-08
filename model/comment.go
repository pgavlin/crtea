package model

import (
	"time"

	"github.com/google/uuid"
)

// CommentType categorizes a review comment.
type CommentType int

const (
	CommentNote CommentType = iota
	CommentSuggestion
	CommentIssue
	CommentPraise
	CommentQuestion
)

// String returns the display name of the comment type.
func (t CommentType) String() string {
	switch t {
	case CommentNote:
		return "Note"
	case CommentSuggestion:
		return "Suggestion"
	case CommentIssue:
		return "Issue"
	case CommentPraise:
		return "Praise"
	case CommentQuestion:
		return "Question"
	default:
		return "Note"
	}
}

// Next returns the next comment type, cycling through all types.
func (t CommentType) Next() CommentType {
	return (t + 1) % 5
}

// LineSide indicates which side of the diff a comment is on.
type LineSide int

const (
	SideOld LineSide = iota
	SideNew
)

// LineRange represents a range of lines for multi-line comments.
type LineRange struct {
	Start int
	End   int
}

// Comment represents a review comment.
type Comment struct {
	ID         string      `json:"id"`
	Content    string      `json:"content"`
	Type       CommentType `json:"type"`
	CreatedAt  time.Time   `json:"created_at"`
	Side       LineSide    `json:"side,omitempty"`
	LineRange  *LineRange  `json:"line_range,omitempty"`
	Author     string      `json:"author,omitempty"`
	ExternalID string      `json:"external_id,omitempty"`
	ReplyToID  string      `json:"reply_to_id,omitempty"`
	Submitted  bool        `json:"submitted,omitempty"`
	IsOutdated bool        `json:"is_outdated,omitempty"`
	ThreadID   string      `json:"thread_id,omitempty"`
	IsResolved bool        `json:"is_resolved,omitempty"`
}

// NewComment creates a new comment.
func NewComment(content string, commentType CommentType, side LineSide) Comment {
	return Comment{
		ID:        uuid.New().String(),
		Content:   content,
		Type:      commentType,
		CreatedAt: time.Now().UTC(),
		Side:      side,
	}
}

// NewRangeComment creates a new multi-line comment.
func NewRangeComment(content string, commentType CommentType, side LineSide, lineRange LineRange) Comment {
	c := NewComment(content, commentType, side)
	c.LineRange = &lineRange
	return c
}
