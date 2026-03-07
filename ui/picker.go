package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/pgavlin/crtea/model"
	"github.com/pgavlin/crtea/vcs"

	tea "charm.land/bubbletea/v2"
)

// appPhase tracks whether we're in the commit picker or review mode.
type appPhase int

const (
	phaseReview appPhase = iota
	phasePicker
)

const worktreeKey = "worktree"

// pickerItem represents one selectable row in the commit picker.
type pickerItem struct {
	isWorkingTree bool
	commit        vcs.CommitInfo
}

// commitListEntry represents one row in the review commit list panel.
type commitListEntry struct {
	key           string
	isWorkingTree bool
	commit        vcs.CommitInfo
}

func (a App) handlePickerKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.Key()
	switch {
	case key.Code == 'q' && key.Mod == 0:
		return &a, func() tea.Msg { return DoneMsg{Session: a.session} }
	case key.Code == 'j' && key.Mod == 0, key.Code == tea.KeyDown:
		if a.pickerCursor < len(a.pickerItems)-1 {
			a.pickerCursor++
		}
	case key.Code == 'k' && key.Mod == 0, key.Code == tea.KeyUp:
		if a.pickerCursor > 0 {
			a.pickerCursor--
		}
	case key.Code == 'g' && key.Mod == 0:
		a.pickerCursor = 0
	case key.Text == "G":
		a.pickerCursor = len(a.pickerItems) - 1
	case key.Code == ' ':
		if a.pickerSelected[a.pickerCursor] {
			delete(a.pickerSelected, a.pickerCursor)
		} else {
			a.pickerSelected[a.pickerCursor] = true
		}
	case key.Code == tea.KeyEnter:
		return a.pickerConfirm()
	}
	return &a, nil
}

func (a App) pickerConfirm() (tea.Model, tea.Cmd) {
	// If nothing selected, use cursor item
	if len(a.pickerSelected) == 0 {
		a.pickerSelected[a.pickerCursor] = true
	}

	includesWorkTree := a.pickerSelected[0]

	// Collect selected commits (newest-first, matching picker order)
	var commits []vcs.CommitInfo
	for i := 1; i < len(a.pickerItems); i++ {
		if a.pickerSelected[i] {
			commits = append(commits, a.pickerItems[i].commit)
		}
	}

	// Fetch per-commit diffs
	commitDiffs := make(map[string][]model.DiffFile)
	enabledCommits := make(map[string]bool)

	for _, c := range commits {
		files, err := a.vcs.GetRevisionDiff(c.ID + "^.." + c.ID)
		if err == nil {
			if a.highlighter != nil {
				a.highlighter.HighlightFiles(files)
			}
			commitDiffs[c.ID] = files
		}
		enabledCommits[c.ID] = true
	}

	if includesWorkTree {
		files, err := a.vcs.GetWorkingTreeDiff()
		if err == nil {
			if a.highlighter != nil {
				a.highlighter.HighlightFiles(files)
			}
			commitDiffs[worktreeKey] = files
		}
		enabledCommits[worktreeKey] = true
	}

	a.reviewCommits = commits
	a.commitDiffs = commitDiffs
	a.enabledCommits = enabledCommits
	a.includesWorkTree = includesWorkTree

	// Merge enabled diffs
	a.diffFiles = a.mergeEnabledDiffs()

	if len(a.diffFiles) == 0 {
		a.setMessage("No changes found", messageWarning)
		return &a, nil
	}

	// Session
	info := a.vcsInfo
	diffSource := model.DiffCommitRange
	if includesWorkTree && len(commits) == 0 {
		diffSource = model.DiffWorkingTree
	}
	session, _ := a.store.LoadLatest(info.RootPath, info.BranchName, diffSource)
	if session == nil {
		session = model.NewSession(info.RootPath, info.BranchName, info.HeadCommit, diffSource)
	}
	for _, f := range a.diffFiles {
		session.GetOrCreateFileReview(f.DisplayPath(), f.Status)
	}

	a.session = session
	a.phase = phaseReview
	a.inputMode = modeNormal
	a.focusedPanel = panelDiff
	a.showFileList = true
	a.rebuildFileTree()
	a.rebuildAnnotations()

	return &a, nil
}

