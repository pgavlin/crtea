package provider

import (
	"testing"
	"time"

	"github.com/pgavlin/crtea/model"
)

func TestImportComments(t *testing.T) {
	now := time.Now()
	comments := []Comment{
		{
			ExternalID: "c1",
			Author:     "alice",
			Body:       "looks wrong",
			Path:       "main.go",
			Line:       10,
			Side:       "new",
			CreatedAt:  now,
		},
		{
			ExternalID: "c2",
			Author:     "bob",
			Body:       "agreed",
			Path:       "main.go",
			Line:       10,
			Side:       "new",
			ReplyToID:  "c1",
			CreatedAt:  now.Add(time.Minute),
		},
		{
			ExternalID: "c3",
			Author:     "carol",
			Body:       "old side comment",
			Path:       "util.go",
			Line:       5,
			StartLine:  3,
			Side:       "old",
			CreatedAt:  now,
			IsOutdated: true,
		},
	}

	result := ImportComments(comments)

	// Check main.go
	mainComments, ok := result["main.go"]
	if !ok {
		t.Fatal("expected main.go in result")
	}
	line10 := mainComments[10]
	if len(line10) != 2 {
		t.Fatalf("expected 2 comments on main.go:10, got %d", len(line10))
	}
	if line10[0].Author != "alice" {
		t.Errorf("expected author alice, got %s", line10[0].Author)
	}
	if line10[0].ExternalID != "c1" {
		t.Errorf("expected external ID c1, got %s", line10[0].ExternalID)
	}
	if line10[1].ReplyToID != "c1" {
		t.Errorf("expected reply to c1, got %s", line10[1].ReplyToID)
	}
	if line10[0].Side != model.SideNew {
		t.Errorf("expected SideNew, got %d", line10[0].Side)
	}

	// Check util.go
	utilComments, ok := result["util.go"]
	if !ok {
		t.Fatal("expected util.go in result")
	}
	line5 := utilComments[5]
	if len(line5) != 1 {
		t.Fatalf("expected 1 comment on util.go:5, got %d", len(line5))
	}
	if line5[0].Side != model.SideOld {
		t.Errorf("expected SideOld, got %d", line5[0].Side)
	}
	if line5[0].LineRange == nil || line5[0].LineRange.Start != 3 || line5[0].LineRange.End != 5 {
		t.Errorf("expected line range 3-5, got %v", line5[0].LineRange)
	}
	if !line5[0].IsOutdated {
		t.Error("expected IsOutdated to be true")
	}
}

func TestImportCommentsThreadResolution(t *testing.T) {
	comments := []Comment{
		{
			ExternalID: "c10",
			Author:     "alice",
			Body:       "fix this",
			Path:       "main.go",
			Line:       5,
			Side:       "new",
			ThreadID:   "thread-1",
			IsResolved: true,
		},
		{
			ExternalID: "c11",
			Author:     "bob",
			Body:       "agreed",
			Path:       "main.go",
			Line:       5,
			Side:       "new",
			ReplyToID:  "c10",
			ThreadID:   "thread-1",
			IsResolved: true,
		},
	}

	result := ImportComments(comments)
	line5 := result["main.go"][5]
	if len(line5) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(line5))
	}
	if line5[0].ThreadID != "thread-1" {
		t.Errorf("expected ThreadID thread-1, got %s", line5[0].ThreadID)
	}
	if !line5[0].IsResolved {
		t.Error("expected IsResolved true on root comment")
	}
	if line5[1].ThreadID != "thread-1" {
		t.Errorf("expected ThreadID thread-1 on reply, got %s", line5[1].ThreadID)
	}
	if !line5[1].IsResolved {
		t.Error("expected IsResolved true on reply comment")
	}
}

func TestImportReview(t *testing.T) {
	review := Review{
		ExternalID: "r1",
		Author:     "bob",
		Body:       "LGTM",
		State:      ReviewApprove,
	}

	result := ImportReview(review)
	if result.Author != "bob" {
		t.Errorf("expected author bob, got %s", result.Author)
	}
	if result.Status != model.ApprovalApprove {
		t.Errorf("expected ApprovalApprove, got %d", result.Status)
	}
	if result.ExternalID != "r1" {
		t.Errorf("expected external ID r1, got %s", result.ExternalID)
	}
}

func TestImportReviewRequestChanges(t *testing.T) {
	review := Review{State: ReviewRequestChanges}
	result := ImportReview(review)
	if result.Status != model.ApprovalRequestChanges {
		t.Errorf("expected ApprovalRequestChanges, got %d", result.Status)
	}
}

