package ui

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/pgavlin/crtea/internal/testutil"
	"github.com/pgavlin/crtea/model"
	"github.com/pgavlin/crtea/provider"
	"github.com/pgavlin/crtea/theme"
	"github.com/pgavlin/crtea/vcs"
)

var update = flag.Bool("update", false, "update golden files")

// snapshot captures the current View output, strips ANSI codes, and returns it.
func snapshot(app *App) string {
	view := app.View()
	return ansi.Strip(view.Content)
}

// assertGolden compares output against a golden file. If -update is passed,
// it writes the golden file instead.
func assertGolden(t *testing.T, name, got string) {
	t.Helper()

	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden file %s: %v\nRun with -update to create it.", path, err)
	}
	if got != string(want) {
		// Find first differing line for a useful error message.
		gotLines := strings.Split(got, "\n")
		wantLines := strings.Split(string(want), "\n")
		for i := 0; i < len(gotLines) || i < len(wantLines); i++ {
			g, w := "", ""
			if i < len(gotLines) {
				g = gotLines[i]
			}
			if i < len(wantLines) {
				w = wantLines[i]
			}
			if g != w {
				t.Errorf("golden mismatch at line %d:\n  got:  %q\n  want: %q\nRun with -update to regenerate.", i+1, g, w)
				return
			}
		}
		t.Errorf("golden mismatch (different lengths: got %d, want %d lines)\nRun with -update to regenerate.",
			len(gotLines), len(wantLines))
	}
}

// newScenarioApp creates an app suitable for scenario tests: a two-hunk file
// with a VCS backend (for context expansion), a mock provider, and a mock store.
// The diff represents a modified Go file with changes at lines 2 and 10.
func newScenarioApp(t *testing.T) (*App, *testutil.MockProvider, *testutil.MockVCS) {
	t.Helper()

	mockVCS := &testutil.MockVCS{
		VcsInfo: vcs.VcsInfo{
			RootPath:   "/tmp/test",
			HeadCommit: "abc123",
			BranchName: "feature",
			VcsType:    "git",
		},
		ContextLines: []model.DiffLine{
			{Origin: model.OriginContext, Content: "// between hunks line 1", OldLineNo: 4, NewLineNo: 4},
			{Origin: model.OriginContext, Content: "// between hunks line 2", OldLineNo: 5, NewLineNo: 5},
			{Origin: model.OriginContext, Content: "// between hunks line 3", OldLineNo: 6, NewLineNo: 6},
			{Origin: model.OriginContext, Content: "func helper() {}", OldLineNo: 7, NewLineNo: 7},
			{Origin: model.OriginContext, Content: "", OldLineNo: 8, NewLineNo: 8},
			{Origin: model.OriginContext, Content: "// setup", OldLineNo: 9, NewLineNo: 9},
		},
	}

	files := []model.DiffFile{
		{
			OldPath: "auth/handler.go",
			NewPath: "auth/handler.go",
			Status:  model.FileModified,
			Hunks: []model.DiffHunk{
				{
					OldStart: 1, OldCount: 3, NewStart: 1, NewCount: 3,
					Header: "@@ -1,3 +1,3 @@",
					Lines: []model.DiffLine{
						{Origin: model.OriginContext, Content: "package auth", OldLineNo: 1, NewLineNo: 1},
						{Origin: model.OriginDeletion, Content: "func Handle(w http.ResponseWriter, r *http.Request) {", OldLineNo: 2},
						{Origin: model.OriginAddition, Content: "func Handle(ctx context.Context, w http.ResponseWriter, r *http.Request) {", NewLineNo: 2},
						{Origin: model.OriginContext, Content: "\ttoken := r.Header.Get(\"Authorization\")", OldLineNo: 3, NewLineNo: 3},
					},
				},
				{
					OldStart: 10, OldCount: 3, NewStart: 10, NewCount: 4,
					Header: "@@ -10,3 +10,4 @@",
					Lines: []model.DiffLine{
						{Origin: model.OriginContext, Content: "\tif err != nil {", OldLineNo: 10, NewLineNo: 10},
						{Origin: model.OriginDeletion, Content: "\t\treturn err", OldLineNo: 11},
						{Origin: model.OriginAddition, Content: "\t\tlog.Printf(\"auth error: %v\", err)", NewLineNo: 11},
						{Origin: model.OriginAddition, Content: "\t\treturn fmt.Errorf(\"auth: %w\", err)", NewLineNo: 12},
						{Origin: model.OriginContext, Content: "\t}", OldLineNo: 12, NewLineNo: 13},
					},
				},
			},
		},
	}

	session := model.NewSession("/tmp/test", "feature", "abc123", model.DiffPullRequest)
	session.Provider = &model.ProviderInfo{Name: "mock", ID: "99", URL: "https://example.com/pr/99"}
	session.Reviewer = "testuser"
	for _, f := range files {
		session.GetOrCreateFileReview(f.DisplayPath(), f.Status)
	}

	mockProv := testutil.NewMockProvider()
	store := testutil.NewMockStore()

	app := NewApp(mockVCS, files, session, theme.Dark(), nil, store)
	app.SetSize(100, 30)
	app.SetProvider(mockProv, "99")

	return &app, mockProv, mockVCS
}

