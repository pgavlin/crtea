package ui

import (
	"testing"

	"github.com/pgavlin/crtea/model"
)

func TestComposeHunkSets_NoOverlap(t *testing.T) {
	// Commit A changes line 2, commit B changes line 10 — no overlap
	aHunks := []model.DiffHunk{{
		OldStart: 1, OldCount: 3, NewStart: 1, NewCount: 3,
		Header: "@@ -1,3 +1,3 @@",
		Lines: []model.DiffLine{
			{Origin: model.OriginContext, Content: "line1", OldLineNo: 1, NewLineNo: 1},
			{Origin: model.OriginDeletion, Content: "old2", OldLineNo: 2},
			{Origin: model.OriginAddition, Content: "new2", NewLineNo: 2},
			{Origin: model.OriginContext, Content: "line3", OldLineNo: 3, NewLineNo: 3},
		},
	}}
	bHunks := []model.DiffHunk{{
		OldStart: 9, OldCount: 3, NewStart: 9, NewCount: 3,
		Header: "@@ -9,3 +9,3 @@",
		Lines: []model.DiffLine{
			{Origin: model.OriginContext, Content: "line9", OldLineNo: 9, NewLineNo: 9},
			{Origin: model.OriginDeletion, Content: "old10", OldLineNo: 10},
			{Origin: model.OriginAddition, Content: "new10", NewLineNo: 10},
			{Origin: model.OriginContext, Content: "line11", OldLineNo: 11, NewLineNo: 11},
		},
	}}

	result := composeHunkSets(aHunks, bHunks)

	if len(result) != 2 {
		t.Fatalf("expected 2 hunks, got %d", len(result))
	}

	// First hunk should be A's change
	h := result[0]
	if h.OldStart != 1 {
		t.Errorf("hunk 0: expected OldStart=1, got %d", h.OldStart)
	}
	hasDel, hasAdd := false, false
	for _, l := range h.Lines {
		if l.Origin == model.OriginDeletion && l.Content == "old2" {
			hasDel = true
		}
		if l.Origin == model.OriginAddition && l.Content == "new2" {
			hasAdd = true
		}
	}
	if !hasDel || !hasAdd {
		t.Errorf("hunk 0: missing expected deletion/addition")
	}

	// Second hunk should be B's change
	h = result[1]
	hasDel, hasAdd = false, false
	for _, l := range h.Lines {
		if l.Origin == model.OriginDeletion && l.Content == "old10" {
			hasDel = true
		}
		if l.Origin == model.OriginAddition && l.Content == "new10" {
			hasAdd = true
		}
	}
	if !hasDel || !hasAdd {
		t.Errorf("hunk 1: missing expected deletion/addition")
	}
}

func TestComposeHunkSets_SameLineModified(t *testing.T) {
	// Commit A: line 5 "foo" → "bar"
	// Commit B: line 5 "bar" → "baz"
	// Combined should show: line 5 "foo" → "baz"
	aHunks := []model.DiffHunk{{
		OldStart: 4, OldCount: 3, NewStart: 4, NewCount: 3,
		Header: "@@ -4,3 +4,3 @@",
		Lines: []model.DiffLine{
			{Origin: model.OriginContext, Content: "ctx4", OldLineNo: 4, NewLineNo: 4},
			{Origin: model.OriginDeletion, Content: "foo", OldLineNo: 5},
			{Origin: model.OriginAddition, Content: "bar", NewLineNo: 5},
			{Origin: model.OriginContext, Content: "ctx6", OldLineNo: 6, NewLineNo: 6},
		},
	}}
	bHunks := []model.DiffHunk{{
		OldStart: 4, OldCount: 3, NewStart: 4, NewCount: 3,
		Header: "@@ -4,3 +4,3 @@",
		Lines: []model.DiffLine{
			{Origin: model.OriginContext, Content: "ctx4", OldLineNo: 4, NewLineNo: 4},
			{Origin: model.OriginDeletion, Content: "bar", OldLineNo: 5},
			{Origin: model.OriginAddition, Content: "baz", NewLineNo: 5},
			{Origin: model.OriginContext, Content: "ctx6", OldLineNo: 6, NewLineNo: 6},
		},
	}}

	result := composeHunkSets(aHunks, bHunks)

	if len(result) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(result))
	}

	h := result[0]
	var hasFooDel, hasBazAdd, hasBarDel, hasBarAdd bool
	for _, l := range h.Lines {
		if l.Origin == model.OriginDeletion && l.Content == "foo" {
			hasFooDel = true
		}
		if l.Origin == model.OriginAddition && l.Content == "baz" {
			hasBazAdd = true
		}
		if l.Origin == model.OriginDeletion && l.Content == "bar" {
			hasBarDel = true
		}
		if l.Origin == model.OriginAddition && l.Content == "bar" {
			hasBarAdd = true
		}
	}
	if !hasFooDel {
		t.Error("expected deletion of 'foo' (original)")
	}
	if !hasBazAdd {
		t.Error("expected addition of 'baz' (final)")
	}
	if hasBarDel {
		t.Error("should not have deletion of 'bar' (intermediate)")
	}
	if hasBarAdd {
		t.Error("should not have addition of 'bar' (intermediate)")
	}
}

