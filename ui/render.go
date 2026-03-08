package ui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/charmbracelet/x/ansi"
	"github.com/pgavlin/crtea/model"
)

// fileStatusColor returns the theme color for a file status.
func (a *App) fileStatusColor(status model.FileStatus) color.Color {
	th := a.theme
	switch status {
	case model.FileAdded:
		return th.FileAdded
	case model.FileDeleted:
		return th.FileDeleted
	case model.FileRenamed:
		return th.FileRenamed
	default:
		return th.FileModified
	}
}

// Rendering styles - these are created from the theme at render time

func (a *App) renderCommitList(width, height int) string {
	th := a.theme
	items := a.commitListItems()

	isFocused := a.focusedPanel == panelCommitList

	// Determine visible range (last line is separator)
	visibleItems := height - 1
	if visibleItems < 1 {
		visibleItems = 1
	}
	scrollStart := 0
	if a.commitCursor >= scrollStart+visibleItems {
		scrollStart = a.commitCursor - visibleItems + 1
	}
	scrollEnd := scrollStart + visibleItems
	if scrollEnd > len(items) {
		scrollEnd = len(items)
	}

	var lines []string
	for i := scrollStart; i < scrollEnd; i++ {
		item := items[i]
		isCursor := i == a.commitCursor && isFocused
		isEnabled := a.enabledCommits[item.key]
		row := a.renderCommitRow(item.isWorkingTree, item.commit, isCursor, isEnabled, width)
		lines = append(lines, truncateOrPad(row, width))
	}

	// Pad to fill visible area
	for len(lines) < visibleItems {
		lines = append(lines, strings.Repeat(" ", width))
	}

	// Separator line
	borderColor := th.BorderUnfocused
	if isFocused {
		borderColor = th.BorderFocused
	}
	sep := lipgloss.NewStyle().Foreground(borderColor).Render(strings.Repeat("─", width))
	lines = append(lines, sep)

	return strings.Join(lines, "\n")
}

func (a *App) renderDescription(width, height int) string {
	th := a.theme
	isFocused := a.focusedPanel == panelCommitList

	desc := ""
	if a.session != nil {
		desc = a.session.Description
	}

	// Wrap text to fit within the panel (minus indent)
	indent := "  "
	wrapWidth := width - lipgloss.Width(indent)
	if wrapWidth < 10 {
		wrapWidth = 10
	}
	descLines := wrapText(desc, wrapWidth)

	visibleLines := height - 1 // last line is separator
	if visibleLines < 1 {
		visibleLines = 1
	}

	// Scroll bounds
	if a.descScroll > len(descLines)-visibleLines {
		a.descScroll = len(descLines) - visibleLines
	}
	if a.descScroll < 0 {
		a.descScroll = 0
	}

	scrollEnd := a.descScroll + visibleLines
	if scrollEnd > len(descLines) {
		scrollEnd = len(descLines)
	}

	contentStyle := lipgloss.NewStyle().Foreground(th.FgPrimary)

	var lines []string
	for i := a.descScroll; i < scrollEnd; i++ {
		line := indent + contentStyle.Render(descLines[i])
		lines = append(lines, truncateOrPad(line, width))
	}

	for len(lines) < visibleLines {
		lines = append(lines, strings.Repeat(" ", width))
	}

	// Separator
	borderColor := th.BorderUnfocused
	if isFocused {
		borderColor = th.BorderFocused
	}
	sep := lipgloss.NewStyle().Foreground(borderColor).Render(strings.Repeat("─", width))
	lines = append(lines, sep)

	return strings.Join(lines, "\n")
}

func (a *App) renderConversation(width, height int, isFocused bool) string {
	th := a.theme

	borderColor := th.BorderUnfocused
	if isFocused {
		borderColor = th.BorderFocused
	}

	// Title separator
	titleStyle := lipgloss.NewStyle().Foreground(borderColor)
	title := titleStyle.Render(" Conversation ")
	sepStyle := lipgloss.NewStyle().Foreground(borderColor)
	titleWidth := lipgloss.Width(title)
	leftWidth := 2
	rightWidth := width - leftWidth - titleWidth
	if rightWidth < 0 {
		rightWidth = 0
	}
	sep := sepStyle.Render(strings.Repeat("─", leftWidth)) + title
	if rightWidth > 0 {
		sep += sepStyle.Render(strings.Repeat("─", rightWidth))
	}

	// Editor takes some lines at the bottom when composing
	editorHeight := 0
	if a.inputMode == modeConversation {
		editorHeight = 4 // top border + 2 content lines + bottom border
		if editorHeight > height-2 {
			editorHeight = height - 2
		}
		if editorHeight < 3 {
			editorHeight = 3
		}
	}

	visibleLines := height - 1 - editorHeight // -1 for top separator
	if visibleLines < 1 {
		visibleLines = 1
	}

	authorStyle := lipgloss.NewStyle().Foreground(th.BranchName).Bold(true)
	bodyStyle := lipgloss.NewStyle().Foreground(th.FgPrimary)
	timeStyle := lipgloss.NewStyle().Foreground(th.FgDim)

	var contentLines []string
	if a.session != nil {
		for _, cc := range a.session.Conversation {
			header := "  " + authorStyle.Render("@"+cc.Author)
			if !cc.CreatedAt.IsZero() {
				header += " " + timeStyle.Render(cc.CreatedAt.Format("Jan 2 15:04"))
			}
			contentLines = append(contentLines, truncateOrPad(header, width))
			for _, bodyLine := range strings.Split(cc.Body, "\n") {
				contentLines = append(contentLines, truncateOrPad("    "+bodyStyle.Render(bodyLine), width))
			}
		}
	}

	if len(contentLines) == 0 && a.inputMode != modeConversation {
		emptyMsg := lipgloss.NewStyle().Foreground(th.FgDim).Render("  No conversation yet. Press c to start one.")
		contentLines = append(contentLines, truncateOrPad(emptyMsg, width))
	}

	// Auto-scroll to bottom when composing
	if a.inputMode == modeConversation {
		a.convScroll = len(contentLines) - visibleLines
	}

	// Scroll
	if a.convScroll > len(contentLines)-visibleLines {
		a.convScroll = len(contentLines) - visibleLines
	}
	if a.convScroll < 0 {
		a.convScroll = 0
	}
	scrollEnd := a.convScroll + visibleLines
	if scrollEnd > len(contentLines) {
		scrollEnd = len(contentLines)
	}

	lines := []string{sep}
	for i := a.convScroll; i < scrollEnd; i++ {
		lines = append(lines, contentLines[i])
	}
	for len(lines) < height-editorHeight {
		lines = append(lines, strings.Repeat(" ", width))
	}

	// Inline editor
	if editorHeight > 0 {
		lines = append(lines, a.renderConversationEditor(width, editorHeight)...)
	}

	return strings.Join(lines, "\n")
}

