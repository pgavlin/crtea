package model

import "testing"

func TestFileStatusString(t *testing.T) {
	tests := []struct {
		status FileStatus
		want   string
	}{
		{FileAdded, "A"},
		{FileModified, "M"},
		{FileDeleted, "D"},
		{FileRenamed, "R"},
		{FileCopied, "C"},
		{FileStatus(99), "?"},
	}
	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Errorf("FileStatus(%d).String() = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestCommentTypeString(t *testing.T) {
	tests := []struct {
		ct   CommentType
		want string
	}{
		{CommentNote, "Note"},
		{CommentSuggestion, "Suggestion"},
		{CommentIssue, "Issue"},
		{CommentPraise, "Praise"},
		{CommentQuestion, "Question"},
		{CommentType(99), "Note"},
	}
	for _, tt := range tests {
		if got := tt.ct.String(); got != tt.want {
			t.Errorf("CommentType(%d).String() = %q, want %q", tt.ct, got, tt.want)
		}
	}
}

func TestCommentTypeNext(t *testing.T) {
	tests := []struct {
		ct   CommentType
		want CommentType
	}{
		{CommentNote, CommentSuggestion},
		{CommentSuggestion, CommentIssue},
		{CommentIssue, CommentPraise},
		{CommentPraise, CommentQuestion},
		{CommentQuestion, CommentNote}, // wraps
	}
	for _, tt := range tests {
		if got := tt.ct.Next(); got != tt.want {
			t.Errorf("CommentType(%d).Next() = %d, want %d", tt.ct, got, tt.want)
		}
	}
}

func TestDiffFileDisplayPath(t *testing.T) {
	tests := []struct {
		name string
		file DiffFile
		want string
	}{
		{"new path preferred", DiffFile{OldPath: "old.go", NewPath: "new.go"}, "new.go"},
		{"old path fallback", DiffFile{OldPath: "old.go"}, "old.go"},
		{"empty", DiffFile{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.file.DisplayPath(); got != tt.want {
				t.Errorf("DisplayPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewSession(t *testing.T) {
	s := NewSession("/repo", "main", "abc123", DiffWorkingTree)
	if s.RepoPath != "/repo" {
		t.Errorf("RepoPath = %q, want /repo", s.RepoPath)
	}
	if s.BranchName != "main" {
		t.Errorf("BranchName = %q, want main", s.BranchName)
	}
	if s.BaseCommit != "abc123" {
		t.Errorf("BaseCommit = %q, want abc123", s.BaseCommit)
	}
	if s.DiffSource != DiffWorkingTree {
		t.Errorf("DiffSource = %d, want DiffWorkingTree", s.DiffSource)
	}
	if s.Files == nil {
		t.Error("Files map should be initialized")
	}
	if s.ID == "" {
		t.Error("ID should not be empty")
	}
}

func TestGetOrCreateFileReview(t *testing.T) {
	s := NewSession("/repo", "main", "abc", DiffWorkingTree)

	fr1 := s.GetOrCreateFileReview("foo.go", FileModified)
	if fr1.Path != "foo.go" {
		t.Errorf("Path = %q, want foo.go", fr1.Path)
	}
	if fr1.Status != FileModified {
		t.Errorf("Status = %d, want FileModified", fr1.Status)
	}

	// Second call returns same instance
	fr2 := s.GetOrCreateFileReview("foo.go", FileAdded)
	if fr1 != fr2 {
		t.Error("expected same FileReview instance on second call")
	}
}

func TestGetFileReview(t *testing.T) {
	s := NewSession("/repo", "main", "abc", DiffWorkingTree)

	if got := s.GetFileReview("missing.go"); got != nil {
		t.Error("expected nil for missing file")
	}

	s.GetOrCreateFileReview("exists.go", FileAdded)
	if got := s.GetFileReview("exists.go"); got == nil {
		t.Error("expected non-nil for existing file")
	}
}

func TestFileReviewComments(t *testing.T) {
	fr := NewFileReview("test.go", FileModified)

	if fr.HasComments() {
		t.Error("new FileReview should have no comments")
	}

	c1 := NewComment("file comment", CommentNote, SideNew)
	fr.AddFileComment(c1)
	if !fr.HasComments() {
		t.Error("HasComments should be true after AddFileComment")
	}
	if len(fr.FileComments) != 1 {
		t.Errorf("FileComments len = %d, want 1", len(fr.FileComments))
	}

	c2 := NewComment("line comment", CommentIssue, SideNew)
	fr.AddLineComment(42, c2)
	if len(fr.LineComments[42]) != 1 {
		t.Errorf("LineComments[42] len = %d, want 1", len(fr.LineComments[42]))
	}
}

func TestReviewedCount(t *testing.T) {
	s := NewSession("/repo", "main", "abc", DiffWorkingTree)
	s.GetOrCreateFileReview("a.go", FileModified).Reviewed = true
	s.GetOrCreateFileReview("b.go", FileModified)
	s.GetOrCreateFileReview("c.go", FileAdded).Reviewed = true

	if got := s.ReviewedCount(); got != 2 {
		t.Errorf("ReviewedCount() = %d, want 2", got)
	}
}

func TestTotalComments(t *testing.T) {
	s := NewSession("/repo", "main", "abc", DiffWorkingTree)
	fr := s.GetOrCreateFileReview("a.go", FileModified)
	fr.AddFileComment(NewComment("fc1", CommentNote, SideNew))
	fr.AddFileComment(NewComment("fc2", CommentNote, SideNew))
	fr.AddLineComment(10, NewComment("lc1", CommentIssue, SideNew))

	if got := s.TotalComments(); got != 3 {
		t.Errorf("TotalComments() = %d, want 3", got)
	}
}

func TestNewRangeComment(t *testing.T) {
	lr := LineRange{Start: 10, End: 20}
	c := NewRangeComment("range", CommentNote, SideNew, lr)
	if c.LineRange == nil {
		t.Fatal("LineRange should not be nil")
	}
	if c.LineRange.Start != 10 || c.LineRange.End != 20 {
		t.Errorf("LineRange = %+v, want {10, 20}", c.LineRange)
	}
}