// findAnnotation returns the index of the first annotation matching the predicate,
// or -1 if not found.
func findAnnotation(app *App, pred func(annotatedLine) bool) int {
	for i, ann := range app.annotations {
		if pred(ann) {
			return i
		}
	}
	return -1
}

// findLineComment returns the annotation index of a line comment on the given line number.
func findLineComment(app *App, lineNo int) int {
	return findAnnotation(app, func(ann annotatedLine) bool {
		return ann.Type == annLineComment && (ann.NewLineNo == lineNo || ann.OldLineNo == lineNo)
	})
}

// findDiffLine returns the annotation index of a diff line with the given new line number.
func findDiffLine(app *App, newLineNo int) int {
	return findAnnotation(app, func(ann annotatedLine) bool {
		return ann.Type == annDiffLine && ann.NewLineNo == newLineNo
	})
}

// findExpander returns the annotation index of the first gap expander.
func findExpander(app *App) int {
	return findAnnotation(app, func(ann annotatedLine) bool {
		return ann.Type == annExpander
	})
}

// --- Scenario 1: Refresh that brings in new comments and a changed diff ---

func TestScenarioRefreshWithNewCommentsAndDiff(t *testing.T) {
	app, mockProv, _ := newScenarioApp(t)

	// Take initial snapshot.
	assertGolden(t, "refresh_initial", snapshot(app))

	// Simulate a refresh that returns new inline comments, a review, and a changed diff.
	newDiff := `diff --git a/auth/handler.go b/auth/handler.go
--- a/auth/handler.go
+++ b/auth/handler.go
@@ -1,3 +1,3 @@
 package auth
-func Handle(w http.ResponseWriter, r *http.Request) {
+func Handle(ctx context.Context, w http.ResponseWriter, r *http.Request) {
 	token := r.Header.Get("Authorization")
@@ -10,3 +10,4 @@
 	if err != nil {
-		return err
+		log.Printf("auth error: %v", err)
+		return fmt.Errorf("auth: %w", err)
 	}
diff --git a/auth/validate.go b/auth/validate.go
new file mode 100644
--- /dev/null
+++ b/auth/validate.go
@@ -0,0 +1,3 @@
+package auth
+
+func Validate(token string) error { return nil }
`

	mockProv.SetNextRefresh(&provider.RefreshResult{
		Request: &mockProv.Request,
		NewComments: []provider.Comment{
			testutil.NewProviderComment("rc1", "reviewer-bob", "Should we validate the token before using ctx?", "auth/handler.go", 2, "new"),
			testutil.NewProviderComment("rc2", "reviewer-carol", "Consider wrapping with %w for error chains", "auth/handler.go", 12, "new"),
		},
		NewReviews: []provider.Review{
			{
				ExternalID: "rev1",
				Author:     "reviewer-bob",
				Body:       "A few nits, otherwise looks good.",
				State:      provider.ReviewComment,
				CreatedAt:  time.Date(2025, 1, 15, 11, 0, 0, 0, time.UTC),
			},
		},
		NewConversation: []provider.ConversationComment{
			{
				ExternalID: "conv1",
				Author:     "reviewer-bob",
				Body:       "Pushed a new commit with the validate helper.",
				CreatedAt:  time.Date(2025, 1, 15, 11, 5, 0, 0, time.UTC),
			},
		},
		DiffChanged: true,
		Diff:        newDiff,
	})

	app.refreshFromProvider()

	// Verify new data was imported.
	if len(app.session.Reviews) != 1 {
		t.Fatalf("expected 1 review, got %d", len(app.session.Reviews))
	}
	if len(app.session.Conversation) != 1 {
		t.Fatalf("expected 1 conversation comment, got %d", len(app.session.Conversation))
	}
	// New file should appear.
	if len(app.diffFiles) != 2 {
		t.Fatalf("expected 2 diff files after refresh, got %d", len(app.diffFiles))
	}

	assertGolden(t, "refresh_after", snapshot(app))

	// Show conversation panel to verify the conversation comment appeared.
	app.showConversation = true
	assertGolden(t, "refresh_conversation", snapshot(app))
}

// --- Scenario 2: Replying to multiple comments ---