func (a *App) renderConversationEditor(width, height int) []string {
	th := a.theme
	label := lipgloss.NewStyle().Foreground(th.FgPrimary).Bold(true).Render(" New message ")
	rendered := a.renderEditorBoxFull(width, height, th.BorderFocused, a.convBuffer, a.convCursor, label)
	return strings.Split(rendered, "\n")
}

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

	var left string
	dirtyMark := ""
	if a.dirty {
		dirtyMark = " [+]"
	}

	if a.session != nil && a.session.Provider != nil {
		pi := a.session.Provider
		label := fmt.Sprintf(" [%s:%s] %s #%s", a.vcsInfo.VcsType, branchStyle.Render(a.vcsInfo.BranchName), pi.Name, pi.ID)
		if a.session.Description != "" {
			// Show just the first line of the description (title)
			title := strings.SplitN(a.session.Description, "\n", 2)[0]
			maxTitle := a.width / 2
			if len([]rune(title)) > maxTitle {
				title = string([]rune(title)[:maxTitle-1]) + "…"
			}
			label += ": " + title
		}
		if a.session.IsDraft {
			label += " [DRAFT]"
		}
		left = label + dirtyMark + " "
	} else {
		left = fmt.Sprintf(" [%s:%s]%s ", a.vcsInfo.VcsType, branchStyle.Render(a.vcsInfo.BranchName), dirtyMark)
	}

	// Right: review progress + scroll position + horizontal scroll
	reviewed := a.session.ReviewedCount()
	total := len(a.diffFiles)
	right := fmt.Sprintf(" %d/%d reviewed", reviewed, total)

	// Scroll position percentage
	if len(a.annotations) > 0 {
		vpHeight := a.diffViewportHeight()
		if a.cursorLine == 0 && len(a.annotations) <= vpHeight {
			right += "  All"
		} else if a.cursorLine == 0 {
			right += "  Top"
		} else if a.cursorLine >= len(a.annotations)-1 {
			right += "  Bot"
		} else {
			pct := a.cursorLine * 100 / (len(a.annotations) - 1)
			right += fmt.Sprintf("  %d%%", pct)
		}
	}

	// Horizontal scroll indicator
	if a.scrollX > 0 {
		right += fmt.Sprintf("  Col %d", a.scrollX)
	}

	right += " "

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
	case modeCommand:
		content = ":" + a.commandBuffer + "█"
	case modeSearch:
		content = "/" + a.searchBuffer + "█"
	case modeComment:
		if a.replyToID != "" {
			content = " Reply | Enter: save | Esc: cancel | Shift-Enter: newline"
		} else {
			content = " Enter: save | Esc: cancel | Tab: cycle type | Shift-Enter: newline"
		}
	case modeConversation:
		content = " Enter: post | Esc: cancel | Shift-Enter: newline"
	case modeReview:
		content = " Enter: save | Esc: cancel | Tab: cycle status | Shift-Enter: newline"
	case modeEditPR:
		content = " Enter: save | Esc: cancel | Shift-Enter: newline"
	case modeBug:
		content = " Enter: submit | Esc: cancel | Shift-Enter: newline"
	case modeConfirm:
		content = " " + a.confirmPrompt + " [y/n]"
	case modeVisualSelect:
		modeStyle := lipgloss.NewStyle().
			Background(th.ModeBg).
			Foreground(th.ModeFg).
			Bold(true).
			Padding(0, 1)
		content = modeStyle.Render("VISUAL") + " Select lines, then press c to comment"
	case modeHelp:
		content = " Press ? or Esc to close help"
	default:
		if a.message != nil {
			msgStyle := lipgloss.NewStyle().Bold(true)
			switch a.message.level {
			case messageInfo:
				msgStyle = msgStyle.Foreground(th.MessageInfoBg).Background(th.StatusBarBg)
			case messageWarning:
				msgStyle = msgStyle.Foreground(th.MessageWarnBg).Background(th.StatusBarBg)
			case messageError:
				msgStyle = msgStyle.Foreground(th.MessageErrorBg).Background(th.StatusBarBg)
			}
			content = " " + msgStyle.Render(a.message.text)
		} else if a.focusedPanel == panelConversation {
			content = " j/k: scroll | c/Enter: new message | P: close | Tab: switch panel"
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
	if a.focusedPanel == panelFileList {
		borderColor = th.BorderFocused
	}

	headerStyle := lipgloss.NewStyle().
		Foreground(th.FgSecondary).
		Bold(true)

	var lines []string
	lines = append(lines, headerStyle.Render(truncateOrPad("Files", width-2)))

	visibleRows := height - 1 // header takes 1 line
	scrollEnd := a.fileListScroll + visibleRows
	if scrollEnd > len(a.fileTreeRows) {
		scrollEnd = len(a.fileTreeRows)
	}

	for i := a.fileListScroll; i < scrollEnd; i++ {
		row := a.fileTreeRows[i]
		node := row.Node
		isSelected := i == a.fileListCursor && a.focusedPanel == panelFileList

		// When selected, all styles get highlight background
		bg := func(s lipgloss.Style) lipgloss.Style {
			if isSelected {
				return s.Background(th.BgHighlight)
			}
			return s
		}

		// Build tree guide prefix
		var prefix strings.Builder
		for d := 0; d < row.Depth; d++ {
			if d < len(row.IsLast) && row.IsLast[d] {
				prefix.WriteString("  ")
			} else {
				prefix.WriteString("│ ")
			}
		}
		if row.Depth > 0 {
			if row.IsLast[len(row.IsLast)-1] {
				prefix.WriteString("└ ")
			} else {
				prefix.WriteString("├ ")
			}
		}

		treePrefix := bg(lipgloss.NewStyle().Foreground(th.FgDim)).Render(prefix.String())

		var line string
		if row.IsDir {
			dirStyle := bg(lipgloss.NewStyle().Foreground(th.FgSecondary).Bold(true))
			arrow := "▼ "
			if a.collapsedDirs[node.Path] {
				arrow = "▶ "
			}
			arrowStyle := bg(lipgloss.NewStyle().Foreground(th.FgDim))
			line = bg(lipgloss.NewStyle()).Render(" ") + treePrefix + arrowStyle.Render(arrow) + dirStyle.Render(node.Name+"/")
		} else {
			fileIdx := node.FileIdx
			file := a.diffFiles[fileIdx]
			path := file.DisplayPath()

			statusStyle := bg(lipgloss.NewStyle().Foreground(a.fileStatusColor(file.Status)))

			marker := bg(lipgloss.NewStyle()).Render(" ")
			if fr := a.session.GetFileReview(path); fr != nil && fr.Reviewed {
				marker = bg(lipgloss.NewStyle().Foreground(th.Reviewed)).Render("✓")
			}

			commentCount := ""
			if fr := a.session.GetFileReview(path); fr != nil && fr.HasComments() {
				total := len(fr.FileComments)
				for _, cs := range fr.LineComments {
					total += len(cs)
				}
				commentCount = bg(lipgloss.NewStyle().Foreground(th.CommentNote)).Render(fmt.Sprintf(" (%d)", total))
			}

			nameStyle := bg(lipgloss.NewStyle().Foreground(th.FgPrimary))
			line = bg(lipgloss.NewStyle()).Render(" ") + treePrefix + marker + statusStyle.Render(file.Status.String()) + bg(lipgloss.NewStyle()).Render(" ") + nameStyle.Render(node.Name) + commentCount
		}

		lineWidth := width - 2
		w := lipgloss.Width(line)
		pad := lineWidth - w
		if pad > 0 {
			line = line + bg(lipgloss.NewStyle()).Render(strings.Repeat(" ", pad))
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

	editorInjected := false
	for i := a.scrollOffset; i < end && len(lines) < vpHeight; i++ {
		ann := a.annotations[i]
		isCursor := (i == a.cursorLine && a.focusedPanel == panelDiff)
		isVisualSelected := a.isVisualSelected(i)

		// When editing an existing comment, replace its annotation lines with the editor
		if a.inputMode == modeComment && a.editingID != "" && a.isEditingAnnotation(ann) {
			if !editorInjected {
				editorInjected = true
				for _, el := range a.renderCommentEditor(width) {
					if len(lines) >= vpHeight {
						break
					}
					lines = append(lines, el)
				}
			}
			continue // skip the original comment's annotation lines
		}

		line := a.renderAnnotatedLine(ann, width, isCursor, isVisualSelected)
		lines = append(lines, line)

		// Inject inline comment editor after the cursor line (new comments)
		if i == a.cursorLine && a.inputMode == modeComment && a.editingID == "" {
			for _, el := range a.renderCommentEditor(width) {
				if len(lines) >= vpHeight {
					break
				}
				lines = append(lines, el)
			}
		}
	}

	// Pad remaining lines
	padWidth := width - 1
	if padWidth < 0 {
		padWidth = 0
	}
	for len(lines) < vpHeight {
		lines = append(lines, lipgloss.NewStyle().Foreground(th.FgDim).Render("~"+strings.Repeat(" ", padWidth)))
	}

	return strings.Join(lines, "\n")
}

func (a *App) isVisualSelected(idx int) bool {
	if a.inputMode != modeVisualSelect {
		return false
	}
	if idx < 0 || idx >= len(a.annotations) {
		return false
	}
	ann := a.annotations[idx]
	if ann.Type != annDiffLine {
		return false
	}

	cursorAnn := a.annotations[a.cursorLine]

	// Determine cursor side: old-only (deletion) vs new (addition or context).
	cursorSide := model.SideNew
	if cursorAnn.NewLineNo == 0 {
		cursorSide = model.SideOld
	}

	anchorSide := a.visualAnchorSide

	if anchorSide == model.SideOld && cursorSide == model.SideOld {
		// Both endpoints are old-side: only select lines that have OldLineNo.
		if ann.OldLineNo == 0 {
			return false
		}
		lo, hi := a.visualAnchor, cursorAnn.OldLineNo
		if lo > hi {
			lo, hi = hi, lo
		}
		return ann.OldLineNo >= lo && ann.OldLineNo <= hi
	}

	if anchorSide == model.SideNew && cursorSide == model.SideNew {
		// Both endpoints are new-side: only select lines that have NewLineNo.
		if ann.NewLineNo == 0 {
			return false
		}
		lo, hi := a.visualAnchor, cursorAnn.NewLineNo
		if lo > hi {
			lo, hi = hi, lo
		}
		return ann.NewLineNo >= lo && ann.NewLineNo <= hi
	}

	// Mixed selection (one old, one new): select all diff lines between
	// the anchor and cursor screen positions.
	anchorIdx := a.visualAnchorIdx()
	if anchorIdx < 0 {
		return false
	}
	lo, hi := anchorIdx, a.cursorLine
	if lo > hi {
		lo, hi = hi, lo
	}
	return idx >= lo && idx <= hi
}

// visualAnchorIdx finds the current annotation index for the visual anchor.
func (a *App) visualAnchorIdx() int {
	for i, ann := range a.annotations {
		if ann.Type != annDiffLine {
			continue
		}
		if a.visualAnchorSide == model.SideOld && ann.OldLineNo == a.visualAnchor {
			return i
		}
		if a.visualAnchorSide == model.SideNew && ann.NewLineNo == a.visualAnchor {
			return i
		}
	}
	return -1
}

func (a *App) renderAnnotatedLine(ann annotatedLine, width int, isCursor, isVisualSelected bool) string {
	if width < 0 {
		width = 0
	}
	th := a.theme

	switch ann.Type {
	case annFileHeader:
		return a.renderFileHeader(ann, width, isCursor)

	case annHunkHeader:
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

	case annDiffLine:
		return a.renderDiffLine(ann, width, isCursor, isVisualSelected)

	case annFileComment:
		return a.renderCommentLine(ann, width, isCursor, true)

	case annLineComment:
		return a.renderCommentLine(ann, width, isCursor, false)

	case annExpander:
		style := lipgloss.NewStyle().Foreground(th.FgDim)
		if isCursor {
			style = style.Background(th.BgHighlight)
		}
		// Show line count in expander hint
		gapLines := 0
		if ann.FileIdx >= 0 && ann.FileIdx < len(a.diffFiles) {
			file := a.diffFiles[ann.FileIdx]
			if ann.gapID.HunkIdx > 0 && ann.gapID.HunkIdx < len(file.Hunks) {
				prevHunk := file.Hunks[ann.gapID.HunkIdx-1]
				thisHunk := file.Hunks[ann.gapID.HunkIdx]
				gapLines = thisHunk.NewStart - (prevHunk.NewStart + prevHunk.NewCount)
			}
		}
		hint := " ⋯ expand context [Enter to expand]"
		if gapLines > 0 {
			hint = fmt.Sprintf(" ⋯ %d lines hidden [Enter to expand]", gapLines)
		}
		return style.Render(truncateOrPad(hint, width))

	case annExpandedContext:
		return a.renderExpandedContextLine(ann, width, isCursor)

	case annBinaryOrEmpty:
		style := lipgloss.NewStyle().Foreground(th.FgDim).Italic(true)
		return style.Render(truncateOrPad(" Binary file", width))

	case annSpacing:
		if width < 0 {
			width = 0
		}
		return strings.Repeat(" ", width)

	default:
		return strings.Repeat(" ", width)
	}
}

func (a *App) renderFileHeader(ann annotatedLine, width int, isCursor bool) string {
	th := a.theme
	if ann.FileIdx < 0 || ann.FileIdx >= len(a.diffFiles) {
		return strings.Repeat(" ", width)
	}
	file := a.diffFiles[ann.FileIdx]
	path := file.DisplayPath()

	statusColor := a.fileStatusColor(file.Status)

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

func (a *App) renderDiffLine(ann annotatedLine, width int, isCursor, isVisualSelected bool) string {
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
		gutterStyle = gutterStyle.Background(th.BgHighlight)
	}
	if isVisualSelected {
		contentStyle = contentStyle.Background(th.BgHighlight).Bold(true)
		gutterStyle = gutterStyle.Background(th.BgHighlight)
	}

	gutter := gutterStyle.Render(oldNo + " " + newNo + " ")

	gutterWidth := 10                       // "1234 5678 "
	contentWidth := width - gutterWidth - 1 // -1 for marker
	markerStyle := contentStyle

	// Determine background for syntax-highlighted spans
	var bgColor color.Color
	switch line.Origin {
	case model.OriginAddition:
		bgColor = th.DiffAddBg
	case model.OriginDeletion:
		bgColor = th.DiffDelBg
	}
	if isCursor || isVisualSelected {
		bgColor = th.BgHighlight
	}

	// Search highlight pattern (case-insensitive)
	searchPattern := strings.ToLower(a.searchHighlight)

	if line.Spans != nil {
		rendered := a.renderSyntaxContent(line.Spans, contentWidth, contentStyle, bgColor, searchPattern)
		return gutter + markerStyle.Render(marker) + rendered
	}

	rendered := a.renderPlainContent(line.Content, contentWidth, contentStyle, searchPattern)
	return gutter + markerStyle.Render(marker) + rendered
}

// spanSegment represents a visible span of text with its foreground color and column position.
type spanSegment struct {
	text  string
	fg    color.Color
	start int
}

// renderSyntaxContent renders syntax-highlighted spans with horizontal scroll and search highlighting.
func (a *App) renderSyntaxContent(spans []model.StyledSpan, contentWidth int, contentStyle lipgloss.Style, bgColor color.Color, searchPattern string) string {
	th := a.theme
	var segments []spanSegment
	col := 0
	skipped := 0

	for _, span := range spans {
		if col >= contentWidth {
			break
		}
		text := expandTabs(span.Text)
		textWidth := lipgloss.Width(text)

		if a.scrollX > 0 && skipped < a.scrollX {
			if skipped+textWidth <= a.scrollX {
				skipped += textWidth
				continue
			}
			trimCols := a.scrollX - skipped
			prefix := ansi.Truncate(text, trimCols, "")
			text = text[len(prefix):]
			textWidth = lipgloss.Width(text)
			skipped = a.scrollX
		}

		remaining := contentWidth - col
		if textWidth > remaining {
			text = ansi.Truncate(text, remaining, "")
			textWidth = lipgloss.Width(text)
		}

		var fg color.Color
		if span.FG != "" {
			fg = lipgloss.Color(span.FG)
		} else {
			fg = contentStyle.GetForeground()
		}
		segments = append(segments, spanSegment{text: text, fg: fg, start: col})
		col += textWidth
	}

	var matchRanges [][2]int
	if searchPattern != "" {
		var fullText strings.Builder
		for _, seg := range segments {
			fullText.WriteString(seg.text)
		}
		matchRanges = findMatchRanges(fullText.String(), searchPattern)
	}

	var rendered strings.Builder
	if len(matchRanges) > 0 {
		hlStyle := lipgloss.NewStyle().
			Background(th.SearchMatch).
			Foreground(th.SearchMatchFg)
		charOffset := 0
		matchIdx := 0
		for _, seg := range segments {
			segEnd := charOffset + len(seg.text)
			pos := 0
			for pos < len(seg.text) && matchIdx < len(matchRanges) {
				mStart := matchRanges[matchIdx][0] - charOffset
				mEnd := matchRanges[matchIdx][1] - charOffset
				if mStart >= len(seg.text) {
					break
				}
				if mEnd <= 0 {
					matchIdx++
					continue
				}
				if mStart < 0 {
					mStart = 0
				}
				if mEnd > len(seg.text) {
					mEnd = len(seg.text)
				}
				if mStart > pos {
					beforeStyle := lipgloss.NewStyle().Foreground(seg.fg)
					if bgColor != nil {
						beforeStyle = beforeStyle.Background(bgColor)
					}
					rendered.WriteString(beforeStyle.Render(seg.text[pos:mStart]))
				}
				rendered.WriteString(hlStyle.Render(seg.text[mStart:mEnd]))
				pos = mEnd
				if matchRanges[matchIdx][1] <= segEnd {
					matchIdx++
				} else {
					break
				}
			}
			if pos < len(seg.text) {
				remStyle := lipgloss.NewStyle().Foreground(seg.fg)
				if bgColor != nil {
					remStyle = remStyle.Background(bgColor)
				}
				rendered.WriteString(remStyle.Render(seg.text[pos:]))
			}
			charOffset = segEnd
		}
	} else {
		for _, seg := range segments {
			spanStyle := lipgloss.NewStyle().Foreground(seg.fg)
			if bgColor != nil {
				spanStyle = spanStyle.Background(bgColor)
			}
			rendered.WriteString(spanStyle.Render(seg.text))
		}
	}

	if col < contentWidth {
		padStyle := lipgloss.NewStyle()
		if bgColor != nil {
			padStyle = padStyle.Background(bgColor)
		}
		rendered.WriteString(padStyle.Render(strings.Repeat(" ", contentWidth-col)))
	}
	return rendered.String()
}

// renderPlainContent renders plain text content with horizontal scroll and search highlighting.
func (a *App) renderPlainContent(rawContent string, contentWidth int, contentStyle lipgloss.Style, searchPattern string) string {
	th := a.theme
	content := expandTabs(rawContent)
	if a.scrollX > 0 {
		if lipgloss.Width(content) > a.scrollX {
			prefix := ansi.Truncate(content, a.scrollX, "")
			content = content[len(prefix):]
		} else {
			content = ""
		}
	}
	padded := truncateOrPad(content, contentWidth)

	if searchPattern != "" {
		matchRanges := findMatchRanges(padded, searchPattern)
		if len(matchRanges) > 0 {
			hlStyle := lipgloss.NewStyle().
				Background(th.SearchMatch).
				Foreground(th.SearchMatchFg)
			var rendered strings.Builder
			pos := 0
			for _, mr := range matchRanges {
				if mr[0] > pos {
					rendered.WriteString(contentStyle.Render(padded[pos:mr[0]]))
				}
				rendered.WriteString(hlStyle.Render(padded[mr[0]:mr[1]]))
				pos = mr[1]
			}
			if pos < len(padded) {
				rendered.WriteString(contentStyle.Render(padded[pos:]))
			}
			return rendered.String()
		}
	}

	return contentStyle.Render(padded)
}

func (a *App) renderExpandedContextLine(ann annotatedLine, width int, isCursor bool) string {
	th := a.theme

	expanded, ok := a.expandedGaps[ann.gapID]
	if !ok || ann.LineIdx < 0 || ann.LineIdx >= len(expanded) {
		return strings.Repeat(" ", width)
	}
	line := expanded[ann.LineIdx]

	oldNo := fmt.Sprintf("%4d", line.OldLineNo)
	newNo := fmt.Sprintf("%4d", line.NewLineNo)

	gutterStyle := lipgloss.NewStyle().Foreground(th.FgDim)

	contentStyle := lipgloss.NewStyle().Foreground(th.ExpandedCtxFg)
	if isCursor {
		contentStyle = contentStyle.Background(th.BgHighlight)
		gutterStyle = gutterStyle.Background(th.BgHighlight)
	}

	gutter := gutterStyle.Render(oldNo + " " + newNo + " ")

	content := expandTabs(line.Content)
	if a.scrollX > 0 {
		if lipgloss.Width(content) > a.scrollX {
			prefix := ansi.Truncate(content, a.scrollX, "")
			content = content[len(prefix):]
		} else {
			content = ""
		}
	}

	gutterWidth := 10
	contentWidth := width - gutterWidth - 1
	return gutter + contentStyle.Render(" "+truncateOrPad(content, contentWidth))
}

func (a *App) renderCommentLine(ann annotatedLine, width int, isCursor, isFileLevel bool) string {
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
		lineNo := ann.NewLineNo
		if ann.Side == model.SideOld {
			lineNo = ann.OldLineNo
		}
		if comments, ok := fr.LineComments[lineNo]; ok && ann.CommentIdx < len(comments) {
			comment = &comments[ann.CommentIdx]
		}
	}

	if comment == nil {
		return strings.Repeat(" ", width)
	}

	typeColor := a.commentTypeColor(comment.Type)

	gutter := "          " // same width as line number gutter (10 chars)
	boxWidth := width - len(gutter)
	if boxWidth < 10 {
		boxWidth = 10
	}
	innerWidth := boxWidth - 4 // "│ " + content + " │"

	borderStyle := lipgloss.NewStyle().Foreground(typeColor)
	contentStyle := lipgloss.NewStyle().Foreground(th.FgPrimary)
	if isCursor {
		contentStyle = contentStyle.Background(th.BgHighlight)
	}

	isFirst := ann.CommentLine == 0
	isLast := ann.CommentLine == ann.CommentLines-1

	// Determine if this is a remote comment from someone else
	isRemote := comment.Author != "" && a.session != nil && comment.Author != a.session.Reviewer

	if isRemote {
		contentStyle = lipgloss.NewStyle().Foreground(th.FgDim)
		if isCursor {
			contentStyle = contentStyle.Background(th.BgHighlight)
		}
	}

	if isFirst {
		if ann.IsReply {
			// Thread separator instead of full top border
			badgeText := "@" + comment.Author
			if comment.Author == "" {
				badgeText = "Reply"
			}
			badge := lipgloss.NewStyle().
				Foreground(th.FgDim).
				Render(" " + badgeText + " ")
			badgeWidth := lipgloss.Width(badge)
			restWidth := boxWidth - 3 - badgeWidth // "├" + "─" + badge + "─"×rest + "┤"
			if restWidth < 1 {
				restWidth = 1
			}
			line := gutter + borderStyle.Render("├") + borderStyle.Render("─") + badge + borderStyle.Render(strings.Repeat("─", restWidth)+"┤")
			return truncateOrPad(line, width)
		}

		// Top border + type badge (with optional author)
		badgeText := comment.Type.String()
		if comment.Author != "" {
			badgeText += " (@" + comment.Author + ")"
		}
		if comment.IsOutdated {
			badgeText += " [outdated]"
		}
		if comment.IsResolved {
			badgeText += " [resolved]"
		}
		typeBadge := lipgloss.NewStyle().
			Background(typeColor).
			Foreground(th.ModeFg).
			Bold(true).
			Padding(0, 1).
			Render(badgeText)

		badgeWidth := lipgloss.Width(typeBadge)
		restWidth := boxWidth - 2 - badgeWidth // "╭" + badge + "───╮"
		if restWidth < 1 {
			restWidth = 1
		}
		line := gutter + borderStyle.Render("╭") + typeBadge + borderStyle.Render(strings.Repeat("─", restWidth)+"╮")
		return truncateOrPad(line, width)
	}

	if isLast && ann.CommentLine > 0 && !ann.HasReplyAfter {
		// Bottom border
		line := gutter + borderStyle.Render("╰"+strings.Repeat("─", boxWidth-2)+"╯")
		return truncateOrPad(line, width)
	}

	// Content lines (wrapped)
	wrapWidth := innerWidth
	if wrapWidth < 10 {
		wrapWidth = 0
	}
	wrapped := wrapComment(comment.Content, wrapWidth)
	lineIdx := ann.CommentLine - 1
	text := ""
	if lineIdx >= 0 && lineIdx < len(wrapped) {
		text = wrapped[lineIdx]
	}

	inner := truncateOrPad(text, innerWidth)
	line := gutter + borderStyle.Render("│") + " " + contentStyle.Render(inner) + " " + borderStyle.Render("│")
	return truncateOrPad(line, width)
}

// isEditingAnnotation returns true if the annotation belongs to the comment currently being edited.
func (a *App) isEditingAnnotation(ann annotatedLine) bool {
	if a.editingID == "" {
		return false
	}

	isCommentAnn := ann.Type == annFileComment || ann.Type == annLineComment
	if !isCommentAnn {
		return false
	}

	path := ""
	if ann.FileIdx >= 0 && ann.FileIdx < len(a.diffFiles) {
		path = a.diffFiles[ann.FileIdx].DisplayPath()
	}
	fr := a.session.GetFileReview(path)
	if fr == nil {
		return false
	}

	if ann.Type == annFileComment {
		if ann.CommentIdx < len(fr.FileComments) {
			return fr.FileComments[ann.CommentIdx].ID == a.editingID
		}
	} else {
		lineNo := ann.NewLineNo
		if ann.Side == model.SideOld {
			lineNo = ann.OldLineNo
		}
		if comments, ok := fr.LineComments[lineNo]; ok && ann.CommentIdx < len(comments) {
			return comments[ann.CommentIdx].ID == a.editingID
		}
	}

	return false
}

func (a *App) renderCommentEditor(width int) []string {
	th := a.theme
	typeColor := a.commentTypeColor(a.commentType)

	gutter := "          "
	boxWidth := width - len(gutter)
	if boxWidth < 10 {
		boxWidth = 10
	}
	innerWidth := boxWidth - 4 // "│ " + content + " │"

	borderStyle := lipgloss.NewStyle().Foreground(typeColor)
	contentStyle := lipgloss.NewStyle().Foreground(th.FgPrimary)

	// Type badge for top border
	typeBadge := lipgloss.NewStyle().
		Background(typeColor).
		Foreground(th.ModeFg).
		Bold(true).
		Padding(0, 1).
		Render(a.commentType.String())

	// Top border
	badgeWidth := lipgloss.Width(typeBadge)
	restWidth := boxWidth - 2 - badgeWidth
	if restWidth < 1 {
		restWidth = 1
	}
	var lines []string
	lines = append(lines, gutter+borderStyle.Render("╭")+typeBadge+borderStyle.Render(strings.Repeat("─", restWidth)+"╮"))

	// Wrap the buffer content, inserting cursor
	buf := a.commentBuffer
	cursor := a.commentCursor
	// Insert cursor marker
	before := buf[:cursor]
	after := buf[cursor:]
	display := before + "█" + after

	wrapWidth := innerWidth
	if wrapWidth < 10 {
		wrapWidth = 0
	}
	wrapped := wrapComment(display, wrapWidth)
	if len(wrapped) == 0 {
		wrapped = []string{"█"}
	}

	for _, wl := range wrapped {
		inner := truncateOrPad(wl, innerWidth)
		lines = append(lines, gutter+borderStyle.Render("│")+" "+contentStyle.Render(inner)+" "+borderStyle.Render("│"))
	}

	// Bottom border
	lines = append(lines, gutter+borderStyle.Render("╰"+strings.Repeat("─", boxWidth-2)+"╯"))

	return lines
}

// renderEditorBoxFull renders a bordered text editor box with header content, buffer, and cursor.
func (a *App) renderEditorBoxFull(width, height int, borderColor color.Color, buf string, cursor int, headerParts ...string) string {
	th := a.theme
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)

	boxWidth := width
	innerWidth := boxWidth - 4 // "│ " + content + " │"
	if innerWidth < 10 {
		innerWidth = 10
	}

	// Top border with header parts
	header := strings.Join(headerParts, "")
	headerWidth := lipgloss.Width(header)
	restWidth := boxWidth - 2 - headerWidth
	if restWidth < 1 {
		restWidth = 1
	}

	var lines []string
	lines = append(lines, borderStyle.Render("╭")+header+borderStyle.Render(strings.Repeat("─", restWidth)+"╮"))

	// Content lines with cursor
	before := buf[:cursor]
	after := buf[cursor:]
	display := before + "█" + after

	wrapWidth := innerWidth
	if wrapWidth < 10 {
		wrapWidth = 0
	}
	wrapped := wrapComment(display, wrapWidth)
	if len(wrapped) == 0 {
		wrapped = []string{"█"}
	}

	contentStyle := lipgloss.NewStyle().Foreground(th.FgPrimary)
	maxContentLines := height - 2 // top + bottom border
	for i, wl := range wrapped {
		if i >= maxContentLines {
			break
		}
		inner := truncateOrPad(wl, innerWidth)
		lines = append(lines, borderStyle.Render("│")+" "+contentStyle.Render(inner)+" "+borderStyle.Render("│"))
	}

	// Pad to fill remaining height
	for len(lines) < height-1 {
		lines = append(lines, borderStyle.Render("│")+" "+contentStyle.Render(strings.Repeat(" ", innerWidth))+" "+borderStyle.Render("│"))
	}

	// Bottom border
	lines = append(lines, borderStyle.Render("╰"+strings.Repeat("─", boxWidth-2)+"╯"))

	return strings.Join(lines, "\n")
}

