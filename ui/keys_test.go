package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/pgavlin/crtea/internal/testutil"
	"github.com/pgavlin/crtea/model"
	"github.com/pgavlin/crtea/provider/mock"
	"github.com/pgavlin/crtea/theme"
	"github.com/pgavlin/crtea/vcs"
)

// newTestApp creates a minimal App with a diff file containing a few lines,
// suitable for testing key handling and mode transitions.
func newTestApp(t *testing.T) *App {
	t.Helper()

	files := []model.DiffFile{
		{
			OldPath: "a.go",
			NewPath: "a.go",
			Status:  model.FileModified,
			Hunks: []model.DiffHunk{
				{
					OldStart: 1, OldCount: 3,
					NewStart: 1, NewCount: 4,
					Header: "@@ -1,3 +1,4 @@",
					Lines: []model.DiffLine{
						{Origin: model.OriginContext, Content: "package main", OldLineNo: 1, NewLineNo: 1},
						{Origin: model.OriginDeletion, Content: "old line", OldLineNo: 2},
						{Origin: model.OriginAddition, Content: "new line", NewLineNo: 2},
						{Origin: model.OriginAddition, Content: "added line", NewLineNo: 3},
						{Origin: model.OriginContext, Content: "end", OldLineNo: 3, NewLineNo: 4},
					},
				},
			},
		},
	}

	session := model.NewSession("/tmp/test", "main", "abc123", model.DiffWorkingTree)
	for _, f := range files {
		session.GetOrCreateFileReview(f.DisplayPath(), f.Status)
	}

	app := NewApp(nil, files, session, theme.Dark(), nil, nil)
	app.SetSize(120, 40)
	return &app
}

// newTwoFileApp creates an App with two files for file/hunk navigation tests.
func newTwoFileApp(t *testing.T) *App {
	t.Helper()

	files := []model.DiffFile{
		{
			OldPath: "first.go",
			NewPath: "first.go",
			Status:  model.FileModified,
			Hunks: []model.DiffHunk{
				{
					OldStart: 1, OldCount: 3, NewStart: 1, NewCount: 3,
					Header: "@@ -1,3 +1,3 @@",
					Lines: []model.DiffLine{
						{Origin: model.OriginContext, Content: "package first", OldLineNo: 1, NewLineNo: 1},
						{Origin: model.OriginDeletion, Content: "old", OldLineNo: 2},
						{Origin: model.OriginAddition, Content: "new", NewLineNo: 2},
						{Origin: model.OriginContext, Content: "end", OldLineNo: 3, NewLineNo: 3},
					},
				},
				{
					OldStart: 10, OldCount: 2, NewStart: 10, NewCount: 2,
					Header: "@@ -10,2 +10,2 @@",
					Lines: []model.DiffLine{
						{Origin: model.OriginDeletion, Content: "old10", OldLineNo: 10},
						{Origin: model.OriginAddition, Content: "new10", NewLineNo: 10},
						{Origin: model.OriginContext, Content: "end10", OldLineNo: 11, NewLineNo: 11},
					},
				},
			},
		},
		{
			OldPath: "second.go",
			NewPath: "second.go",
			Status:  model.FileAdded,
			Hunks: []model.DiffHunk{
				{
					OldStart: 0, OldCount: 0, NewStart: 1, NewCount: 2,
					Header: "@@ -0,0 +1,2 @@",
					Lines: []model.DiffLine{
						{Origin: model.OriginAddition, Content: "package second", NewLineNo: 1},
						{Origin: model.OriginAddition, Content: "var y = 1", NewLineNo: 2},
					},
				},
			},
		},
	}

	session := model.NewSession("/tmp/test", "main", "abc123", model.DiffWorkingTree)
	for _, f := range files {
		session.GetOrCreateFileReview(f.DisplayPath(), f.Status)
	}

	app := NewApp(nil, files, session, theme.Dark(), nil, nil)
	app.SetSize(120, 40)
	return &app
}

// key helpers

func keyPress(char rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: char, Text: string(char)})
}

func keySpecial(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}

func keyMod(char rune, mod tea.KeyMod) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: char, Mod: mod})
}

func upperKey(lower, upper rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: lower, Text: string(upper), ShiftedCode: upper, Mod: tea.ModShift})
}

// sendKeys sends a sequence of key messages through the app, returning the final state.
func sendKeys(app *App, keys ...tea.KeyPressMsg) *App {
	var m tea.Model = app
	for _, k := range keys {
		m, _ = m.Update(k)
	}
	return m.(*App)
}

// --- Test harness validation ---

func TestHarnessInitialState(t *testing.T) {
	app := newTestApp(t)

	if app.inputMode != modeNormal {
		t.Errorf("initial mode = %d, want modeNormal", app.inputMode)
	}
	if app.focusedPanel != panelDiff {
		t.Errorf("initial panel = %d, want panelDiff", app.focusedPanel)
	}
	if len(app.annotations) == 0 {
		t.Fatal("expected annotations to be built")
	}
	if app.session == nil {
		t.Fatal("expected session to be set")
	}
}

// --- Mode transitions ---

func TestModeNormalToCommandAndBack(t *testing.T) {
	app := newTestApp(t)

	// : enters command mode
	app = sendKeys(app, keyPress(':'))
	if app.inputMode != modeCommand {
		t.Fatalf("after ':', mode = %d, want modeCommand", app.inputMode)
	}

	// Escape returns to normal
	app = sendKeys(app, keySpecial(tea.KeyEscape))
	if app.inputMode != modeNormal {
		t.Fatalf("after Escape, mode = %d, want modeNormal", app.inputMode)
	}
}

func TestModeNormalToSearchAndBack(t *testing.T) {
	app := newTestApp(t)

	// / enters search mode
	app = sendKeys(app, keyPress('/'))
	if app.inputMode != modeSearch {
		t.Fatalf("after '/', mode = %d, want modeSearch", app.inputMode)
	}

	// Escape returns to normal
	app = sendKeys(app, keySpecial(tea.KeyEscape))
	if app.inputMode != modeNormal {
		t.Fatalf("after Escape, mode = %d, want modeNormal", app.inputMode)
	}
}

func TestModeNormalToHelpAndBack(t *testing.T) {
	app := newTestApp(t)

	// ? enters help mode
	app = sendKeys(app, keyPress('?'))
	if app.inputMode != modeHelp {
		t.Fatalf("after '?', mode = %d, want modeHelp", app.inputMode)
	}

	// ? again returns to normal
	app = sendKeys(app, keyPress('?'))
	if app.inputMode != modeNormal {
		t.Fatalf("after second '?', mode = %d, want modeNormal", app.inputMode)
	}
}

func TestModeHelpEscapeReturns(t *testing.T) {
	app := newTestApp(t)
	app = sendKeys(app, keyPress('?'))
	app = sendKeys(app, keySpecial(tea.KeyEscape))
	if app.inputMode != modeNormal {
		t.Fatalf("after Escape from help, mode = %d, want modeNormal", app.inputMode)
	}
}

func TestModeNormalToVisualAndBack(t *testing.T) {
	app := newTestApp(t)

	// Move cursor to a diff line (skip file header and hunk header)
	for app.cursorLine < len(app.annotations) && app.annotations[app.cursorLine].Type != annDiffLine {
		app = sendKeys(app, keyPress('j'))
	}
	if app.cursorLine >= len(app.annotations) {
		t.Fatal("could not find a diff line")
	}

	// v enters visual mode
	app = sendKeys(app, keyPress('v'))
	if app.inputMode != modeVisualSelect {
		t.Fatalf("after 'v', mode = %d, want modeVisualSelect", app.inputMode)
	}

	// Escape returns to normal
	app = sendKeys(app, keySpecial(tea.KeyEscape))
	if app.inputMode != modeNormal {
		t.Fatalf("after Escape from visual, mode = %d, want modeNormal", app.inputMode)
	}
}

func TestModeVisualToComment(t *testing.T) {
	app := newTestApp(t)

	// Move to a diff line
	for app.cursorLine < len(app.annotations) && app.annotations[app.cursorLine].Type != annDiffLine {
		app = sendKeys(app, keyPress('j'))
	}

	// Enter visual mode, move down, then 'c' to enter comment
	app = sendKeys(app, keyPress('v'))
	app = sendKeys(app, keyPress('j'))
	app = sendKeys(app, keyPress('c'))
	if app.inputMode != modeComment {
		t.Fatalf("after 'c' in visual, mode = %d, want modeComment", app.inputMode)
	}
	if app.commentLineRange == nil {
		t.Fatal("expected commentLineRange to be set for visual comment")
	}
}

func TestModeNormalToLineComment(t *testing.T) {
	app := newTestApp(t)

	// Move to a diff line
	for app.cursorLine < len(app.annotations) && app.annotations[app.cursorLine].Type != annDiffLine {
		app = sendKeys(app, keyPress('j'))
	}

	// 'c' enters comment mode on a diff line
	app = sendKeys(app, keyPress('c'))
	if app.inputMode != modeComment {
		t.Fatalf("after 'c', mode = %d, want modeComment", app.inputMode)
	}
	if app.commentIsFile {
		t.Fatal("expected line comment, not file comment")
	}
}

func TestModeNormalToFileComment(t *testing.T) {
	app := newTestApp(t)

	// 'C' enters file comment mode
	app = sendKeys(app, upperKey('c', 'C'))
	if app.inputMode != modeComment {
		t.Fatalf("after 'C', mode = %d, want modeComment", app.inputMode)
	}
	if !app.commentIsFile {
		t.Fatal("expected file comment")
	}
}

func TestModeNormalToReview(t *testing.T) {
	app := newTestApp(t)

	// 'R' enters review mode
	app = sendKeys(app, upperKey('r', 'R'))
	if app.inputMode != modeReview {
		t.Fatalf("after 'R', mode = %d, want modeReview", app.inputMode)
	}

	// Escape returns to normal
	app = sendKeys(app, keySpecial(tea.KeyEscape))
	if app.inputMode != modeNormal {
		t.Fatalf("after Escape from review, mode = %d, want modeNormal", app.inputMode)
	}
}

func TestModeCommandEnterExecutesAndReturns(t *testing.T) {
	app := newTestApp(t)

	// Type ":review" which enters review mode
	app = sendKeys(app, keyPress(':'))
	app = sendKeys(app, keyPress('r'), keyPress('e'), keyPress('v'), keyPress('i'), keyPress('e'), keyPress('w'))
	if app.commandBuffer != "review" {
		t.Fatalf("commandBuffer = %q, want %q", app.commandBuffer, "review")
	}
	// Enter executes command
	app = sendKeys(app, keySpecial(tea.KeyEnter))
	if app.inputMode != modeReview {
		t.Fatalf("after :review, mode = %d, want modeReview", app.inputMode)
	}
}

func TestModeSearchEnterExecutesAndReturns(t *testing.T) {
	app := newTestApp(t)

	// /main Enter
	app = sendKeys(app, keyPress('/'))
	app = sendKeys(app, keyPress('m'), keyPress('a'), keyPress('i'), keyPress('n'))
	if app.searchBuffer != "main" {
		t.Fatalf("searchBuffer = %q, want %q", app.searchBuffer, "main")
	}
	app = sendKeys(app, keySpecial(tea.KeyEnter))
	if app.inputMode != modeNormal {
		t.Fatalf("after search Enter, mode = %d, want modeNormal", app.inputMode)
	}
	if app.lastSearch != "main" {
		t.Fatalf("lastSearch = %q, want %q", app.lastSearch, "main")
	}
	if app.searchHighlight != "main" {
		t.Fatalf("searchHighlight = %q, want %q", app.searchHighlight, "main")
	}
}

func TestEscapeClearsSearchHighlight(t *testing.T) {
	app := newTestApp(t)

	// Search for something
	app = sendKeys(app, keyPress('/'))
	app = sendKeys(app, keyPress('x'))
	app = sendKeys(app, keySpecial(tea.KeyEnter))
	if app.searchHighlight != "x" {
		t.Fatalf("searchHighlight = %q, want %q", app.searchHighlight, "x")
	}

	// Escape clears highlight
	app = sendKeys(app, keySpecial(tea.KeyEscape))
	if app.searchHighlight != "" {
		t.Fatalf("after Escape, searchHighlight = %q, want empty", app.searchHighlight)
	}
}