func TestScenarioReplyToMultipleComments(t *testing.T) {
	app, _, _ := newScenarioApp(t)

	// Import two remote comments on different lines.
	fr := app.session.GetOrCreateFileReview("auth/handler.go", model.FileModified)
	fr.AddLineComment(2, model.Comment{
		ID:         "ext-c1",
		Content:    "Why did you add the context parameter here?",
		Type:       model.CommentNote,
		Side:       model.SideNew,
		Author:     "reviewer-bob",
		ExternalID: "ext-c1",
		CreatedAt:  time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
	})
	fr.AddLineComment(11, model.Comment{
		ID:         "ext-c2",
		Content:    "The log message should include the request ID.",
		Type:       model.CommentIssue,
		Side:       model.SideNew,
		Author:     "reviewer-carol",
		ExternalID: "ext-c2",
		CreatedAt:  time.Date(2025, 1, 15, 10, 35, 0, 0, time.UTC),
	})
	app.rebuildAnnotations()

	assertGolden(t, "reply_initial", snapshot(app))

	// Reply to first comment (line 2).
	idx := findLineComment(app, 2)
	if idx < 0 {
		t.Fatal("could not find comment on line 2")
	}
	app.cursorLine = idx
	app.replyToCommentAtCursor()
	if app.inputMode != modeComment {
		t.Fatal("expected comment mode after reply")
	}

	// Type a reply.
	for _, ch := range "Good point, I added it for request tracing." {
		app = sendKeys(app, keyPress(ch))
	}
	assertGolden(t, "reply_typing_first", snapshot(app))

	// Submit the reply.
	app = sendKeys(app, keySpecial(tea.KeyEnter))
	if app.inputMode != modeNormal {
		t.Fatal("expected normal mode after submitting reply")
	}

	// Reply to second comment (line 11).
	idx = findLineComment(app, 11)
	if idx < 0 {
		t.Fatal("could not find comment on line 11")
	}
	// Find the remote comment (ext-c2), not our local reply
	for i := idx; i < len(app.annotations); i++ {
		ann := app.annotations[i]
		if ann.Type == annLineComment && ann.NewLineNo == 11 {
			path := app.diffFiles[ann.FileIdx].DisplayPath()
			fr := app.session.GetFileReview(path)
			if comments, ok := fr.LineComments[11]; ok && ann.CommentIdx < len(comments) {
				if comments[ann.CommentIdx].ExternalID == "ext-c2" {
					idx = i
					break
				}
			}
		}
	}
	app.cursorLine = idx
	app.replyToCommentAtCursor()

	for _, ch := range "Will add request ID to the log format string." {
		app = sendKeys(app, keyPress(ch))
	}
	app = sendKeys(app, keySpecial(tea.KeyEnter))

	assertGolden(t, "reply_both_done", snapshot(app))
}

// --- Scenario 3: Adding a comment, then editing it ---

func TestScenarioAddThenEditComment(t *testing.T) {
	app, _, _ := newScenarioApp(t)

	// Move cursor to a diff line (new line 2: the addition).
	idx := findDiffLine(app, 2)
	if idx < 0 {
		t.Fatal("could not find diff line for new line 2")
	}
	app.cursorLine = idx

	// Enter comment mode ('c').
	app = sendKeys(app, keyPress('c'))
	if app.inputMode != modeComment {
		t.Fatal("expected comment mode")
	}

	// Type a comment.
	for _, ch := range "This context parameter should be documented." {
		app = sendKeys(app, keyPress(ch))
	}
	assertGolden(t, "add_comment_typing", snapshot(app))

	// Submit the comment (Enter).
	app = sendKeys(app, keySpecial(tea.KeyEnter))
	assertGolden(t, "add_comment_submitted", snapshot(app))

	// Find the comment we just added and move cursor there.
	idx = findLineComment(app, 2)
	if idx < 0 {
		t.Fatal("could not find the comment we just added")
	}
	app.cursorLine = idx

	// Edit the comment ('i').
	app = sendKeys(app, keyPress('i'))
	if app.inputMode != modeComment {
		t.Fatal("expected comment mode for editing")
	}
	if app.editingID == "" {
		t.Fatal("expected editingID to be set")
	}

	// Select all text and replace it: Ctrl+A (Home) then type new content.
	// Since there's no select-all, we'll clear with backspaces then type new text.
	for range app.commentBuffer {
		app = sendKeys(app, keySpecial(tea.KeyBackspace))
	}
	for _, ch := range "This context parameter MUST be documented in the godoc." {
		app = sendKeys(app, keyPress(ch))
	}
	assertGolden(t, "edit_comment_typing", snapshot(app))

	// Submit the edit.
	app = sendKeys(app, keySpecial(tea.KeyEnter))
	assertGolden(t, "edit_comment_done", snapshot(app))
}

// --- Scenario 4: Expanding context after adding a comment ---

func TestScenarioExpandContextAfterComment(t *testing.T) {
	app, _, _ := newScenarioApp(t)

	// First, add a comment on a diff line near the gap.
	idx := findDiffLine(app, 3)
	if idx < 0 {
		t.Fatal("could not find diff line for new line 3")
	}
	app.cursorLine = idx

	app = sendKeys(app, keyPress('c'))
	for _, ch := range "This line needs a nil check for the token." {
		app = sendKeys(app, keyPress(ch))
	}
	app = sendKeys(app, keySpecial(tea.KeyEnter))

	assertGolden(t, "expand_after_comment", snapshot(app))

	// Now find and expand the gap between hunks.
	expanderIdx := findExpander(app)
	if expanderIdx < 0 {
		t.Fatal("could not find expander annotation")
	}
	app.cursorLine = expanderIdx
	app.toggleExpandAtCursor()

	if len(app.expandedGaps) == 0 {
		t.Fatal("gap should be expanded")
	}

	assertGolden(t, "expand_context_expanded", snapshot(app))

	// Verify the expanded lines are visible in the view.
	view := snapshot(app)
	if !strings.Contains(view, "between hunks line 1") {
		t.Error("expanded context should be visible in the view")
	}

	// Collapse it back.
	// After expand, the expander is replaced by expanded context lines.
	// Find an expanded context line.
	ctxIdx := findAnnotation(app, func(ann annotatedLine) bool {
		return ann.Type == annExpandedContext
	})
	if ctxIdx < 0 {
		t.Fatal("could not find expanded context annotation")
	}
	app.cursorLine = ctxIdx
	app.toggleExpandAtCursor()

	assertGolden(t, "expand_context_collapsed", snapshot(app))
}

