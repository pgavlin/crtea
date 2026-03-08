package ui

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/pgavlin/crtea/model"
	"github.com/pgavlin/crtea/persistence"
	"github.com/pgavlin/crtea/provider"
	"github.com/pgavlin/crtea/syntax"
	"github.com/pgavlin/crtea/theme"
	"github.com/pgavlin/crtea/vcs"
)

var spinnerChars = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// spinnerTickMsg advances the spinner animation.
type spinnerTickMsg struct{}

// providerLoadedMsg carries the result of async provider loading.
type providerLoadedMsg struct {
	err      error
	session  *model.ReviewSession
	files    []model.DiffFile
	commits  []vcs.CommitInfo
	diffs    map[string][]model.DiffFile
	provider provider.Provider
	id       string
}

// NewProviderApp creates an App that starts in the loading phase and fetches
// provider data asynchronously.
func NewProviderApp(
	backend vcs.Backend,
	p provider.Provider,
	id string,
	th theme.Theme,
	hl *syntax.Highlighter,
	store persistence.Store,
) App {
	return App{
		vcs:           backend,
		vcsInfo:       backend.Info(),
		highlighter:   hl,
		store:         store,
		theme:         th,
		phase:         phaseLoading,
		loadingStatus: "Fetching pull request…",
		provider:      p,
		providerID:    id,
		expandedGaps:  make(map[gapID][]model.DiffLine),
		collapsedDirs: make(map[string]bool),
	}
}

// loadProviderCmd returns a tea.Cmd that fetches all provider data in the background.
func (a *App) loadProviderCmd() tea.Cmd {
	p := a.provider
	id := a.providerID
	info := a.vcsInfo
	hl := a.highlighter
	store := a.store

	return func() tea.Msg {
		slog.Info("loading provider data", "provider", p.Name(), "id", id)

		// Fetch review request metadata
		rr, err := p.GetReviewRequest(id)
		if err != nil {
			slog.Error("failed to fetch review request", "id", id, "error", err)
			return providerLoadedMsg{err: fmt.Errorf("fetching review request: %w", err)}
		}

		// Fetch diff
		diffText, err := p.GetDiff(id)
		if err != nil {
			slog.Error("failed to fetch diff", "id", id, "error", err)
			return providerLoadedMsg{err: fmt.Errorf("fetching diff: %w", err)}
		}

		files := vcs.ParseDiff(diffText)
		if len(files) == 0 {
			slog.Warn("no changes found in diff", "id", id)
			return providerLoadedMsg{err: fmt.Errorf("no changes to review")}
		}
		hl.HighlightFiles(files)

		// Session
		diffSource := model.DiffPullRequest
		session, _ := store.LoadLatest(info.RootPath, info.BranchName, diffSource)
		if session == nil {
			session = model.NewSession(info.RootPath, info.BranchName, info.HeadCommit, diffSource)
		}
		session.Provider = &model.ProviderInfo{
			Name: p.Name(),
			ID:   id,
			URL:  rr.URL,
		}
		session.IsDraft = rr.IsDraft
		if session.Description == "" {
			session.Description = rr.Title
			if rr.Body != "" {
				session.Description += "\n\n" + rr.Body
			}
		}
		for _, f := range files {
			session.GetOrCreateFileReview(f.DisplayPath(), f.Status)
		}

		// Fetch authenticated user
		if user, err := p.GetAuthenticatedUser(); err == nil {
			session.Reviewer = user
		} else {
			slog.Warn("failed to fetch authenticated user", "error", err)
		}

		// Fetch existing reviews
		reviews, err := p.ListReviews(id)
		if err != nil {
			slog.Warn("failed to fetch reviews", "id", id, "error", err)
		}
		if len(reviews) > 0 {
			session.Reviews = make([]model.OverallReview, len(reviews))
			for i, r := range reviews {
				session.Reviews[i] = provider.ImportReview(r)
			}
		}

		// Fetch existing inline comments
		comments, err := p.ListComments(id)
		if err != nil {
			slog.Warn("failed to fetch comments", "id", id, "error", err)
		}
		if len(comments) > 0 {
			imported := provider.ImportComments(comments)
			for path, lineComments := range imported {
				fr := session.GetOrCreateFileReview(path, model.FileModified)
				provider.MergeImportedComments(fr, lineComments)
			}
		}

		// Fetch conversation
		convComments, err := p.ListConversation(id)
		if err != nil {
			slog.Warn("failed to fetch conversation", "id", id, "error", err)
		}
		if len(convComments) > 0 {
			session.Conversation = provider.ImportConversation(convComments)
		}

		// Seed the provider's refresh baseline
		p.Seed(rr, comments, reviews, convComments)

		// Fetch commits
		var commitInfos []vcs.CommitInfo
		commitDiffs := make(map[string][]model.DiffFile)
		if commits, err := p.ListCommits(id); err == nil && len(commits) > 0 {
			commitInfos = make([]vcs.CommitInfo, len(commits))
			for i, c := range commits {
				commitInfos[i] = vcs.CommitInfo{
					ID:      c.ID,
					ShortID: c.ShortID,
					Summary: c.Summary,
					Author:  c.Author,
					Time:    c.Time,
				}
				if cdiff, err := p.GetCommitDiff(id, c.ID); err == nil {
					cf := vcs.ParseDiff(cdiff)
					hl.HighlightFiles(cf)
					commitDiffs[c.ID] = cf
				} else {
					slog.Warn("failed to fetch commit diff", "commit", c.ShortID, "error", err)
				}
			}
		} else if err != nil {
			slog.Warn("failed to fetch commits", "id", id, "error", err)
		}

		slog.Info("provider data loaded", "files", len(files), "commits", len(commitInfos), "comments", len(comments))

		return providerLoadedMsg{
			session:  session,
			files:    files,
			commits:  commitInfos,
			diffs:    commitDiffs,
			provider: p,
			id:       id,
		}
	}
}