func TestCommandBackspaceEmptyExitsMode(t *testing.T) {
	app := newTestApp(t)

	app = sendKeys(app, keyPress(':'))
	// Backspace on empty buffer exits command mode
	app = sendKeys(app, keySpecial(tea.KeyBackspace))
	if app.inputMode != modeNormal {
		t.Fatalf("after backspace on empty command, mode = %d, want modeNormal", app.inputMode)
	}
}

// --- :clear filtering ---

func TestClearRemovesOnlyDraftComments(t *testing.T) {
	app := newTestApp(t)
	path := app.diffFiles[0].DisplayPath()
	fr := app.session.GetOrCreateFileReview(path, model.FileModified)

	// Add draft file comment
	draft1 := model.NewComment("draft file comment", model.CommentNote, model.SideNew)
	fr.AddFileComment(draft1)

	// Add submitted file comment (should survive)
	submitted := model.NewComment("submitted", model.CommentNote, model.SideNew)
	submitted.Submitted = true
	submitted.ExternalID = "ext-1"
	fr.AddFileComment(submitted)

	// Add remote file comment (should survive)
	remote := model.NewComment("remote", model.CommentNote, model.SideNew)
	remote.ExternalID = "ext-2"
	fr.AddFileComment(remote)

	// Add draft line comment
	draft2 := model.NewComment("draft line comment", model.CommentNote, model.SideNew)
	fr.AddLineComment(2, draft2)

	// Add submitted line comment (should survive)
	submittedLine := model.NewComment("submitted line", model.CommentNote, model.SideNew)
	submittedLine.Submitted = true
	submittedLine.ExternalID = "ext-3"
	fr.AddLineComment(2, submittedLine)

	// Add draft overall review
	app.session.OverallReview = &model.OverallReview{
		Body:   "my review",
		Status: model.ApprovalApprove,
	}

	app.rebuildAnnotations()

	// Execute :clear and confirm with 'y'
	app = sendKeys(app, keyPress(':'))
	app = sendKeys(app, keyPress('c'), keyPress('l'), keyPress('e'), keyPress('a'), keyPress('r'))
	app = sendKeys(app, keySpecial(tea.KeyEnter))

	if app.inputMode != modeConfirm {
		t.Fatalf("expected modeConfirm, got %d", app.inputMode)
	}
	if app.confirmPrompt != "Clear draft comments?" {
		t.Fatalf("confirmPrompt = %q, want %q", app.confirmPrompt, "Clear draft comments?")
	}

	app = sendKeys(app, keyPress('y'))

	// Verify drafts removed
	fr = app.session.GetFileReview(path)

	// File comments: should keep submitted + remote (2), remove draft (1)
	if len(fr.FileComments) != 2 {
		t.Fatalf("FileComments count = %d, want 2", len(fr.FileComments))
	}
	for _, c := range fr.FileComments {
		if c.ExternalID == "" && !c.Submitted {
			t.Errorf("found draft file comment that should have been cleared: %q", c.Content)
		}
	}

	// Line comments: should keep submitted (1), remove draft (1)
	lineComments := fr.LineComments[2]
	if len(lineComments) != 1 {
		t.Fatalf("LineComments[2] count = %d, want 1", len(lineComments))
	}
	if lineComments[0].ExternalID != "ext-3" {
		t.Errorf("surviving line comment ExternalID = %q, want %q", lineComments[0].ExternalID, "ext-3")
	}

	// Overall review should be cleared (was draft)
	if app.session.OverallReview != nil {
		t.Error("expected OverallReview to be nil after clearing draft")
	}

	// Check message
	if app.message == nil || app.message.text != "Cleared 3 draft comment(s)" {
		var msg string
		if app.message != nil {
			msg = app.message.text
		}
		t.Errorf("message = %q, want %q", msg, "Cleared 3 draft comment(s)")
	}
}

func TestClearPreservesRemoteOverallReview(t *testing.T) {
	app := newTestApp(t)

	// Add a remote overall review (has ExternalID)
	app.session.OverallReview = &model.OverallReview{
		Body:       "remote review",
		Status:     model.ApprovalApprove,
		ExternalID: "ext-review-1",
	}

	// :clear should report no drafts
	app = sendKeys(app, keyPress(':'))
	app = sendKeys(app, keyPress('c'), keyPress('l'), keyPress('e'), keyPress('a'), keyPress('r'))
	app = sendKeys(app, keySpecial(tea.KeyEnter))

	// Should NOT enter confirm mode — no drafts to clear
	if app.inputMode != modeNormal {
		t.Fatalf("expected modeNormal (no drafts), got %d", app.inputMode)
	}
	if app.message == nil || app.message.text != "No draft comments to clear" {
		var msg string
		if app.message != nil {
			msg = app.message.text
		}
		t.Errorf("message = %q, want %q", msg, "No draft comments to clear")
	}

	// Overall review should still be there
	if app.session.OverallReview == nil {
		t.Fatal("remote OverallReview should not have been cleared")
	}
}

func TestClearNoDraftsSkipsConfirm(t *testing.T) {
	app := newTestApp(t)

	// No comments at all
	app = sendKeys(app, keyPress(':'))
	app = sendKeys(app, keyPress('c'), keyPress('l'), keyPress('e'), keyPress('a'), keyPress('r'))
	app = sendKeys(app, keySpecial(tea.KeyEnter))

	if app.inputMode != modeNormal {
		t.Fatalf("expected modeNormal, got %d", app.inputMode)
	}
	if app.message == nil || app.message.text != "No draft comments to clear" {
		var msg string
		if app.message != nil {
			msg = app.message.text
		}
		t.Errorf("message = %q, want %q", msg, "No draft comments to clear")
	}
}

func TestClearConfirmDeny(t *testing.T) {
	app := newTestApp(t)
	path := app.diffFiles[0].DisplayPath()
	fr := app.session.GetOrCreateFileReview(path, model.FileModified)

	draft := model.NewComment("draft", model.CommentNote, model.SideNew)
	fr.AddFileComment(draft)
	app.rebuildAnnotations()

	// :clear then deny with 'n'
	app = sendKeys(app, keyPress(':'))
	app = sendKeys(app, keyPress('c'), keyPress('l'), keyPress('e'), keyPress('a'), keyPress('r'))
	app = sendKeys(app, keySpecial(tea.KeyEnter))
	app = sendKeys(app, keyPress('n'))

	// Should return to normal without clearing
	if app.inputMode != modeNormal {
		t.Fatalf("expected modeNormal after deny, got %d", app.inputMode)
	}

	// Draft comment should still be there
	fr = app.session.GetFileReview(path)
	if len(fr.FileComments) != 1 {
		t.Fatalf("FileComments count = %d, want 1 (deny should preserve)", len(fr.FileComments))
	}
}

func TestClearRemovesEmptyLineCommentEntries(t *testing.T) {
	app := newTestApp(t)
	path := app.diffFiles[0].DisplayPath()
	fr := app.session.GetOrCreateFileReview(path, model.FileModified)

	// Add a draft line comment as the only comment on line 3
	draft := model.NewComment("draft", model.CommentNote, model.SideNew)
	fr.AddLineComment(3, draft)
	app.rebuildAnnotations()

	// :clear and confirm
	app = sendKeys(app, keyPress(':'))
	app = sendKeys(app, keyPress('c'), keyPress('l'), keyPress('e'), keyPress('a'), keyPress('r'))
	app = sendKeys(app, keySpecial(tea.KeyEnter))
	app = sendKeys(app, keyPress('y'))

	// Line 3 entry should be removed from the map entirely
	fr = app.session.GetFileReview(path)
	if _, ok := fr.LineComments[3]; ok {
		t.Error("expected LineComments[3] to be deleted, not just empty")
	}
}

// --- Navigation ---

func TestNavigationJK(t *testing.T) {
	app := newTestApp(t)
	start := app.cursorLine

	app = sendKeys(app, keyPress('j'))
	if app.cursorLine != start+1 {
		t.Fatalf("after 'j', cursorLine = %d, want %d", app.cursorLine, start+1)
	}

	app = sendKeys(app, keyPress('k'))
	if app.cursorLine != start {
		t.Fatalf("after 'k', cursorLine = %d, want %d", app.cursorLine, start)
	}
}

func TestNavigationGAndShiftG(t *testing.T) {
	app := newTestApp(t)

	// G goes to last
	app = sendKeys(app, upperKey('g', 'G'))
	if app.cursorLine != len(app.annotations)-1 {
		t.Fatalf("after 'G', cursorLine = %d, want %d", app.cursorLine, len(app.annotations)-1)
	}

	// g (lowercase) goes to first
	app = sendKeys(app, keyPress('g'))
	if app.cursorLine != 0 {
		t.Fatalf("after 'g', cursorLine = %d, want 0", app.cursorLine)
	}
}

func TestHorizontalScroll(t *testing.T) {
	app := newTestApp(t)

	app = sendKeys(app, keyPress('l'))
	if app.scrollX != 1 {
		t.Fatalf("after 'l', scrollX = %d, want 1", app.scrollX)
	}

	app = sendKeys(app, keyPress('h'))
	if app.scrollX != 0 {
		t.Fatalf("after 'h', scrollX = %d, want 0", app.scrollX)
	}
}

func TestZeroResetsHorizontalScroll(t *testing.T) {
	app := newTestApp(t)

	// Scroll right a few times
	app = sendKeys(app, keyPress('l'), keyPress('l'), keyPress('l'))
	if app.scrollX != 3 {
		t.Fatalf("scrollX = %d, want 3", app.scrollX)
	}

	// 0 resets
	app = sendKeys(app, keyPress('0'))
	if app.scrollX != 0 {
		t.Fatalf("after '0', scrollX = %d, want 0", app.scrollX)
	}
}

func TestToggleReviewed(t *testing.T) {
	app := newTestApp(t)
	path := app.diffFiles[0].DisplayPath()

	fr := app.session.GetFileReview(path)
	if fr.Reviewed {
		t.Fatal("file should not be reviewed initially")
	}

	app = sendKeys(app, keyPress('r'))
	fr = app.session.GetFileReview(path)
	if !fr.Reviewed {
		t.Fatal("file should be reviewed after 'r'")
	}

	app = sendKeys(app, keyPress('r'))
	fr = app.session.GetFileReview(path)
	if fr.Reviewed {
		t.Fatal("file should be un-reviewed after second 'r'")
	}
}

// --- Comment creation via key sequence ---

func TestCommentSaveAndClear(t *testing.T) {
	app := newTestApp(t)
	path := app.diffFiles[0].DisplayPath()

	// Move to a diff line
	for app.cursorLine < len(app.annotations) && app.annotations[app.cursorLine].Type != annDiffLine {
		app = sendKeys(app, keyPress('j'))
	}

	// Enter comment mode, type text, save
	app = sendKeys(app, keyPress('c'))
	if app.inputMode != modeComment {
		t.Fatalf("expected modeComment, got %d", app.inputMode)
	}

	app = sendKeys(app, keyPress('h'), keyPress('i'))
	app = sendKeys(app, keySpecial(tea.KeyEnter)) // save

	if app.inputMode != modeNormal {
		t.Fatalf("after save, mode = %d, want modeNormal", app.inputMode)
	}

	fr := app.session.GetFileReview(path)
	totalComments := 0
	for _, comments := range fr.LineComments {
		totalComments += len(comments)
	}
	if totalComments != 1 {
		t.Fatalf("expected 1 line comment, got %d", totalComments)
	}

	// Now :clear should remove it
	app = sendKeys(app, keyPress(':'))
	app = sendKeys(app, keyPress('c'), keyPress('l'), keyPress('e'), keyPress('a'), keyPress('r'))
	app = sendKeys(app, keySpecial(tea.KeyEnter))
	app = sendKeys(app, keyPress('y'))

	fr = app.session.GetFileReview(path)
	totalComments = 0
	for _, comments := range fr.LineComments {
		totalComments += len(comments)
	}
	if totalComments != 0 {
		t.Fatalf("expected 0 line comments after clear, got %d", totalComments)
	}
}

