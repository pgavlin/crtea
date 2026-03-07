package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/pgavlin/crtea/model"
	"github.com/pgavlin/crtea/output"
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

	info := backend.Info()

	var files []model.DiffFile
	diffSource := model.DiffWorkingTree

	if *revisions != "" {
		diffSource = model.DiffCommitRange
		files, err = backend.GetRevisionDiff(*revisions)
	} else {
		files, err = backend.GetWorkingTreeDiff()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting diff: %v\n", err)
		os.Exit(1)
	}

	if len(files) == 0 {
		fmt.Println("No changes to review.")
		os.Exit(0)
	}

	// Load or create session
	session, _ := persistence.LoadLatest(info.RootPath, info.BranchName, diffSource)
	if session == nil {
		session = model.NewSession(info.RootPath, info.BranchName, info.HeadCommit, diffSource)
	}

	for _, f := range files {
		session.GetOrCreateFileReview(f.DisplayPath(), f.Status)
	}

	// Select theme and syntax highlighting style
	th := theme.Dark()
	chromaStyle := "monokai"
	if *themeFlag == "light" {
		th = theme.Light()
		chromaStyle = "github"
	}

	// Apply syntax highlighting
	highlighter := syntax.NewHighlighter(chromaStyle)
	highlighter.HighlightFiles(files)

	app := ui.NewApp(backend, files, session, th, highlighter)
	p := tea.NewProgram(&app)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// If --stdout, print comments as markdown
	if *stdoutFlag && session.TotalComments() > 0 {
		fmt.Print(output.GenerateMarkdown(session))
	}
}