// newMultiFileScenarioApp creates an app with two files for scenarios that need
// file navigation, file list interaction, and multiple review targets.
func newMultiFileScenarioApp(t *testing.T) (*App, *testutil.MockProvider, *testutil.MockVCS) {
	t.Helper()

	mockVCS := &testutil.MockVCS{
		VcsInfo: vcs.VcsInfo{
			RootPath:   "/tmp/test",
			HeadCommit: "abc123",
			BranchName: "feature",
			VcsType:    "git",
		},
		WorkingTreeDiff: []model.DiffFile{
			{
				OldPath: "pkg/server.go", NewPath: "pkg/server.go", Status: model.FileModified,
				Hunks: []model.DiffHunk{{
					OldStart: 1, OldCount: 2, NewStart: 1, NewCount: 3,
					Header: "@@ -1,2 +1,3 @@",
					Lines: []model.DiffLine{
						{Origin: model.OriginContext, Content: "package server", OldLineNo: 1, NewLineNo: 1},
						{Origin: model.OriginAddition, Content: "import \"log\"", NewLineNo: 2},
						{Origin: model.OriginContext, Content: "func Start() {}", OldLineNo: 2, NewLineNo: 3},
					},
				}},
			},
		},
	}

	files := []model.DiffFile{
		{
			OldPath: "pkg/server.go",
			NewPath: "pkg/server.go",
			Status:  model.FileModified,
			Hunks: []model.DiffHunk{
				{
					OldStart: 1, OldCount: 3, NewStart: 1, NewCount: 4,
					Header: "@@ -1,3 +1,4 @@",
					Lines: []model.DiffLine{
						{Origin: model.OriginContext, Content: "package server", OldLineNo: 1, NewLineNo: 1},
						{Origin: model.OriginDeletion, Content: "func Start() {", OldLineNo: 2},
						{Origin: model.OriginAddition, Content: "func Start(ctx context.Context) {", NewLineNo: 2},
						{Origin: model.OriginAddition, Content: "\tlog.Println(\"starting\")", NewLineNo: 3},
						{Origin: model.OriginContext, Content: "}", OldLineNo: 3, NewLineNo: 4},
					},
				},
			},
		},
		{
			OldPath: "pkg/config.go",
			NewPath: "pkg/config.go",
			Status:  model.FileAdded,
			Hunks: []model.DiffHunk{
				{
					OldStart: 0, OldCount: 0, NewStart: 1, NewCount: 4,
					Header: "@@ -0,0 +1,4 @@",
					Lines: []model.DiffLine{
						{Origin: model.OriginAddition, Content: "package server", NewLineNo: 1},
						{Origin: model.OriginAddition, Content: "", NewLineNo: 2},
						{Origin: model.OriginAddition, Content: "type Config struct {", NewLineNo: 3},
						{Origin: model.OriginAddition, Content: "}", NewLineNo: 4},
					},
				},
			},
		},
	}

	session := model.NewSession("/tmp/test", "feature", "abc123", model.DiffPullRequest)
	session.Provider = &model.ProviderInfo{Name: "mock", ID: "99"}
	session.Reviewer = "testuser"
	for _, f := range files {
		session.GetOrCreateFileReview(f.DisplayPath(), f.Status)
	}

	mockProv := testutil.NewMockProvider()
	store := testutil.NewMockStore()

	app := NewApp(mockVCS, files, session, theme.Dark(), nil, store)
	app.SetSize(100, 30)
	app.SetProvider(mockProv, "99")

	return &app, mockProv, mockVCS
}

// typeString sends each character as a keypress.
func typeString(app *App, s string) *App {
	for _, ch := range s {
		app = sendKeys(app, keyPress(ch))
	}
	return app
}

// --- Scenario 5: Visual select → range comment ---