// --- Overall review ---

func TestOverallReviewSaveAndClear(t *testing.T) {
	app := newTestApp(t)

	// R to enter review mode
	app = sendKeys(app, upperKey('r', 'R'))
	if app.inputMode != modeReview {
		t.Fatalf("expected modeReview, got %d", app.inputMode)
	}

	// Type review text
	app = sendKeys(app, keyPress('l'), keyPress('g'), keyPress('t'), keyPress('m'))

	// Tab to cycle approval status
	app = sendKeys(app, keySpecial(tea.KeyTab))
	if app.reviewStatus != model.ApprovalApprove {
		t.Fatalf("after Tab, reviewStatus = %d, want ApprovalApprove", app.reviewStatus)
	}

	// Enter to save
	app = sendKeys(app, keySpecial(tea.KeyEnter))
	if app.inputMode != modeNormal {
		t.Fatalf("after save, mode = %d, want modeNormal", app.inputMode)
	}

	if app.session.OverallReview == nil {
		t.Fatal("expected OverallReview to be set")
	}
	if app.session.OverallReview.Body != "lgtm" {
		t.Fatalf("OverallReview.Body = %q, want %q", app.session.OverallReview.Body, "lgtm")
	}
	if app.session.OverallReview.Status != model.ApprovalApprove {
		t.Fatalf("OverallReview.Status = %d, want ApprovalApprove", app.session.OverallReview.Status)
	}

	// :clear should remove the draft review
	app = sendKeys(app, keyPress(':'))
	app = sendKeys(app, keyPress('c'), keyPress('l'), keyPress('e'), keyPress('a'), keyPress('r'))
	app = sendKeys(app, keySpecial(tea.KeyEnter))
	app = sendKeys(app, keyPress('y'))

	if app.session.OverallReview != nil {
		t.Fatal("expected OverallReview to be nil after clear")
	}
}

// --- Confirm dialog ---

func TestConfirmEscapeCancels(t *testing.T) {
	app := newTestApp(t)
	path := app.diffFiles[0].DisplayPath()
	fr := app.session.GetOrCreateFileReview(path, model.FileModified)
	fr.AddFileComment(model.NewComment("draft", model.CommentNote, model.SideNew))
	app.rebuildAnnotations()

	// :clear
	app = sendKeys(app, keyPress(':'))
	app = sendKeys(app, keyPress('c'), keyPress('l'), keyPress('e'), keyPress('a'), keyPress('r'))
	app = sendKeys(app, keySpecial(tea.KeyEnter))

	if app.inputMode != modeConfirm {
		t.Fatalf("expected modeConfirm, got %d", app.inputMode)
	}

	// Escape cancels
	app = sendKeys(app, keySpecial(tea.KeyEscape))
	if app.inputMode != modeNormal {
		t.Fatalf("after Escape, mode = %d, want modeNormal", app.inputMode)
	}

	// Comment preserved
	fr = app.session.GetFileReview(path)
	if len(fr.FileComments) != 1 {
		t.Fatalf("FileComments count = %d, want 1", len(fr.FileComments))
	}
}

// --- Dirty flag ---

func TestClearSetsDirtyFlag(t *testing.T) {
	app := newTestApp(t)
	path := app.diffFiles[0].DisplayPath()
	fr := app.session.GetOrCreateFileReview(path, model.FileModified)
	fr.AddFileComment(model.NewComment("draft", model.CommentNote, model.SideNew))
	app.rebuildAnnotations()
	app.dirty = false

	app = sendKeys(app, keyPress(':'))
	app = sendKeys(app, keyPress('c'), keyPress('l'), keyPress('e'), keyPress('a'), keyPress('r'))
	app = sendKeys(app, keySpecial(tea.KeyEnter))
	app = sendKeys(app, keyPress('y'))

	if !app.dirty {
		t.Fatal("expected dirty flag to be set after clearing comments")
	}
}

// --- File/hunk navigation ---

func TestFileNavigation(t *testing.T) {
	app := newTwoFileApp(t)

	// Initially on first file
	if app.currentFileIdx() != 0 {
		t.Fatalf("initial file = %d, want 0", app.currentFileIdx())
	}

	// } jumps to next file
	app = sendKeys(app, keyPress('}'))
	if app.currentFileIdx() != 1 {
		t.Fatalf("after }, file = %d, want 1", app.currentFileIdx())
	}

	// { jumps back to first file
	app = sendKeys(app, keyPress('{'))
	if app.currentFileIdx() != 0 {
		t.Fatalf("after {, file = %d, want 0", app.currentFileIdx())
	}
}

func TestHunkNavigation(t *testing.T) {
	app := newTwoFileApp(t)

	// ] jumps to next hunk
	app = sendKeys(app, keyPress(']'))
	ann := app.annotations[app.cursorLine]
	if ann.Type != annHunkHeader {
		t.Fatalf("after ], expected annHunkHeader, got %d", ann.Type)
	}
	firstHunk := app.cursorLine

	// ] again jumps to next hunk
	app = sendKeys(app, keyPress(']'))
	if app.cursorLine <= firstHunk {
		t.Fatal("second ] should advance cursor")
	}
	ann = app.annotations[app.cursorLine]
	if ann.Type != annHunkHeader {
		t.Fatalf("after second ], expected annHunkHeader, got %d", ann.Type)
	}
	secondHunk := app.cursorLine

	// [ goes back from secondHunk
	app = sendKeys(app, keyPress('['))
	if app.cursorLine >= secondHunk {
		t.Fatalf("after [, cursor should go back from %d, got %d", secondHunk, app.cursorLine)
	}
}

// --- Pending key sequences ---

func TestPendingDD(t *testing.T) {
	app := newTestApp(t)
	path := app.diffFiles[0].DisplayPath()
	fr := app.session.GetOrCreateFileReview(path, model.FileModified)

	// Move to diff line and add a comment
	for app.cursorLine < len(app.annotations) && app.annotations[app.cursorLine].Type != annDiffLine {
		app = sendKeys(app, keyPress('j'))
	}
	app = sendKeys(app, keyPress('c'))
	app = sendKeys(app, keyPress('h'), keyPress('i'))
	app = sendKeys(app, keySpecial(tea.KeyEnter))

	// Verify comment exists
	total := 0
	for _, c := range fr.LineComments {
		total += len(c)
	}
	if total != 1 {
		t.Fatalf("expected 1 comment, got %d", total)
	}

	// Navigate to the comment annotation
	found := false
	for i, ann := range app.annotations {
		if ann.Type == annLineComment {
			app.cursorLine = i
			found = true
			break
		}
	}
	if !found {
		t.Fatal("could not find comment annotation")
	}

	// dd deletes comment
	app = sendKeys(app, keyPress('d'), keyPress('d'))

	fr = app.session.GetFileReview(path)
	total = 0
	for _, c := range fr.LineComments {
		total += len(c)
	}
	if total != 0 {
		t.Fatalf("after dd, expected 0 comments, got %d", total)
	}
	if app.message == nil || app.message.text != "Comment deleted" {
		t.Error("expected 'Comment deleted' message")
	}
}

func TestPendingDDOnNonComment(t *testing.T) {
	app := newTestApp(t)

	// dd on a non-comment line should show warning
	app = sendKeys(app, keyPress('d'), keyPress('d'))
	if app.message == nil || app.message.text != "Move cursor to a comment to delete" {
		var msg string
		if app.message != nil {
			msg = app.message.text
		}
		t.Errorf("expected delete warning, got %q", msg)
	}
}

func TestPendingZZ(t *testing.T) {
	app := newTestApp(t)

	// Move down several lines
	for i := 0; i < 5; i++ {
		app = sendKeys(app, keyPress('j'))
	}
	cursorBefore := app.cursorLine
	scrollBefore := app.scrollOffset

	// zz centers cursor
	app = sendKeys(app, keyPress('z'), keyPress('z'))

	// Cursor should stay the same, scroll may change
	if app.cursorLine != cursorBefore {
		t.Fatalf("zz should not change cursorLine, was %d now %d", cursorBefore, app.cursorLine)
	}
	_ = scrollBefore // scroll offset may or may not change depending on viewport
}

func TestPendingSemicolonE(t *testing.T) {
	app := newTestApp(t)
	if !app.showFileList {
		t.Fatal("file list should be shown initially")
	}

	// ;e toggles file list
	app = sendKeys(app, keyPress(';'), keyPress('e'))
	if app.showFileList {
		t.Fatal("file list should be hidden after ;e")
	}

	app = sendKeys(app, keyPress(';'), keyPress('e'))
	if !app.showFileList {
		t.Fatal("file list should be shown after second ;e")
	}
}

func TestPendingSemicolonHL(t *testing.T) {
	app := newTestApp(t)

	// ;h focuses file list
	app = sendKeys(app, keyPress(';'), keyPress('h'))
	if app.focusedPanel != panelFileList {
		t.Fatalf("after ;h, focusedPanel = %d, want panelFileList", app.focusedPanel)
	}

	// ;l focuses diff
	app = sendKeys(app, keyPress(';'), keyPress('l'))
	if app.focusedPanel != panelDiff {
		t.Fatalf("after ;l, focusedPanel = %d, want panelDiff", app.focusedPanel)
	}
}

func TestPendingSemicolonNP(t *testing.T) {
	app := newTwoFileApp(t)

	// ;n goes to next unreviewed file
	app = sendKeys(app, keyPress(';'), keyPress('n'))
	if app.currentFileIdx() != 1 {
		t.Fatalf("after ;n, file = %d, want 1", app.currentFileIdx())
	}

	// ;p goes to prev unreviewed file
	app = sendKeys(app, keyPress(';'), keyPress('p'))
	if app.currentFileIdx() != 0 {
		t.Fatalf("after ;p, file = %d, want 0", app.currentFileIdx())
	}
}

func TestPendingSemicolonNAllReviewed(t *testing.T) {
	app := newTwoFileApp(t)

	// Mark all files as reviewed
	for _, fr := range app.session.Files {
		fr.Reviewed = true
	}

	app = sendKeys(app, keyPress(';'), keyPress('n'))
	if app.message == nil || app.message.text != "All files reviewed" {
		var msg string
		if app.message != nil {
			msg = app.message.text
		}
		t.Errorf("expected 'All files reviewed' message, got %q", msg)
	}
}

func TestDigitAccumulatorG(t *testing.T) {
	app := newTestApp(t)

	// Type "2G" to jump to source line 2
	app = sendKeys(app, keyPress('2'), upperKey('g', 'G'))

	ann := app.annotations[app.cursorLine]
	if ann.NewLineNo != 2 {
		t.Fatalf("after 2G, expected NewLineNo=2, got %d", ann.NewLineNo)
	}
}

// --- Tab panel cycling ---

func TestTabCyclesPanels(t *testing.T) {
	app := newTestApp(t)
	if app.focusedPanel != panelDiff {
		t.Fatalf("initial panel = %d, want panelDiff", app.focusedPanel)
	}

	// Tab: diff -> file list
	app = sendKeys(app, keySpecial(tea.KeyTab))
	if app.focusedPanel != panelFileList {
		t.Fatalf("after Tab, panel = %d, want panelFileList", app.focusedPanel)
	}

	// Tab: file list -> diff (no commit list or conversation)
	app = sendKeys(app, keySpecial(tea.KeyTab))
	if app.focusedPanel != panelDiff {
		t.Fatalf("after second Tab, panel = %d, want panelDiff", app.focusedPanel)
	}
}

func TestTabSkipsFileListWhenHidden(t *testing.T) {
	app := newTestApp(t)
	app.showFileList = false

	// Tab should cycle back to diff since file list is hidden
	app = sendKeys(app, keySpecial(tea.KeyTab))
	if app.focusedPanel != panelDiff {
		t.Fatalf("with file list hidden, Tab should stay on panelDiff, got %d", app.focusedPanel)
	}
}

// --- Half/full page navigation ---