// handleProviderLoaded transitions from loading phase to review phase.
func (a *App) handleProviderLoaded(msg providerLoadedMsg) tea.Cmd {
	if msg.err != nil {
		slog.Error("provider loading failed", "error", msg.err)
		return func() tea.Msg { return DoneMsg{Err: msg.err} }
	}

	a.session = msg.session
	a.diffFiles = msg.files
	a.provider = msg.provider
	a.providerID = msg.id
	a.phase = phaseReview
	a.inputMode = modeNormal
	a.focusedPanel = panelDiff
	a.showFileList = true
	a.rebuildFileTree()
	a.rebuildAnnotations()

	if len(msg.commits) > 0 {
		a.SetCommits(msg.commits, msg.diffs)
	}

	return nil
}

// renderLoading renders the loading screen with a spinner.
func (a *App) renderLoading() string {
	th := a.theme
	var b strings.Builder

	// Center vertically
	topPad := a.height/2 - 2
	if topPad < 0 {
		topPad = 0
	}
	for i := 0; i < topPad; i++ {
		b.WriteString("\n")
	}

	spinner := spinnerChars[a.spinnerFrame%len(spinnerChars)]
	spinnerStyle := lipgloss.NewStyle().Foreground(th.FgPrimary).Bold(true)
	statusStyle := lipgloss.NewStyle().Foreground(th.FgSecondary)

	line := spinnerStyle.Render(spinner) + " " + statusStyle.Render(a.loadingStatus)
	// Center horizontally
	lineWidth := lipgloss.Width(line)
	pad := (a.width - lineWidth) / 2
	if pad < 0 {
		pad = 0
	}
	b.WriteString(strings.Repeat(" ", pad) + line)

	return b.String()
}

// spinnerTick returns a command that sends a spinnerTickMsg after a delay.
func spinnerTick() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}

