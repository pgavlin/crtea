package output

import (
	"strings"
	"testing"

	"github.com/pgavlin/crtea/model"
)

func TestGenerateMarkdownEmpty(t *testing.T) {
	session := model.NewSession("/repo", "main", "abc", model.DiffWorkingTree)
	got := GenerateMarkdown(session)
	if got != "" {
		t.Errorf("expected empty string for session with no comments, got %q", got)
	}
}

func TestGenerateMarkdownFileComment(t *testing.T) {
	session := model.NewSession("/repo", "main", "abc", model.DiffWorkingTree)
	fr := session.GetOrCreateFileReview("main.go", model.FileModified)
	fr.AddFileComment(model.NewComment("looks good", model.CommentPraise, model.SideNew))

	got := GenerateMarkdown(session)

	if !strings.Contains(got, "# Code Review Comments") {
		t.Error("missing header")
	}
	if !strings.Contains(got, "## main.go") {
		t.Error("missing file header")
	}
	if !strings.Contains(got, "**PRAISE**") {
		t.Error("missing comment type badge")
	}
	if !strings.Contains(got, "looks good") {
		t.Error("missing comment content")
	}
}

func TestGenerateMarkdownLineComment(t *testing.T) {
	session := model.NewSession("/repo", "main", "abc", model.DiffWorkingTree)
	fr := session.GetOrCreateFileReview("app.go", model.FileModified)
	fr.AddLineComment(42, model.NewComment("fix this", model.CommentIssue, model.SideNew))

	got := GenerateMarkdown(session)

	if !strings.Contains(got, "**ISSUE**") {
		t.Error("missing comment type")
	}
	if !strings.Contains(got, "`app.go:42`") {
		t.Error("missing line reference")
	}
	if !strings.Contains(got, "fix this") {
		t.Error("missing comment content")
	}
}

func TestGenerateMarkdownRangeComment(t *testing.T) {
	session := model.NewSession("/repo", "main", "abc", model.DiffWorkingTree)
	fr := session.GetOrCreateFileReview("app.go", model.FileModified)
	lr := model.LineRange{Start: 10, End: 20}
	fr.AddLineComment(10, model.NewRangeComment("refactor", model.CommentSuggestion, model.SideNew, lr))

	got := GenerateMarkdown(session)

	if !strings.Contains(got, "`app.go:10-20`") {
		t.Error("missing range reference")
	}
}

func TestGenerateMarkdownSortedFiles(t *testing.T) {
	session := model.NewSession("/repo", "main", "abc", model.DiffWorkingTree)

	fr1 := session.GetOrCreateFileReview("z.go", model.FileModified)
	fr1.AddFileComment(model.NewComment("z", model.CommentNote, model.SideNew))

	fr2 := session.GetOrCreateFileReview("a.go", model.FileModified)
	fr2.AddFileComment(model.NewComment("a", model.CommentNote, model.SideNew))

	got := GenerateMarkdown(session)

	aIdx := strings.Index(got, "## a.go")
	zIdx := strings.Index(got, "## z.go")
	if aIdx < 0 || zIdx < 0 {
		t.Fatal("missing file headers")
	}
	if aIdx > zIdx {
		t.Error("files should be sorted alphabetically: a.go before z.go")
	}
}