func TestCtrlDU(t *testing.T) {
	app := newTwoFileApp(t)

	// Ctrl-d moves down half page
	start := app.cursorLine
	app = sendKeys(app, keyMod('d', tea.ModCtrl))
	if app.cursorLine <= start {
		t.Fatalf("Ctrl-d should move cursor down, was %d now %d", start, app.cursorLine)
	}

	// Ctrl-u moves back up
	mid := app.cursorLine
	app = sendKeys(app, keyMod('u', tea.ModCtrl))
	if app.cursorLine >= mid {
		t.Fatalf("Ctrl-u should move cursor up, was %d now %d", mid, app.cursorLine)
	}
}

func TestCtrlFB(t *testing.T) {
	app := newTwoFileApp(t)

	start := app.cursorLine
	app = sendKeys(app, keyMod('f', tea.ModCtrl))
	if app.cursorLine <= start {
		t.Fatalf("Ctrl-f should move cursor down, was %d now %d", start, app.cursorLine)
	}

	mid := app.cursorLine
	app = sendKeys(app, keyMod('b', tea.ModCtrl))
	if app.cursorLine >= mid {
		t.Fatalf("Ctrl-b should move cursor up, was %d now %d", mid, app.cursorLine)
	}
}

// --- Edit comment ---

func TestEditComment(t *testing.T) {
	app := newTestApp(t)
	path := app.diffFiles[0].DisplayPath()
	fr := app.session.GetOrCreateFileReview(path, model.FileModified)

	// Add a file comment
	c := model.NewComment("original text", model.CommentNote, model.SideNew)
	fr.AddFileComment(c)
	app.rebuildAnnotations()

	// Find the comment annotation
	for i, ann := range app.annotations {
		if ann.Type == annFileComment {
			app.cursorLine = i
			break
		}
	}

	// 'i' enters edit mode
	app = sendKeys(app, keyPress('i'))
	if app.inputMode != modeComment {
		t.Fatalf("expected modeComment, got %d", app.inputMode)
	}
	if app.editingID == "" {
		t.Fatal("editingID should be set when editing")
	}
	if app.commentBuffer != "original text" {
		t.Fatalf("commentBuffer = %q, want %q", app.commentBuffer, "original text")
	}

	// Clear and type new text
	app.commentBuffer = "updated text"
	app.commentCursor = len("updated text")
	app = sendKeys(app, keySpecial(tea.KeyEnter))

	fr = app.session.GetFileReview(path)
	if len(fr.FileComments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(fr.FileComments))
	}
	if fr.FileComments[0].Content != "updated text" {
		t.Fatalf("comment content = %q, want %q", fr.FileComments[0].Content, "updated text")
	}
}

func TestEditOthersCommentBlocked(t *testing.T) {
	app := newTestApp(t)
	app.session.Reviewer = "me"
	path := app.diffFiles[0].DisplayPath()
	fr := app.session.GetOrCreateFileReview(path, model.FileModified)

	c := model.NewComment("their comment", model.CommentNote, model.SideNew)
	c.Author = "someone_else"
	fr.AddFileComment(c)
	app.rebuildAnnotations()

	for i, ann := range app.annotations {
		if ann.Type == annFileComment {
			app.cursorLine = i
			break
		}
	}

	app = sendKeys(app, keyPress('i'))
	if app.inputMode != modeNormal {
		t.Fatal("should not enter edit mode for others' comments")
	}
	if app.message == nil || app.message.text != "Cannot edit others' comments" {
		t.Error("expected warning about editing others' comments")
	}
}

// --- Delete file comment with dd ---

func TestDeleteFileComment(t *testing.T) {
	app := newTestApp(t)
	path := app.diffFiles[0].DisplayPath()
	fr := app.session.GetOrCreateFileReview(path, model.FileModified)
	fr.AddFileComment(model.NewComment("to delete", model.CommentNote, model.SideNew))
	app.rebuildAnnotations()

	for i, ann := range app.annotations {
		if ann.Type == annFileComment {
			app.cursorLine = i
			break
		}
	}

	app = sendKeys(app, keyPress('d'), keyPress('d'))
	fr = app.session.GetFileReview(path)
	if len(fr.FileComments) != 0 {
		t.Fatalf("expected 0 file comments after dd, got %d", len(fr.FileComments))
	}
}

func TestDeleteOthersCommentBlocked(t *testing.T) {
	app := newTestApp(t)
	app.session.Reviewer = "me"
	path := app.diffFiles[0].DisplayPath()
	fr := app.session.GetOrCreateFileReview(path, model.FileModified)

	c := model.NewComment("their comment", model.CommentNote, model.SideNew)
	c.Author = "someone_else"
	fr.AddFileComment(c)
	app.rebuildAnnotations()

	for i, ann := range app.annotations {
		if ann.Type == annFileComment {
			app.cursorLine = i
			break
		}
	}

	app = sendKeys(app, keyPress('d'), keyPress('d'))
	fr = app.session.GetFileReview(path)
	if len(fr.FileComments) != 1 {
		t.Fatal("should not delete others' comments")
	}
	if app.message == nil || app.message.text != "Cannot delete others' comments" {
		t.Error("expected warning about deleting others' comments")
	}
}

// --- Comment empty body discards ---

func TestEmptyCommentDiscarded(t *testing.T) {
	app := newTestApp(t)
	path := app.diffFiles[0].DisplayPath()

	for app.cursorLine < len(app.annotations) && app.annotations[app.cursorLine].Type != annDiffLine {
		app = sendKeys(app, keyPress('j'))
	}

	// Enter comment mode, save with empty body
	app = sendKeys(app, keyPress('c'))
	app = sendKeys(app, keySpecial(tea.KeyEnter))

	fr := app.session.GetFileReview(path)
	total := 0
	for _, c := range fr.LineComments {
		total += len(c)
	}
	if total != 0 {
		t.Fatalf("empty comment should be discarded, got %d comments", total)
	}
}

// --- Message clearing ---

func TestInfoMessageClearsOnKeypress(t *testing.T) {
	app := newTestApp(t)
	app.setMessage("test info", messageInfo)

	// Any keypress clears info message
	app = sendKeys(app, keyPress('j'))
	if app.message != nil {
		t.Error("info message should be cleared on keypress")
	}
}

func TestErrorMessagePersistsUntilEscape(t *testing.T) {
	app := newTestApp(t)
	app.setMessage("test error", messageError)

	// Regular keypress should NOT clear error
	app = sendKeys(app, keyPress('j'))
	if app.message == nil {
		t.Fatal("error message should persist on regular keypress")
	}

	// Escape clears it
	app = sendKeys(app, keySpecial(tea.KeyEscape))
	if app.message != nil {
		t.Error("error message should be cleared on Escape")
	}
}

// --- Search ---

func TestSearchFindsMatch(t *testing.T) {
	app := newTestApp(t)

	// Search for "added" — should jump to the line containing "added line"
	app = sendKeys(app, keyPress('/'))
	app = sendKeys(app, keyPress('a'), keyPress('d'), keyPress('d'), keyPress('e'), keyPress('d'))
	app = sendKeys(app, keySpecial(tea.KeyEnter))

	ann := app.annotations[app.cursorLine]
	if ann.Type != annDiffLine {
		t.Fatalf("search should land on a diff line, got type %d", ann.Type)
	}
}

func TestSearchNRepeats(t *testing.T) {
	app := newTwoFileApp(t)

	// Search for "new"
	app = sendKeys(app, keyPress('/'))
	app = sendKeys(app, keyPress('n'), keyPress('e'), keyPress('w'))
	app = sendKeys(app, keySpecial(tea.KeyEnter))
	first := app.cursorLine

	// n goes to next match
	app = sendKeys(app, keyPress('n'))
	if app.cursorLine <= first {
		t.Fatalf("n should advance past first match, was %d now %d", first, app.cursorLine)
	}
}

// --- Cursor boundary ---

func TestCursorDoesNotGoBelowAnnotations(t *testing.T) {
	app := newTestApp(t)

	// Press j many times
	for i := 0; i < len(app.annotations)+10; i++ {
		app = sendKeys(app, keyPress('j'))
	}
	if app.cursorLine >= len(app.annotations) {
		t.Fatalf("cursor went past annotations: %d >= %d", app.cursorLine, len(app.annotations))
	}
}

func TestCursorDoesNotGoAboveZero(t *testing.T) {
	app := newTestApp(t)

	app = sendKeys(app, keyPress('k'), keyPress('k'), keyPress('k'))
	if app.cursorLine < 0 {
		t.Fatalf("cursor went negative: %d", app.cursorLine)
	}
}

// --- Unknown command ---

func TestUnknownCommand(t *testing.T) {
	app := newTestApp(t)

	app = sendKeys(app, keyPress(':'))
	app = sendKeys(app, keyPress('x'), keyPress('y'), keyPress('z'))
	app = sendKeys(app, keySpecial(tea.KeyEnter))

	if app.message == nil || app.message.text != "Unknown command: xyz" {
		var msg string
		if app.message != nil {
			msg = app.message.text
		}
		t.Errorf("expected unknown command message, got %q", msg)
	}
}

// --- File list panel navigation ---

func TestFileListNavigation(t *testing.T) {
	app := newTwoFileApp(t)

	// Focus file list
	app = sendKeys(app, keyPress(';'), keyPress('h'))
	if app.focusedPanel != panelFileList {
		t.Fatalf("expected panelFileList, got %d", app.focusedPanel)
	}

	// j moves down in file list
	start := app.fileListCursor
	app = sendKeys(app, keyPress('j'))
	if app.fileListCursor <= start {
		t.Fatalf("j in file list should move cursor down, was %d now %d", start, app.fileListCursor)
	}

	// Enter on a file jumps to it
	app = sendKeys(app, keySpecial(tea.KeyEnter))
	if app.focusedPanel != panelDiff {
		t.Fatalf("Enter in file list should focus diff, got %d", app.focusedPanel)
	}
}

// --- Semicolon e hides file list and moves focus ---

func TestSemicolonEMovesFocusWhenHiding(t *testing.T) {
	app := newTestApp(t)

	// Focus file list first
	app = sendKeys(app, keyPress(';'), keyPress('h'))
	if app.focusedPanel != panelFileList {
		t.Fatalf("expected panelFileList, got %d", app.focusedPanel)
	}

	// ;e should hide file list AND move focus to diff
	app = sendKeys(app, keyPress(';'), keyPress('e'))
	if app.showFileList {
		t.Fatal("file list should be hidden")
	}
	if app.focusedPanel != panelDiff {
		t.Fatalf("focus should move to panelDiff when hiding file list, got %d", app.focusedPanel)
	}
}

// --- Comment key handling ---

func TestCommentShiftEnterNewline(t *testing.T) {
	app := newTestApp(t)
	for app.cursorLine < len(app.annotations) && app.annotations[app.cursorLine].Type != annDiffLine {
		app = sendKeys(app, keyPress('j'))
	}

	app = sendKeys(app, keyPress('c'))
	app = sendKeys(app, keyPress('a'), keyPress('b'))
	app = sendKeys(app, keyMod(tea.KeyEnter, tea.ModShift))
	app = sendKeys(app, keyPress('c'), keyPress('d'))

	if app.commentBuffer != "ab\ncd" {
		t.Fatalf("commentBuffer = %q, want %q", app.commentBuffer, "ab\ncd")
	}
	if app.commentCursor != 5 {
		t.Fatalf("commentCursor = %d, want 5", app.commentCursor)
	}
}

func TestCommentBackspace(t *testing.T) {
	app := newTestApp(t)
	for app.cursorLine < len(app.annotations) && app.annotations[app.cursorLine].Type != annDiffLine {
		app = sendKeys(app, keyPress('j'))
	}

	app = sendKeys(app, keyPress('c'))
	app = sendKeys(app, keyPress('a'), keyPress('b'), keyPress('c'))
	app = sendKeys(app, keySpecial(tea.KeyBackspace))

	if app.commentBuffer != "ab" {
		t.Fatalf("commentBuffer = %q, want %q", app.commentBuffer, "ab")
	}
}