func (a *App) renderReviewEditor(width, height int) string {
	th := a.theme

	var statusColor color.Color
	switch a.reviewStatus {
	case model.ApprovalApprove:
		statusColor = th.Reviewed
	case model.ApprovalRequestChanges:
		statusColor = th.CommentIssue
	default:
		statusColor = th.FgDim
	}

	badge := lipgloss.NewStyle().
		Background(statusColor).
		Foreground(th.ModeFg).
		Bold(true).
		Padding(0, 1).
		Render(a.reviewStatus.String())

	title := lipgloss.NewStyle().Foreground(th.FgPrimary).Bold(true).Render(" Overall Review ")

	return a.renderEditorBoxFull(width, height, statusColor, a.reviewBuffer, a.reviewCursor, badge, title)
}

func (a *App) renderBugEditor(width, height int) string {
	th := a.theme
	title := lipgloss.NewStyle().Foreground(th.FgPrimary).Bold(true).Render(" Bug Report ")
	return a.renderEditorBoxFull(width, height, th.CommentIssue, a.bugBuffer, a.bugCursor, title)
}

func (a *App) renderPREditor(width, height int) string {
	th := a.theme
	title := lipgloss.NewStyle().Foreground(th.FgPrimary).Bold(true).Render(" Edit PR Description ")
	return a.renderEditorBoxFull(width, height, th.FgPrimary, a.reviewBuffer, a.reviewCursor, title)
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
	case model.CommentQuestion:
		return th.CommentQuestion
	default:
		return th.CommentNote
	}
}