func TestComposeHunkSets_AddThenDelete(t *testing.T) {
	// Commit A: adds a line after line 3
	// Commit B: deletes that same added line
	// Combined: no change
	aHunks := []model.DiffHunk{{
		OldStart: 3, OldCount: 2, NewStart: 3, NewCount: 3,
		Header: "@@ -3,2 +3,3 @@",
		Lines: []model.DiffLine{
			{Origin: model.OriginContext, Content: "line3", OldLineNo: 3, NewLineNo: 3},
			{Origin: model.OriginAddition, Content: "added", NewLineNo: 4},
			{Origin: model.OriginContext, Content: "line4", OldLineNo: 4, NewLineNo: 5},
		},
	}}
	bHunks := []model.DiffHunk{{
		OldStart: 3, OldCount: 3, NewStart: 3, NewCount: 2,
		Header: "@@ -3,3 +3,2 @@",
		Lines: []model.DiffLine{
			{Origin: model.OriginContext, Content: "line3", OldLineNo: 3, NewLineNo: 3},
			{Origin: model.OriginDeletion, Content: "added", OldLineNo: 4},
			{Origin: model.OriginContext, Content: "line4", OldLineNo: 5, NewLineNo: 4},
		},
	}}

	result := composeHunkSets(aHunks, bHunks)

	// The changes cancel out — all lines are context only
	for _, h := range result {
		for _, l := range h.Lines {
			if l.Origin != model.OriginContext {
				t.Errorf("expected only context lines, got %v: %q", l.Origin, l.Content)
			}
		}
	}
}

func TestMergeFileHunks_ThreeCommits(t *testing.T) {
	// Three commits each modify line 5
	// A: "original" → "v1"
	// B: "v1" → "v2"
	// C: "v2" → "v3"
	// Combined: "original" → "v3"
	hunks := [][]model.DiffHunk{
		{{ // Commit A
			OldStart: 5, OldCount: 1, NewStart: 5, NewCount: 1,
			Lines: []model.DiffLine{
				{Origin: model.OriginDeletion, Content: "original", OldLineNo: 5},
				{Origin: model.OriginAddition, Content: "v1", NewLineNo: 5},
			},
		}},
		{{ // Commit B
			OldStart: 5, OldCount: 1, NewStart: 5, NewCount: 1,
			Lines: []model.DiffLine{
				{Origin: model.OriginDeletion, Content: "v1", OldLineNo: 5},
				{Origin: model.OriginAddition, Content: "v2", NewLineNo: 5},
			},
		}},
		{{ // Commit C
			OldStart: 5, OldCount: 1, NewStart: 5, NewCount: 1,
			Lines: []model.DiffLine{
				{Origin: model.OriginDeletion, Content: "v2", OldLineNo: 5},
				{Origin: model.OriginAddition, Content: "v3", NewLineNo: 5},
			},
		}},
	}

	result := mergeFileHunks(hunks)

	if len(result) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(result))
	}

	var hasOrigDel, hasV3Add bool
	for _, l := range result[0].Lines {
		if l.Origin == model.OriginDeletion && l.Content == "original" {
			hasOrigDel = true
		}
		if l.Origin == model.OriginAddition && l.Content == "v3" {
			hasV3Add = true
		}
		// Intermediate values should not appear
		if l.Content == "v1" || l.Content == "v2" {
			t.Errorf("intermediate value %q should not appear in composed result", l.Content)
		}
	}
	if !hasOrigDel {
		t.Error("expected deletion of 'original'")
	}
	if !hasV3Add {
		t.Error("expected addition of 'v3'")
	}
}

func TestComposeHunkSets_EmptyInputs(t *testing.T) {
	hunks := []model.DiffHunk{{
		OldStart: 1, OldCount: 1, NewStart: 1, NewCount: 1,
		Lines: []model.DiffLine{
			{Origin: model.OriginDeletion, Content: "old", OldLineNo: 1},
			{Origin: model.OriginAddition, Content: "new", NewLineNo: 1},
		},
	}}

	// Empty A, non-empty B → returns B
	result := composeHunkSets(nil, hunks)
	if len(result) != 1 {
		t.Fatalf("expected 1 hunk with nil A, got %d", len(result))
	}

	// Non-empty A, empty B → returns A
	result = composeHunkSets(hunks, nil)
	if len(result) != 1 {
		t.Fatalf("expected 1 hunk with nil B, got %d", len(result))
	}
}