func TestCommentBackspaceAtStart(t *testing.T) {
	app := newTestApp(t)
	for app.cursorLine < len(app.annotations) && app.annotations[app.cursorLine].Type != annDiffLine {
		app = sendKeys(app, keyPress('j'))
	}

	app = sendKeys(app, keyPress('c'))
	// Backspace at position 0 should be a no-op
	app = sendKeys(app, keySpecial(tea.KeyBackspace))
	if app.commentCursor != 0 {
		t.Fatalf("backspace at start should keep cursor at 0, got %d", app.commentCursor)
	}
}

func TestCommentTabCyclesType(t *testing.T) {
	app := newTestApp(t)
	for app.cursorLine < len(app.annotations) && app.annotations[app.cursorLine].Type != annDiffLine {
		app = sendKeys(app, keyPress('j'))
	}

	app = sendKeys(app, keyPress('c'))
	if app.commentType != model.CommentNote {
		t.Fatalf("initial type = %d, want CommentNote", app.commentType)
	}

	app = sendKeys(app, keySpecial(tea.KeyTab))
	if app.commentType != model.CommentSuggestion {
		t.Fatalf("after Tab, type = %d, want CommentSuggestion", app.commentType)
	}

	app = sendKeys(app, keySpecial(tea.KeyTab))
	if app.commentType != model.CommentIssue {
		t.Fatalf("after second Tab, type = %d, want CommentIssue", app.commentType)
	}
}

func TestCommentEscapeDiscards(t *testing.T) {
	app := newTestApp(t)
	path := app.diffFiles[0].DisplayPath()
	for app.cursorLine < len(app.annotations) && app.annotations[app.cursorLine].Type != annDiffLine {
		app = sendKeys(app, keyPress('j'))
	}

	app = sendKeys(app, keyPress('c'))
	app = sendKeys(app, keyPress('h'), keyPress('i'))
	app = sendKeys(app, keySpecial(tea.KeyEscape))

	if app.inputMode != modeNormal {
		t.Fatalf("after Escape, mode = %d, want modeNormal", app.inputMode)
	}

	fr := app.session.GetFileReview(path)
	total := 0
	for _, c := range fr.LineComments {
		total += len(c)
	}
	if total != 0 {
		t.Fatalf("escaped comment should not be saved, got %d comments", total)
	}
}

// --- Bug report mode ---

func TestBugModeEntryAndEscape(t *testing.T) {
	app := newTestApp(t)

	// :bug enters bug mode
	app = sendKeys(app, keyPress(':'))
	app = sendKeys(app, keyPress('b'), keyPress('u'), keyPress('g'))
	app = sendKeys(app, keySpecial(tea.KeyEnter))

	if app.inputMode != modeBug {
		t.Fatalf("after :bug, mode = %d, want modeBug", app.inputMode)
	}

	// Type some text
	app = sendKeys(app, keyPress('h'), keyPress('i'))
	if app.bugBuffer != "hi" {
		t.Fatalf("bugBuffer = %q, want %q", app.bugBuffer, "hi")
	}

	// Escape exits
	app = sendKeys(app, keySpecial(tea.KeyEscape))
	if app.inputMode != modeNormal {
		t.Fatalf("after Escape, mode = %d, want modeNormal", app.inputMode)
	}
	if app.bugBuffer != "" {
		t.Fatalf("bugBuffer should be cleared after Escape, got %q", app.bugBuffer)
	}
}

func TestBugModeShiftEnterNewline(t *testing.T) {
	app := newTestApp(t)
	app.inputMode = modeBug
	app.bugBuffer = ""
	app.bugCursor = 0

	app = sendKeys(app, keyPress('a'))
	app = sendKeys(app, keyMod(tea.KeyEnter, tea.ModShift))
	app = sendKeys(app, keyPress('b'))

	if app.bugBuffer != "a\nb" {
		t.Fatalf("bugBuffer = %q, want %q", app.bugBuffer, "a\nb")
	}
}

func TestBugModeBackspace(t *testing.T) {
	app := newTestApp(t)
	app.inputMode = modeBug
	app.bugBuffer = ""
	app.bugCursor = 0

	app = sendKeys(app, keyPress('a'), keyPress('b'))
	app = sendKeys(app, keySpecial(tea.KeyBackspace))

	if app.bugBuffer != "a" {
		t.Fatalf("bugBuffer = %q, want %q", app.bugBuffer, "a")
	}
}

// --- Conversation mode ---

func TestConversationModeEntryAndEscape(t *testing.T) {
	app := newTestApp(t)
	app.inputMode = modeConversation
	app.convBuffer = ""
	app.convCursor = 0

	app = sendKeys(app, keyPress('h'), keyPress('i'))
	if app.convBuffer != "hi" {
		t.Fatalf("convBuffer = %q, want %q", app.convBuffer, "hi")
	}

	app = sendKeys(app, keySpecial(tea.KeyEscape))
	if app.inputMode != modeNormal {
		t.Fatalf("after Escape, mode = %d, want modeNormal", app.inputMode)
	}
	if app.convBuffer != "" {
		t.Fatalf("convBuffer should be cleared, got %q", app.convBuffer)
	}
}

func TestConversationShiftEnter(t *testing.T) {
	app := newTestApp(t)
	app.inputMode = modeConversation
	app.convBuffer = ""
	app.convCursor = 0

	app = sendKeys(app, keyPress('a'))
	app = sendKeys(app, keyMod(tea.KeyEnter, tea.ModShift))
	app = sendKeys(app, keyPress('b'))

	if app.convBuffer != "a\nb" {
		t.Fatalf("convBuffer = %q, want %q", app.convBuffer, "a\nb")
	}
}

func TestConversationBackspace(t *testing.T) {
	app := newTestApp(t)
	app.inputMode = modeConversation
	app.convBuffer = ""
	app.convCursor = 0

	app = sendKeys(app, keyPress('x'), keyPress('y'))
	app = sendKeys(app, keySpecial(tea.KeyBackspace))

	if app.convBuffer != "x" {
		t.Fatalf("convBuffer = %q, want %q", app.convBuffer, "x")
	}
}

func TestConversationSave(t *testing.T) {
	app := newTestApp(t)
	app.inputMode = modeConversation
	app.convBuffer = ""
	app.convCursor = 0

	app = sendKeys(app, keyPress('h'), keyPress('i'))
	app = sendKeys(app, keySpecial(tea.KeyEnter))

	if app.inputMode != modeNormal {
		t.Fatalf("after Enter, mode = %d, want modeNormal", app.inputMode)
	}
	if len(app.session.Conversation) != 1 {
		t.Fatalf("expected 1 conversation comment, got %d", len(app.session.Conversation))
	}
	if app.session.Conversation[0].Body != "hi" {
		t.Fatalf("conversation body = %q, want %q", app.session.Conversation[0].Body, "hi")
	}
}

func TestConversationEmptyDiscards(t *testing.T) {
	app := newTestApp(t)
	app.inputMode = modeConversation
	app.convBuffer = ""
	app.convCursor = 0

	// Enter with empty body should not create comment
	app = sendKeys(app, keySpecial(tea.KeyEnter))
	if len(app.session.Conversation) != 0 {
		t.Fatalf("empty conversation should not be saved, got %d", len(app.session.Conversation))
	}
}

// --- Review key handling ---

func TestReviewShiftEnterNewline(t *testing.T) {
	app := newTestApp(t)
	app = sendKeys(app, upperKey('r', 'R'))

	app = sendKeys(app, keyPress('a'))
	app = sendKeys(app, keyMod(tea.KeyEnter, tea.ModShift))
	app = sendKeys(app, keyPress('b'))

	if app.reviewBuffer != "a\nb" {
		t.Fatalf("reviewBuffer = %q, want %q", app.reviewBuffer, "a\nb")
	}
}

func TestReviewBackspace(t *testing.T) {
	app := newTestApp(t)
	app = sendKeys(app, upperKey('r', 'R'))

	app = sendKeys(app, keyPress('a'), keyPress('b'))
	app = sendKeys(app, keySpecial(tea.KeyBackspace))

	if app.reviewBuffer != "a" {
		t.Fatalf("reviewBuffer = %q, want %q", app.reviewBuffer, "a")
	}
}

func TestReviewEmptyNeutralClearsReview(t *testing.T) {
	app := newTestApp(t)
	// Set an existing review
	app.session.OverallReview = &model.OverallReview{Body: "old", Status: model.ApprovalApprove}

	app = sendKeys(app, upperKey('r', 'R'))
	// Clear buffer and set neutral
	app.reviewBuffer = ""
	app.reviewCursor = 0
	app.reviewStatus = model.ApprovalNeutral

	app = sendKeys(app, keySpecial(tea.KeyEnter))
	if app.session.OverallReview != nil {
		t.Fatal("empty neutral review should clear OverallReview")
	}
}

// --- Help key handling ---

func TestHelpNavigationKeys(t *testing.T) {
	app := newTestApp(t)
	app = sendKeys(app, keyPress('?'))

	// j scrolls down
	app = sendKeys(app, keyPress('j'))
	if app.helpScroll != 1 {
		t.Fatalf("after j, helpScroll = %d, want 1", app.helpScroll)
	}

	// k scrolls back
	app = sendKeys(app, keyPress('k'))
	if app.helpScroll != 0 {
		t.Fatalf("after k, helpScroll = %d, want 0", app.helpScroll)
	}

	// k at 0 stays at 0
	app = sendKeys(app, keyPress('k'))
	if app.helpScroll != 0 {
		t.Fatalf("k at 0 should stay 0, got %d", app.helpScroll)
	}

	// Ctrl-d half page
	app = sendKeys(app, keyMod('d', tea.ModCtrl))
	if app.helpScroll <= 0 {
		t.Fatal("Ctrl-d should scroll down in help")
	}
	saved := app.helpScroll

	// Ctrl-u half page back
	app = sendKeys(app, keyMod('u', tea.ModCtrl))
	if app.helpScroll >= saved {
		t.Fatal("Ctrl-u should scroll up in help")
	}

	// g goes to top
	app.helpScroll = 10
	app = sendKeys(app, keyPress('g'))
	if app.helpScroll != 0 {
		t.Fatalf("g should go to top, got %d", app.helpScroll)
	}

	// G goes to bottom
	app = sendKeys(app, upperKey('g', 'G'))
	if app.helpScroll == 0 {
		t.Fatal("G should go to bottom")
	}
}

// --- :q with unsaved changes ---

func TestQuitWarnsOnDirty(t *testing.T) {
	app := newTestApp(t)
	app.dirty = true

	app = sendKeys(app, keyPress(':'))
	app = sendKeys(app, keyPress('q'))
	app = sendKeys(app, keySpecial(tea.KeyEnter))

	if app.message == nil || !strings.Contains(app.message.text, "Unsaved changes") {
		var msg string
		if app.message != nil {
			msg = app.message.text
		}
		t.Errorf("expected unsaved changes warning, got %q", msg)
	}
	if !app.quitWarned {
		t.Fatal("quitWarned should be set")
	}
}

// --- :e / :reload without VCS ---

func TestReloadWithoutVCS(t *testing.T) {
	app := newTestApp(t)

	app = sendKeys(app, keyPress(':'))
	app = sendKeys(app, keyPress('e'))
	app = sendKeys(app, keySpecial(tea.KeyEnter))

	if app.message == nil || app.message.text != "Reload requires a VCS backend" {
		var msg string
		if app.message != nil {
			msg = app.message.text
		}
		t.Errorf("expected VCS warning, got %q", msg)
	}
}

// --- :clip / :export ---

func TestExportNoComments(t *testing.T) {
	app := newTestApp(t)

	app = sendKeys(app, keyPress(':'))
	app = sendKeys(app, keyPress('c'), keyPress('l'), keyPress('i'), keyPress('p'))
	app = sendKeys(app, keySpecial(tea.KeyEnter))

	if app.message == nil || app.message.text != "No comments to export" {
		var msg string
		if app.message != nil {
			msg = app.message.text
		}
		t.Errorf("expected no comments warning, got %q", msg)
	}
}

