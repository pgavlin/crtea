package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/urfave/cli/v3"

	"github.com/pgavlin/crtea/logging"
	"github.com/pgavlin/crtea/model"
	"github.com/pgavlin/crtea/persistence"
	gh "github.com/pgavlin/crtea/provider/github"
	"github.com/pgavlin/crtea/syntax"
	"github.com/pgavlin/crtea/theme"
	"github.com/pgavlin/crtea/ui"
	"github.com/pgavlin/crtea/vcs"
)

// appWrapper adapts the ui.App component for use as a top-level tea.Program model.
// It translates tea.WindowSizeMsg into SetSize calls, and converts ui.DoneMsg /
// ui.ClipboardMsg back into program-level actions (tea.Quit, tea.SetClipboard).
type appWrapper struct {
	app     ui.App
	session *model.ReviewSession
	err     error
}

func (w *appWrapper) Init() tea.Cmd {
	return w.app.Init()
}

func (w *appWrapper) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		w.app.SetSize(msg.Width, msg.Height)
		return w, nil
	case ui.DoneMsg:
		w.session = msg.Session
		w.err = msg.Err
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

func (w *appWrapper) View() tea.View {
	return w.app.View()
}

func main() {
	cmd := &cli.Command{
		Name:  "crtea",
		Usage: "Interactive terminal code review tool",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "theme",
				Aliases: []string{"t"},
				Usage:   "color theme: dark, light, glamour-dark, glamour-light (auto-detected if omitted)",
			},
			&cli.StringFlag{
				Name:    "revisions",
				Aliases: []string{"r"},
				Usage:   "review specific commits (e.g. main~5..HEAD)",
			},
			&cli.StringFlag{
				Name:    "pr",
				Aliases: []string{"p"},
				Usage:   "review a pull/merge request (e.g. 123)",
			},
		},
		Action: run,
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cmd *cli.Command) error {
	// Initialize logging
	logger, logFile, err := logging.Init()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not initialize logging: %v\n", err)
		logger = slog.Default()
	} else {
		defer logFile.Close()
	}
	logger.Info("crtea starting")

	dir, err := os.Getwd()
	if err != nil {
		logger.Error("failed to get working directory", "error", err)
		return err
	}

	backend, err := vcs.NewGitBackend(logger, dir)
	if err != nil {
		logger.Error("failed to initialize git backend", "dir", dir, "error", err)
		return err
	}

	store, err := persistence.NewFileStore(logger)
	if err != nil {
		logger.Error("failed to initialize session store", "error", err)
		return fmt.Errorf("initializing session store: %w", err)
	}

	// Select theme and syntax highlighting style
	th, chromaStyle := selectTheme(cmd.String("theme"))
	highlighter := syntax.NewHighlighter(chromaStyle)

	var app ui.App
	if prID := cmd.String("pr"); prID != "" {
		info := backend.Info()
		owner, repo, err := gh.DetectRemote(info.RootPath)
		if err != nil {
			logger.Error("failed to detect GitHub remote", "path", info.RootPath, "error", err)
			return fmt.Errorf("detecting GitHub remote: %w", err)
		}
		p := gh.New(logger, owner, repo)
		app = ui.NewProviderApp(logger, backend, p, prID, th, highlighter, store)
	} else if revisions := cmd.String("revisions"); revisions != "" {
		info := backend.Info()

		// Get commits in the range
		commits, err := backend.GetCommitsInRange(revisions)
		if err != nil {
			logger.Error("failed to list commits in range", "revisions", revisions, "error", err)
			return fmt.Errorf("listing commits: %w", err)
		}

		// Determine if working tree should be included: range endpoint is HEAD
		includesWorkTree := rangeIncludesHead(revisions)

		// Fetch per-commit diffs
		commitDiffs := make(map[string][]model.DiffFile)
		enabledCommits := make(map[string]bool)
		for _, c := range commits {
			files, err := backend.GetRevisionDiff(c.ID + "^.." + c.ID)
			if err == nil {
				highlighter.HighlightFiles(files)
				commitDiffs[c.ID] = files
			}
			enabledCommits[c.ID] = true
		}

		if includesWorkTree {
			files, err := backend.GetWorkingTreeDiff()
			if err == nil && len(files) > 0 {
				highlighter.HighlightFiles(files)
				commitDiffs["worktree"] = files
				enabledCommits["worktree"] = true
			} else {
				// No working tree changes; don't show the entry
				includesWorkTree = false
			}
		}

		if len(commitDiffs) == 0 {
			fmt.Println("No changes to review.")
			return nil
		}

		diffSource := model.DiffCommitRange
		session, _ := store.LoadLatest(info.RootPath, info.BranchName, diffSource)
		if session == nil {
			session = model.NewSession(info.RootPath, info.BranchName, info.HeadCommit, diffSource)
		}
		if session.Description == "" {
			session.Description = buildRevisionDescription(commits, includesWorkTree, revisions)
		}
		app = ui.NewAppWithCommits(logger, backend, commits, commitDiffs, enabledCommits, includesWorkTree, session, th, highlighter, store)
	} else {
		app = ui.NewPickerApp(logger, backend, th, highlighter, store)
	}

	w := &appWrapper{app: app}
	p := tea.NewProgram(w)

	if _, err := p.Run(); err != nil {
		logger.Error("tea program error", "error", err)
		return err
	}
	if w.err != nil {
		logger.Error("application error", "error", w.err)
		return w.err
	}

	logger.Info("crtea exiting")
	return nil
}

