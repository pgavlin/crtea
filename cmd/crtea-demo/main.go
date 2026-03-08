// crtea-demo launches crtea with a mock provider for UX exploration.
// No git repo, network, or gh CLI required.
package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/pgavlin/crtea/model"
	"github.com/pgavlin/crtea/persistence"
	"github.com/pgavlin/crtea/provider"
	"github.com/pgavlin/crtea/provider/mock"
	"github.com/pgavlin/crtea/syntax"
	"github.com/pgavlin/crtea/theme"
	"github.com/pgavlin/crtea/ui"
	"github.com/pgavlin/crtea/vcs"
)

type appWrapper struct {
	app     ui.App
	session *model.ReviewSession
}

func (w *appWrapper) Init() tea.Cmd  { return w.app.Init() }
func (w *appWrapper) View() tea.View { return w.app.View() }
func (w *appWrapper) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		w.app.SetSize(msg.Width, msg.Height)
		return w, nil
	case ui.DoneMsg:
		w.session = msg.Session
		return w, tea.Quit
	case ui.ClipboardMsg:
		return w, tea.SetClipboard(msg.Content)
	}
	m, cmd := w.app.Update(msg)
	if a, ok := m.(*ui.App); ok {
		w.app = *a
	}
	return w, cmd
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	m := mock.New()

	// Select theme
	var th theme.Theme
	var chromaStyle string
	if lipgloss.HasDarkBackground(os.Stdin, os.Stdout) {
		th = theme.Dark()
		chromaStyle = "monokai"
	} else {
		th = theme.Light()
		chromaStyle = "github"
	}
	highlighter := syntax.NewHighlighter(chromaStyle)

	// Parse the combined diff
	diffText, _ := m.GetDiff("42")
	files := vcs.ParseDiff(diffText)
	highlighter.HighlightFiles(files)

	// Parse per-commit diffs
	commits, _ := m.ListCommits("42")
	commitInfos := make([]vcs.CommitInfo, len(commits))
	commitDiffs := make(map[string][]model.DiffFile, len(commits))
	for i, c := range commits {
		commitInfos[i] = vcs.CommitInfo{
			ID:      c.ID,
			ShortID: c.ShortID,
			Summary: c.Summary,
			Author:  c.Author,
			Time:    c.Time,
		}
		cdiff, _ := m.GetCommitDiff("42", c.ID)
		cf := vcs.ParseDiff(cdiff)
		highlighter.HighlightFiles(cf)
		commitDiffs[c.ID] = cf
	}

	// Build session
	rr, _ := m.GetReviewRequest("42")
	session := model.NewSession(".", "feature/auth", rr.HeadSHA, model.DiffPullRequest)
	session.Provider = &model.ProviderInfo{
		Name: m.Name(),
		ID:   rr.ID,
		URL:  rr.URL,
	}
	session.Description = rr.Title + "\n\n" + rr.Body
	for _, f := range files {
		session.GetOrCreateFileReview(f.DisplayPath(), f.Status)
	}

	// Import user identity
	user, _ := m.GetAuthenticatedUser()
	session.Reviewer = user

	// Import reviews
	reviews, _ := m.ListReviews("42")
	session.Reviews = make([]model.OverallReview, len(reviews))
	for i, r := range reviews {
		session.Reviews[i] = provider.ImportReview(r)
	}

	// Import inline comments
	comments, _ := m.ListComments("42")
	imported := provider.ImportComments(comments)
	for path, lineComments := range imported {
		fr := session.GetOrCreateFileReview(path, model.FileModified)
		for line, cs := range lineComments {
			for _, c := range cs {
				fr.AddLineComment(line, c)
			}
		}
	}

	// Import conversation
	convComments, _ := m.ListConversation("42")
	session.Conversation = provider.ImportConversation(convComments)

	// Create a no-op store since we don't have a real filesystem session
	store, err := persistence.NewFileStore()
	if err != nil {
		return err
	}

	// Create app — pass nil backend since we don't need VCS operations
	app := ui.NewApp(nil, files, session, th, highlighter, store)
	app.SetProvider(m, "42")
	app.SetCommits(commitInfos, commitDiffs)

	w := &appWrapper{app: app}
	p := tea.NewProgram(w)
	_, err = p.Run()
	return err
}