func TestScenarioVisualSelectRangeComment(t *testing.T) {
	app, _, _ := newScenarioApp(t)

	// Move cursor to a diff line (new line 11: first addition in second hunk).
	idx := findDiffLine(app, 11)
	if idx < 0 {
		t.Fatal("could not find diff line for new line 11")
	}
	app.cursorLine = idx

	// Enter visual mode.
	app = sendKeys(app, keyPress('v'))
	if app.inputMode != modeVisualSelect {
		t.Fatal("expected visual select mode")
	}

	// Extend selection down one line to cover lines 11-12.
	app = sendKeys(app, keyPress('j'))
	assertGolden(t, "visual_select_active", snapshot(app))

	// Press 'c' to create a range comment.
	app = sendKeys(app, keyPress('c'))
	if app.inputMode != modeComment {
		t.Fatal("expected comment mode from visual")
	}
	if app.commentLineRange == nil {
		t.Fatal("expected line range to be set")
	}

	app = typeString(app, "Both error handling lines need tests.")
	assertGolden(t, "visual_select_typing", snapshot(app))

	// Submit the comment.
	app = sendKeys(app, keySpecial(tea.KeyEnter))
	assertGolden(t, "visual_select_done", snapshot(app))
}

// --- Scenario 6: Submit review with approval status ---

func TestScenarioSubmitReviewWithApproval(t *testing.T) {
	app, mockProv, _ := newScenarioApp(t)

	// Add a draft comment first.
	idx := findDiffLine(app, 2)
	app.cursorLine = idx
	app = sendKeys(app, keyPress('c'))
	app = typeString(app, "LGTM on this change.")
	app = sendKeys(app, keySpecial(tea.KeyEnter))

	// Enter overall review (R).
	app = sendKeys(app, upperKey('r', 'R'))
	if app.inputMode != modeReview {
		t.Fatal("expected review mode")
	}

	// Type review body.
	app = typeString(app, "Looks good overall, just one minor comment.")

	// Cycle approval to Approve (Tab once = Approve).
	app = sendKeys(app, keySpecial(tea.KeyTab))
	assertGolden(t, "submit_review_editor", snapshot(app))

	// Save the review.
	app = sendKeys(app, keySpecial(tea.KeyEnter))
	if app.inputMode != modeNormal {
		t.Fatal("expected normal mode after saving review")
	}
	if app.session.OverallReview == nil {
		t.Fatal("expected overall review to be set")
	}
	if app.session.OverallReview.Status != model.ApprovalApprove {
		t.Fatalf("expected Approve status, got %v", app.session.OverallReview.Status)
	}

	// Now submit to provider (:submit).
	app = sendKeys(app, keyPress(':'))
	app = typeString(app, "submit")
	app = sendKeys(app, keySpecial(tea.KeyEnter))
	if app.inputMode != modeConfirm {
		t.Fatal("expected confirm mode")
	}
	assertGolden(t, "submit_review_confirm", snapshot(app))

	// Confirm with 'y'.
	app = sendKeys(app, keyPress('y'))
	assertGolden(t, "submit_review_done", snapshot(app))

	// Verify provider received the submission.
	if len(mockProv.SubmittedReviews) != 1 {
		t.Fatalf("expected 1 submitted review, got %d", len(mockProv.SubmittedReviews))
	}
	if mockProv.SubmittedReviews[0].State != provider.ReviewApprove {
		t.Errorf("expected Approve state, got %v", mockProv.SubmittedReviews[0].State)
	}
}

// --- Scenario 7: Search and highlight ---

func TestScenarioSearchAndHighlight(t *testing.T) {
	app, _, _ := newScenarioApp(t)

	// Enter search mode.
	app = sendKeys(app, keyPress('/'))
	if app.inputMode != modeSearch {
		t.Fatal("expected search mode")
	}

	// Type a search pattern.
	app = typeString(app, "token")
	assertGolden(t, "search_typing", snapshot(app))

	// Submit search.
	app = sendKeys(app, keySpecial(tea.KeyEnter))
	if app.inputMode != modeNormal {
		t.Fatal("expected normal mode after search")
	}
	if app.searchHighlight != "token" {
		t.Fatalf("expected searchHighlight='token', got %q", app.searchHighlight)
	}
	assertGolden(t, "search_result", snapshot(app))

	// Press 'n' for next match.
	prevCursor := app.cursorLine
	app = sendKeys(app, keyPress('n'))
	// Cursor should have moved (or stayed if only one match).
	assertGolden(t, "search_next", snapshot(app))

	// Press 'N' for previous match.
	app = sendKeys(app, upperKey('n', 'N'))
	assertGolden(t, "search_prev", snapshot(app))

	// Clear search with Escape.
	app = sendKeys(app, keySpecial(tea.KeyEscape))
	if app.searchHighlight != "" {
		t.Error("search highlight should be cleared on Escape")
	}
	assertGolden(t, "search_cleared", snapshot(app))
	_ = prevCursor
}

// --- Scenario 8: Picker → review transition ---