func TestExportWithComments(t *testing.T) {
	app := newTestApp(t)
	path := app.diffFiles[0].DisplayPath()
	fr := app.session.GetOrCreateFileReview(path, model.FileModified)
	fr.AddFileComment(model.NewComment("note", model.CommentNote, model.SideNew))

	app = sendKeys(app, keyPress(':'))
	app = sendKeys(app, keyPress('c'), keyPress('l'), keyPress('i'), keyPress('p'))

	// Capture the command returned by executeCommand
	m, cmd := app.Update(keySpecial(tea.KeyEnter))
	app = m.(*App)

	if cmd == nil {
		t.Fatal("export should return a command")
	}
	if app.message == nil || !strings.Contains(app.message.text, "Exported") {
		var msg string
		if app.message != nil {
			msg = app.message.text
		}
		t.Errorf("expected export message, got %q", msg)
	}
}

// --- Search backward (N) ---

func TestSearchBackward(t *testing.T) {
	app := newTwoFileApp(t)

	// Search for "new"
	app = sendKeys(app, keyPress('/'))
	app = sendKeys(app, keyPress('n'), keyPress('e'), keyPress('w'))
	app = sendKeys(app, keySpecial(tea.KeyEnter))
	first := app.cursorLine

	// Go forward
	app = sendKeys(app, keyPress('n'))
	second := app.cursorLine
	if second <= first {
		t.Fatalf("n should go forward, was %d now %d", first, second)
	}

	// N goes backward
	app = sendKeys(app, upperKey('n', 'N'))
	if app.cursorLine >= second {
		t.Fatalf("N should go backward, was %d now %d", second, app.cursorLine)
	}
}

func TestSearchNotFound(t *testing.T) {
	app := newTestApp(t)

	app = sendKeys(app, keyPress('/'))
	app = sendKeys(app, keyPress('z'), keyPress('z'), keyPress('z'), keyPress('z'))
	app = sendKeys(app, keySpecial(tea.KeyEnter))

	if app.message == nil || !strings.Contains(app.message.text, "Pattern not found") {
		var msg string
		if app.message != nil {
			msg = app.message.text
		}
		t.Errorf("expected not found message, got %q", msg)
	}
}

// --- File list directory collapse/expand ---

func TestFileListDirCollapse(t *testing.T) {
	app := newMultiFileApp(t)

	// Focus file list
	app = sendKeys(app, keyPress(';'), keyPress('h'))
	initialRows := len(app.fileTreeRows)

	// Find a directory row and collapse it
	for i, row := range app.fileTreeRows {
		if row.IsDir {
			app.fileListCursor = i
			app = sendKeys(app, keySpecial(tea.KeyEnter))

			if len(app.fileTreeRows) >= initialRows {
				t.Error("collapsing dir should reduce visible rows")
			}

			// Expand again
			app = sendKeys(app, keySpecial(tea.KeyEnter))
			if len(app.fileTreeRows) != initialRows {
				t.Errorf("expanding should restore rows: got %d want %d", len(app.fileTreeRows), initialRows)
			}
			return
		}
	}
	t.Skip("no directory rows in test data")
}

// --- Cursor movement in file list panel ---

func TestCursorDownInFileListPanel(t *testing.T) {
	app := newTwoFileApp(t)
	app = sendKeys(app, keyPress(';'), keyPress('h'))

	start := app.fileListCursor
	app = sendKeys(app, keyPress('j'), keyPress('j'))
	if app.fileListCursor <= start {
		t.Fatalf("j in file list should move down, was %d now %d", start, app.fileListCursor)
	}

	app = sendKeys(app, keyPress('k'))
	if app.fileListCursor >= start+2 {
		t.Fatalf("k in file list should move up")
	}
}

func TestCursorBoundsInFileList(t *testing.T) {
	app := newTwoFileApp(t)
	app = sendKeys(app, keyPress(';'), keyPress('h'))

	// Many k's shouldn't go negative
	for i := 0; i < 20; i++ {
		app = sendKeys(app, keyPress('k'))
	}
	if app.fileListCursor < 0 {
		t.Fatalf("cursor went negative: %d", app.fileListCursor)
	}

	// Many j's shouldn't exceed bounds
	for i := 0; i < 100; i++ {
		app = sendKeys(app, keyPress('j'))
	}
	if app.fileListCursor >= len(app.fileTreeRows) {
		t.Fatalf("cursor exceeded bounds: %d >= %d", app.fileListCursor, len(app.fileTreeRows))
	}
}

// --- Pending prefix cancelled by unknown key ---

func TestPendingPrefixCancelledByUnknownKey(t *testing.T) {
	app := newTestApp(t)

	// 'd' sets pending prefix, then 'x' (not 'd') should cancel
	app = sendKeys(app, keyPress('d'))
	if app.pendingPrefix != 'd' {
		t.Fatalf("pendingPrefix = %c, want 'd'", app.pendingPrefix)
	}

	app = sendKeys(app, keyPress('x'))
	if app.pendingPrefix != 0 {
		t.Fatalf("pendingPrefix should be cleared after unknown key, got %c", app.pendingPrefix)
	}
}

// --- Home key ---

func TestHomeKeyResetsScroll(t *testing.T) {
	app := newTestApp(t)
	app = sendKeys(app, keyPress('l'), keyPress('l'), keyPress('l'))
	app = sendKeys(app, keySpecial(tea.KeyHome))
	if app.scrollX != 0 {
		t.Fatalf("Home should reset scrollX, got %d", app.scrollX)
	}
}

// --- Visual select on non-diff line ---

func TestVisualSelectOnNonDiffLine(t *testing.T) {
	app := newTestApp(t)
	// Cursor starts on file header (non-diff line)
	app = sendKeys(app, keyPress('v'))
	if app.inputMode == modeVisualSelect {
		t.Fatal("should not enter visual select on non-diff line")
	}
	if app.message == nil || !strings.Contains(app.message.text, "diff line") {
		t.Error("expected warning about needing a diff line")
	}
}

// --- Line comment on non-diff line ---

func TestCommentOnNonDiffLine(t *testing.T) {
	app := newTestApp(t)
	// Cursor on file header
	app = sendKeys(app, keyPress('c'))
	if app.inputMode == modeComment {
		t.Fatal("should not enter comment mode on non-diff line")
	}
}

// --- Edit on non-comment line ---

func TestEditOnNonCommentLine(t *testing.T) {
	app := newTestApp(t)
	app = sendKeys(app, keyPress('i'))
	if app.message == nil || !strings.Contains(app.message.text, "comment to edit") {
		var msg string
		if app.message != nil {
			msg = app.message.text
		}
		t.Errorf("expected edit warning, got %q", msg)
	}
}

// --- Picker tests ---

func newPickerApp(t *testing.T) *App {
	t.Helper()

	mockVCS := &testutil.MockVCS{
		VcsInfo: vcs.VcsInfo{
			RootPath:   "/tmp/test",
			HeadCommit: "abc123",
			BranchName: "main",
			VcsType:    "git",
		},
		RecentCommits: []vcs.CommitInfo{
			{ID: "aaa", ShortID: "aaa", Summary: "First commit", Author: "alice", Time: testTime.Add(-1 * time.Hour)},
			{ID: "bbb", ShortID: "bbb", Summary: "Second commit", Author: "bob", Time: testTime.Add(-2 * time.Hour)},
		},
		WorkingTreeDiff: []model.DiffFile{
			{
				OldPath: "wt.go", NewPath: "wt.go", Status: model.FileModified,
				Hunks: []model.DiffHunk{{
					OldStart: 1, OldCount: 1, NewStart: 1, NewCount: 2,
					Header: "@@ -1,1 +1,2 @@",
					Lines: []model.DiffLine{
						{Origin: model.OriginContext, Content: "pkg", OldLineNo: 1, NewLineNo: 1},
						{Origin: model.OriginAddition, Content: "new", NewLineNo: 2},
					},
				}},
			},
		},
		RevisionDiff: []model.DiffFile{
			{
				OldPath: "rev.go", NewPath: "rev.go", Status: model.FileModified,
				Hunks: []model.DiffHunk{{
					OldStart: 1, OldCount: 1, NewStart: 1, NewCount: 1,
					Header: "@@ -1,1 +1,1 @@",
					Lines: []model.DiffLine{
						{Origin: model.OriginDeletion, Content: "old", OldLineNo: 1},
						{Origin: model.OriginAddition, Content: "new", NewLineNo: 1},
					},
				}},
			},
		},
	}

	store := testutil.NewMockStore()
	app := NewPickerApp(mockVCS, theme.Dark(), nil, store)
	app.SetSize(120, 40)
	return &app
}

func newProviderApp(t *testing.T) *App {
	t.Helper()

	app := newTestApp(t)
	app.session.Provider = &model.ProviderInfo{Name: "github", ID: "42", URL: "https://example.com/pr/42"}
	app.session.Reviewer = "me"
	app.provider = mock.New()
	app.providerID = "42"
	app.store = testutil.NewMockStore()
	return app
}

func newVCSApp(t *testing.T) *App {
	t.Helper()

	mockVCS := &testutil.MockVCS{
		VcsInfo: vcs.VcsInfo{RootPath: "/tmp/test", HeadCommit: "abc", BranchName: "main", VcsType: "git"},
		ContextLines: []model.DiffLine{
			{Origin: model.OriginContext, Content: "expanded line 1", OldLineNo: 4, NewLineNo: 4},
			{Origin: model.OriginContext, Content: "expanded line 2", OldLineNo: 5, NewLineNo: 5},
		},
	}

	// Create a file with two hunks so there's an expandable gap
	files := []model.DiffFile{
		{
			OldPath: "gap.go", NewPath: "gap.go", Status: model.FileModified,
			Hunks: []model.DiffHunk{
				{
					OldStart: 1, OldCount: 3, NewStart: 1, NewCount: 3,
					Header: "@@ -1,3 +1,3 @@",
					Lines: []model.DiffLine{
						{Origin: model.OriginContext, Content: "pkg", OldLineNo: 1, NewLineNo: 1},
						{Origin: model.OriginDeletion, Content: "old", OldLineNo: 2},
						{Origin: model.OriginAddition, Content: "new", NewLineNo: 2},
						{Origin: model.OriginContext, Content: "end", OldLineNo: 3, NewLineNo: 3},
					},
				},
				{
					OldStart: 10, OldCount: 2, NewStart: 10, NewCount: 2,
					Header: "@@ -10,2 +10,2 @@",
					Lines: []model.DiffLine{
						{Origin: model.OriginDeletion, Content: "old10", OldLineNo: 10},
						{Origin: model.OriginAddition, Content: "new10", NewLineNo: 10},
						{Origin: model.OriginContext, Content: "end10", OldLineNo: 11, NewLineNo: 11},
					},
				},
			},
		},
	}

	session := model.NewSession("/tmp/test", "main", "abc", model.DiffWorkingTree)
	for _, f := range files {
		session.GetOrCreateFileReview(f.DisplayPath(), f.Status)
	}

	app := NewApp(mockVCS, files, session, theme.Dark(), nil, nil)
	app.SetSize(120, 40)
	return &app
}

func TestPickerPhase(t *testing.T) {
	app := newPickerApp(t)
	if app.phase != phasePicker {
		t.Fatal("expected picker phase")
	}
	// Should have working tree + 2 commits = 3 items
	if len(app.pickerItems) != 3 {
		t.Fatalf("expected 3 picker items, got %d", len(app.pickerItems))
	}
}

func TestPickerNavigation(t *testing.T) {
	app := newPickerApp(t)
	if app.pickerCursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", app.pickerCursor)
	}

	app = sendKeys(app, keyPress('j'))
	if app.pickerCursor != 1 {
		t.Fatalf("expected cursor at 1, got %d", app.pickerCursor)
	}

	app = sendKeys(app, keyPress('k'))
	if app.pickerCursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", app.pickerCursor)
	}

	// G goes to end
	app = sendKeys(app, upperKey('g', 'G'))
	if app.pickerCursor != 2 {
		t.Fatalf("expected cursor at 2, got %d", app.pickerCursor)
	}

	// g goes to start
	app = sendKeys(app, keyPress('g'))
	if app.pickerCursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", app.pickerCursor)
	}
}