func TestExportComments(t *testing.T) {
	session := model.NewSession("/repo", "main", "abc123", model.DiffPullRequest)
	fr := session.GetOrCreateFileReview("main.go", model.FileModified)
	fr.AddLineComment(10, model.Comment{
		ID:      "local1",
		Content: "fix this",
		Side:    model.SideNew,
	})
	// Already submitted — should be skipped
	fr.AddLineComment(20, model.Comment{
		ID:        "local2",
		Content:   "already sent",
		Side:      model.SideNew,
		Submitted: true,
	})
	// Remote comment — should be skipped
	fr.AddLineComment(30, model.Comment{
		ID:         "remote1",
		Content:    "from someone else",
		Author:     "other",
		ExternalID: "ext1",
		Side:       model.SideOld,
	})

	drafts := ExportComments(session, "me")
	if len(drafts) != 1 {
		t.Fatalf("expected 1 draft, got %d", len(drafts))
	}
	if drafts[0].Path != "main.go" {
		t.Errorf("expected path main.go, got %s", drafts[0].Path)
	}
	if drafts[0].Line != 10 {
		t.Errorf("expected line 10, got %d", drafts[0].Line)
	}
	if drafts[0].Side != "new" {
		t.Errorf("expected side new, got %s", drafts[0].Side)
	}
}

func TestExportReviewState(t *testing.T) {
	tests := []struct {
		in  model.ApprovalStatus
		out ReviewState
	}{
		{model.ApprovalApprove, ReviewApprove},
		{model.ApprovalRequestChanges, ReviewRequestChanges},
		{model.ApprovalNeutral, ReviewComment},
	}
	for _, tt := range tests {
		got := ExportReviewState(tt.in)
		if got != tt.out {
			t.Errorf("ExportReviewState(%d) = %d, want %d", tt.in, got, tt.out)
		}
	}
}

func TestImportConversation(t *testing.T) {
	now := time.Now()
	comments := []ConversationComment{
		{ExternalID: "cc1", Author: "alice", Body: "Ready for review", CreatedAt: now},
		{ExternalID: "cc2", Author: "bob", Body: "On it", CreatedAt: now.Add(time.Minute)},
	}

	result := ImportConversation(comments)
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
	if result[0].Author != "alice" {
		t.Errorf("expected alice, got %s", result[0].Author)
	}
	if result[1].Body != "On it" {
		t.Errorf("expected 'On it', got %s", result[1].Body)
	}
}

func TestMergeImportedComments_DedupsSubmitted(t *testing.T) {
	fr := &model.FileReview{
		Path:         "main.go",
		LineComments: map[int][]model.Comment{},
	}
	// Simulate a locally-submitted comment (no ExternalID)
	fr.AddLineComment(6, model.Comment{
		ID:        "local-1",
		Content:   "fix this",
		Side:      model.SideNew,
		Author:    "alice",
		Submitted: true,
	})

	// Import the same comment back from the server (with ExternalID)
	remote := map[int][]model.Comment{
		6: {
			{
				ID:         "ext-100",
				Content:    "fix this",
				Side:       model.SideNew,
				Author:     "alice",
				ExternalID: "ext-100",
			},
		},
	}

	added := MergeImportedComments(fr, remote)
	if added != 1 {
		t.Errorf("expected 1 added, got %d", added)
	}
	if len(fr.LineComments[6]) != 1 {
		t.Fatalf("expected 1 comment on line 6, got %d", len(fr.LineComments[6]))
	}
	if fr.LineComments[6][0].ExternalID != "ext-100" {
		t.Errorf("expected remote comment to remain, got ExternalID=%q", fr.LineComments[6][0].ExternalID)
	}
}

func TestMergeImportedComments_SkipsAlreadyImported(t *testing.T) {
	fr := &model.FileReview{
		Path:         "main.go",
		LineComments: map[int][]model.Comment{},
	}
	fr.AddLineComment(10, model.Comment{
		ID:         "ext-200",
		Content:    "looks wrong",
		Side:       model.SideNew,
		Author:     "bob",
		ExternalID: "ext-200",
	})

	remote := map[int][]model.Comment{
		10: {
			{
				ID:         "ext-200",
				Content:    "looks wrong",
				Side:       model.SideNew,
				Author:     "bob",
				ExternalID: "ext-200",
			},
		},
	}

	added := MergeImportedComments(fr, remote)
	if added != 0 {
		t.Errorf("expected 0 added (already exists), got %d", added)
	}
	if len(fr.LineComments[10]) != 1 {
		t.Errorf("expected 1 comment, got %d", len(fr.LineComments[10]))
	}
}
