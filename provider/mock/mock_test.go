package mock

import (
	"testing"

	"github.com/pgavlin/crtea/provider"
)

func TestNewReturnsPopulatedMock(t *testing.T) {
	m := New()

	user, err := m.GetAuthenticatedUser()
	if err != nil {
		t.Fatal(err)
	}
	if user != "you" {
		t.Errorf("expected user 'you', got %q", user)
	}

	rr, err := m.GetReviewRequest("42")
	if err != nil {
		t.Fatal(err)
	}
	if rr.Title != "Add user authentication middleware" {
		t.Errorf("unexpected title: %s", rr.Title)
	}
	if rr.Author != "alice" {
		t.Errorf("unexpected author: %s", rr.Author)
	}

	diff, err := m.GetDiff("42")
	if err != nil {
		t.Fatal(err)
	}
	if len(diff) == 0 {
		t.Error("expected non-empty diff")
	}
}

func TestListReviews(t *testing.T) {
	m := New()
	reviews, err := m.ListReviews("42")
	if err != nil {
		t.Fatal(err)
	}
	if len(reviews) != 2 {
		t.Fatalf("expected 2 reviews, got %d", len(reviews))
	}
	if reviews[0].Author != "bob" {
		t.Errorf("expected bob, got %s", reviews[0].Author)
	}
	if reviews[0].State != provider.ReviewRequestChanges {
		t.Errorf("expected RequestChanges, got %d", reviews[0].State)
	}
}

func TestListComments(t *testing.T) {
	m := New()
	comments, err := m.ListComments("42")
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 6 {
		t.Fatalf("expected 6 comments, got %d", len(comments))
	}

	// Check threading
	if comments[1].ReplyToID != "c1" {
		t.Errorf("expected reply to c1, got %s", comments[1].ReplyToID)
	}
	if comments[2].ReplyToID != "c1" {
		t.Errorf("expected reply to c1, got %s", comments[2].ReplyToID)
	}

	// Check outdated
	if !comments[5].IsOutdated {
		t.Error("expected comment c6 to be outdated")
	}
}

func TestListConversation(t *testing.T) {
	m := New()
	conv, err := m.ListConversation("42")
	if err != nil {
		t.Fatal(err)
	}
	if len(conv) != 3 {
		t.Fatalf("expected 3 conversation comments, got %d", len(conv))
	}
}

func TestSubmitReview(t *testing.T) {
	m := New()
	err := m.SubmitReview("42", provider.SubmitReviewRequest{
		Body:  "LGTM",
		State: provider.ReviewApprove,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.submitted) != 1 {
		t.Fatalf("expected 1 submitted, got %d", len(m.submitted))
	}
}

func TestPostConversationComment(t *testing.T) {
	m := New()
	err := m.PostConversationComment("42", "Thanks!")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.posted) != 1 {
		t.Fatalf("expected 1 posted, got %d", len(m.posted))
	}
	if m.posted[0].Author != "you" {
		t.Errorf("expected author 'you', got %s", m.posted[0].Author)
	}
}

func TestRefreshReturnsPosted(t *testing.T) {
	m := New()

	// First refresh — nothing new
	result, err := m.Refresh("42")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.NewConversation) != 0 {
		t.Fatalf("expected 0 new, got %d", len(result.NewConversation))
	}

	// Post a comment
	m.PostConversationComment("42", "new comment")

	// Refresh — should see the new comment
	result, err = m.Refresh("42")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.NewConversation) != 1 {
		t.Fatalf("expected 1 new, got %d", len(result.NewConversation))
	}

	// Refresh again — nothing new
	result, err = m.Refresh("42")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.NewConversation) != 0 {
		t.Fatalf("expected 0 new after second refresh, got %d", len(result.NewConversation))
	}
}

func TestReplyToComment(t *testing.T) {
	m := New()
	err := m.ReplyToComment("42", "c1", "I agree")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.replies) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(m.replies))
	}
	if m.replies[0].CommentID != "c1" {
		t.Errorf("expected comment ID c1, got %s", m.replies[0].CommentID)
	}
}

func TestListCommits(t *testing.T) {
	m := New()
	commits, err := m.ListCommits("42")
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}
	if commits[0].ShortID != "abc1234" {
		t.Errorf("expected abc1234, got %s", commits[0].ShortID)
	}
	if commits[1].Summary != "Add middleware unit tests" {
		t.Errorf("unexpected summary: %s", commits[1].Summary)
	}
}

func TestGetCommitDiff(t *testing.T) {
	m := New()

	// Commit 1: middleware + server
	diff1, err := m.GetCommitDiff("42", "abc1234")
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(diff1, "auth/middleware.go") {
		t.Error("commit 1 diff missing auth/middleware.go")
	}
	if !containsString(diff1, "server.go") {
		t.Error("commit 1 diff missing server.go")
	}
	if containsString(diff1, "middleware_test.go") {
		t.Error("commit 1 diff should not contain test file")
	}

	// Commit 2: tests
	diff2, err := m.GetCommitDiff("42", "def5678")
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(diff2, "middleware_test.go") {
		t.Error("commit 2 diff missing middleware_test.go")
	}

	// Unknown commit
	_, err = m.GetCommitDiff("42", "unknown")
	if err == nil {
		t.Error("expected error for unknown commit")
	}
}

func TestDiffParseable(t *testing.T) {
	m := New()
	diff, _ := m.GetDiff("42")

	// Verify it contains the expected file markers
	if !containsString(diff, "auth/middleware.go") {
		t.Error("diff missing auth/middleware.go")
	}
	if !containsString(diff, "server.go") {
		t.Error("diff missing server.go")
	}
	if !containsString(diff, "auth/middleware_test.go") {
		t.Error("diff missing auth/middleware_test.go")
	}
}

func containsString(haystack, needle string) bool {
	return len(haystack) > 0 && len(needle) > 0 && // avoid trivial matches
		indexOf(haystack, needle) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
