package github

import (
	"encoding/json"
	"testing"

	"github.com/pgavlin/crtea/provider"
)

func TestParsePullRequest(t *testing.T) {
	data := `{
		"number": 42,
		"title": "Fix login bug",
		"body": "This fixes the login issue.",
		"state": "open",
		"user": {"login": "alice"},
		"html_url": "https://github.com/octocat/repo/pull/42",
		"base": {"ref": "main"},
		"head": {"ref": "fix-login", "sha": "abc1234567890"},
		"merged": false
	}`

	var pr ghPullRequest
	if err := json.Unmarshal([]byte(data), &pr); err != nil {
		t.Fatal(err)
	}

	rr := pr.toProvider()
	if rr.ID != "42" {
		t.Errorf("ID = %q, want 42", rr.ID)
	}
	if rr.Title != "Fix login bug" {
		t.Errorf("Title = %q", rr.Title)
	}
	if rr.Author != "alice" {
		t.Errorf("Author = %q", rr.Author)
	}
	if rr.State != "open" {
		t.Errorf("State = %q", rr.State)
	}
	if rr.HeadSHA != "abc1234567890" {
		t.Errorf("HeadSHA = %q", rr.HeadSHA)
	}
}

func TestParsePullRequestMerged(t *testing.T) {
	data := `{
		"number": 10,
		"title": "Merged PR",
		"body": "",
		"state": "closed",
		"user": {"login": "bob"},
		"html_url": "https://github.com/octocat/repo/pull/10",
		"base": {"ref": "main"},
		"head": {"ref": "feature", "sha": "def456"},
		"merged": true
	}`

	var pr ghPullRequest
	json.Unmarshal([]byte(data), &pr)
	rr := pr.toProvider()
	if rr.State != "merged" {
		t.Errorf("State = %q, want merged", rr.State)
	}
}

func TestParseReview(t *testing.T) {
	data := `{
		"id": 100,
		"user": {"login": "bob"},
		"body": "LGTM",
		"state": "APPROVED",
		"submitted_at": "2025-01-15T10:30:00Z"
	}`

	var r ghReview
	if err := json.Unmarshal([]byte(data), &r); err != nil {
		t.Fatal(err)
	}

	review := r.toProvider()
	if review.ExternalID != "100" {
		t.Errorf("ExternalID = %q", review.ExternalID)
	}
	if review.Author != "bob" {
		t.Errorf("Author = %q", review.Author)
	}
	if review.State != provider.ReviewApprove {
		t.Errorf("State = %d, want ReviewApprove", review.State)
	}
}

func TestParseReviewChangesRequested(t *testing.T) {
	data := `{
		"id": 101,
		"user": {"login": "carol"},
		"body": "Needs work",
		"state": "CHANGES_REQUESTED",
		"submitted_at": "2025-01-15T11:00:00Z"
	}`

	var r ghReview
	json.Unmarshal([]byte(data), &r)
	review := r.toProvider()
	if review.State != provider.ReviewRequestChanges {
		t.Errorf("State = %d, want ReviewRequestChanges", review.State)
	}
}

func TestParseComment(t *testing.T) {
	line := 25
	data := `{
		"id": 200,
		"user": {"login": "bob"},
		"body": "Check this line",
		"path": "src/auth.go",
		"line": 25,
		"side": "RIGHT",
		"start_side": "",
		"created_at": "2025-01-15T12:00:00Z",
		"position": 10,
		"original_position": 10
	}`

	var c ghComment
	if err := json.Unmarshal([]byte(data), &c); err != nil {
		t.Fatal(err)
	}

	comment := c.toProvider()
	if comment.ExternalID != "200" {
		t.Errorf("ExternalID = %q", comment.ExternalID)
	}
	if comment.Path != "src/auth.go" {
		t.Errorf("Path = %q", comment.Path)
	}
	if comment.Line != line {
		t.Errorf("Line = %d, want %d", comment.Line, line)
	}
	if comment.Side != "new" {
		t.Errorf("Side = %q, want new", comment.Side)
	}
	if comment.IsOutdated {
		t.Error("should not be outdated")
	}
}

func TestParseCommentReply(t *testing.T) {
	data := `{
		"id": 201,
		"user": {"login": "alice"},
		"body": "Good catch",
		"path": "src/auth.go",
		"line": 25,
		"side": "RIGHT",
		"in_reply_to_id": 200,
		"created_at": "2025-01-15T12:30:00Z",
		"position": 10
	}`

	var c ghComment
	json.Unmarshal([]byte(data), &c)
	comment := c.toProvider()
	if comment.ReplyToID != "200" {
		t.Errorf("ReplyToID = %q, want 200", comment.ReplyToID)
	}
}