func TestScenarioPickerToReview(t *testing.T) {
	mockVCS := &testutil.MockVCS{
		VcsInfo: vcs.VcsInfo{
			RootPath:   "/tmp/test",
			HeadCommit: "abc123",
			BranchName: "feature",
			VcsType:    "git",
		},
		RecentCommits: []vcs.CommitInfo{
			{ID: "aaa", ShortID: "aaa1234", Summary: "Add authentication middleware", Author: "alice", Time: time.Now().Add(-1 * time.Hour)},
			{ID: "bbb", ShortID: "bbb5678", Summary: "Add unit tests for auth", Author: "alice", Time: time.Now().Add(-30 * time.Minute)},
		},
		WorkingTreeDiff: []model.DiffFile{
			{
				OldPath: "main.go", NewPath: "main.go", Status: model.FileModified,
				Hunks: []model.DiffHunk{{
					OldStart: 1, OldCount: 1, NewStart: 1, NewCount: 2,
					Header: "@@ -1,1 +1,2 @@",
					Lines: []model.DiffLine{
						{Origin: model.OriginContext, Content: "package main", OldLineNo: 1, NewLineNo: 1},
						{Origin: model.OriginAddition, Content: "import \"auth\"", NewLineNo: 2},
					},
				}},
			},
		},
		RevisionDiff: []model.DiffFile{
			{
				OldPath: "auth.go", NewPath: "auth.go", Status: model.FileAdded,
				Hunks: []model.DiffHunk{{
					OldStart: 0, OldCount: 0, NewStart: 1, NewCount: 2,
					Header: "@@ -0,0 +1,2 @@",
					Lines: []model.DiffLine{
						{Origin: model.OriginAddition, Content: "package auth", NewLineNo: 1},
						{Origin: model.OriginAddition, Content: "func Validate() {}", NewLineNo: 2},
					},
				}},
			},
		},
	}

	store := testutil.NewMockStore()
	app := NewPickerApp(mockVCS, theme.Dark(), nil, store)
	app.SetSize(100, 30)

	// Snapshot the picker phase.
	assertGolden(t, "picker_initial", snapshot(&app))

	// Select working tree (cursor is on it by default).
	app2 := sendKeys(&app, keyPress(' '))

	// Move down and select a commit.
	app2 = sendKeys(app2, keyPress('j'), keyPress(' '))
	assertGolden(t, "picker_selected", snapshot(app2))

	// Confirm.
	app2 = sendKeys(app2, keySpecial(tea.KeyEnter))
	if app2.phase != phaseReview {
		t.Fatal("expected review phase")
	}
	assertGolden(t, "picker_review", snapshot(app2))
}

// --- Scenario 9: Delete comment (dd) ---

func TestScenarioDeleteComment(t *testing.T) {
	app, _, _ := newScenarioApp(t)

	// Add a comment on line 2.
	idx := findDiffLine(app, 2)
	app.cursorLine = idx
	app = sendKeys(app, keyPress('c'))
	app = typeString(app, "This comment will be deleted.")
	app = sendKeys(app, keySpecial(tea.KeyEnter))

	assertGolden(t, "delete_comment_before", snapshot(app))

	// Move cursor to the comment.
	idx = findLineComment(app, 2)
	if idx < 0 {
		t.Fatal("could not find comment to delete")
	}
	app.cursorLine = idx

	// Delete with 'dd'.
	app = sendKeys(app, keyPress('d'), keyPress('d'))
	if app.message == nil || !strings.Contains(app.message.text, "deleted") {
		var msg string
		if app.message != nil {
			msg = app.message.text
		}
		t.Fatalf("expected delete message, got %q", msg)
	}

	assertGolden(t, "delete_comment_after", snapshot(app))
}

// --- Scenario 10: File comment (C) ---

func TestScenarioFileComment(t *testing.T) {
	app, _, _ := newScenarioApp(t)

	// Move cursor to any line of the first file.
	idx := findDiffLine(app, 1)
	app.cursorLine = idx

	// Enter file comment mode (C).
	app = sendKeys(app, upperKey('c', 'C'))
	if app.inputMode != modeComment {
		t.Fatal("expected comment mode")
	}
	if !app.commentIsFile {
		t.Fatal("expected file-level comment")
	}

	app = typeString(app, "This file needs better error handling throughout.")
	assertGolden(t, "file_comment_typing", snapshot(app))

	// Submit.
	app = sendKeys(app, keySpecial(tea.KeyEnter))
	assertGolden(t, "file_comment_done", snapshot(app))

	// Verify it appears as a file comment in the session.
	path := app.diffFiles[0].DisplayPath()
	fr := app.session.GetFileReview(path)
	if len(fr.FileComments) != 1 {
		t.Fatalf("expected 1 file comment, got %d", len(fr.FileComments))
	}
}

// --- Scenario 11: Commit list toggle and description view ---

