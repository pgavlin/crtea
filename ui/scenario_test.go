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
