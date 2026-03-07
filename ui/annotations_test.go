package ui

import (
	"testing"

	"github.com/pgavlin/crtea/model"
)

func TestWrapComment(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wrapWidth int
		want      []string
	}{
		{"no wrap needed", "short", 80, []string{"short"}},
		{"no wrapping when width 0", "hello world", 0, []string{"hello world"}},
		{"wraps at space", "hello world foo", 12, []string{"hello world", "foo"}},
		{"wraps long word", "abcdefghij", 5, []string{"abcde", "fghij"}},
		{"preserves newlines", "line1\nline2", 80, []string{"line1", "line2"}},
		{"empty content", "", 80, []string{""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapComment(tt.content, tt.wrapWidth)
			if len(got) != len(tt.want) {
				t.Fatalf("wrapComment() = %v (len %d), want %v (len %d)", got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("line %d = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestBuildAnnotationsBasic(t *testing.T) {
	files := []model.DiffFile{
		{
			NewPath: "test.go",
			Status:  model.FileModified,
			Hunks: []model.DiffHunk{
				{
					Header:   "@@ -1,3 +1,3 @@",
					OldStart: 1,
					NewStart: 1,
					Lines: []model.DiffLine{
						{Origin: model.OriginContext, Content: "package main", OldLineNo: 1, NewLineNo: 1},
						{Origin: model.OriginDeletion, Content: "var x = 1", OldLineNo: 2},
						{Origin: model.OriginAddition, Content: "var x = 2", NewLineNo: 2},
						{Origin: model.OriginContext, Content: "", OldLineNo: 3, NewLineNo: 3},
					},
				},
			},
		},
	}

	anns := buildAnnotations(files, nil, nil, 0)

	// Expected: fileHeader, hunkHeader, 4 diffLines, spacing
	expectedTypes := []annotationType{
		annFileHeader, annHunkHeader,
		annDiffLine, annDiffLine, annDiffLine, annDiffLine,
		annSpacing,
	}

	if len(anns) != len(expectedTypes) {
		t.Fatalf("got %d annotations, want %d", len(anns), len(expectedTypes))
	}
	for i, want := range expectedTypes {
		if anns[i].Type != want {
			t.Errorf("annotation[%d].Type = %d, want %d", i, anns[i].Type, want)
		}
	}
}

func TestBuildAnnotationsBinaryFile(t *testing.T) {
	files := []model.DiffFile{
		{NewPath: "image.png", IsBinary: true},
	}

	anns := buildAnnotations(files, nil, nil, 0)

	expectedTypes := []annotationType{annFileHeader, annBinaryOrEmpty, annSpacing}
	if len(anns) != len(expectedTypes) {
		t.Fatalf("got %d annotations, want %d", len(anns), len(expectedTypes))
	}
	for i, want := range expectedTypes {
		if anns[i].Type != want {
			t.Errorf("annotation[%d].Type = %d, want %d", i, anns[i].Type, want)
		}
	}
}

func TestBuildAnnotationsWithExpander(t *testing.T) {
	files := []model.DiffFile{
		{
			NewPath: "test.go",
			Hunks: []model.DiffHunk{
				{Header: "@@ -1,1 +1,1 @@", OldStart: 1, NewStart: 1, Lines: []model.DiffLine{
					{Origin: model.OriginContext, Content: "a", OldLineNo: 1, NewLineNo: 1},
				}},
				{Header: "@@ -10,1 +10,1 @@", OldStart: 10, NewStart: 10, Lines: []model.DiffLine{
					{Origin: model.OriginContext, Content: "b", OldLineNo: 10, NewLineNo: 10},
				}},
			},
		},
	}

	anns := buildAnnotations(files, nil, nil, 0)

	// fileHeader, hunkHeader, diffLine, expander, hunkHeader, diffLine, spacing
	var hasExpander bool
	for _, a := range anns {
		if a.Type == annExpander {
			hasExpander = true
			if a.gapID.FileIdx != 0 || a.gapID.HunkIdx != 1 {
				t.Errorf("expander gapID = %+v, want {0, 1}", a.gapID)
			}
		}
	}
	if !hasExpander {
		t.Error("expected an expander annotation between hunks")
	}
}

func TestBuildAnnotationsWithComments(t *testing.T) {
	files := []model.DiffFile{
		{
			NewPath: "test.go",
			Status:  model.FileModified,
			Hunks: []model.DiffHunk{
				{
					Header:   "@@ -1,1 +1,1 @@",
					OldStart: 1,
					NewStart: 1,
					Lines: []model.DiffLine{
						{Origin: model.OriginAddition, Content: "new line", NewLineNo: 1},
					},
				},
			},
		},
	}

	session := model.NewSession("/repo", "main", "abc", model.DiffWorkingTree)
	fr := session.GetOrCreateFileReview("test.go", model.FileModified)
	fr.AddLineComment(1, model.NewComment("test comment", model.CommentNote, model.SideNew))

	anns := buildAnnotations(files, session, nil, 0)

	var hasLineComment bool
	for _, a := range anns {
		if a.Type == annLineComment {
			hasLineComment = true
		}
	}
	if !hasLineComment {
		t.Error("expected line comment annotations")
	}
}