func TestPickerToggleSelection(t *testing.T) {
	app := newPickerApp(t)
	// Space toggles selection
	app = sendKeys(app, keyPress(' '))
	if !app.pickerSelected[0] {
		t.Fatal("item 0 should be selected")
	}
	app = sendKeys(app, keyPress(' '))
	if app.pickerSelected[0] {
		t.Fatal("item 0 should be deselected")
	}
}

func TestPickerConfirmWithSelection(t *testing.T) {
	app := newPickerApp(t)
	// Select working tree and first commit
	app = sendKeys(app, keyPress(' ')) // select working tree
	app = sendKeys(app, keyPress('j')) // move to commit
	app = sendKeys(app, keyPress(' ')) // select commit
	app = sendKeys(app, keySpecial(tea.KeyEnter))
	if app.phase != phaseReview {
		t.Fatal("expected review phase after confirm")
	}
	if len(app.diffFiles) == 0 {
		t.Fatal("expected diff files after confirm")
	}
	if app.session == nil {
		t.Fatal("expected session after confirm")
	}
}

func TestPickerConfirmNoSelection(t *testing.T) {
	app := newPickerApp(t)
	// Enter with no selection => uses cursor item
	app = sendKeys(app, keySpecial(tea.KeyEnter))
	if app.phase != phaseReview {
		t.Fatal("expected review phase")
	}
}

func TestPickerQuit(t *testing.T) {
	app := newPickerApp(t)
	_, cmd := app.Update(keyPress('q'))
	if cmd == nil {
		t.Fatal("expected quit command")
	}
	msg := cmd()
	if _, ok := msg.(DoneMsg); !ok {
		t.Fatalf("expected DoneMsg, got %T", msg)
	}
}

func TestPickerRenderView(t *testing.T) {
	app := newPickerApp(t)
	view := app.View()
	if !strings.Contains(view.Content, "Select what to review") {
		t.Error("picker view should contain title")
	}
	if !strings.Contains(view.Content, "Working tree") {
		t.Error("picker view should show working tree option")
	}
	if !strings.Contains(view.Content, "First commit") {
		t.Error("picker view should show commit summaries")
	}
}

func TestPickerDownArrow(t *testing.T) {
	app := newPickerApp(t)
	app = sendKeys(app, keySpecial(tea.KeyDown))
	if app.pickerCursor != 1 {
		t.Fatalf("expected cursor 1, got %d", app.pickerCursor)
	}
	app = sendKeys(app, keySpecial(tea.KeyUp))
	if app.pickerCursor != 0 {
		t.Fatalf("expected cursor 0, got %d", app.pickerCursor)
	}
}

func TestPickerBoundsCheck(t *testing.T) {
	app := newPickerApp(t)
	// Try going above 0
	app = sendKeys(app, keyPress('k'))
	if app.pickerCursor != 0 {
		t.Fatalf("cursor should stay at 0, got %d", app.pickerCursor)
	}
	// Go past end
	for i := 0; i < 10; i++ {
		app = sendKeys(app, keyPress('j'))
	}
	if app.pickerCursor != len(app.pickerItems)-1 {
		t.Fatalf("cursor should be at last item")
	}
}

// --- SetCommits and commit list tests ---

func TestSetCommits(t *testing.T) {
	app := newTestApp(t)
	commits := []vcs.CommitInfo{
		{ID: "c1", ShortID: "c1", Summary: "Commit 1"},
		{ID: "c2", ShortID: "c2", Summary: "Commit 2"},
	}
	diffs := map[string][]model.DiffFile{
		"c1": {{OldPath: "f1.go", NewPath: "f1.go", Status: model.FileModified}},
		"c2": {{OldPath: "f2.go", NewPath: "f2.go", Status: model.FileModified}},
	}
	app.SetCommits(commits, diffs)

	if len(app.reviewCommits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(app.reviewCommits))
	}
	if !app.enabledCommits["c1"] || !app.enabledCommits["c2"] {
		t.Fatal("all commits should be enabled")
	}
	if app.combinedDiffFiles == nil {
		t.Fatal("combinedDiffFiles should be saved")
	}
}

func TestCommitListRender(t *testing.T) {
	app := newTestApp(t)
	commits := []vcs.CommitInfo{
		{ID: "c1", ShortID: "c1", Summary: "Commit 1", Author: "alice", Time: testTime.Add(-10 * time.Minute)},
		{ID: "c2", ShortID: "c2", Summary: "Commit 2", Author: "bob", Time: testTime.Add(-1 * time.Hour)},
	}
	diffs := map[string][]model.DiffFile{
		"c1": app.diffFiles,
		"c2": app.diffFiles,
	}
	app.SetCommits(commits, diffs)

	output := app.renderCommitList(120, 8)
	if !strings.Contains(output, "Commit 1") {
		t.Error("commit list should show commit summaries")
	}
	if !strings.Contains(output, "alice") {
		t.Error("commit list should show author")
	}
}

func TestToggleCommitAtCursor(t *testing.T) {
	app := newTestApp(t)
	commits := []vcs.CommitInfo{
		{ID: "c1", ShortID: "c1", Summary: "Commit 1"},
	}
	diffs := map[string][]model.DiffFile{
		"c1": app.diffFiles,
	}
	app.SetCommits(commits, diffs)
	app.commitCursor = 0

	app.toggleCommitAtCursor()
	if app.enabledCommits["c1"] {
		t.Fatal("commit should be disabled after toggle")
	}
	app.toggleCommitAtCursor()
	if !app.enabledCommits["c1"] {
		t.Fatal("commit should be re-enabled after second toggle")
	}
}

func TestAllCommitsEnabled(t *testing.T) {
	app := newTestApp(t)
	commits := []vcs.CommitInfo{
		{ID: "c1", ShortID: "c1"},
		{ID: "c2", ShortID: "c2"},
	}
	diffs := map[string][]model.DiffFile{
		"c1": app.diffFiles,
		"c2": app.diffFiles,
	}
	app.SetCommits(commits, diffs)
	if !app.allCommitsEnabled() {
		t.Fatal("all commits should be enabled initially")
	}

	delete(app.enabledCommits, "c1")
	if app.allCommitsEnabled() {
		t.Fatal("should not be all enabled after deleting one")
	}
}

func TestMergeEnabledDiffs(t *testing.T) {
	app := newTestApp(t)
	f1 := model.DiffFile{
		OldPath: "a.go", NewPath: "a.go", Status: model.FileModified,
		Hunks: []model.DiffHunk{{Header: "@@ -1 +1 @@"}},
	}
	f2 := model.DiffFile{
		OldPath: "b.go", NewPath: "b.go", Status: model.FileAdded,
		Hunks: []model.DiffHunk{{Header: "@@ -0,0 +1 @@"}},
	}
	commits := []vcs.CommitInfo{
		{ID: "c1", ShortID: "c1"},
		{ID: "c2", ShortID: "c2"},
	}
	diffs := map[string][]model.DiffFile{
		"c1": {f1},
		"c2": {f2},
	}
	app.SetCommits(commits, diffs)

	// Disable c2
	delete(app.enabledCommits, "c2")
	merged := app.mergeEnabledDiffs()
	if len(merged) != 1 {
		t.Fatalf("expected 1 merged file, got %d", len(merged))
	}
	if merged[0].NewPath != "a.go" {
		t.Errorf("expected a.go, got %s", merged[0].NewPath)
	}
}

func TestRebuildFromCommits(t *testing.T) {
	app := newTestApp(t)
	f1 := model.DiffFile{
		OldPath: "x.go", NewPath: "x.go", Status: model.FileModified,
		Hunks: []model.DiffHunk{{
			OldStart: 1, OldCount: 1, NewStart: 1, NewCount: 1,
			Header: "@@ -1,1 +1,1 @@",
			Lines:  []model.DiffLine{{Origin: model.OriginContext, Content: "pkg", OldLineNo: 1, NewLineNo: 1}},
		}},
	}
	commits := []vcs.CommitInfo{{ID: "c1", ShortID: "c1"}}
	diffs := map[string][]model.DiffFile{"c1": {f1}}
	app.SetCommits(commits, diffs)

	// When all enabled, uses combinedDiffFiles
	app.rebuildFromCommits()
	// Should not crash and should maintain a valid state
	if len(app.annotations) == 0 {
		t.Fatal("annotations should be rebuilt")
	}
}

func TestTopPanelHeight(t *testing.T) {
	app := newTestApp(t)
	// No commits => height 0
	if h := app.topPanelHeight(); h != 0 {
		t.Fatalf("expected 0 with no commits, got %d", h)
	}

	// With commits
	commits := []vcs.CommitInfo{{ID: "c1", ShortID: "c1"}}
	diffs := map[string][]model.DiffFile{"c1": app.diffFiles}
	app.SetCommits(commits, diffs)
	h := app.topPanelHeight()
	if h < 2 {
		t.Fatalf("expected at least 2 for 1 commit + separator, got %d", h)
	}
}

// --- Description rendering ---

func TestRenderDescription(t *testing.T) {
	app := newTestApp(t)
	app.session.Description = "Line 1\nLine 2\nLine 3"
	app.showDescription = true

	output := app.renderDescription(120, 8)
	if !strings.Contains(output, "Line 1") || !strings.Contains(output, "Line 2") {
		t.Error("description should show content lines")
	}
}

func TestDescriptionScroll(t *testing.T) {
	app := newTestApp(t)
	app.session.Description = "L1\nL2\nL3\nL4\nL5\nL6\nL7\nL8\nL9\nL10"
	app.showDescription = true
	app.descScroll = 5

	output := app.renderDescription(120, 5)
	if output == "" {
		t.Error("description should render with scroll")
	}
}

// --- Conversation rendering ---

func TestRenderConversation(t *testing.T) {
	app := newTestApp(t)
	app.session.Conversation = []model.ConversationComment{
		{Author: "alice", Body: "Hello", CreatedAt: testTime},
		{Author: "bob", Body: "World", CreatedAt: testTime},
	}
	app.showConversation = true

	output := app.renderConversation(120, 10, false)
	if !strings.Contains(output, "alice") {
		t.Error("should show author")
	}
	if !strings.Contains(output, "Hello") {
		t.Error("should show message body")
	}
}

func TestRenderConversationEmpty(t *testing.T) {
	app := newTestApp(t)
	app.showConversation = true

	output := app.renderConversation(120, 10, false)
	if !strings.Contains(output, "No conversation") {
		t.Error("should show empty message")
	}
}

func TestRenderConversationWithEditor(t *testing.T) {
	app := newTestApp(t)
	app.showConversation = true
	app.inputMode = modeConversation
	app.convBuffer = "typing..."
	app.convCursor = len(app.convBuffer)

	output := app.renderConversation(120, 12, true)
	if !strings.Contains(output, "New message") {
		t.Error("should show editor")
	}
}

// --- View with conversation panel ---

func TestViewWithConversationPanel(t *testing.T) {
	app := newTestApp(t)
	app.session.Conversation = []model.ConversationComment{
		{Author: "alice", Body: "test", CreatedAt: testTime},
	}
	app.showConversation = true

	view := app.View()
	if !strings.Contains(view.Content, "Conversation") {
		t.Error("view should include conversation panel")
	}
}

// --- View with description panel ---

func TestViewWithDescriptionPanel(t *testing.T) {
	app := newTestApp(t)
	app.session.Description = "PR description text"
	app.showDescription = true
	commits := []vcs.CommitInfo{{ID: "c1", ShortID: "c1"}}
	diffs := map[string][]model.DiffFile{"c1": app.diffFiles}
	app.SetCommits(commits, diffs)

	view := app.View()
	if !strings.Contains(view.Content, "PR description text") {
		t.Error("view should include description")
	}
}

// --- Provider-dependent commands ---

func TestSubmitToProviderNoProvider(t *testing.T) {
	app := newTestApp(t)
	app.submitToProvider()
	if app.message == nil || !strings.Contains(app.message.text, "Not connected") {
		t.Error("should warn about no provider")
	}
}

