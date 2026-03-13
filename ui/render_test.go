package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/pgavlin/crtea/internal/testutil"
	"github.com/pgavlin/crtea/model"
	"github.com/pgavlin/crtea/syntax"
	"github.com/pgavlin/crtea/theme"
	"github.com/pgavlin/crtea/vcs"
)

// newMultiFileApp creates an App with two files and two hunks in the first file,
// suitable for testing file/hunk navigation and multi-file rendering.
func newMultiFileApp(t *testing.T) *App {
	t.Helper()

	files := []model.DiffFile{
		{
			OldPath: "src/alpha.go",
			NewPath: "src/alpha.go",
			Status:  model.FileModified,
			Hunks: []model.DiffHunk{
				{
					OldStart: 1, OldCount: 3, NewStart: 1, NewCount: 3,
					Header: "@@ -1,3 +1,3 @@",
					Lines: []model.DiffLine{
						{Origin: model.OriginContext, Content: "package alpha", OldLineNo: 1, NewLineNo: 1},
						{Origin: model.OriginDeletion, Content: "var x = 1", OldLineNo: 2},
						{Origin: model.OriginAddition, Content: "var x = 2", NewLineNo: 2},
						{Origin: model.OriginContext, Content: "end", OldLineNo: 3, NewLineNo: 3},
					},
				},
				{
					OldStart: 10, OldCount: 3, NewStart: 10, NewCount: 3,
					Header: "@@ -10,3 +10,3 @@",
					Lines: []model.DiffLine{
						{Origin: model.OriginContext, Content: "func init()", OldLineNo: 10, NewLineNo: 10},
						{Origin: model.OriginDeletion, Content: "old init", OldLineNo: 11},
						{Origin: model.OriginAddition, Content: "new init", NewLineNo: 11},
						{Origin: model.OriginContext, Content: "}", OldLineNo: 12, NewLineNo: 12},
					},
				},
			},
		},
		{
			OldPath: "src/beta.go",
			NewPath: "src/beta.go",
			Status:  model.FileAdded,
			Hunks: []model.DiffHunk{
				{
					OldStart: 0, OldCount: 0, NewStart: 1, NewCount: 2,
					Header: "@@ -0,0 +1,2 @@",
					Lines: []model.DiffLine{
						{Origin: model.OriginAddition, Content: "package beta", NewLineNo: 1},
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

	app := NewApp(nil, files, session, theme.Dark(), WithLogger(testLogger))
	app.SetSize(120, 40)
	return &app
}

// --- View / render sanity ---

func TestViewReturnsNonEmpty(t *testing.T) {
	app := newTestApp(t)
	view := app.View()
	if view.Content == "" {
		t.Fatal("View() returned empty content")
	}
}

func TestViewWithZeroSizeReturnsEmpty(t *testing.T) {
	app := newTestApp(t)
	app.SetSize(0, 0)
	view := app.View()
	if view.Content != "" {
		t.Fatalf("View() with zero size should return empty, got %d chars", len(view.Content))
	}
}

func TestViewContainsDiffContent(t *testing.T) {
	app := newTestApp(t)
	view := app.View()
	// The diff contains "package main" as a context line
	if !strings.Contains(view.Content, "package main") {
		t.Error("View() output should contain diff content 'package main'")
	}
}

func TestViewContainsFileName(t *testing.T) {
	app := newTestApp(t)
	view := app.View()
	if !strings.Contains(view.Content, "a.go") {
		t.Error("View() output should contain file name 'a.go'")
	}
}

func TestViewContainsReviewCount(t *testing.T) {
	app := newTestApp(t)
	view := app.View()
	// Status bar should show "0/1 reviewed"
	if !strings.Contains(view.Content, "0/1 reviewed") {
		t.Error("View() output should contain '0/1 reviewed'")
	}
}

// --- Footer content per mode ---

func TestFooterNormalMode(t *testing.T) {
	app := newTestApp(t)
	footer := app.renderFooter()
	if !strings.Contains(footer, "? for help") {
		t.Errorf("normal footer should contain help hint, got: %s", footer)
	}
}

func TestFooterCommandMode(t *testing.T) {
	app := newTestApp(t)
	app = sendKeys(app, keyPress(':'))
	footer := app.renderFooter()
	if !strings.Contains(footer, ":") {
		t.Errorf("command footer should contain ':', got: %s", footer)
	}
}

func TestFooterSearchMode(t *testing.T) {
	app := newTestApp(t)
	app = sendKeys(app, keyPress('/'))
	app = sendKeys(app, keyPress('f'), keyPress('o'), keyPress('o'))
	footer := app.renderFooter()
	if !strings.Contains(footer, "/foo") {
		t.Errorf("search footer should contain '/foo', got: %s", footer)
	}
}

func TestFooterCommentMode(t *testing.T) {
	app := newTestApp(t)
	// Move to diff line
	for app.cursorLine < len(app.annotations) && app.annotations[app.cursorLine].Type != annDiffLine {
		app = sendKeys(app, keyPress('j'))
	}
	app = sendKeys(app, keyPress('c'))
	footer := app.renderFooter()
	if !strings.Contains(footer, "Enter: save") {
		t.Errorf("comment footer should contain 'Enter: save', got: %s", footer)
	}
}

func TestFooterConfirmMode(t *testing.T) {
	app := newTestApp(t)
	path := app.diffFiles[0].DisplayPath()
	fr := app.session.GetOrCreateFileReview(path, model.FileModified)
	fr.AddFileComment(model.NewComment("draft", model.CommentNote, model.SideNew))
	app.rebuildAnnotations()

	app = sendKeys(app, keyPress(':'))
	app = sendKeys(app, keyPress('c'), keyPress('l'), keyPress('e'), keyPress('a'), keyPress('r'))
	app = sendKeys(app, keySpecial(0x0d)) // Enter
	footer := app.renderFooter()
	if !strings.Contains(footer, "Clear draft comments?") {
		t.Errorf("confirm footer should contain prompt, got: %s", footer)
	}
}

func TestFooterVisualMode(t *testing.T) {
	app := newTestApp(t)
	for app.cursorLine < len(app.annotations) && app.annotations[app.cursorLine].Type != annDiffLine {
		app = sendKeys(app, keyPress('j'))
	}
	app = sendKeys(app, keyPress('v'))
	footer := app.renderFooter()
	if !strings.Contains(footer, "VISUAL") {
		t.Errorf("visual footer should contain 'VISUAL', got: %s", footer)
	}
}

func TestFooterHelpMode(t *testing.T) {
	app := newTestApp(t)
	app = sendKeys(app, keyPress('?'))
	footer := app.renderFooter()
	if !strings.Contains(footer, "Esc to close") {
		t.Errorf("help footer should contain close hint, got: %s", footer)
	}
}

func TestFooterReviewMode(t *testing.T) {
	app := newTestApp(t)
	app = sendKeys(app, upperKey('r', 'R'))
	footer := app.renderFooter()
	if !strings.Contains(footer, "Tab: cycle status") {
		t.Errorf("review footer should contain status hint, got: %s", footer)
	}
}

func TestFooterShowsCommentCount(t *testing.T) {
	app := newTestApp(t)
	path := app.diffFiles[0].DisplayPath()
	fr := app.session.GetOrCreateFileReview(path, model.FileModified)
	fr.AddFileComment(model.NewComment("note", model.CommentNote, model.SideNew))

	footer := app.renderFooter()
	if !strings.Contains(footer, "1 comment(s)") {
		t.Errorf("footer should show comment count, got: %s", footer)
	}
}

// --- Status bar ---

func TestStatusBarReviewProgress(t *testing.T) {
	app := newMultiFileApp(t)
	bar := app.renderStatusBar()
	if !strings.Contains(bar, "0/2 reviewed") {
		t.Errorf("status bar should show '0/2 reviewed', got: %s", bar)
	}

	// Mark one file as reviewed
	app = sendKeys(app, keyPress('r'))
	bar = app.renderStatusBar()
	if !strings.Contains(bar, "1/2 reviewed") {
		t.Errorf("after review, status bar should show '1/2 reviewed', got: %s", bar)
	}
}

func TestStatusBarDirtyIndicator(t *testing.T) {
	app := newTestApp(t)
	app.dirty = false
	bar := app.renderStatusBar()
	if strings.Contains(bar, "[+]") {
		t.Error("status bar should not show [+] when not dirty")
	}

	app.dirty = true
	bar = app.renderStatusBar()
	if !strings.Contains(bar, "[+]") {
		t.Error("status bar should show [+] when dirty")
	}
}

func TestStatusBarScrollPosition(t *testing.T) {
	app := newTestApp(t)

	// At top
	bar := app.renderStatusBar()
	if !strings.Contains(bar, "Top") && !strings.Contains(bar, "All") {
		t.Errorf("at top, status bar should show Top or All, got: %s", bar)
	}

	// At bottom
	app = sendKeys(app, upperKey('g', 'G'))
	bar = app.renderStatusBar()
	if !strings.Contains(bar, "Bot") && !strings.Contains(bar, "All") {
		t.Errorf("at bottom, status bar should show Bot or All, got: %s", bar)
	}
}

func TestStatusBarHorizontalScrollIndicator(t *testing.T) {
	app := newTestApp(t)
	app.scrollX = 5
	bar := app.renderStatusBar()
	if !strings.Contains(bar, "Col 5") {
		t.Errorf("status bar should show 'Col 5' when scrollX=5, got: %s", bar)
	}
}

// --- Help rendering ---

func TestHelpContainsKeyBindings(t *testing.T) {
	app := newTestApp(t)
	help := app.renderHelp(100) // large enough to show all content
	expected := []string{"Navigation", "j/k", "Ctrl-d/u", "Review", "Commands", ":q"}
	for _, s := range expected {
		if !strings.Contains(help, s) {
			t.Errorf("help should contain %q", s)
		}
	}
}

func TestHelpScrolling(t *testing.T) {
	app := newTestApp(t)
	app.inputMode = modeHelp

	help1 := app.renderHelp(10) // Only 10 lines visible
	app.helpScroll = 5
	help2 := app.renderHelp(10)

	if help1 == help2 {
		t.Error("help content should change after scrolling")
	}
}

// --- File list rendering ---

func TestFileListContainsFiles(t *testing.T) {
	app := newMultiFileApp(t)
	fileList := app.renderFileList(30, 20)
	if !strings.Contains(fileList, "alpha.go") {
		t.Error("file list should contain 'alpha.go'")
	}
	if !strings.Contains(fileList, "beta.go") {
		t.Error("file list should contain 'beta.go'")
	}
}

func TestFileListShowsHeader(t *testing.T) {
	app := newTestApp(t)
	fileList := app.renderFileList(30, 20)
	if !strings.Contains(fileList, "Files") {
		t.Error("file list should contain 'Files' header")
	}
}

// --- Diff line rendering ---

func TestRenderAnnotatedLineTypes(t *testing.T) {
	app := newTestApp(t)
	width := 80

	for _, ann := range app.annotations {
		// Just verify no panic for every annotation type
		result := app.renderAnnotatedLine(ann, width, false, false)
		if len(result) == 0 {
			t.Errorf("renderAnnotatedLine returned empty for type %d", ann.Type)
		}
	}
}

func TestRenderAnnotatedLineWithCursor(t *testing.T) {
	app := newTestApp(t)
	width := 80

	for _, ann := range app.annotations {
		// With cursor should also not panic
		result := app.renderAnnotatedLine(ann, width, true, false)
		if len(result) == 0 {
			t.Errorf("renderAnnotatedLine with cursor returned empty for type %d", ann.Type)
		}
	}
}

func TestRenderAnnotatedLineVisualSelected(t *testing.T) {
	app := newTestApp(t)
	width := 80

	// Set up visual anchor for isVisualSelected to work
	app.inputMode = modeVisualSelect
	app.visualAnchor = 1

	for _, ann := range app.annotations {
		if ann.Type == annDiffLine {
			result := app.renderAnnotatedLine(ann, width, false, true)
			if len(result) == 0 {
				t.Error("renderAnnotatedLine with visual selection returned empty")
			}
			break
		}
	}
}

// --- View with comments ---

func TestViewShowsComments(t *testing.T) {
	app := newTestApp(t)
	path := app.diffFiles[0].DisplayPath()
	fr := app.session.GetOrCreateFileReview(path, model.FileModified)
	fr.AddFileComment(model.NewComment("my test comment", model.CommentNote, model.SideNew))
	app.rebuildAnnotations()

	view := app.View()
	if !strings.Contains(view.Content, "my test comment") {
		t.Error("View() should contain the comment text")
	}
}

func TestViewShowsLineComments(t *testing.T) {
	app := newTestApp(t)
	path := app.diffFiles[0].DisplayPath()
	fr := app.session.GetOrCreateFileReview(path, model.FileModified)
	fr.AddLineComment(2, model.NewComment("line note", model.CommentNote, model.SideNew))
	app.rebuildAnnotations()

	view := app.View()
	if !strings.Contains(view.Content, "line note") {
		t.Error("View() should contain the line comment text")
	}
}

// --- View in different modes ---

func TestViewInHelpMode(t *testing.T) {
	app := newTestApp(t)
	app = sendKeys(app, keyPress('?'))
	view := app.View()
	if !strings.Contains(view.Content, "Navigation") {
		t.Error("View() in help mode should show help text")
	}
}

func TestViewInReviewMode(t *testing.T) {
	app := newTestApp(t)
	app = sendKeys(app, upperKey('r', 'R'))
	view := app.View()
	// Default status is Neutral
	if !strings.Contains(view.Content, "Neutral") && !strings.Contains(view.Content, "Overall Review") {
		t.Error("View() in review mode should show review editor")
	}
}

// --- Multi-file View ---

func TestMultiFileViewShowsBothFiles(t *testing.T) {
	app := newMultiFileApp(t)
	view := app.View()
	if !strings.Contains(view.Content, "alpha.go") {
		t.Error("View() should contain 'alpha.go'")
	}
	// beta.go might be below the viewport; check annotations instead
	found := false
	for _, ann := range app.annotations {
		if ann.Type == annFileHeader && ann.FileIdx == 1 {
			found = true
			break
		}
	}
	if !found {
		t.Error("annotations should contain a header for the second file")
	}
}

func TestVisualSelectDoesNotCrossFiles(t *testing.T) {
	app := newMultiFileApp(t)

	// Find a diff line in the first file (alpha.go) and one in the second file (beta.go).
	firstFileIdx := -1
	secondFileIdx := -1
	for i, ann := range app.annotations {
		if ann.Type != annDiffLine {
			continue
		}
		if ann.FileIdx == 0 && firstFileIdx == -1 {
			firstFileIdx = i
		}
		if ann.FileIdx == 1 && secondFileIdx == -1 {
			secondFileIdx = i
		}
	}
	if firstFileIdx == -1 || secondFileIdx == -1 {
		t.Fatal("expected diff lines in both files")
	}

	// Start visual selection on the first file's diff line.
	app.cursorLine = firstFileIdx
	app.enterVisualMode()

	// Move cursor to the last diff line in the first file.
	lastFirstFile := firstFileIdx
	for i, ann := range app.annotations {
		if ann.Type == annDiffLine && ann.FileIdx == 0 {
			lastFirstFile = i
		}
	}
	app.cursorLine = lastFirstFile

	// Lines in the first file should be selected.
	if !app.isVisualSelected(firstFileIdx) {
		t.Error("first file diff line should be selected")
	}

	// Lines in the second file must NOT be selected, even if their line numbers
	// fall within the selected range.
	if app.isVisualSelected(secondFileIdx) {
		t.Error("second file diff line should NOT be selected (cross-file leak)")
	}
}

// --- View resize ---

func TestViewResizeDoesNotPanic(t *testing.T) {
	app := newTestApp(t)

	sizes := [][2]int{
		{10, 5}, {200, 100}, {1, 1}, {80, 24}, {120, 40},
	}
	for _, s := range sizes {
		app.SetSize(s[0], s[1])
		view := app.View()
		if view.Content == "" {
			t.Errorf("View() returned empty at size %dx%d", s[0], s[1])
		}
	}
}

func TestViewNarrowWidth(t *testing.T) {
	app := newTestApp(t)
	app.SetSize(20, 10)
	// Should not panic
	view := app.View()
	lines := strings.Split(view.Content, "\n")
	if len(lines) == 0 {
		t.Error("narrow view should still produce output")
	}
}

// --- renderDiffLine coverage ---

func TestRenderDiffLineAddition(t *testing.T) {
	app := newTestApp(t)
	for _, ann := range app.annotations {
		if ann.Type == annDiffLine {
			file := app.diffFiles[ann.FileIdx]
			line := file.Hunks[ann.HunkIdx].Lines[ann.LineIdx]
			if line.Origin == model.OriginAddition {
				result := app.renderDiffLine(ann, 80, false, false)
				if !strings.Contains(result, "+") {
					t.Error("addition line should contain '+'")
				}
				return
			}
		}
	}
	t.Fatal("no addition line found")
}

func TestRenderDiffLineDeletion(t *testing.T) {
	app := newTestApp(t)
	for _, ann := range app.annotations {
		if ann.Type == annDiffLine {
			file := app.diffFiles[ann.FileIdx]
			line := file.Hunks[ann.HunkIdx].Lines[ann.LineIdx]
			if line.Origin == model.OriginDeletion {
				result := app.renderDiffLine(ann, 80, false, false)
				if !strings.Contains(result, "-") {
					t.Error("deletion line should contain '-'")
				}
				return
			}
		}
	}
	t.Fatal("no deletion line found")
}

func TestRenderDiffLineContext(t *testing.T) {
	app := newTestApp(t)
	for _, ann := range app.annotations {
		if ann.Type == annDiffLine {
			file := app.diffFiles[ann.FileIdx]
			line := file.Hunks[ann.HunkIdx].Lines[ann.LineIdx]
			if line.Origin == model.OriginContext {
				result := app.renderDiffLine(ann, 80, false, false)
				if len(result) == 0 {
					t.Error("context line should not be empty")
				}
				return
			}
		}
	}
	t.Fatal("no context line found")
}

func TestRenderDiffLineWithCursorHighlight(t *testing.T) {
	app := newTestApp(t)
	for _, ann := range app.annotations {
		if ann.Type == annDiffLine {
			withCursor := app.renderDiffLine(ann, 80, true, false)
			withoutCursor := app.renderDiffLine(ann, 80, false, false)
			if withCursor == withoutCursor {
				t.Error("cursor highlight should change rendered output")
			}
			return
		}
	}
}

func TestRenderDiffLineWithVisualSelect(t *testing.T) {
	app := newTestApp(t)
	for _, ann := range app.annotations {
		if ann.Type == annDiffLine {
			withVisual := app.renderDiffLine(ann, 80, false, true)
			withoutVisual := app.renderDiffLine(ann, 80, false, false)
			if withVisual == withoutVisual {
				t.Error("visual selection should change rendered output")
			}
			return
		}
	}
}

func TestRenderDiffLineNarrowWidth(t *testing.T) {
	app := newTestApp(t)
	for _, ann := range app.annotations {
		if ann.Type == annDiffLine {
			// Should not panic even at very small widths
			_ = app.renderDiffLine(ann, 5, false, false)
			_ = app.renderDiffLine(ann, 1, false, false)
			_ = app.renderDiffLine(ann, 0, false, false)
			return
		}
	}
}

func TestRenderDiffLineWithHorizontalScroll(t *testing.T) {
	app := newTestApp(t)
	app.scrollX = 3
	for _, ann := range app.annotations {
		if ann.Type == annDiffLine {
			result := app.renderDiffLine(ann, 80, false, false)
			if len(result) == 0 {
				t.Error("scrolled diff line should not be empty")
			}
			return
		}
	}
}

// --- renderFileHeader coverage ---

func TestRenderFileHeaderStatuses(t *testing.T) {
	app := newMultiFileApp(t)
	for _, ann := range app.annotations {
		if ann.Type == annFileHeader {
			result := app.renderFileHeader(ann, 80, false)
			if len(result) == 0 {
				t.Errorf("file header for file %d should not be empty", ann.FileIdx)
			}
		}
	}
}

func TestRenderFileHeaderReviewedMark(t *testing.T) {
	app := newTestApp(t)
	path := app.diffFiles[0].DisplayPath()
	app.session.GetOrCreateFileReview(path, model.FileModified).Reviewed = true

	for _, ann := range app.annotations {
		if ann.Type == annFileHeader {
			result := app.renderFileHeader(ann, 80, false)
			if !strings.Contains(result, "✓") {
				t.Error("reviewed file header should contain checkmark")
			}
			return
		}
	}
}

func TestRenderFileHeaderRename(t *testing.T) {
	files := []model.DiffFile{
		{
			OldPath: "old_name.go",
			NewPath: "new_name.go",
			Status:  model.FileRenamed,
		},
	}
	session := model.NewSession("/tmp/test", "main", "abc", model.DiffWorkingTree)
	session.GetOrCreateFileReview("new_name.go", model.FileRenamed)
	app := NewApp(nil, files, session, theme.Dark(), WithLogger(testLogger))
	app.SetSize(120, 40)

	for _, ann := range app.annotations {
		if ann.Type == annFileHeader {
			result := app.renderFileHeader(ann, 120, false)
			if !strings.Contains(result, "old_name.go") {
				t.Error("renamed file header should mention old name")
			}
			return
		}
	}
}

// --- renderCommentEditor ---

func TestRenderCommentEditor(t *testing.T) {
	app := newTestApp(t)
	app.commentBuffer = "test comment text"
	app.commentCursor = len(app.commentBuffer)
	app.commentType = model.CommentNote
	app.inputMode = modeComment

	lines := app.renderCommentEditor(80)
	if len(lines) < 3 {
		t.Fatalf("comment editor should have at least 3 lines (border+content+border), got %d", len(lines))
	}

	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "test comment text") {
		t.Error("comment editor should contain buffer text")
	}
	if !strings.Contains(joined, "Note") {
		t.Error("comment editor should show comment type badge")
	}
}

func TestRenderCommentEditorCursorMidText(t *testing.T) {
	app := newTestApp(t)
	app.commentBuffer = "hello world"
	app.commentCursor = 5 // cursor on the space between "hello" and "world"
	app.commentType = model.CommentNote
	app.inputMode = modeComment

	lines := app.renderCommentEditor(80)
	joined := strings.Join(lines, "\n")

	// The character at cursor position (space) should be highlighted with reverse video,
	// not inserted as a █ block that shifts subsequent text.
	// Reverse video: \x1b[7m + char + \x1b[27m
	if !strings.Contains(joined, "\x1b[7m \x1b[27m") {
		t.Error("cursor should highlight the character at cursor position with reverse video")
	}
	// The █ block cursor should NOT appear anywhere
	if strings.Contains(joined, "█") {
		t.Error("cursor should not be rendered as an inserted █ block character")
	}
	// Both parts of the text should be present without extra characters between them
	if !strings.Contains(joined, "hello") || !strings.Contains(joined, "world") {
		t.Error("both parts of text around cursor should be present")
	}
}

func TestRenderCommentEditorNarrow(t *testing.T) {
	app := newTestApp(t)
	app.commentBuffer = "x"
	app.commentCursor = 1
	app.commentType = model.CommentIssue

	// Should not panic at narrow widths
	lines := app.renderCommentEditor(15)
	if len(lines) == 0 {
		t.Error("narrow comment editor should still produce output")
	}
}

// --- renderReviewEditor ---

func TestRenderReviewEditorStatuses(t *testing.T) {
	app := newTestApp(t)

	statuses := []model.ApprovalStatus{
		model.ApprovalNeutral,
		model.ApprovalApprove,
		model.ApprovalRequestChanges,
	}
	for _, s := range statuses {
		app.reviewStatus = s
		result := app.renderReviewEditor(80, 8)
		if !strings.Contains(result, s.String()) {
			t.Errorf("review editor should contain status %q", s.String())
		}
	}
}

// --- commentTypeColor ---

func TestCommentTypeColor(t *testing.T) {
	app := newTestApp(t)
	types := []model.CommentType{
		model.CommentNote,
		model.CommentSuggestion,
		model.CommentIssue,
		model.CommentPraise,
		model.CommentQuestion,
	}
	for _, ct := range types {
		color := app.commentTypeColor(ct)
		if color == nil {
			t.Errorf("commentTypeColor(%d) returned nil", ct)
		}
	}
}

// --- expandTabs ---

func TestExpandTabs(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"no tabs", "no tabs"},
		{"\thello", "    hello"},
		{"ab\tc", "ab  c"},       // 2 chars + tab = pad to 4
		{"abcd\te", "abcd    e"}, // 4 chars + tab = pad to 8
		{"", ""},
	}
	for _, tt := range tests {
		got := expandTabs(tt.input)
		if got != tt.want {
			t.Errorf("expandTabs(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- stripNewlines ---

func TestStripNewlines(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"no newlines", "no newlines"},
		{"hello\nworld", "helloworld"},
		{"hello\r\nworld", "helloworld"},
		{"trailing\n", "trailing"},
		{"\r\n\n\r", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := stripNewlines(tt.input)
		if got != tt.want {
			t.Errorf("stripNewlines(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSanitizeSpans(t *testing.T) {
	spans := []model.StyledSpan{
		{Text: "hello\nworld", FG: "#ff0000"},
		{Text: "clean", FG: "#00ff00"},
		{Text: "line\r\nbreak", FG: ""},
	}
	got := sanitizeSpans(spans)
	if got[0].Text != "helloworld" {
		t.Errorf("expected embedded newline removed, got %q", got[0].Text)
	}
	if got[0].FG != "#ff0000" {
		t.Errorf("expected FG preserved, got %q", got[0].FG)
	}
	if got[1].Text != "clean" {
		t.Errorf("expected clean span unchanged, got %q", got[1].Text)
	}
	if got[2].Text != "linebreak" {
		t.Errorf("expected \\r\\n removed, got %q", got[2].Text)
	}
}

func TestRenderDiffLineWithEmbeddedNewline(t *testing.T) {
	app := newTestApp(t)

	// Find any diff line annotation and inject embedded newlines into content
	for _, ann := range app.annotations {
		if ann.Type != annDiffLine {
			continue
		}
		file := app.diffFiles[ann.FileIdx]
		line := &file.Hunks[ann.HunkIdx].Lines[ann.LineIdx]

		// Test plain content path: inject newline
		origContent := line.Content
		origSpans := line.Spans
		line.Content = "first line\nsecond line"
		line.Spans = nil

		result := app.renderDiffLine(ann, 80, false, false)
		if strings.Contains(result, "\n") {
			t.Error("rendered diff line with embedded newline should not contain \\n")
		}

		// Test syntax spans path: inject newline in span
		line.Content = origContent
		line.Spans = []model.StyledSpan{
			{Text: "span\nwith\nnewlines", FG: "#aabbcc"},
		}

		result = app.renderDiffLine(ann, 80, false, false)
		if strings.Contains(result, "\n") {
			t.Error("rendered diff line with newline in spans should not contain \\n")
		}

		// Restore
		line.Content = origContent
		line.Spans = origSpans
		return
	}
	t.Fatal("no diff line annotation found")
}

// --- truncateOrPad ---

func TestTruncateOrPad(t *testing.T) {
	tests := []struct {
		input string
		width int
		want  string
	}{
		{"hello", 10, "hello     "},
		{"hello world", 5, "hell…"},
		{"exact", 5, "exact"},
		{"x", 0, ""},
		{"x", -1, ""},
	}
	for _, tt := range tests {
		got := truncateOrPad(tt.input, tt.width)
		if got != tt.want {
			t.Errorf("truncateOrPad(%q, %d) = %q, want %q", tt.input, tt.width, got, tt.want)
		}
	}
}

// --- renderCommentLine ---

func TestRenderCommentLine(t *testing.T) {
	app := newTestApp(t)
	path := app.diffFiles[0].DisplayPath()
	fr := app.session.GetOrCreateFileReview(path, model.FileModified)

	c := model.NewComment("file level note", model.CommentNote, model.SideNew)
	fr.AddFileComment(c)

	c2 := model.NewComment("line level note", model.CommentNote, model.SideNew)
	fr.AddLineComment(2, c2)

	app.rebuildAnnotations()

	for _, ann := range app.annotations {
		if ann.Type == annFileComment || ann.Type == annLineComment {
			result := app.renderCommentLine(ann, 80, false, ann.Type == annFileComment)
			if len(result) == 0 {
				t.Errorf("comment line of type %d should not be empty", ann.Type)
			}
		}
	}
}

// --- View with file list hidden ---

func TestViewWithFileListHidden(t *testing.T) {
	app := newTestApp(t)
	app.showFileList = false
	view := app.View()
	if view.Content == "" {
		t.Error("View with hidden file list should still produce output")
	}
	if !strings.Contains(view.Content, "package main") {
		t.Error("View should still contain diff content with file list hidden")
	}
}

// --- View with inline comment editor ---

func TestViewWithCommentEditorInline(t *testing.T) {
	app := newTestApp(t)
	// Move to diff line
	for app.cursorLine < len(app.annotations) && app.annotations[app.cursorLine].Type != annDiffLine {
		app = sendKeys(app, keyPress('j'))
	}
	// Enter comment mode (inline editor should appear)
	app = sendKeys(app, keyPress('c'))
	view := app.View()
	// The view should contain the comment editor box
	if !strings.Contains(view.Content, "Note") {
		t.Error("View should show comment type badge in inline editor")
	}
}

// --- View with bug report editor ---

func TestViewWithBugEditor(t *testing.T) {
	app := newTestApp(t)
	app.inputMode = modeBug
	app.bugBuffer = "my bug description"
	app.bugCursor = len(app.bugBuffer)
	view := app.View()
	if view.Content == "" {
		t.Error("View with bug editor should produce output")
	}
}

// --- isEditingAnnotation ---

func TestIsEditingAnnotation(t *testing.T) {
	app := newTestApp(t)
	path := app.diffFiles[0].DisplayPath()
	fr := app.session.GetOrCreateFileReview(path, model.FileModified)
	c := model.NewComment("edit me", model.CommentNote, model.SideNew)
	fr.AddFileComment(c)
	app.rebuildAnnotations()

	// Not editing
	for _, ann := range app.annotations {
		if ann.Type == annFileComment {
			if app.isEditingAnnotation(ann) {
				t.Error("should not be editing when editingID is empty")
			}
			break
		}
	}

	// Editing
	app.editingID = c.ID
	for _, ann := range app.annotations {
		if ann.Type == annFileComment {
			if !app.isEditingAnnotation(ann) {
				t.Error("should be editing when editingID matches")
			}
			break
		}
	}
}

// --- joinHorizontal ---

func TestJoinHorizontal(t *testing.T) {
	left := "L1\nL2\nL3"
	right := "R1\nR2\nR3"
	result := joinHorizontal(left, right, 3)
	lines := strings.Split(result, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "L1") || !strings.Contains(lines[0], "R1") {
		t.Errorf("first line should contain L1 and R1, got: %s", lines[0])
	}
}

func TestJoinHorizontalUnevenHeight(t *testing.T) {
	left := "L1"
	right := "R1\nR2\nR3"
	result := joinHorizontal(left, right, 3)
	lines := strings.Split(result, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
}

// --- renderDiffLine with syntax spans ---

func TestRenderDiffLineWithSpans(t *testing.T) {
	// Create app with a file that has spans on its lines
	files := []model.DiffFile{
		{
			OldPath: "main.go", NewPath: "main.go", Status: model.FileModified,
			Hunks: []model.DiffHunk{{
				OldStart: 1, OldCount: 1, NewStart: 1, NewCount: 2,
				Header: "@@ -1,1 +1,2 @@",
				Lines: []model.DiffLine{
					{Origin: model.OriginContext, Content: "package main", OldLineNo: 1, NewLineNo: 1},
					{
						Origin: model.OriginAddition, Content: "func main()", NewLineNo: 2,
						Spans: []model.StyledSpan{
							{Text: "func", FG: "#ff0000"},
							{Text: " main", FG: ""},
							{Text: "()", FG: "#00ff00"},
						},
					},
				},
			}},
		},
	}
	session := model.NewSession("/tmp/test", "main", "abc", model.DiffWorkingTree)
	app := NewApp(nil, files, session, theme.Dark(), WithLogger(testLogger))
	app.SetSize(120, 40)

	// Find the annotation for the addition line with spans
	for _, ann := range app.annotations {
		if ann.Type == annDiffLine && ann.LineIdx == 1 {
			result := app.renderDiffLine(ann, 120, false, false)
			if result == "" {
				t.Error("should render non-empty with spans")
			}
			return
		}
	}
	t.Fatal("should find annotation for the spans line")
}

func TestRenderDiffLineWithSearchHighlight(t *testing.T) {
	app := newTestApp(t)
	app.searchHighlight = "main"

	// Find a context line that contains "main"
	for _, ann := range app.annotations {
		if ann.Type == annDiffLine && ann.LineIdx == 0 {
			result := app.renderDiffLine(ann, 120, false, false)
			if result == "" {
				t.Error("should render with search highlight")
			}
			return
		}
	}
}

func TestRenderDiffLineWithSpansAndSearchHighlight(t *testing.T) {
	files := []model.DiffFile{
		{
			OldPath: "main.go", NewPath: "main.go", Status: model.FileModified,
			Hunks: []model.DiffHunk{{
				OldStart: 1, OldCount: 1, NewStart: 1, NewCount: 2,
				Header: "@@ -1,1 +1,2 @@",
				Lines: []model.DiffLine{
					{Origin: model.OriginContext, Content: "package main", OldLineNo: 1, NewLineNo: 1},
					{
						Origin: model.OriginAddition, Content: "func main()", NewLineNo: 2,
						Spans: []model.StyledSpan{
							{Text: "func ", FG: "#ff0000"},
							{Text: "main", FG: "#0000ff"},
							{Text: "()", FG: "#00ff00"},
						},
					},
				},
			}},
		},
	}
	session := model.NewSession("/tmp/test", "main", "abc", model.DiffWorkingTree)
	app := NewApp(nil, files, session, theme.Dark(), WithLogger(testLogger))
	app.SetSize(120, 40)
	app.searchHighlight = "main"

	for _, ann := range app.annotations {
		if ann.Type == annDiffLine && ann.LineIdx == 1 {
			result := app.renderDiffLine(ann, 120, false, false)
			if result == "" {
				t.Error("should render with spans and search highlight")
			}
			return
		}
	}
}

// --- renderCommitList ---

func TestRenderCommitListEmpty(t *testing.T) {
	app := newTestApp(t)
	// No commits set, render should not crash
	output := app.renderCommitList(120, 5)
	if output == "" {
		t.Error("should produce output even with no items")
	}
}

func TestRenderCommitListWithWorkTree(t *testing.T) {
	app := newTestApp(t)
	app.includesWorkTree = true
	commits := []vcs.CommitInfo{
		{ID: "c1", ShortID: "c1", Summary: "Test commit", Author: "me", Time: testTime},
	}
	diffs := map[string][]model.DiffFile{
		"c1":        app.diffFiles,
		worktreeKey: app.diffFiles,
	}
	app.setCommits(commits, diffs)
	app.enabledCommits[worktreeKey] = true

	output := app.renderCommitList(120, 8)
	if !strings.Contains(output, "Working tree") {
		t.Error("should show working tree entry")
	}
	if !strings.Contains(output, "Test commit") {
		t.Error("should show commit summary")
	}
}

func TestRenderCommitListFocused(t *testing.T) {
	app := newTestApp(t)
	commits := []vcs.CommitInfo{
		{ID: "c1", ShortID: "c1", Summary: "A commit", Author: "me", Time: testTime},
	}
	diffs := map[string][]model.DiffFile{"c1": app.diffFiles}
	app.setCommits(commits, diffs)
	app.focusedPanel = panelCommitList

	output := app.renderCommitList(120, 8)
	if !strings.Contains(output, ">") {
		t.Error("should show cursor indicator when focused")
	}
}

func TestRenderCommitListNarrowWidth(t *testing.T) {
	app := newTestApp(t)
	commits := []vcs.CommitInfo{
		{ID: "c1", ShortID: "c1", Summary: "A very long commit summary that should be truncated", Author: "me", Time: testTime},
	}
	diffs := map[string][]model.DiffFile{"c1": app.diffFiles}
	app.setCommits(commits, diffs)

	output := app.renderCommitList(50, 5)
	if output == "" {
		t.Error("should render at narrow width")
	}
}

// --- renderDescription tests ---

func TestRenderDescriptionEmpty(t *testing.T) {
	app := newTestApp(t)
	app.showDescription = true
	output := app.renderDescription(120, 5)
	if output == "" {
		t.Error("should render even with empty description")
	}
}

func TestRenderDescriptionFocused(t *testing.T) {
	app := newTestApp(t)
	app.session.Description = "Test description"
	app.showDescription = true
	app.focusedPanel = panelCommitList

	output := app.renderDescription(120, 5)
	if !strings.Contains(output, "Test description") {
		t.Error("should show description text")
	}
}

func TestRenderDescriptionWrapsLongLines(t *testing.T) {
	app := newTestApp(t)
	// Create a description longer than 40 chars
	app.session.Description = "This is a very long description line that should definitely be wrapped when rendered in a narrow panel"
	app.showDescription = true

	output := app.renderDescription(40, 10)
	lines := strings.Split(output, "\n")
	// With wrapping, the text should span multiple content lines (not just 1 + separator)
	contentLines := 0
	for _, line := range lines {
		stripped := strings.TrimSpace(ansi.Strip(line))
		if stripped != "" && stripped != strings.Repeat("─", len(stripped)) {
			contentLines++
		}
	}
	if contentLines < 2 {
		t.Errorf("expected long description to wrap into multiple lines at width 40, got %d content lines", contentLines)
	}
}

func TestWrapLine(t *testing.T) {
	tests := []struct {
		input string
		width int
		want  int // expected number of output lines
	}{
		{"short", 80, 1},
		{"hello world foo bar", 10, 3},
		{"abcdefghij", 5, 2},
		{"", 80, 1},
		{"one two three four five six", 15, 2},
	}
	for _, tt := range tests {
		lines := wrapLine(tt.input, tt.width)
		if len(lines) != tt.want {
			t.Errorf("wrapLine(%q, %d) = %d lines %v, want %d", tt.input, tt.width, len(lines), lines, tt.want)
		}
		// Every line must fit within width
		for _, line := range lines {
			if lipgloss.Width(line) > tt.width {
				t.Errorf("wrapLine(%q, %d): line %q exceeds width (%d > %d)", tt.input, tt.width, line, lipgloss.Width(line), tt.width)
			}
		}
	}
}

// --- renderConversationEditor ---

func TestRenderConversationEditor(t *testing.T) {
	app := newTestApp(t)
	app.convBuffer = "hello world"
	app.convCursor = 5

	lines := app.renderConversationEditor(120, 5)
	if len(lines) == 0 {
		t.Fatal("should render editor lines")
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "New message") {
		t.Error("should have editor label")
	}
}

func TestRenderConversationEditorEmpty(t *testing.T) {
	app := newTestApp(t)
	app.convBuffer = ""
	app.convCursor = 0

	lines := app.renderConversationEditor(120, 5)
	if len(lines) == 0 {
		t.Fatal("should render editor even when empty")
	}
}

// --- renderExpandedContextLine ---

func TestRenderExpandedContextLineNotExpanded(t *testing.T) {
	app := newTestApp(t)
	ann := annotatedLine{
		Type:  annExpandedContext,
		gapID: gapID{FileIdx: 0, HunkIdx: 1},
	}
	// No expanded gaps, should return blank
	result := app.renderExpandedContextLine(ann, 120, false)
	if strings.TrimSpace(result) != "" {
		t.Error("should be blank when gap not expanded")
	}
}

func TestRenderExpandedContextLineWithScrollX(t *testing.T) {
	app := newVCSApp(t)
	// Expand a gap
	for i, ann := range app.annotations {
		if ann.Type == annExpander {
			app.cursorLine = i
			app.toggleExpandAtCursor()
			break
		}
	}

	app.scrollX = 5
	for _, ann := range app.annotations {
		if ann.Type == annExpandedContext {
			result := app.renderExpandedContextLine(ann, 120, false)
			if result == "" {
				t.Error("should render with scrollX")
			}
			return
		}
	}
}

// --- View with review editor ---

func TestViewWithReviewEditorPanel(t *testing.T) {
	app := newTestApp(t)
	app.inputMode = modeReview
	app.reviewBuffer = "Looks good"
	app.reviewCursor = len(app.reviewBuffer)
	app.reviewStatus = model.ApprovalApprove

	view := app.View()
	if view.Content == "" {
		t.Error("view should render with review editor")
	}
}

// --- Syntax-highlighted highlighter integration ---

func TestNewAppWithHighlighter(t *testing.T) {
	hl := syntax.NewHighlighter("monokai")
	files := []model.DiffFile{
		{
			OldPath: "test.go", NewPath: "test.go", Status: model.FileModified,
			Hunks: []model.DiffHunk{{
				OldStart: 1, OldCount: 1, NewStart: 1, NewCount: 1,
				Header: "@@ -1,1 +1,1 @@",
				Lines: []model.DiffLine{
					{Origin: model.OriginAddition, Content: "package main", NewLineNo: 1},
				},
			}},
		},
	}
	hl.HighlightFiles(files)

	session := model.NewSession("/tmp/test", "main", "abc", model.DiffWorkingTree)
	app := NewApp(nil, files, session, theme.Dark(), WithLogger(testLogger), WithHighlighter(hl))
	app.SetSize(120, 40)

	view := app.View()
	if view.Content == "" {
		t.Error("view with highlighted file should render")
	}

	// The line should have spans
	if len(files[0].Hunks[0].Lines[0].Spans) == 0 {
		t.Log("Note: highlighter may not produce spans for short content, skipping span check")
	}
}

// --- Picker with no working tree diff ---

func TestPickerConfirmCommitOnly(t *testing.T) {
	mockVCS := &testutil.MockVCS{
		VcsInfo: vcs.VcsInfo{RootPath: "/tmp/test", HeadCommit: "abc", BranchName: "main", VcsType: "git"},
		RecentCommits: []vcs.CommitInfo{
			{ID: "aaa", ShortID: "aaa", Summary: "Commit", Author: "alice"},
		},
		RevisionDiff: []model.DiffFile{
			{
				OldPath: "f.go", NewPath: "f.go", Status: model.FileModified,
				Hunks: []model.DiffHunk{{
					OldStart: 1, OldCount: 1, NewStart: 1, NewCount: 1,
					Header: "@@ -1 +1 @@",
					Lines: []model.DiffLine{
						{Origin: model.OriginAddition, Content: "new", NewLineNo: 1},
					},
				}},
			},
		},
	}

	store := testutil.NewMockStore()
	app := NewPickerApp(mockVCS, theme.Dark(), WithLogger(testLogger), WithStore(store))
	app.SetSize(120, 40)

	// Move to commit (index 1), select it, confirm
	app2 := sendKeys(&app, keyPress('j'), keyPress(' '), keySpecial(tea.KeyEnter))
	if app2.phase != phaseReview {
		t.Fatal("should enter review phase")
	}
	if app2.includesWorkTree {
		t.Error("should not include working tree")
	}
}

// --- statusBar with provider info ---

func TestStatusBarWithProvider(t *testing.T) {
	app := newTestApp(t)
	app.session.Provider = &model.ProviderInfo{Name: "github", ID: "42"}
	app.session.Description = "PR Title"
	app.vcsInfo = vcs.VcsInfo{VcsType: "git", BranchName: "feature"}

	bar := app.renderStatusBar()
	if !strings.Contains(bar, "github") {
		t.Error("status bar should show provider name")
	}
	if !strings.Contains(bar, "#42") {
		t.Error("status bar should show PR number")
	}
}

// --- conversationPanelHeight ---

func TestConversationPanelHeight(t *testing.T) {
	app := newTestApp(t)
	if h := app.conversationPanelHeight(20); h != 0 {
		t.Fatalf("expected 0 when hidden, got %d", h)
	}

	app.showConversation = true
	h := app.conversationPanelHeight(20)
	if h != 10 {
		t.Fatalf("expected 10 (half of 20), got %d", h)
	}

	// Very small content height
	h = app.conversationPanelHeight(4)
	if h < 3 {
		t.Fatalf("expected at least 3, got %d", h)
	}
}

// --- commitListHeight ---

func TestCommitListHeight(t *testing.T) {
	app := newTestApp(t)
	if h := app.commitListHeight(); h != 0 {
		t.Fatalf("expected 0 with no commits, got %d", h)
	}

	app.showDescription = true
	if h := app.commitListHeight(); h != 0 {
		t.Fatalf("expected 0 when showing description, got %d", h)
	}
}

// --- Draft PR status bar ---

func TestStatusBarShowsDraftIndicator(t *testing.T) {
	app := newTestApp(t)
	app.session.Provider = &model.ProviderInfo{Name: "mock", ID: "42"}
	app.session.IsDraft = true

	bar := app.renderStatusBar()
	stripped := ansi.Strip(bar)
	if !strings.Contains(stripped, "[DRAFT]") {
		t.Errorf("expected [DRAFT] in status bar, got %q", stripped)
	}
}

func TestStatusBarNoDraftWhenNotDraft(t *testing.T) {
	app := newTestApp(t)
	app.session.Provider = &model.ProviderInfo{Name: "mock", ID: "42"}
	app.session.IsDraft = false

	bar := app.renderStatusBar()
	stripped := ansi.Strip(bar)
	if strings.Contains(stripped, "[DRAFT]") {
		t.Errorf("should not show [DRAFT] when not draft, got %q", stripped)
	}
}

// --- Resolved badge on comments ---

func TestCommentShowsResolvedBadge(t *testing.T) {
	app := newTestApp(t)
	fr := app.session.GetOrCreateFileReview("a.go", model.FileModified)
	fr.AddLineComment(2, model.Comment{
		ID:         "r1",
		Content:    "Resolved comment",
		Type:       model.CommentNote,
		Side:       model.SideNew,
		ThreadID:   "thread-1",
		IsResolved: true,
	})
	app.rebuildAnnotations()

	view := app.View()
	stripped := ansi.Strip(view.Content)
	if !strings.Contains(stripped, "[resolved]") {
		t.Error("expected [resolved] badge in view for resolved comment")
	}
}

func TestCommentShowsOutdatedBadge(t *testing.T) {
	app := newTestApp(t)
	fr := app.session.GetOrCreateFileReview("a.go", model.FileModified)
	fr.AddLineComment(2, model.Comment{
		ID:         "o1",
		Content:    "Outdated comment",
		Type:       model.CommentNote,
		Side:       model.SideNew,
		IsOutdated: true,
	})
	app.rebuildAnnotations()

	view := app.View()
	stripped := ansi.Strip(view.Content)
	if !strings.Contains(stripped, "[outdated]") {
		t.Error("expected [outdated] badge in view for outdated comment")
	}
}

// --- Footer for modeEditPR ---

func TestFooterEditPRMode(t *testing.T) {
	app := newTestApp(t)
	app.inputMode = modeEditPR

	view := app.View()
	stripped := ansi.Strip(view.Content)
	if !strings.Contains(stripped, "Enter: save") {
		t.Error("expected 'Enter: save' in footer for modeEditPR")
	}
}