func TestParseCommentOutdated(t *testing.T) {
	origLine := 10
	data := `{
		"id": 202,
		"user": {"login": "carol"},
		"body": "Typo here",
		"path": "src/main.go",
		"original_line": 10,
		"side": "RIGHT",
		"created_at": "2025-01-15T09:00:00Z",
		"position": null,
		"original_position": 5
	}`

	var c ghComment
	json.Unmarshal([]byte(data), &c)
	comment := c.toProvider()
	if !comment.IsOutdated {
		t.Error("should be outdated")
	}
	if comment.Line != origLine {
		t.Errorf("Line = %d, want %d (from original_line)", comment.Line, origLine)
	}
}

func TestParseCommentLeftSide(t *testing.T) {
	data := `{
		"id": 203,
		"user": {"login": "bob"},
		"body": "Was this removed intentionally?",
		"path": "server.go",
		"line": 15,
		"side": "LEFT",
		"created_at": "2025-01-15T12:00:00Z",
		"position": 3
	}`

	var c ghComment
	json.Unmarshal([]byte(data), &c)
	comment := c.toProvider()
	if comment.Side != "old" {
		t.Errorf("Side = %q, want old", comment.Side)
	}
}

func TestParseCommit(t *testing.T) {
	data := `{
		"sha": "abc1234567890def",
		"commit": {
			"message": "Add auth middleware\n\nDetailed description here",
			"author": {
				"name": "Alice Smith",
				"date": "2025-01-14T08:00:00Z"
			}
		},
		"author": {"login": "alice"}
	}`

	var c ghCommit
	if err := json.Unmarshal([]byte(data), &c); err != nil {
		t.Fatal(err)
	}

	commit := c.toProvider()
	if commit.ID != "abc1234567890def" {
		t.Errorf("ID = %q", commit.ID)
	}
	if commit.ShortID != "abc1234" {
		t.Errorf("ShortID = %q, want abc1234", commit.ShortID)
	}
	if commit.Summary != "Add auth middleware" {
		t.Errorf("Summary = %q", commit.Summary)
	}
	if commit.Author != "alice" {
		t.Errorf("Author = %q", commit.Author)
	}
}

func TestParseCommitNoGitHubAuthor(t *testing.T) {
	data := `{
		"sha": "def456",
		"commit": {
			"message": "Fix bug",
			"author": {
				"name": "External User",
				"date": "2025-01-14T08:00:00Z"
			}
		},
		"author": null
	}`

	var c ghCommit
	json.Unmarshal([]byte(data), &c)
	commit := c.toProvider()
	if commit.Author != "External User" {
		t.Errorf("Author = %q, want External User (fallback to git author)", commit.Author)
	}
}

func TestParseIssueComment(t *testing.T) {
	data := `{
		"id": 300,
		"user": {"login": "alice"},
		"body": "Ready for review!",
		"created_at": "2025-01-15T08:00:00Z"
	}`

	var c ghIssueComment
	if err := json.Unmarshal([]byte(data), &c); err != nil {
		t.Fatal(err)
	}

	conv := c.toProvider()
	if conv.ExternalID != "300" {
		t.Errorf("ExternalID = %q", conv.ExternalID)
	}
	if conv.Author != "alice" {
		t.Errorf("Author = %q", conv.Author)
	}
	if conv.Body != "Ready for review!" {
		t.Errorf("Body = %q", conv.Body)
	}
}

func TestExportSide(t *testing.T) {
	tests := []struct {
		side string
		want string
	}{
		{"new", "RIGHT"},
		{"old", "LEFT"},
		{"", "RIGHT"},
	}
	for _, tt := range tests {
		got := exportSide(tt.side)
		if got != tt.want {
			t.Errorf("exportSide(%q) = %q, want %q", tt.side, got, tt.want)
		}
	}
}

func TestExportReviewEvent(t *testing.T) {
	tests := []struct {
		state provider.ReviewState
		want  string
	}{
		{provider.ReviewApprove, "APPROVE"},
		{provider.ReviewRequestChanges, "REQUEST_CHANGES"},
		{provider.ReviewComment, "COMMENT"},
	}
	for _, tt := range tests {
		got := exportReviewEvent(tt.state)
		if got != tt.want {
			t.Errorf("exportReviewEvent(%d) = %q, want %q", tt.state, got, tt.want)
		}
	}
}
