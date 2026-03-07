package main

import (
	"context"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/urfave/cli/v3"

	"github.com/pgavlin/crtea/model"
	"github.com/pgavlin/crtea/persistence"
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
		},
		Action: run,
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cmd *cli.Command) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	backend, err := vcs.NewGitBackend(dir)
	if err != nil {
		return err
	}

	store, err := persistence.NewFileStore()
	if err != nil {
		return fmt.Errorf("initializing session store: %w", err)
	}

	// Select theme and syntax highlighting style
	th, chromaStyle := selectTheme(cmd.String("theme"))
	highlighter := syntax.NewHighlighter(chromaStyle)

	var app ui.App
	if revisions := cmd.String("revisions"); revisions != "" {
		info := backend.Info()
		files, err := backend.GetRevisionDiff(revisions)
		if err != nil {
			return fmt.Errorf("getting diff: %w", err)
		}
		if len(files) == 0 {
			fmt.Println("No changes to review.")
			return nil
		}
		highlighter.HighlightFiles(files)

		diffSource := model.DiffCommitRange
		session, _ := store.LoadLatest(info.RootPath, info.BranchName, diffSource)
		if session == nil {
			session = model.NewSession(info.RootPath, info.BranchName, info.HeadCommit, diffSource)
		}
		for _, f := range files {
			session.GetOrCreateFileReview(f.DisplayPath(), f.Status)
		}
		if session.Description == "" {
			session.Description = revisions
		}
		app = ui.NewApp(backend, files, session, th, highlighter, store)
	} else {
		app = ui.NewPickerApp(backend, th, highlighter, store)
	}

	w := &appWrapper{app: app}
	p := tea.NewProgram(w)

	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
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
