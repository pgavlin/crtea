package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/pgavlin/crtea/model"
	"github.com/pgavlin/crtea/persistence"
	"github.com/pgavlin/crtea/theme"
	"github.com/pgavlin/crtea/ui"
	"github.com/pgavlin/crtea/vcs"
)

func main() {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Initialize Git backend
	backend, err := vcs.NewGitBackend(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	info := backend.Info()

	// Get diff
	files, err := backend.GetWorkingTreeDiff()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting diff: %v\n", err)
		os.Exit(1)
	}

	if len(files) == 0 {
		fmt.Println("No changes to review.")
		os.Exit(0)
	}

	// Load or create session
	session, _ := persistence.LoadLatest(info.RootPath, info.BranchName, model.DiffWorkingTree)
	if session == nil {
		session = model.NewSession(info.RootPath, info.BranchName, info.HeadCommit, model.DiffWorkingTree)
	}

	// Ensure all files have review entries
	for _, f := range files {
		session.GetOrCreateFileReview(f.DisplayPath(), f.Status)
	}

	// Detect theme
	th := theme.Dark()

	// Create and run the app
	app := ui.NewApp(backend, files, session, th)
	p := tea.NewProgram(&app)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Save session on exit if dirty
	// (The app handles this via :w and :x commands)
}
