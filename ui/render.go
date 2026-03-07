package ui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/pgavlin/crtea/model"
)

// Rendering styles - these are created from the theme at render time

func (a *App) renderStatusBar() string {
	th := a.theme

	style := lipgloss.NewStyle().
		Background(th.StatusBarBg).
		Foreground(th.StatusBarFg)

	// Left: VCS info
	branchStyle := lipgloss.NewStyle().
		Foreground(th.BranchName).
		Background(th.StatusBarBg).
		Bold(true)

	left := fmt.Sprintf(" [%s:%s] ", a.vcsInfo.VcsType, branchStyle.Render(a.vcsInfo.BranchName))

	// Right: review progress
	reviewed := a.session.ReviewedCount()
	total := len(a.diffFiles)
	right := fmt.Sprintf(" %d/%d reviewed ", reviewed, total)

	// Pad middle
	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	padding := a.width - leftW - rightW
	if padding < 0 {
		padding = 0
	}

	return style.Render(left + strings.Repeat(" ", padding) + right)
}

func (a *App) renderFooter() string {
	th := a.theme
	style := lipgloss.NewStyle().
		Background(th.StatusBarBg).
		Foreground(th.StatusBarFg)

	var content string
	switch a.inputMode {
	case ModeCommand:
		content = ":" + a.commandBuffer + "█"
	case ModeSearch:
		content = "/" + a.searchBuffer + "█"
	case ModeComment:
		modeStyle := lipgloss.NewStyle().
			Background(a.commentTypeColor(a.commentType)).
			Foreground(th.ModeFg).
			Bold(true).
			Padding(0, 1)
		content = modeStyle.Render(a.commentType.String()) + " " + a.commentBuffer + "█"
	case ModeVisualSelect:
		modeStyle := lipgloss.NewStyle().
			Background(th.ModeBg).
			Foreground(th.ModeFg).
			Bold(true).
			Padding(0, 1)
		content = modeStyle.Render("VISUAL") + " Select lines, then press c to comment"
	case ModeHelp:
		content = " Press ? or Esc to close help"
	default:
		if a.message != nil {
			msgStyle := lipgloss.NewStyle().Bold(true)
			switch a.message.Level {
			case MessageInfo:
				msgStyle = msgStyle.Foreground(th.MessageInfoBg).Background(th.StatusBarBg)
			case MessageWarning:
				msgStyle = msgStyle.Foreground(th.MessageWarnBg).Background(th.StatusBarBg)
			case MessageError:
				msgStyle = msgStyle.Foreground(th.MessageErrorBg).Background(th.StatusBarBg)
			}
			content = " " + msgStyle.Render(a.message.Text)
		} else {
			comments := a.session.TotalComments()
			if comments > 0 {
				content = fmt.Sprintf(" %d comment(s) | :clip to export | ? for help", comments)
			} else {
				content = " ? for help | c to comment | r to mark reviewed"
			}
		}
	}

	w := lipgloss.Width(content)
	padding := a.width - w
	if padding < 0 {
		padding = 0
	}

	return style.Render(content + strings.Repeat(" ", padding))
}

func (a *App) renderFileList(width, height int) string {
	th := a.theme

	borderColor := th.BorderUnfocused
	if a.focusedPanel == PanelFileList {
		borderColor = th.BorderFocused
	}

	headerStyle := lipgloss.NewStyle().
		Foreground(th.FgSecondary).
		Bold(true)

	var lines []string
	lines = append(lines, headerStyle.Render(truncateOrPad("Files", width-2)))

	for i, file := range a.diffFiles {
		path := file.DisplayPath()
		status := file.Status

		statusColor := th.FileModified
		switch status {
		case model.FileAdded:
			statusColor = th.FileAdded
		case model.FileDeleted:
			statusColor = th.FileDeleted
		case model.FileRenamed:
			statusColor = th.FileRenamed
		}

		statusStyle := lipgloss.NewStyle().Foreground(statusColor)
		nameStyle := lipgloss.NewStyle().Foreground(th.FgPrimary)

		// Review marker
		marker := " "
		if fr := a.session.GetFileReview(path); fr != nil && fr.Reviewed {
			marker = lipgloss.NewStyle().Foreground(th.Reviewed).Render("✓")
		}

		// Comment count
		commentCount := ""
		if fr := a.session.GetFileReview(path); fr != nil && fr.HasComments() {
			total := len(fr.FileComments)
			for _, cs := range fr.LineComments {
				total += len(cs)
			}
			commentCount = lipgloss.NewStyle().Foreground(th.CommentNote).Render(fmt.Sprintf(" (%d)", total))
		}

		line := fmt.Sprintf(" %s %s %s%s",
			marker,
			statusStyle.Render(status.String()),
			nameStyle.Render(truncate(path, width-8)),
			commentCount,
		)

		if i == a.fileListCursor && a.focusedPanel == PanelFileList {
			line = lipgloss.NewStyle().
				Background(th.BgHighlight).
				Render(truncateOrPad(line, width-2))
		} else {
			line = truncateOrPad(line, width-2)
		}

		lines = append(lines, line)
	}

	// Pad to fill height
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width-2))
	}
	if len(lines) > height {
		lines = lines[:height]
	}

	// Apply border on right side
	var result []string
	borderChar := lipgloss.NewStyle().Foreground(borderColor).Render("│")
	for _, line := range lines {
		result = append(result, truncateOrPad(line, width-1)+borderChar)
	}

	return strings.Join(result, "\n")
}