func TestRefreshFromProviderNoProvider(t *testing.T) {
	app := newTestApp(t)
	app.refreshFromProvider()
	if app.message == nil || !strings.Contains(app.message.text, "Not connected") {
		t.Error("should warn about no provider")
	}
}

func TestPostConversationCommentNoProvider(t *testing.T) {
	app := newTestApp(t)
	app.postConversationComment()
	if app.message == nil || !strings.Contains(app.message.text, "Not connected") {
		t.Error("should warn about no provider")
	}
}

func TestPostConversationCommentWithProvider(t *testing.T) {
	app := newProviderApp(t)
	app.postConversationComment()
	if app.inputMode != modeConversation {
		t.Fatal("should enter conversation mode")
	}
	if !app.showConversation {
		t.Fatal("should show conversation panel")
	}
}

func TestSubmitToProviderWithProvider(t *testing.T) {
	app := newProviderApp(t)
	app.submitToProvider()
	if app.inputMode != modeConfirm {
		t.Fatal("should enter confirm mode")
	}
	if !strings.Contains(app.confirmPrompt, "Submit review") {
		t.Errorf("prompt should mention submit, got %q", app.confirmPrompt)
	}
}

func TestSubmitToProviderConfirm(t *testing.T) {
	app := newProviderApp(t)
	// Add a comment to submit
	fr := app.session.GetOrCreateFileReview("a.go", model.FileModified)
	c := model.NewComment("test comment", model.CommentNote, model.SideNew)
	fr.AddLineComment(2, c)

	app.submitToProvider()
	if app.inputMode != modeConfirm {
		t.Fatal("should enter confirm mode")
	}
	// Simulate confirm (y)
	app = sendKeys(app, keyPress('y'))
	if app.message == nil || !strings.Contains(app.message.text, "submitted") {
		var msg string
		if app.message != nil {
			msg = app.message.text
		}
		t.Errorf("expected submitted message, got %q", msg)
	}
}

func TestRefreshFromProviderWithProvider(t *testing.T) {
	app := newProviderApp(t)
	app.refreshFromProvider()
	if app.message == nil || !strings.Contains(app.message.text, "Refreshed") {
		var msg string
		if app.message != nil {
			msg = app.message.text
		}
		t.Errorf("expected refresh message, got %q", msg)
	}
}

func TestConversationModeTypingAndSubmit(t *testing.T) {
	app := newProviderApp(t)
	app.postConversationComment()
	if app.inputMode != modeConversation {
		t.Fatal("should be in conversation mode")
	}
	// Type a message
	app = sendKeys(app,
		keyPress('h'), keyPress('e'), keyPress('l'), keyPress('l'), keyPress('o'),
	)
	if app.convBuffer != "hello" {
		t.Fatalf("expected 'hello', got %q", app.convBuffer)
	}
	// Submit with enter
	app = sendKeys(app, keySpecial(tea.KeyEnter))
	if app.inputMode != modeNormal {
		t.Fatal("should return to normal mode after submit")
	}
	if len(app.session.Conversation) == 0 {
		t.Fatal("should have posted conversation comment")
	}
}

func TestConversationModeEscape(t *testing.T) {
	app := newProviderApp(t)
	app.postConversationComment()
	app = sendKeys(app, keyPress('h'), keyPress('i'))
	app = sendKeys(app, keySpecial(tea.KeyEscape))
	if app.inputMode != modeNormal {
		t.Fatal("escape should return to normal mode")
	}
	if app.convBuffer != "" {
		t.Error("buffer should be cleared on escape")
	}
}

func TestConversationModeBackspace(t *testing.T) {
	app := newProviderApp(t)
	app.postConversationComment()
	app = sendKeys(app, keyPress('a'), keyPress('b'))
	app = sendKeys(app, keySpecial(tea.KeyBackspace))
	if app.convBuffer != "a" {
		t.Fatalf("expected 'a', got %q", app.convBuffer)
	}
}

func TestConversationModeNewline(t *testing.T) {
	app := newProviderApp(t)
	app.postConversationComment()
	app = sendKeys(app, keyPress('a'))
	app = sendKeys(app, keyMod(tea.KeyEnter, tea.ModShift))
	app = sendKeys(app, keyPress('b'))
	if app.convBuffer != "a\nb" {
		t.Fatalf("expected 'a\\nb', got %q", app.convBuffer)
	}
}

// --- Context expansion with VCS mock ---

func TestExpandGap(t *testing.T) {
	app := newVCSApp(t)
	// Find the expander annotation
	expanderIdx := -1
	for i, ann := range app.annotations {
		if ann.Type == annExpander {
			expanderIdx = i
			break
		}
	}
	if expanderIdx < 0 {
		t.Fatal("should have an expander annotation between two hunks")
	}

	// Move cursor to expander and toggle
	app.cursorLine = expanderIdx
	app.toggleExpandAtCursor()

	// Should have expanded
	if len(app.expandedGaps) == 0 {
		t.Fatal("gap should be expanded")
	}

	// Verify FetchContextLines was called
	mockVCS := app.vcs.(*testutil.MockVCS)
	if len(mockVCS.FetchContextCalls) != 1 {
		t.Fatalf("expected 1 FetchContextLines call, got %d", len(mockVCS.FetchContextCalls))
	}
	call := mockVCS.FetchContextCalls[0]
	if call.FilePath != "gap.go" {
		t.Errorf("expected file gap.go, got %s", call.FilePath)
	}
}

func TestExpandGapNoVCS(t *testing.T) {
	app := newTwoFileApp(t)
	// Find expander
	for i, ann := range app.annotations {
		if ann.Type == annExpander {
			app.cursorLine = i
			app.toggleExpandAtCursor()
			break
		}
	}
	if app.message == nil || !strings.Contains(app.message.text, "VCS backend") {
		t.Error("should warn about missing VCS")
	}
}

func TestCollapseGap(t *testing.T) {
	app := newVCSApp(t)
	// Find and expand
	for i, ann := range app.annotations {
		if ann.Type == annExpander {
			app.cursorLine = i
			app.toggleExpandAtCursor()
			break
		}
	}
	if len(app.expandedGaps) == 0 {
		t.Fatal("should be expanded")
	}

	// Find expanded context line and collapse
	for i, ann := range app.annotations {
		if ann.Type == annExpandedContext {
			app.cursorLine = i
			app.toggleExpandAtCursor()
			break
		}
	}
	if len(app.expandedGaps) != 0 {
		t.Fatal("should be collapsed")
	}
}

// --- Commit list cursor navigation keys ---

func TestCommitListCursorNavigation(t *testing.T) {
	app := newTestApp(t)
	commits := []vcs.CommitInfo{
		{ID: "c1", ShortID: "c1", Summary: "C1"},
		{ID: "c2", ShortID: "c2", Summary: "C2"},
		{ID: "c3", ShortID: "c3", Summary: "C3"},
	}
	diffs := map[string][]model.DiffFile{
		"c1": app.diffFiles,
		"c2": app.diffFiles,
		"c3": app.diffFiles,
	}
	app.SetCommits(commits, diffs)
	app.focusedPanel = panelCommitList

	// j moves down
	app = sendKeys(app, keyPress('j'))
	if app.commitCursor != 1 {
		t.Fatalf("expected commit cursor 1, got %d", app.commitCursor)
	}
	// k moves up
	app = sendKeys(app, keyPress('k'))
	if app.commitCursor != 0 {
		t.Fatalf("expected commit cursor 0, got %d", app.commitCursor)
	}
}

// --- Reply to comment ---

func TestReplyToCommentNoComment(t *testing.T) {
	app := newTestApp(t)
	// Move to a diff line (not a comment)
	app.cursorLine = 1
	app.replyToCommentAtCursor()
	if app.message == nil || !strings.Contains(app.message.text, "comment to reply") {
		var msg string
		if app.message != nil {
			msg = app.message.text
		}
		t.Errorf("expected reply warning, got %q", msg)
	}
}

func TestReplyToLocalComment(t *testing.T) {
	app := newTestApp(t)
	// Add a local comment (no ExternalID)
	path := app.diffFiles[0].DisplayPath()
	fr := app.session.GetOrCreateFileReview(path, model.FileModified)
	c := model.NewComment("local comment", model.CommentNote, model.SideNew)
	fr.AddLineComment(2, c)
	app.rebuildAnnotations()

	// Find the comment annotation
	for i, ann := range app.annotations {
		if ann.Type == annLineComment {
			app.cursorLine = i
			break
		}
	}
	app.replyToCommentAtCursor()
	if app.message == nil || !strings.Contains(app.message.text, "remote comments") {
		var msg string
		if app.message != nil {
			msg = app.message.text
		}
		t.Errorf("expected remote-only warning, got %q", msg)
	}
}

// --- SetProvider ---

func TestSetProvider(t *testing.T) {
	app := newTestApp(t)
	m := mock.New()
	app.SetProvider(m, "42")
	if app.provider == nil {
		t.Fatal("provider should be set")
	}
	if app.providerID != "42" {
		t.Fatalf("providerID = %q, want '42'", app.providerID)
	}
}

// --- View with commit list panel ---

func TestViewWithCommitListPanel(t *testing.T) {
	app := newTestApp(t)
	commits := []vcs.CommitInfo{
		{ID: "c1", ShortID: "c1", Summary: "My commit", Author: "me", Time: testTime},
	}
	diffs := map[string][]model.DiffFile{"c1": app.diffFiles}
	app.SetCommits(commits, diffs)

	view := app.View()
	if !strings.Contains(view.Content, "My commit") {
		t.Error("view should include commit list")
	}
}

// --- renderExpandedContextLine ---

func TestRenderExpandedContextLine(t *testing.T) {
	app := newVCSApp(t)
	// Expand a gap first
	for i, ann := range app.annotations {
		if ann.Type == annExpander {
			app.cursorLine = i
			app.toggleExpandAtCursor()
			break
		}
	}

	// Find and render an expanded context line
	for _, ann := range app.annotations {
		if ann.Type == annExpandedContext {
			result := app.renderExpandedContextLine(ann, 120, false)
			if result == "" {
				t.Error("expanded context should render non-empty")
			}
			if !strings.Contains(result, "expanded line") {
				t.Error("should contain the expanded line content")
			}

			// Also test with cursor
			cursorResult := app.renderExpandedContextLine(ann, 120, true)
			if cursorResult == "" {
				t.Error("cursor expanded context should render")
			}
			return
		}
	}
	t.Fatal("should have found expanded context annotation")
}

// --- buildDefaultDescription ---

func TestBuildDefaultDescription(t *testing.T) {
	app := newPickerApp(t)
	commits := []vcs.CommitInfo{
		{ID: "c1", Summary: "First", Body: "detail line"},
		{ID: "c2", Summary: "Second"},
	}
	desc := app.buildDefaultDescription(commits, true)
	if !strings.Contains(desc, "Working tree") {
		t.Error("should include working tree")
	}
	if !strings.Contains(desc, "First") || !strings.Contains(desc, "Second") {
		t.Error("should include commit summaries")
	}
	if !strings.Contains(desc, "detail line") {
		t.Error("should include commit body")
	}
}

// --- relativeTime ---

func TestRelativeTime(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "just now"},
		{5 * time.Minute, "5 minutes ago"},
		{1 * time.Minute, "1 minute ago"},
		{3 * time.Hour, "3 hours ago"},
		{1 * time.Hour, "1 hour ago"},
		{2 * 24 * time.Hour, "2 days ago"},
		{1 * 24 * time.Hour, "1 day ago"},
		{60 * 24 * time.Hour, ""},
	}
	for _, tc := range tests {
		result := relativeTime(time.Now().Add(-tc.d))
		if tc.want == "" {
			// Should be a date format
			if strings.Contains(result, "ago") {
				t.Errorf("relativeTime(%v) = %q, expected date format", tc.d, result)
			}
		} else if result != tc.want {
			t.Errorf("relativeTime(%v) = %q, want %q", tc.d, result, tc.want)
		}
	}

	if result := relativeTime(time.Time{}); result != "" {
		t.Errorf("relativeTime(zero) = %q, want empty", result)
	}
}