// buildRevisionDescription creates a description from commit messages in a revision range.
func buildRevisionDescription(commits []vcs.CommitInfo, includesWorkTree bool, revisions string) string {
	if len(commits) == 0 && !includesWorkTree {
		return revisions
	}
	var sections []string
	if includesWorkTree {
		sections = append(sections, "* Working tree changes")
	}
	// Commits are newest-first; list oldest-first for chronological order
	for i := len(commits) - 1; i >= 0; i-- {
		c := commits[i]
		entry := "* " + c.Summary
		if c.Body != "" {
			for _, line := range strings.Split(c.Body, "\n") {
				entry += "\n  " + line
			}
		}
		sections = append(sections, entry)
	}
	return strings.Join(sections, "\n\n")
}

// rangeIncludesHead checks whether a revision range's endpoint resolves to HEAD.
// This determines whether working tree changes should be included.
func rangeIncludesHead(revSpec string) bool {
	// No ".." means git diff diffs against working tree
	if !strings.Contains(revSpec, "..") {
		return true
	}
	// Extract the right side of "A..B" or "A...B"
	endpoint := revSpec[strings.Index(revSpec, "..")+2:]
	if strings.HasPrefix(endpoint, ".") {
		endpoint = endpoint[1:]
	}
	if endpoint == "" || strings.EqualFold(endpoint, "HEAD") {
		return true
	}
	// Resolve the endpoint and compare to HEAD
	cmd := exec.Command("git", "rev-parse", endpoint)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	resolved := strings.TrimSpace(string(out))
	cmd2 := exec.Command("git", "rev-parse", "HEAD")
	out2, err := cmd2.Output()
	if err != nil {
		return false
	}
	return resolved == strings.TrimSpace(string(out2))
}

func selectTheme(name string) (theme.Theme, string) {
	switch name {
	case "dark":
		return theme.Dark(), "monokai"
	case "light":
		return theme.Light(), "github"
	case "glamour-dark":
		return theme.GlamourDark(), "monokai"
	case "glamour-light":
		return theme.GlamourLight(), "github"
	default:
		if lipgloss.HasDarkBackground(os.Stdin, os.Stdout) {
			return theme.Dark(), "monokai"
		}
		return theme.Light(), "github"
	}
}
