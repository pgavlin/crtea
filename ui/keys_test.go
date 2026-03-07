package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/pgavlin/crtea/model"
	"github.com/pgavlin/crtea/theme"
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
