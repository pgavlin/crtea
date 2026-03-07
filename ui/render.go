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
		content = " Enter: save | Esc: cancel | Tab: cycle type | Shift-Enter: newline"
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

	for i, row := range a.fileTreeRows {
		node := row.Node
		isSelected := i == a.fileListCursor && a.focusedPanel == PanelFileList

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

			statusColor := th.FileModified
			switch file.Status {
			case model.FileAdded:
				statusColor = th.FileAdded
			case model.FileDeleted:
				statusColor = th.FileDeleted
			case model.FileRenamed:
				statusColor = th.FileRenamed
			}

			statusStyle := bg(lipgloss.NewStyle().Foreground(statusColor))

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
		isCursor := (i == a.cursorLine && a.focusedPanel == PanelDiff)
		isVisualSelected := a.isVisualSelected(i)

		// When editing an existing comment, replace its annotation lines with the editor
		if a.inputMode == ModeComment && a.editingID != "" && a.isEditingAnnotation(ann) {
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
		if i == a.cursorLine && a.inputMode == ModeComment && a.editingID == "" {
			for _, el := range a.renderCommentEditor(width) {
				if len(lines) >= vpHeight {
					break
				}
				lines = append(lines, el)
			}
		}
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
		return style.Render(truncateOrPad(" ⋯ expand context (Enter to expand)", width))

	case AnnExpandedContext:
		return a.renderExpandedContextLine(ann, width, isCursor)

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

	if line.Spans != nil && a.scrollX == 0 {
		// Render with syntax highlighting
		var rendered strings.Builder
		col := 0
		for _, span := range line.Spans {
			if col >= contentWidth {
				break
			}
			text := span.Text
			remaining := contentWidth - col
			if len(text) > remaining {
				text = text[:remaining]
			}
			spanStyle := lipgloss.NewStyle()
			if span.FG != "" {
				spanStyle = spanStyle.Foreground(lipgloss.Color(span.FG))
			} else {
				spanStyle = spanStyle.Foreground(contentStyle.GetForeground())
			}
			if bgColor != nil {
				spanStyle = spanStyle.Background(bgColor)
			}
			rendered.WriteString(spanStyle.Render(text))
			col += len(text)
		}
		// Pad remaining width
		if col < contentWidth {
			padStyle := lipgloss.NewStyle()
			if bgColor != nil {
				padStyle = padStyle.Background(bgColor)
			}
			rendered.WriteString(padStyle.Render(strings.Repeat(" ", contentWidth-col)))
		}
		return gutter + markerStyle.Render(marker) + rendered.String()
	}

	// Fallback: no syntax highlighting or horizontal scroll active
	content := line.Content
	if a.scrollX > 0 && len(content) > a.scrollX {
		content = content[a.scrollX:]
	} else if a.scrollX > 0 {
		content = ""
	}
	renderedContent := contentStyle.Render(truncateOrPad(content, contentWidth))
	return gutter + markerStyle.Render(marker) + renderedContent
}

func (a *App) renderExpandedContextLine(ann AnnotatedLine, width int, isCursor bool) string {
	th := a.theme

	expanded, ok := a.expandedGaps[ann.GapID]
	if !ok || ann.LineIdx < 0 || ann.LineIdx >= len(expanded) {
		return strings.Repeat(" ", width)
	}
	line := expanded[ann.LineIdx]

	oldNo := fmt.Sprintf("%4d", line.OldLineNo)
	newNo := fmt.Sprintf("%4d", line.NewLineNo)

	gutterStyle := lipgloss.NewStyle().Foreground(th.FgDim)
	gutter := gutterStyle.Render(oldNo + " " + newNo)

	contentStyle := lipgloss.NewStyle().Foreground(th.ExpandedCtxFg)
	if isCursor {
		contentStyle = contentStyle.Background(th.BgHighlight)
	}

	content := line.Content
	if a.scrollX > 0 && len(content) > a.scrollX {
		content = content[a.scrollX:]
	} else if a.scrollX > 0 {
		content = ""
	}

	gutterWidth := 10
	contentWidth := width - gutterWidth - 1
	return gutter + contentStyle.Render(" "+truncateOrPad(content, contentWidth))
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

	if isFirst {
		// Top border + type badge
		typeBadge := lipgloss.NewStyle().
			Background(typeColor).
			Foreground(th.ModeFg).
			Bold(true).
			Padding(0, 1).
			Render(comment.Type.String())

		badgeWidth := lipgloss.Width(typeBadge)
		restWidth := boxWidth - 2 - badgeWidth // "╭" + badge + "───╮"
		if restWidth < 1 {
			restWidth = 1
		}
		line := gutter + borderStyle.Render("╭") + typeBadge + borderStyle.Render(strings.Repeat("─", restWidth)+"╮")
		return truncateOrPad(line, width)
	}

	if isLast && ann.CommentLine > 0 {
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
func (a *App) isEditingAnnotation(ann AnnotatedLine) bool {
	if a.editingID == "" {
		return false
	}

	isCommentAnn := ann.Type == AnnFileComment || ann.Type == AnnLineComment
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

	if ann.Type == AnnFileComment {
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