func TestScenarioCommitListAndDescription(t *testing.T) {
	app, _, _ := newScenarioApp(t)

	// Set up commits.
	commits := []vcs.CommitInfo{
		{ID: "c1", ShortID: "c1abc", Summary: "Add auth handler", Author: "alice", Time: time.Now().Add(-1 * time.Hour)},
		{ID: "c2", ShortID: "c2def", Summary: "Add error wrapping", Author: "bob", Time: time.Now().Add(-30 * time.Minute)},
	}
	diffs := map[string][]model.DiffFile{
		"c1": app.diffFiles,
		"c2": {{
			OldPath: "pkg/errors.go", NewPath: "pkg/errors.go", Status: model.FileAdded,
			Hunks: []model.DiffHunk{{
				OldStart: 0, OldCount: 0, NewStart: 1, NewCount: 1,
				Header: "@@ -0,0 +1,1 @@",
				Lines:  []model.DiffLine{{Origin: model.OriginAddition, Content: "package errors", NewLineNo: 1}},
			}},
		}},
	}
	app.SetCommits(commits, diffs)
	app.session.Description = "## Auth Refactor\n\nThis PR refactors the auth layer to use context."

	assertGolden(t, "commitlist_view", snapshot(app))

	// Tab to focus commit list.
	app = sendKeys(app, keySpecial(tea.KeyTab)) // file list
	app = sendKeys(app, keySpecial(tea.KeyTab)) // commit list
	if app.focusedPanel != panelCommitList {
		t.Fatalf("expected commit list focus, got %d", app.focusedPanel)
	}

	// Toggle commit c2 off.
	app = sendKeys(app, keyPress('j')) // move to c2
	app = sendKeys(app, keyPress(' ')) // toggle off
	if app.enabledCommits["c2"] {
		t.Fatal("c2 should be disabled")
	}
	assertGolden(t, "commitlist_toggled", snapshot(app))

	// Re-enable.
	app = sendKeys(app, keyPress(' '))

	// Switch to description view (D).
	app = sendKeys(app, upperKey('d', 'D'))
	if !app.showDescription {
		t.Fatal("expected description view")
	}
	assertGolden(t, "commitlist_description", snapshot(app))

	// Toggle back to commit list.
	app = sendKeys(app, upperKey('d', 'D'))
	if app.showDescription {
		t.Fatal("expected commit list view")
	}
}

// --- Scenario 12: Clear drafts with mixed comments ---

func TestScenarioClearDrafts(t *testing.T) {
	app, _, _ := newScenarioApp(t)

	// Add a remote comment (has ExternalID).
	fr := app.session.GetOrCreateFileReview("auth/handler.go", model.FileModified)
	fr.AddLineComment(2, model.Comment{
		ID:         "ext-c1",
		Content:    "Remote comment from reviewer",
		Type:       model.CommentNote,
		Side:       model.SideNew,
		Author:     "reviewer-bob",
		ExternalID: "ext-c1",
	})

	// Add a local draft comment.
	idx := findDiffLine(app, 11)
	app.cursorLine = idx
	app = sendKeys(app, keyPress('c'))
	app = typeString(app, "Draft: needs error handling.")
	app = sendKeys(app, keySpecial(tea.KeyEnter))

	// Add a file comment (also draft).
	app = sendKeys(app, upperKey('c', 'C'))
	app = typeString(app, "Draft file comment.")
	app = sendKeys(app, keySpecial(tea.KeyEnter))

	app.rebuildAnnotations()
	assertGolden(t, "clear_before", snapshot(app))

	// Execute :clear.
	app = sendKeys(app, keyPress(':'))
	app = typeString(app, "clear")
	app = sendKeys(app, keySpecial(tea.KeyEnter))
	if app.inputMode != modeConfirm {
		t.Fatal("expected confirm mode")
	}
	assertGolden(t, "clear_confirm", snapshot(app))

	// Confirm.
	app = sendKeys(app, keyPress('y'))
	assertGolden(t, "clear_after", snapshot(app))

	// Verify: remote comment survives, drafts are gone.
	fr = app.session.GetFileReview("auth/handler.go")
	if len(fr.FileComments) != 0 {
		t.Errorf("expected 0 file comments, got %d", len(fr.FileComments))
	}
	totalLineComments := 0
	for _, comments := range fr.LineComments {
		totalLineComments += len(comments)
	}
	if totalLineComments != 1 {
		t.Errorf("expected 1 line comment (remote), got %d", totalLineComments)
	}
	// The surviving comment should be the remote one.
	if comments, ok := fr.LineComments[2]; ok {
		if len(comments) != 1 || comments[0].ExternalID != "ext-c1" {
			t.Error("surviving comment should be the remote one")
		}
	} else {
		t.Error("remote comment on line 2 should survive")
	}
}

// --- Scenario 13: Dirty quit warning ---

func TestScenarioDirtyQuitWarning(t *testing.T) {
	app, _, _ := newScenarioApp(t)

	// Make a change to set dirty flag.
	idx := findDiffLine(app, 2)
	app.cursorLine = idx
	app = sendKeys(app, keyPress('c'))
	app = typeString(app, "A comment.")
	app = sendKeys(app, keySpecial(tea.KeyEnter))

	if !app.dirty {
		t.Fatal("expected dirty flag")
	}

	// Try to quit.
	app = sendKeys(app, keyPress('q'))
	if app.message == nil || !strings.Contains(app.message.text, "Unsaved") {
		t.Fatal("expected unsaved warning")
	}
	assertGolden(t, "quit_warning", snapshot(app))

	// Second q should produce a DoneMsg.
	_, cmd := app.Update(keyPress('q'))
	if cmd == nil {
		t.Fatal("expected quit command on second q")
	}
	msg := cmd()
	if _, ok := msg.(DoneMsg); !ok {
		t.Fatalf("expected DoneMsg, got %T", msg)
	}
}

// --- Scenario 14: Panel focus cycling ---