func (a *App) renderPicker() string {
	th := a.theme
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(th.FgPrimary)
	subtitleStyle := lipgloss.NewStyle().Foreground(th.FgDim)

	b.WriteString("\n")
	b.WriteString(titleStyle.Render("  Select what to review"))
	b.WriteString("\n")
	branchStyle := lipgloss.NewStyle().Foreground(th.BranchName).Bold(true)
	b.WriteString(subtitleStyle.Render("  branch: ") + branchStyle.Render(a.vcsInfo.BranchName))
	b.WriteString("\n\n")

	// Determine visible range for scrolling
	listHeight := a.height - 7 // title + subtitle + blank + footer area
	if listHeight < 5 {
		listHeight = 5
	}
	scrollStart := 0
	if a.pickerCursor >= scrollStart+listHeight {
		scrollStart = a.pickerCursor - listHeight + 1
	}
	scrollEnd := scrollStart + listHeight
	if scrollEnd > len(a.pickerItems) {
		scrollEnd = len(a.pickerItems)
	}

	for i := scrollStart; i < scrollEnd; i++ {
		item := a.pickerItems[i]
		isCursor := i == a.pickerCursor
		isSelected := a.pickerSelected[i]

		cursor := "  "
		if isCursor {
			cursor = "> "
		}

		check := "○"
		if isSelected {
			check = "●"
		}

		cursorStyle := lipgloss.NewStyle().Foreground(th.FgPrimary).Bold(isCursor)
		checkStyle := lipgloss.NewStyle().Foreground(th.CommentNote)
		if isSelected {
			checkStyle = checkStyle.Bold(true)
		}

		if item.isWorkingTree {
			label := "Working tree changes"
			labelStyle := lipgloss.NewStyle().Foreground(th.FgPrimary)
			if isCursor {
				labelStyle = labelStyle.Background(th.BgHighlight)
			}
			b.WriteString(cursorStyle.Render(cursor) + checkStyle.Render(check) + " " + labelStyle.Render(label))
			b.WriteString("\n")
		} else {
			c := item.commit
			hashStyle := lipgloss.NewStyle().Foreground(th.FileModified)
			summaryStyle := lipgloss.NewStyle().Foreground(th.FgPrimary)
			metaStyle := lipgloss.NewStyle().Foreground(th.FgDim)
			if isCursor {
				hashStyle = hashStyle.Background(th.BgHighlight)
				summaryStyle = summaryStyle.Background(th.BgHighlight)
				metaStyle = metaStyle.Background(th.BgHighlight)
			}

			summary := c.Summary
			maxSummary := a.width - 40
			if maxSummary < 20 {
				maxSummary = 20
			}
			if len(summary) > maxSummary {
				summary = summary[:maxSummary-1] + "…"
			}

			line := cursorStyle.Render(cursor) +
				checkStyle.Render(check) + " " +
				hashStyle.Render(c.ShortID) + " " +
				summaryStyle.Render(summary) +
				metaStyle.Render("  "+c.Author+", "+relativeTime(c.Time))
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	// Pad to fill
	linesWritten := scrollEnd - scrollStart
	for i := linesWritten; i < listHeight; i++ {
		b.WriteString("\n")
	}

	// Footer
	b.WriteString("\n")
	footerStyle := lipgloss.NewStyle().Foreground(th.FgDim)
	b.WriteString(footerStyle.Render("  Space: toggle  Enter: review  q: quit"))

	if a.message != nil {
		b.WriteString("\n")
		msgStyle := lipgloss.NewStyle().Bold(true)
		switch a.message.level {
		case messageWarning:
			msgStyle = msgStyle.Foreground(th.MessageWarnBg)
		case messageError:
			msgStyle = msgStyle.Foreground(th.MessageErrorBg)
		default:
			msgStyle = msgStyle.Foreground(th.MessageInfoBg)
		}
		b.WriteString("  " + msgStyle.Render(a.message.text))
	}

	return b.String()
}

func relativeTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	case d < 30*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	default:
		return t.Format("Jan 2, 2006")
	}
}
