package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/pgavlin/crtea/model"
	"github.com/pgavlin/crtea/persistence"
	"github.com/pgavlin/crtea/syntax"
	"github.com/pgavlin/crtea/theme"
	"github.com/pgavlin/crtea/ui"
	"github.com/pgavlin/crtea/vcs"
)

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
	th := theme.Dark()
	chromaStyle := "monokai"
	if *themeFlag == "light" {
		th = theme.Light()
		chromaStyle = "github"
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

	p := tea.NewProgram(&app)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// If --stdout, print comments as markdown
	if *stdoutFlag {
		app.ExportMarkdown(os.Stdout)
	}
}