func (a *App) renderHelp(height int) string {
	th := a.theme

	helpText := []string{
		"Navigation",
		"  j/k, ↑/↓          Move cursor up/down",
		"  Ctrl-d/u          Half page down/up",
		"  Ctrl-f/b          Full page down/up",
		"  g/G               Go to first/last line",
		"  {N}G              Go to source line N",
		"  h/l, ←/→          Scroll horizontally",
		"  0/Home            Reset horizontal scroll",
		"  {/}               Next/previous file",
		"  [/]               Next/previous hunk",
		"  ;n/;p             Next/previous unreviewed file",
		"  Tab               Switch panel focus",
		"  zz                Center cursor on screen",
		"",
		"Review",
		"  r                 Toggle file as reviewed",
		"  c                 Add line comment",
		"  C                 Add file comment",
		"  i                 Edit comment at cursor",
		"  a                 Reply to comment at cursor",
		"  dd                Delete comment at cursor",
		"  v                 Visual select lines",
		"  R                 Overall review",
		"  ;r                Resolve/unresolve thread",
		"  Tab (in comment)  Cycle comment type",
		"",
		"File List",
		"  ;e                Toggle file list",
		"  ;h/;l             Focus file list / diff",
		"  Enter             Jump to file (in file list)",
		"",
		"Commit List / Description",
		"  Tab               Cycle panel focus",
		"  Space             Toggle commit on/off",
		"  D                 Toggle description/commit list",
		"  P                 Toggle conversation panel",
		"",
		"Commands",
		"  :q / :quit        Quit",
		"  :q!               Force quit",
		"  :w / :write       Save session",
		"  :x / :wq          Save and quit",
		"  :e / :reload      Reload diff",
		"  :clip / :export   Export comments to clipboard",
		"  :review           Open overall review",
		"  :bug              File a bug report",
		"  :submit           Submit review to remote",
		"  :refresh          Refresh from remote",
		"  :comment          Post conversation comment",
		"  :clear            Clear draft comments",
		"  :ready            Mark draft PR as ready",
		"  :edit-pr / :desc  Edit PR title/description",
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

	var allLines []string
	for _, line := range helpText {
		if line == "" {
			allLines = append(allLines, "")
		} else if !strings.HasPrefix(line, "  ") {
			allLines = append(allLines, headerStyle.Render(line))
		} else {
			parts := strings.SplitN(line, "  ", 3)
			if len(parts) >= 3 {
				key := dimStyle.Render(parts[1])
				desc := lineStyle.Render(parts[2])
				allLines = append(allLines, "  "+key+"  "+desc)
			} else {
				allLines = append(allLines, lineStyle.Render(line))
			}
		}
	}

	// Clamp scroll offset
	maxScroll := len(allLines) - height
	if maxScroll < 0 {
		maxScroll = 0
	}
	if a.helpScroll > maxScroll {
		a.helpScroll = maxScroll
	}
	if a.helpScroll < 0 {
		a.helpScroll = 0
	}

	// Slice visible lines
	end := a.helpScroll + height
	if end > len(allLines) {
		end = len(allLines)
	}
	lines := allLines[a.helpScroll:end]

	// Pad to height
	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// Utility functions

const tabWidth = 4

// expandTabs replaces tab characters with spaces aligned to tabWidth stops.
// findMatchRanges returns [start, end) byte ranges where pattern (already lowered)
// occurs in strings.ToLower(text).
func findMatchRanges(text, pattern string) [][2]int {
	if pattern == "" {
		return nil
	}
	lower := strings.ToLower(text)
	patLen := len(pattern)
	var ranges [][2]int
	for idx := 0; idx < len(lower); {
		pos := strings.Index(lower[idx:], pattern)
		if pos < 0 {
			break
		}
		ranges = append(ranges, [2]int{idx + pos, idx + pos + patLen})
		idx += pos + patLen
	}
	return ranges
}

func expandTabs(s string) string {
	if !strings.Contains(s, "\t") {
		return s
	}
	var b strings.Builder
	col := 0
	for _, r := range s {
		if r == '\t' {
			spaces := tabWidth - (col % tabWidth)
			b.WriteString(strings.Repeat(" ", spaces))
			col += spaces
		} else {
			b.WriteRune(r)
			col++
		}
	}
	return b.String()
}

// wrapLine splits a single line into multiple lines so that each fits within
// the given width. It breaks at word boundaries when possible, falling back to
// hard breaks for very long words.
func wrapLine(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	if lipgloss.Width(s) <= width {
		return []string{s}
	}

	var lines []string
	for len(s) > 0 {
		if lipgloss.Width(s) <= width {
			lines = append(lines, s)
			break
		}

		// Find the last space within width
		breakAt := -1
		for i, r := range s {
			if lipgloss.Width(s[:i+len(string(r))]) > width {
				break
			}
			if r == ' ' {
				breakAt = i
			}
		}

		if breakAt > 0 {
			lines = append(lines, s[:breakAt])
			s = s[breakAt+1:] // skip the space
		} else {
			// No space found; hard break at width
			cut := 0
			for i := range s {
				if lipgloss.Width(s[:i+1]) > width {
					break
				}
				cut = i + 1
			}
			if cut == 0 {
				cut = 1 // at least one character to avoid infinite loop
			}
			lines = append(lines, s[:cut])
			s = s[cut:]
		}
	}

	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

// wrapText wraps a multi-line string so each output line fits within width.
func wrapText(text string, width int) []string {
	var result []string
	for _, line := range strings.Split(text, "\n") {
		result = append(result, wrapLine(line, width)...)
	}
	return result
}

func truncateOrPad(s string, width int) string {
	if width <= 0 {
		return ""
	}
	w := lipgloss.Width(s)
	if w > width {
		return ansi.Truncate(s, width, "…")
	}
	if w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}