func (a *App) renderDiffView(width, height int) string {
	th := a.theme
	vpHeight := height

	var lines []string

	end := a.scrollOffset + vpHeight
	if end > len(a.annotations) {
		end = len(a.annotations)
	}

	for i := a.scrollOffset; i < end; i++ {
		ann := a.annotations[i]
		isCursor := (i == a.cursorLine && a.focusedPanel == PanelDiff)
		isVisualSelected := a.isVisualSelected(i)

		line := a.renderAnnotatedLine(ann, width, isCursor, isVisualSelected)
		lines = append(lines, line)
	}

	// Pad remaining lines
	for len(lines) < vpHeight {
		lines = append(lines, lipgloss.NewStyle().Foreground(th.FgDim).Render("~"+strings.Repeat(" ", width-1)))
	}

	return strings.Join(lines, "\n")
}

func (a *App) isVisualSelected(idx int) bool {
	if a.inputMode != ModeVisualSelect {
		return false
	}
	if idx < 0 || idx >= len(a.annotations) {
		return false
	}
	ann := a.annotations[idx]
	if ann.Type != AnnDiffLine {
		return false
	}

	lineNo := ann.NewLineNo
	if lineNo == 0 {
		lineNo = ann.OldLineNo
	}

	cursorAnn := a.annotations[a.cursorLine]
	cursorLineNo := cursorAnn.NewLineNo
	if cursorLineNo == 0 {
		cursorLineNo = cursorAnn.OldLineNo
	}

	lo, hi := a.visualAnchor, cursorLineNo
	if lo > hi {
		lo, hi = hi, lo
	}
	return lineNo >= lo && lineNo <= hi
}

func (a *App) renderAnnotatedLine(ann AnnotatedLine, width int, isCursor, isVisualSelected bool) string {
	th := a.theme

	switch ann.Type {
	case AnnFileHeader:
		return a.renderFileHeader(ann, width, isCursor)

	case AnnHunkHeader:
		if ann.FileIdx >= 0 && ann.FileIdx < len(a.diffFiles) {
			file := a.diffFiles[ann.FileIdx]
			if ann.HunkIdx >= 0 && ann.HunkIdx < len(file.Hunks) {
				hunk := file.Hunks[ann.HunkIdx]
				style := lipgloss.NewStyle().Foreground(th.DiffHunkHdr)
				if isCursor {
					style = style.Background(th.BgHighlight)
				}
				return style.Render(truncateOrPad(hunk.Header, width))
			}
		}
		return strings.Repeat(" ", width)

	case AnnDiffLine:
		return a.renderDiffLine(ann, width, isCursor, isVisualSelected)

	case AnnFileComment:
		return a.renderCommentLine(ann, width, isCursor, true)

	case AnnLineComment:
		return a.renderCommentLine(ann, width, isCursor, false)

	case AnnExpander:
		style := lipgloss.NewStyle().Foreground(th.FgDim)
		if isCursor {
			style = style.Background(th.BgHighlight)
		}
		return style.Render(truncateOrPad(" ⋯ expand context", width))

	case AnnBinaryOrEmpty:
		style := lipgloss.NewStyle().Foreground(th.FgDim).Italic(true)
		return style.Render(truncateOrPad(" Binary file", width))

	case AnnSpacing:
		return strings.Repeat(" ", width)

	default:
		return strings.Repeat(" ", width)
	}
}

