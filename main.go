package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/pgavlin/crtea/model"
	"github.com/pgavlin/crtea/output"
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
	themeFlag := flag.String("theme", "", "color theme: dark or light")
	stdoutFlag := flag.Bool("stdout", false, "export comments to stdout on exit")
	revisions := flag.String("revisions", "", "review specific commits (e.g. main~5..HEAD)")
	flag.Parse()

	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	backend, err := vcs.NewGitBackend(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Initialize persistence
	store, err := persistence.NewFileStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing session store: %v\n", err)
		os.Exit(1)
	}

	// Select theme and syntax highlighting style
	var th theme.Theme
	chromaStyle := "monokai"
	switch *themeFlag {
	case "dark":
		th = theme.Dark()
	case "light":
		th = theme.Light()
		chromaStyle = "github"
	case "glamour-dark":
		th = theme.GlamourDark()
	case "glamour-light":
		th = theme.GlamourLight()
		chromaStyle = "github"
	default:
		// Auto-detect based on terminal background color
		if lipgloss.HasDarkBackground(os.Stdin, os.Stdout) {
			th = theme.Dark()
		} else {
			th = theme.Light()
			chromaStyle = "github"
		}
	}
	highlighter := syntax.NewHighlighter(chromaStyle)

	var app ui.App
	if *revisions != "" {
		info := backend.Info()
		files, err := backend.GetRevisionDiff(*revisions)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting diff: %v\n", err)
			os.Exit(1)
		}
		if len(files) == 0 {
			fmt.Println("No changes to review.")
			os.Exit(0)
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
		app = ui.NewApp(backend, files, session, th, highlighter, store)
	} else {
		app = ui.NewPickerApp(backend, th, highlighter, store)
	}

	w := &appWrapper{app: app}
	p := tea.NewProgram(w)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// If --stdout, print comments as markdown
	if *stdoutFlag && w.session != nil && w.session.TotalComments() > 0 {
		fmt.Fprint(os.Stdout, output.GenerateMarkdown(w.session))
	}
}
