package ui

import (
	"strings"
	"testing"

	"github.com/pgavlin/crtea/model"
	"github.com/pgavlin/crtea/theme"
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

	app := NewApp(nil, files, session, theme.Dark(), nil, nil)
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