func TestScenarioPanelFocusCycling(t *testing.T) {
	app, _, _ := newScenarioApp(t)

	// Set up commits for commit list panel.
	commits := []vcs.CommitInfo{
		{ID: "c1", ShortID: "c1x", Summary: "Commit one", Author: "alice", Time: time.Now().Add(-1 * time.Hour)},
	}
	diffs := map[string][]model.DiffFile{"c1": app.diffFiles}
	app.SetCommits(commits, diffs)
	app.showConversation = true
	app.session.Conversation = []model.ConversationComment{
		{Author: "bob", Body: "Reviewing now.", CreatedAt: time.Now()},
	}

	// Start in diff panel.
	if app.focusedPanel != panelDiff {
		t.Fatalf("expected diff panel, got %d", app.focusedPanel)
	}
	assertGolden(t, "panel_diff", snapshot(app))

	// Tab → file list.
	app = sendKeys(app, keySpecial(tea.KeyTab))
	if app.focusedPanel != panelFileList {
		t.Fatalf("expected file list, got %d", app.focusedPanel)
	}
	assertGolden(t, "panel_filelist", snapshot(app))

	// Tab → commit list.
	app = sendKeys(app, keySpecial(tea.KeyTab))
	if app.focusedPanel != panelCommitList {
		t.Fatalf("expected commit list, got %d", app.focusedPanel)
	}
	assertGolden(t, "panel_commitlist", snapshot(app))

	// Tab → conversation.
	app = sendKeys(app, keySpecial(tea.KeyTab))
	if app.focusedPanel != panelConversation {
		t.Fatalf("expected conversation, got %d", app.focusedPanel)
	}
	assertGolden(t, "panel_conversation", snapshot(app))

	// Tab → back to diff.
	app = sendKeys(app, keySpecial(tea.KeyTab))
	if app.focusedPanel != panelDiff {
		t.Fatalf("expected diff, got %d", app.focusedPanel)
	}
}

// --- Scenario 15: Reload diff from VCS ---

func TestScenarioReloadDiff(t *testing.T) {
	app, _, mockVCS := newMultiFileScenarioApp(t)

	assertGolden(t, "reload_before", snapshot(app))

	// Execute :e to reload.
	app = sendKeys(app, keyPress(':'))
	app = typeString(app, "e")
	app = sendKeys(app, keySpecial(tea.KeyEnter))

	if app.message == nil || !strings.Contains(app.message.text, "Reloaded") {
		var msg string
		if app.message != nil {
			msg = app.message.text
		}
		t.Fatalf("expected reload message, got %q", msg)
	}

	// The diff should now show the working tree diff from the VCS mock.
	if len(app.diffFiles) != len(mockVCS.WorkingTreeDiff) {
		t.Fatalf("expected %d files from VCS, got %d", len(mockVCS.WorkingTreeDiff), len(app.diffFiles))
	}

	assertGolden(t, "reload_after", snapshot(app))
}

// --- Scenario 16: File list collapse/expand and jump ---

func TestScenarioFileListNavigation(t *testing.T) {
	app, _, _ := newMultiFileScenarioApp(t)

	// Focus file list.
	app = sendKeys(app, keySpecial(tea.KeyTab))
	if app.focusedPanel != panelFileList {
		t.Fatalf("expected file list, got %d", app.focusedPanel)
	}
	assertGolden(t, "filelist_focused", snapshot(app))

	// Enter on directory to collapse it.
	// First row should be the pkg/ directory.
	app = sendKeys(app, keySpecial(tea.KeyEnter))
	assertGolden(t, "filelist_collapsed", snapshot(app))

	// Enter again to expand.
	app = sendKeys(app, keySpecial(tea.KeyEnter))
	assertGolden(t, "filelist_expanded", snapshot(app))

	// Navigate down to a file and Enter to jump to it in diff view.
	app = sendKeys(app, keyPress('j')) // move to first file
	app = sendKeys(app, keySpecial(tea.KeyEnter))
	if app.focusedPanel != panelDiff {
		t.Fatal("should switch to diff panel after jumping to file")
	}
	assertGolden(t, "filelist_jumped", snapshot(app))
}

// --- Scenario 17: Horizontal scroll ---

func TestScenarioHorizontalScroll(t *testing.T) {
	app, _, _ := newScenarioApp(t)

	// Move cursor to long line (line 2 has the long addition).
	idx := findDiffLine(app, 2)
	app.cursorLine = idx

	assertGolden(t, "hscroll_initial", snapshot(app))

	// Scroll right several times.
	app = sendKeys(app, keyPress('l'), keyPress('l'), keyPress('l'), keyPress('l'), keyPress('l'))
	if app.scrollX != 5 {
		t.Fatalf("expected scrollX=5, got %d", app.scrollX)
	}
	assertGolden(t, "hscroll_right", snapshot(app))

	// Reset with '0'.
	app = sendKeys(app, keyPress('0'))
	if app.scrollX != 0 {
		t.Fatal("expected scrollX=0 after reset")
	}
	assertGolden(t, "hscroll_reset", snapshot(app))
}