func (a *App) renderFileHeader(ann AnnotatedLine, width int, isCursor bool) string {
	th := a.theme
	if ann.FileIdx < 0 || ann.FileIdx >= len(a.diffFiles) {
		return strings.Repeat(" ", width)
	}
	file := a.diffFiles[ann.FileIdx]
	path := file.DisplayPath()

	statusColor := th.FileModified
	switch file.Status {
	case model.FileAdded:
		statusColor = th.FileAdded
	case model.FileDeleted:
		statusColor = th.FileDeleted
	case model.FileRenamed:
		statusColor = th.FileRenamed
	}

	// Review indicator
	reviewMark := ""
	if fr := a.session.GetFileReview(path); fr != nil && fr.Reviewed {
		reviewMark = lipgloss.NewStyle().Foreground(th.Reviewed).Render(" ✓")
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(th.FgPrimary)
	if isCursor {
		headerStyle = headerStyle.Background(th.BgHighlight)
	}

	statusBadge := lipgloss.NewStyle().
		Foreground(statusColor).
		Bold(true).
		Render("[" + file.Status.String() + "]")

	content := fmt.Sprintf(" %s %s%s", statusBadge, path, reviewMark)

	if file.Status == model.FileRenamed && file.OldPath != "" {
		content += lipgloss.NewStyle().Foreground(th.FgDim).Render(fmt.Sprintf(" (from %s)", file.OldPath))
	}

	return headerStyle.Render(truncateOrPad(content, width))
}

func (a *App) renderDiffLine(ann AnnotatedLine, width int, isCursor, isVisualSelected bool) string {
	th := a.theme

	if ann.FileIdx < 0 || ann.FileIdx >= len(a.diffFiles) {
		return strings.Repeat(" ", width)
	}
	file := a.diffFiles[ann.FileIdx]
	if ann.HunkIdx < 0 || ann.HunkIdx >= len(file.Hunks) {
		return strings.Repeat(" ", width)
	}
	hunk := file.Hunks[ann.HunkIdx]
	if ann.LineIdx < 0 || ann.LineIdx >= len(hunk.Lines) {
		return strings.Repeat(" ", width)
	}
	line := hunk.Lines[ann.LineIdx]

	// Line number gutter
	oldNo := "    "
	newNo := "    "
	if line.OldLineNo > 0 {
		oldNo = fmt.Sprintf("%4d", line.OldLineNo)
	}
	if line.NewLineNo > 0 {
		newNo = fmt.Sprintf("%4d", line.NewLineNo)
	}

	gutterStyle := lipgloss.NewStyle().Foreground(th.FgDim)
	gutter := gutterStyle.Render(oldNo + " " + newNo)

	// Origin marker and content style
	var marker string
	var contentStyle lipgloss.Style

	switch line.Origin {
	case model.OriginAddition:
		marker = "+"
		contentStyle = lipgloss.NewStyle().
			Foreground(th.DiffAdd).
			Background(th.DiffAddBg)
	case model.OriginDeletion:
		marker = "-"
		contentStyle = lipgloss.NewStyle().
			Foreground(th.DiffDel).
			Background(th.DiffDelBg)
	default:
		marker = " "
		contentStyle = lipgloss.NewStyle().Foreground(th.DiffContext)
	}

	if isCursor {
		contentStyle = contentStyle.Background(th.BgHighlight)
	}
	if isVisualSelected {
		contentStyle = contentStyle.Background(th.BgHighlight).Bold(true)
	}

	// Apply horizontal scroll
	content := line.Content
	if a.scrollX > 0 && len(content) > a.scrollX {
		content = content[a.scrollX:]
	} else if a.scrollX > 0 {
		content = ""
	}

	gutterWidth := 10 // "1234 5678 "
	contentWidth := width - gutterWidth - 1 // -1 for marker
	renderedContent := contentStyle.Render(truncateOrPad(content, contentWidth))

	markerStyle := contentStyle
	return gutter + markerStyle.Render(marker) + renderedContent
}

func (a *App) renderCommentLine(ann AnnotatedLine, width int, isCursor, isFileLevel bool) string {
	th := a.theme

	path := ""
	if ann.FileIdx >= 0 && ann.FileIdx < len(a.diffFiles) {
		path = a.diffFiles[ann.FileIdx].DisplayPath()
	}

	fr := a.session.GetFileReview(path)
	if fr == nil {
		return strings.Repeat(" ", width)
	}

	var comment *model.Comment
	if isFileLevel {
		if ann.CommentIdx < len(fr.FileComments) {
			comment = &fr.FileComments[ann.CommentIdx]
		}
	} else {
		// Find the line number by looking at surrounding annotations
		lineNo := a.getCommentLineNoByAnnotation(ann)
		if comments, ok := fr.LineComments[lineNo]; ok && ann.CommentIdx < len(comments) {
			comment = &comments[ann.CommentIdx]
		}
	}

	if comment == nil {
		return strings.Repeat(" ", width)
	}

	typeColor := a.commentTypeColor(comment.Type)
	typeBadge := lipgloss.NewStyle().
		Background(typeColor).
		Foreground(th.ModeFg).
		Bold(true).
		Padding(0, 1).
		Render(comment.Type.String())

	commentStyle := lipgloss.NewStyle().Foreground(th.FgPrimary)
	if isCursor {
		commentStyle = commentStyle.Background(th.BgHighlight)
	}

	// Show first line of comment
	content := comment.Content
	if idx := strings.IndexByte(content, '\n'); idx >= 0 {
		content = content[:idx] + "…"
	}

	prefix := "          " // same width as line number gutter
	if isFileLevel {
		prefix = "   📎     "
	} else {
		prefix = "   💬     "
	}

	line := prefix + typeBadge + " " + content
	return commentStyle.Render(truncateOrPad(line, width))
}

func (a *App) getCommentLineNoByAnnotation(ann AnnotatedLine) int {
	// For line comments, find the line they're attached to by checking previous annotations
	if ann.FileIdx >= 0 && ann.FileIdx < len(a.diffFiles) {
		file := a.diffFiles[ann.FileIdx]
		if ann.LineIdx >= 0 {
			for _, hunk := range file.Hunks {
				if ann.LineIdx < len(hunk.Lines) {
					line := hunk.Lines[ann.LineIdx]
					if line.NewLineNo > 0 {
						return line.NewLineNo
					}
					return line.OldLineNo
				}
			}
		}
	}
	return 0
}

func (a *App) commentTypeColor(ct model.CommentType) color.Color {
	th := a.theme
	switch ct {
	case model.CommentNote:
		return th.CommentNote
	case model.CommentSuggestion:
		return th.CommentSuggestion
	case model.CommentIssue:
		return th.CommentIssue
	case model.CommentPraise:
		return th.CommentPraise
	default:
		return th.CommentNote
	}
}

func (a *App) renderHelp(height int) string {
	th := a.theme

	helpText := []string{
		"Navigation",
		"  j/k, ↑/↓         Move cursor up/down",
		"  Ctrl-d/u          Half page down/up",
		"  Ctrl-f/b          Full page down/up",
		"  g/G               Go to first/last line",
		"  {N}G              Go to source line N",
		"  h/l, ←/→          Scroll horizontally",
		"  {/}               Next/previous file",
		"  [/]               Next/previous hunk",
		"  Tab               Switch panel focus",
		"  zz                Center cursor on screen",
		"",
		"Review",
		"  r                 Toggle file as reviewed",
		"  c                 Add line comment",
		"  C                 Add file comment",
		"  i                 Edit comment at cursor",
		"  dd                Delete comment at cursor",
		"  v                 Visual select lines",
		"  Tab (in comment)  Cycle comment type",
		"",
		"File List",
		"  ;e                Toggle file list",
		"  ;h/;l             Focus file list / diff",
		"  Enter             Jump to file (in file list)",
		"",
		"Commands",
		"  :q / :quit        Quit",
		"  :q!               Force quit",
		"  :w / :write       Save session",
		"  :x / :wq          Save and quit",
		"  :e / :reload      Reload diff",
		"  :clip / :export   Export comments to clipboard",
		"  :clear            Clear all comments",
		"",
		"Search",
		"  /pattern          Search forward",
		"  n/N               Next/previous match",
		"",
		"Press ? or Esc to close",
	}

	headerStyle := lipgloss.NewStyle().
		Foreground(th.FgPrimary).
		Bold(true)

	lineStyle := lipgloss.NewStyle().Foreground(th.FgSecondary)
	dimStyle := lipgloss.NewStyle().Foreground(th.FgDim)

	var lines []string
	for _, line := range helpText {
		if line == "" {
			lines = append(lines, "")
		} else if !strings.HasPrefix(line, "  ") {
			lines = append(lines, headerStyle.Render(line))
		} else {
			parts := strings.SplitN(line, "  ", 3)
			if len(parts) >= 3 {
				key := dimStyle.Render(parts[1])
				desc := lineStyle.Render(parts[2])
				lines = append(lines, "  "+key+"  "+desc)
			} else {
				lines = append(lines, lineStyle.Render(line))
			}
		}
	}

	// Pad to height
	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(lines) > height {
		lines = lines[:height]
	}

	return strings.Join(lines, "\n")
}

// Utility functions

func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-1] + "…"
}

func truncateOrPad(s string, width int) string {
	if width <= 0 {
		return ""
	}
	w := lipgloss.Width(s)
	if w > width {
		return truncate(s, width)
	}
	if w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}
