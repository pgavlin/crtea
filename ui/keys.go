package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/pgavlin/crtea/bugreport"
	"github.com/pgavlin/crtea/model"
	"github.com/pgavlin/crtea/output"
)

func (a App) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Clear message on any keypress
	a.clearMessage()

	if a.phase == phasePicker {
		return a.handlePickerKey(msg)
	}

	switch a.inputMode {
	case modeHelp:
		return a.handleHelpKey(msg)
	case modeCommand:
		return a.handleCommandKey(msg)
	case modeSearch:
		return a.handleSearchKey(msg)
	case modeComment:
		return a.handleCommentKey(msg)
	case modeConfirm:
		return a.handleConfirmKey(msg)
	case modeVisualSelect:
		return a.handleVisualKey(msg)
	case modeReview:
		return a.handleReviewKey(msg)
	case modeBug:
		return a.handleBugKey(msg)
	default:
		return a.handleNormalKey(msg)
	}
}

func (a App) handleNormalKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.Key()

	// Handle pending key sequences
	if a.pendingPrefix != 0 {
		return a.handlePendingKey(msg)
	}

	// Digit accumulation for {N}G
	if key.Text >= "1" && key.Text <= "9" || (key.Text == "0" && a.pendingCount != "") {
		a.pendingCount += key.Text
		return &a, nil
	}

	switch {
	// Quit
	case key.Code == 'q' && key.Mod == 0:
		if a.dirty && !a.quitWarned {
			a.quitWarned = true
			a.setMessage("Unsaved changes. Press q again to quit, or :w to save.", messageWarning)
			return &a, nil
		}
		return &a, func() tea.Msg { return DoneMsg{Session: a.session} }

	case key.Code == tea.KeyEscape:
		a.pendingCount = ""
		a.pendingPrefix = 0
		return &a, nil

	// Navigation
	case key.Code == 'j' && key.Mod == 0, key.Code == tea.KeyDown:
		a.cursorDown(1)
	case key.Code == 'k' && key.Mod == 0, key.Code == tea.KeyUp:
		a.cursorUp(1)
	case key.Code == 'd' && key.Mod == tea.ModCtrl:
		a.cursorDown(a.diffViewportHeight() / 2)
	case key.Code == 'u' && key.Mod == tea.ModCtrl:
		a.cursorUp(a.diffViewportHeight() / 2)
	case key.Code == 'f' && key.Mod == tea.ModCtrl, key.Code == tea.KeyPgDown:
		a.cursorDown(a.diffViewportHeight())
	case key.Code == 'b' && key.Mod == tea.ModCtrl, key.Code == tea.KeyPgUp:
		a.cursorUp(a.diffViewportHeight())
	case key.Code == 'h' && key.Mod == 0, key.Code == tea.KeyLeft:
		if a.focusedPanel == panelDiff {
			if a.scrollX > 0 {
				a.scrollX--
			}
		}
	case key.Code == 'l' && key.Mod == 0, key.Code == tea.KeyRight:
		if a.focusedPanel == panelDiff {
			a.scrollX++
		}

	// Jump
	case key.Text == "G":
		if a.pendingCount != "" {
			n, _ := strconv.Atoi(a.pendingCount)
			a.pendingCount = ""
			a.jumpToSourceLine(n)
		} else {
			a.jumpToEnd()
		}
	case key.Code == 'g' && key.Mod == 0:
		a.pendingCount = ""
		a.jumpToStart()

	// File navigation
	case key.Text == "}":
		a.nextFile()
	case key.Text == "{":
		a.prevFile()
	case key.Text == "]":
		a.nextHunk()
	case key.Text == "[":
		a.prevHunk()

	// Tab to switch panels
	case key.Code == tea.KeyTab:
		hasCommitList := a.commitListHeight() > 0
		switch a.focusedPanel {
		case panelDiff:
			if a.showFileList {
				a.focusedPanel = panelFileList
			} else if hasCommitList {
				a.focusedPanel = panelCommitList
			}
		case panelFileList:
			if hasCommitList {
				a.focusedPanel = panelCommitList
			} else {
				a.focusedPanel = panelDiff
			}
		case panelCommitList:
			a.focusedPanel = panelDiff
		}

	// Enter: jump to file / toggle dir (file list) or toggle expand (diff)
	case key.Code == tea.KeyEnter:
		if a.focusedPanel == panelFileList {
			a.fileListEnter()
		} else if a.focusedPanel == panelDiff {
			a.toggleExpandAtCursor()
		}

	// Review
	case key.Code == 'r' && key.Mod == 0:
		a.toggleReviewed()

	// Comments
	case key.Code == 'c' && key.Mod == 0:
		a.enterLineComment()
	case key.Text == "C":
		a.enterFileComment()
	case key.Code == 'i' && key.Mod == 0:
		a.editCommentAtCursor()

	// Pending sequences
	case key.Code == 'd' && key.Mod == 0:
		a.pendingPrefix = 'd'
	case key.Code == 'z' && key.Mod == 0:
		a.pendingPrefix = 'z'
	case key.Code == ';' && key.Mod == 0:
		a.pendingPrefix = ';'

	// Toggle commit (in commit list)
	case key.Code == ' ':
		if a.focusedPanel == panelCommitList {
			if a.showDescription {
				// no toggle in description view
			} else {
				a.toggleCommitAtCursor()
			}
		}

	// Toggle between commit list and description
	case key.Text == "D":
		if a.commitListItems() != nil || a.showDescription || (a.session != nil && a.session.Description != "") {
			a.showDescription = !a.showDescription
			a.descScroll = 0
		}

	// Visual mode
	case key.Code == 'v' && key.Mod == 0:
		a.enterVisualMode()

	// Command/search
	case key.Text == ":":
		a.inputMode = modeCommand
		a.commandBuffer = ""
	case key.Text == "/":
		a.inputMode = modeSearch
		a.searchBuffer = ""
	case key.Code == 'n' && key.Mod == 0:
		a.searchNext(true)
	case key.Text == "N":
		a.searchNext(false)

	// Overall review
	case key.Text == "R":
		a.enterOverallReview()

	// Help
	case key.Text == "?":
		a.inputMode = modeHelp
	}

	return &a, nil
}

func (a App) handlePendingKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.Key()
	prefix := a.pendingPrefix
	a.pendingPrefix = 0

	switch prefix {
	case 'd':
		if key.Code == 'd' {
			a.deleteCommentAtCursor()
		}
	case 'z':
		if key.Code == 'z' {
			a.centerCursor()
		}
	case ';':
		switch key.Code {
		case 'e':
			a.showFileList = !a.showFileList
			if !a.showFileList && a.focusedPanel == panelFileList {
				a.focusedPanel = panelDiff
			}
		case 'h':
			a.focusedPanel = panelFileList
		case 'l':
			a.focusedPanel = panelDiff
		}
	}

	return &a, nil
}

func (a App) handleHelpKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.Key()
	switch {
	case key.Code == tea.KeyEscape, key.Code == 'q', key.Text == "?":
		a.inputMode = modeNormal
	}
	return &a, nil
}

func (a App) handleCommandKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.Key()
	switch {
	case key.Code == tea.KeyEscape:
		a.inputMode = modeNormal
		a.commandBuffer = ""
	case key.Code == tea.KeyEnter:
		cmd := a.executeCommand(a.commandBuffer)
		a.commandBuffer = ""
		if a.inputMode == modeCommand {
			a.inputMode = modeNormal
		}
		return &a, cmd
	case key.Code == tea.KeyBackspace:
		if len(a.commandBuffer) > 0 {
			a.commandBuffer = a.commandBuffer[:len(a.commandBuffer)-1]
		} else {
			a.inputMode = modeNormal
		}
	default:
		if key.Text != "" {
			a.commandBuffer += key.Text
		}
	}
	return &a, nil
}

func (a App) handleSearchKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.Key()
	switch {
	case key.Code == tea.KeyEscape:
		a.inputMode = modeNormal
		a.searchBuffer = ""
	case key.Code == tea.KeyEnter:
		a.lastSearch = a.searchBuffer
		a.inputMode = modeNormal
		a.searchNext(true)
	case key.Code == tea.KeyBackspace:
		if len(a.searchBuffer) > 0 {
			a.searchBuffer = a.searchBuffer[:len(a.searchBuffer)-1]
		} else {
			a.inputMode = modeNormal
		}
	default:
		if key.Text != "" {
			a.searchBuffer += key.Text
		}
	}
	return &a, nil
}

func (a App) handleCommentKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.Key()
	switch {
	case key.Code == tea.KeyEscape:
		a.inputMode = modeNormal
		a.commentBuffer = ""
		a.editingID = ""
		a.commentLineRange = nil
	case key.Code == tea.KeyEnter && key.Mod == tea.ModShift:
		// Shift-Enter: newline in comment
		a.commentBuffer = a.commentBuffer[:a.commentCursor] + "\n" + a.commentBuffer[a.commentCursor:]
		a.commentCursor++
	case key.Code == tea.KeyEnter:
		a.saveComment()
		a.inputMode = modeNormal
	case key.Code == tea.KeyTab:
		a.commentType = a.commentType.Next()
	case key.Code == tea.KeyBackspace:
		if a.commentCursor > 0 {
			a.commentBuffer = a.commentBuffer[:a.commentCursor-1] + a.commentBuffer[a.commentCursor:]
			a.commentCursor--
		}
	default:
		if key.Text != "" {
			a.commentBuffer = a.commentBuffer[:a.commentCursor] + key.Text + a.commentBuffer[a.commentCursor:]
			a.commentCursor += len(key.Text)
		}
	}
	return &a, nil
}

func (a App) handleConfirmKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.Key()
	switch {
	case key.Code == 'y', key.Code == 'Y':
		a.inputMode = modeNormal
		// Execute confirmed action
	case key.Code == tea.KeyEscape, key.Code == 'n', key.Code == 'N':
		a.inputMode = modeNormal
	}
	return &a, nil
}

func (a App) handleVisualKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.Key()
	switch {
	case key.Code == tea.KeyEscape, key.Code == 'v':
		a.inputMode = modeNormal
		a.commentLineRange = nil
	case key.Code == 'j', key.Code == tea.KeyDown:
		a.cursorDown(1)
	case key.Code == 'k', key.Code == tea.KeyUp:
		a.cursorUp(1)
	case key.Code == 'c':
		a.enterCommentFromVisual()
	}
	return &a, nil
}

// Navigation methods

func (a *App) cursorDown(n int) {
	if a.focusedPanel == panelFileList {
		a.fileListCursor += n
		if a.fileListCursor >= len(a.fileTreeRows) {
			a.fileListCursor = len(a.fileTreeRows) - 1
		}
		if a.fileListCursor < 0 {
			a.fileListCursor = 0
		}
		return
	}
	if a.focusedPanel == panelCommitList {
		if a.showDescription {
			a.descScroll += n
			maxScroll := a.descriptionLineCount() - (a.topPanelHeight() - 1)
			if maxScroll < 0 {
				maxScroll = 0
			}
			if a.descScroll > maxScroll {
				a.descScroll = maxScroll
			}
			if a.descScroll < 0 {
				a.descScroll = 0
			}
		} else {
			items := a.commitListItems()
			a.commitCursor += n
			if a.commitCursor >= len(items) {
				a.commitCursor = len(items) - 1
			}
			if a.commitCursor < 0 {
				a.commitCursor = 0
			}
		}
		return
	}

	a.cursorLine += n
	if a.cursorLine >= len(a.annotations) {
		a.cursorLine = len(a.annotations) - 1
	}
	if a.cursorLine < 0 {
		a.cursorLine = 0
	}
	a.ensureCursorVisible()
}

func (a *App) cursorUp(n int) {
	if a.focusedPanel == panelFileList {
		a.fileListCursor -= n
		if a.fileListCursor < 0 {
			a.fileListCursor = 0
		}
		return
	}
	if a.focusedPanel == panelCommitList {
		if a.showDescription {
			a.descScroll -= n
			if a.descScroll < 0 {
				a.descScroll = 0
			}
		} else {
			a.commitCursor -= n
			if a.commitCursor < 0 {
				a.commitCursor = 0
			}
		}
		return
	}

	a.cursorLine -= n
	if a.cursorLine < 0 {
		a.cursorLine = 0
	}
	a.ensureCursorVisible()
}

func (a *App) ensureCursorVisible() {
	vpHeight := a.diffViewportHeight()
	if a.cursorLine < a.scrollOffset {
		a.scrollOffset = a.cursorLine
	}
	if a.cursorLine >= a.scrollOffset+vpHeight {
		a.scrollOffset = a.cursorLine - vpHeight + 1
	}
}

func (a *App) centerCursor() {
	vpHeight := a.diffViewportHeight()
	a.scrollOffset = a.cursorLine - vpHeight/2
	if a.scrollOffset < 0 {
		a.scrollOffset = 0
	}
}

func (a *App) jumpToStart() {
	a.cursorLine = 0
	a.scrollOffset = 0
}

func (a *App) jumpToEnd() {
	if len(a.annotations) > 0 {
		a.cursorLine = len(a.annotations) - 1
		a.ensureCursorVisible()
	}
}

func (a *App) jumpToFile(fileIdx int) {
	if fileIdx < 0 || fileIdx >= len(a.diffFiles) {
		return
	}
	for i, ann := range a.annotations {
		if ann.Type == annFileHeader && ann.FileIdx == fileIdx {
			a.cursorLine = i
			a.ensureCursorVisible()
			return
		}
	}
}

func (a *App) jumpToSourceLine(lineNo int) {
	// Find the annotation matching this source line number
	for i, ann := range a.annotations {
		if ann.Type == annDiffLine && ann.NewLineNo == lineNo {
			a.cursorLine = i
			a.ensureCursorVisible()
			return
		}
	}
}

func (a *App) nextFile() {
	currentFile := a.currentFileIdx()
	for i := a.cursorLine + 1; i < len(a.annotations); i++ {
		if a.annotations[i].Type == annFileHeader && a.annotations[i].FileIdx > currentFile {
			a.cursorLine = i
			a.ensureCursorVisible()
			return
		}
	}
}

func (a *App) prevFile() {
	currentFile := a.currentFileIdx()
	for i := a.cursorLine - 1; i >= 0; i-- {
		if a.annotations[i].Type == annFileHeader && a.annotations[i].FileIdx < currentFile {
			a.cursorLine = i
			a.ensureCursorVisible()
			return
		}
	}
}

func (a *App) nextHunk() {
	for i := a.cursorLine + 1; i < len(a.annotations); i++ {
		if a.annotations[i].Type == annHunkHeader {
			a.cursorLine = i
			a.ensureCursorVisible()
			return
		}
	}
}

func (a *App) prevHunk() {
	for i := a.cursorLine - 1; i >= 0; i-- {
		if a.annotations[i].Type == annHunkHeader {
			a.cursorLine = i
			a.ensureCursorVisible()
			return
		}
	}
}

// File list actions

func (a *App) fileListEnter() {
	if a.fileListCursor < 0 || a.fileListCursor >= len(a.fileTreeRows) {
		return
	}
	row := a.fileTreeRows[a.fileListCursor]
	if row.IsDir {
		// Toggle collapse
		if a.collapsedDirs[row.Node.Path] {
			delete(a.collapsedDirs, row.Node.Path)
		} else {
			a.collapsedDirs[row.Node.Path] = true
		}
		a.rebuildFileTree()
		// Keep cursor in bounds
		if a.fileListCursor >= len(a.fileTreeRows) {
			a.fileListCursor = len(a.fileTreeRows) - 1
		}
	} else {
		// Jump to file in diff view
		a.jumpToFile(row.Node.FileIdx)
		a.focusedPanel = panelDiff
	}
}

// Context expansion

func (a *App) toggleExpandAtCursor() {
	if a.cursorLine < 0 || a.cursorLine >= len(a.annotations) {
		return
	}
	ann := a.annotations[a.cursorLine]

	switch ann.Type {
	case annExpander:
		a.expandGap(ann.gapID)
	case annExpandedContext:
		a.collapseGap(ann.gapID)
	}
}

func (a *App) expandGap(gid gapID) {
	if _, ok := a.expandedGaps[gid]; ok {
		return // already expanded
	}

	file := a.diffFiles[gid.FileIdx]
	if gid.HunkIdx <= 0 || gid.HunkIdx >= len(file.Hunks) {
		return
	}

	prevHunk := file.Hunks[gid.HunkIdx-1]
	thisHunk := file.Hunks[gid.HunkIdx]

	// Calculate the line range between the two hunks
	startLine := prevHunk.NewStart + prevHunk.NewCount
	endLine := thisHunk.NewStart - 1
	if startLine > endLine {
		return
	}

	lines, err := a.vcs.FetchContextLines(file.DisplayPath(), file.Status, startLine, endLine)
	if err != nil {
		a.setMessage("Failed to expand context: "+err.Error(), messageError)
		return
	}

	a.expandedGaps[gid] = lines
	a.rebuildAnnotations()
}

func (a *App) collapseGap(gid gapID) {
	if _, ok := a.expandedGaps[gid]; !ok {
		return
	}
	delete(a.expandedGaps, gid)
	a.rebuildAnnotations()
}

// Review actions

func (a *App) toggleReviewed() {
	path := a.currentFilePath()
	if path == "" {
		return
	}
	fileIdx := a.currentFileIdx()
	if fileIdx < 0 || fileIdx >= len(a.diffFiles) {
		return
	}
	fr := a.session.GetOrCreateFileReview(path, a.diffFiles[fileIdx].Status)
	fr.Reviewed = !fr.Reviewed
	a.dirty = true
	if fr.Reviewed {
		a.setMessage(fmt.Sprintf("Marked %s as reviewed", path), messageInfo)
	} else {
		a.setMessage(fmt.Sprintf("Unmarked %s as reviewed", path), messageInfo)
	}
}

// Comment actions

func (a *App) enterLineComment() {
	if a.cursorLine < 0 || a.cursorLine >= len(a.annotations) {
		return
	}
	ann := a.annotations[a.cursorLine]
	if ann.Type != annDiffLine {
		a.setMessage("Move cursor to a diff line to comment", messageWarning)
		return
	}

	lineNo := ann.NewLineNo
	side := model.SideNew
	if ann.OldLineNo > 0 && ann.NewLineNo == 0 {
		lineNo = ann.OldLineNo
		side = model.SideOld
	}

	a.commentBuffer = ""
	a.commentCursor = 0
	a.commentType = model.CommentNote
	a.commentIsFile = false
	a.commentLine = lineNo
	a.commentSide = side
	a.editingID = ""
	a.commentLineRange = nil
	a.inputMode = modeComment
}

func (a *App) enterFileComment() {
	if a.currentFilePath() == "" {
		return
	}
	a.commentBuffer = ""
	a.commentCursor = 0
	a.commentType = model.CommentNote
	a.commentIsFile = true
	a.commentLine = 0
	a.editingID = ""
	a.commentLineRange = nil
	a.inputMode = modeComment
}

func (a *App) editCommentAtCursor() {
	if a.cursorLine < 0 || a.cursorLine >= len(a.annotations) {
		return
	}
	ann := a.annotations[a.cursorLine]
	path := a.currentFilePath()
	if path == "" {
		return
	}
	fr := a.session.GetFileReview(path)
	if fr == nil {
		return
	}

	switch ann.Type {
	case annFileComment:
		if ann.CommentIdx < len(fr.FileComments) {
			c := fr.FileComments[ann.CommentIdx]
			a.commentBuffer = c.Content
			a.commentCursor = len(c.Content)
			a.commentType = c.Type
			a.commentIsFile = true
			a.editingID = c.ID
			a.inputMode = modeComment
		}
	case annLineComment:
		lineNo := a.getCommentLineNo(ann)
		if comments, ok := fr.LineComments[lineNo]; ok && ann.CommentIdx < len(comments) {
			c := comments[ann.CommentIdx]
			a.commentBuffer = c.Content
			a.commentCursor = len(c.Content)
			a.commentType = c.Type
			a.commentIsFile = false
			a.commentLine = lineNo
			a.commentSide = c.Side
			a.editingID = c.ID
			a.inputMode = modeComment
		}
	default:
		a.setMessage("Move cursor to a comment to edit", messageWarning)
	}
}

func (a *App) getCommentLineNo(ann annotatedLine) int {
	if ann.Side == model.SideOld && ann.OldLineNo > 0 {
		return ann.OldLineNo
	}
	if ann.NewLineNo > 0 {
		return ann.NewLineNo
	}
	return ann.OldLineNo
}

func (a *App) saveComment() {
	if strings.TrimSpace(a.commentBuffer) == "" {
		a.commentBuffer = ""
		a.editingID = ""
		return
	}

	path := a.currentFilePath()
	if path == "" {
		return
	}
	fileIdx := a.currentFileIdx()
	if fileIdx < 0 {
		return
	}

	fr := a.session.GetOrCreateFileReview(path, a.diffFiles[fileIdx].Status)

	if a.editingID != "" {
		// Edit existing comment
		if a.commentIsFile {
			for i, c := range fr.FileComments {
				if c.ID == a.editingID {
					fr.FileComments[i].Content = a.commentBuffer
					fr.FileComments[i].Type = a.commentType
					break
				}
			}
		} else {
			if comments, ok := fr.LineComments[a.commentLine]; ok {
				for i, c := range comments {
					if c.ID == a.editingID {
						comments[i].Content = a.commentBuffer
						comments[i].Type = a.commentType
						break
					}
				}
				fr.LineComments[a.commentLine] = comments
			}
		}
	} else {
		// New comment
		var comment model.Comment
		if a.commentLineRange != nil {
			comment = model.NewRangeComment(a.commentBuffer, a.commentType, a.commentSide, *a.commentLineRange)
		} else {
			comment = model.NewComment(a.commentBuffer, a.commentType, a.commentSide)
		}

		if a.commentIsFile {
			fr.AddFileComment(comment)
		} else {
			fr.AddLineComment(a.commentLine, comment)
		}
	}

	a.dirty = true
	a.commentBuffer = ""
	a.editingID = ""
	a.commentLineRange = nil
	a.rebuildAnnotations()
}

func (a *App) deleteCommentAtCursor() {
	if a.cursorLine < 0 || a.cursorLine >= len(a.annotations) {
		return
	}
	ann := a.annotations[a.cursorLine]
	path := a.currentFilePath()
	if path == "" {
		return
	}
	fr := a.session.GetFileReview(path)
	if fr == nil {
		return
	}

	switch ann.Type {
	case annFileComment:
		if ann.CommentIdx < len(fr.FileComments) {
			fr.FileComments = append(fr.FileComments[:ann.CommentIdx], fr.FileComments[ann.CommentIdx+1:]...)
			a.dirty = true
			a.rebuildAnnotations()
			a.setMessage("Comment deleted", messageInfo)
		}
	case annLineComment:
		lineNo := a.getCommentLineNo(ann)
		if comments, ok := fr.LineComments[lineNo]; ok && ann.CommentIdx < len(comments) {
			fr.LineComments[lineNo] = append(comments[:ann.CommentIdx], comments[ann.CommentIdx+1:]...)
			if len(fr.LineComments[lineNo]) == 0 {
				delete(fr.LineComments, lineNo)
			}
			a.dirty = true
			a.rebuildAnnotations()
			a.setMessage("Comment deleted", messageInfo)
		}
	default:
		a.setMessage("Move cursor to a comment to delete", messageWarning)
	}
}

// Visual mode

func (a *App) enterVisualMode() {
	if a.cursorLine < 0 || a.cursorLine >= len(a.annotations) {
		return
	}
	ann := a.annotations[a.cursorLine]
	if ann.Type != annDiffLine {
		a.setMessage("Move cursor to a diff line for visual selection", messageWarning)
		return
	}
	lineNo := ann.NewLineNo
	side := model.SideNew
	if ann.NewLineNo == 0 {
		lineNo = ann.OldLineNo
		side = model.SideOld
	}
	a.visualAnchor = lineNo
	a.visualAnchorSide = side
	a.inputMode = modeVisualSelect
}

func (a *App) enterCommentFromVisual() {
	if a.cursorLine < 0 || a.cursorLine >= len(a.annotations) {
		return
	}
	ann := a.annotations[a.cursorLine]
	if ann.Type != annDiffLine {
		return
	}

	cursorLineNo := ann.NewLineNo
	if cursorLineNo == 0 {
		cursorLineNo = ann.OldLineNo
	}

	start := a.visualAnchor
	end := cursorLineNo
	if start > end {
		start, end = end, start
	}

	a.commentLineRange = &model.LineRange{Start: start, End: end}
	a.commentBuffer = ""
	a.commentCursor = 0
	a.commentType = model.CommentNote
	a.commentIsFile = false
	a.commentLine = start
	a.commentSide = a.visualAnchorSide
	a.editingID = ""
	a.inputMode = modeComment
}

// Overall review

func (a *App) enterOverallReview() {
	a.reviewBuffer = ""
	a.reviewCursor = 0
	a.reviewStatus = model.ApprovalNeutral
	if a.session.OverallReview != nil {
		a.reviewBuffer = a.session.OverallReview.Body
		a.reviewCursor = len(a.reviewBuffer)
		a.reviewStatus = a.session.OverallReview.Status
	}
	a.inputMode = modeReview
}

func (a App) handleReviewKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.Key()
	switch {
	case key.Code == tea.KeyEscape:
		a.inputMode = modeNormal
		a.reviewBuffer = ""
	case key.Code == tea.KeyEnter && key.Mod == tea.ModShift:
		a.reviewBuffer = a.reviewBuffer[:a.reviewCursor] + "\n" + a.reviewBuffer[a.reviewCursor:]
		a.reviewCursor++
	case key.Code == tea.KeyEnter:
		a.saveOverallReview()
		a.inputMode = modeNormal
	case key.Code == tea.KeyTab:
		a.reviewStatus = a.reviewStatus.Next()
	case key.Code == tea.KeyBackspace:
		if a.reviewCursor > 0 {
			a.reviewBuffer = a.reviewBuffer[:a.reviewCursor-1] + a.reviewBuffer[a.reviewCursor:]
			a.reviewCursor--
		}
	default:
		if key.Text != "" {
			a.reviewBuffer = a.reviewBuffer[:a.reviewCursor] + key.Text + a.reviewBuffer[a.reviewCursor:]
			a.reviewCursor += len(key.Text)
		}
	}
	return &a, nil
}

func (a *App) saveOverallReview() {
	body := strings.TrimSpace(a.reviewBuffer)
	if body == "" && a.reviewStatus == model.ApprovalNeutral {
		a.session.OverallReview = nil
	} else {
		a.session.OverallReview = &model.OverallReview{
			Body:   body,
			Status: a.reviewStatus,
		}
	}
	a.dirty = true
	a.reviewBuffer = ""
	a.setMessage("Overall review saved", messageInfo)
}

// Bug report

func (a *App) enterBugReport() {
	a.bugBuffer = ""
	a.bugCursor = 0
	a.inputMode = modeBug
}

func (a App) handleBugKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.Key()
	switch {
	case key.Code == tea.KeyEscape:
		a.inputMode = modeNormal
		a.bugBuffer = ""
	case key.Code == tea.KeyEnter && key.Mod == tea.ModShift:
		a.bugBuffer = a.bugBuffer[:a.bugCursor] + "\n" + a.bugBuffer[a.bugCursor:]
		a.bugCursor++
	case key.Code == tea.KeyEnter:
		return a.submitBugReport()
	case key.Code == tea.KeyBackspace:
		if a.bugCursor > 0 {
			a.bugBuffer = a.bugBuffer[:a.bugCursor-1] + a.bugBuffer[a.bugCursor:]
			a.bugCursor--
		}
	default:
		if key.Text != "" {
			a.bugBuffer = a.bugBuffer[:a.bugCursor] + key.Text + a.bugBuffer[a.bugCursor:]
			a.bugCursor += len(key.Text)
		}
	}
	return &a, nil
}

func (a App) submitBugReport() (tea.Model, tea.Cmd) {
	body := strings.TrimSpace(a.bugBuffer)
	if body == "" {
		a.inputMode = modeNormal
		a.bugBuffer = ""
		return &a, nil
	}

	screenContent := a.captureScreen()
	env := bugreport.CollectEnvironment(a.vcsInfo, a.width, a.height)

	report := bugreport.Report{
		Body:        body,
		Environment: env,
		CreatedAt:   time.Now().UTC(),
	}

	path, err := bugreport.Write(report, a.session, screenContent)
	if err != nil {
		a.setMessage("Bug report failed: "+err.Error(), messageError)
		a.inputMode = modeNormal
		a.bugBuffer = ""
		return &a, nil
	}

	a.inputMode = modeNormal
	a.bugBuffer = ""
	a.setMessage(fmt.Sprintf("Bug report saved to %s", path), messageInfo)
	return &a, func() tea.Msg { return ClipboardMsg{Content: path} }
}

// Search

func (a *App) searchNext(forward bool) {
	if a.lastSearch == "" {
		return
	}
	pattern := strings.ToLower(a.lastSearch)

	start := a.cursorLine + 1
	if !forward {
		start = a.cursorLine - 1
	}

	search := func(idx int) bool {
		if idx < 0 || idx >= len(a.annotations) {
			return false
		}
		ann := a.annotations[idx]
		switch ann.Type {
		case annFileHeader:
			if ann.FileIdx >= 0 && ann.FileIdx < len(a.diffFiles) {
				return strings.Contains(strings.ToLower(a.diffFiles[ann.FileIdx].DisplayPath()), pattern)
			}
		case annDiffLine:
			if ann.FileIdx >= 0 && ann.FileIdx < len(a.diffFiles) {
				file := a.diffFiles[ann.FileIdx]
				if ann.HunkIdx >= 0 && ann.HunkIdx < len(file.Hunks) {
					hunk := file.Hunks[ann.HunkIdx]
					if ann.LineIdx >= 0 && ann.LineIdx < len(hunk.Lines) {
						return strings.Contains(strings.ToLower(hunk.Lines[ann.LineIdx].Content), pattern)
					}
				}
			}
		}
		return false
	}

	if forward {
		for i := start; i < len(a.annotations); i++ {
			if search(i) {
				a.cursorLine = i
				a.ensureCursorVisible()
				return
			}
		}
		// Wrap around
		for i := 0; i < start && i < len(a.annotations); i++ {
			if search(i) {
				a.cursorLine = i
				a.ensureCursorVisible()
				a.setMessage("Search wrapped to top", messageInfo)
				return
			}
		}
	} else {
		for i := start; i >= 0; i-- {
			if search(i) {
				a.cursorLine = i
				a.ensureCursorVisible()
				return
			}
		}
		for i := len(a.annotations) - 1; i > start; i-- {
			if search(i) {
				a.cursorLine = i
				a.ensureCursorVisible()
				a.setMessage("Search wrapped to bottom", messageInfo)
				return
			}
		}
	}

	a.setMessage("Pattern not found: "+a.lastSearch, messageWarning)
}

// Command execution

func (a *App) executeCommand(cmd string) tea.Cmd {
	cmd = strings.TrimSpace(cmd)
	switch cmd {
	case "q", "quit":
		if a.dirty && !a.quitWarned {
			a.quitWarned = true
			a.setMessage("Unsaved changes. Use :q! to force quit, or :w to save.", messageWarning)
			return nil
		}
		return func() tea.Msg { return DoneMsg{Session: a.session} }
	case "q!", "quit!":
		return func() tea.Msg { return DoneMsg{Session: a.session} }
	case "w", "write":
		if _, err := a.store.Save(a.session); err != nil {
			a.setMessage("Save failed: "+err.Error(), messageError)
		} else {
			a.setMessage("Session saved", messageInfo)
			a.dirty = false
		}
		return nil
	case "x", "wq":
		a.store.Save(a.session)
		a.dirty = false
		return func() tea.Msg { return DoneMsg{Session: a.session} }
	case "e", "reload":
		files, err := a.vcs.GetWorkingTreeDiff()
		if err != nil {
			a.setMessage("Reload failed: "+err.Error(), messageError)
			return nil
		}
		if a.highlighter != nil {
			a.highlighter.HighlightFiles(files)
		}
		a.diffFiles = files
		a.rebuildFileTree()
		a.rebuildAnnotations()
		a.setMessage(fmt.Sprintf("Reloaded %d files", len(files)), messageInfo)
		return nil
	case "review":
		a.enterOverallReview()
		return nil
	case "bug":
		a.enterBugReport()
		return nil
	case "clip", "export":
		return a.exportComments()
	case "clear":
		for _, fr := range a.session.Files {
			fr.FileComments = nil
			fr.LineComments = make(map[int][]model.Comment)
		}
		a.session.OverallReview = nil
		a.dirty = true
		a.rebuildAnnotations()
		a.setMessage("All comments cleared", messageInfo)
		return nil
	}

	if strings.HasPrefix(cmd, "set ") {
		opt := strings.TrimPrefix(cmd, "set ")
		switch opt {
		case "wrap":
			a.setMessage("Line wrapping enabled", messageInfo)
		case "nowrap":
			a.setMessage("Line wrapping disabled", messageInfo)
		default:
			a.setMessage("Unknown option: "+opt, messageWarning)
		}
		return nil
	}

	a.setMessage("Unknown command: "+cmd, messageWarning)
	return nil
}

func (a *App) exportComments() tea.Cmd {
	total := a.session.TotalComments()
	if total == 0 && a.session.OverallReview == nil {
		a.setMessage("No comments to export", messageWarning)
		return nil
	}
	md := output.GenerateMarkdown(a.session)
	a.setMessage(fmt.Sprintf("Exported %d comments to clipboard", total), messageInfo)
	return func() tea.Msg { return ClipboardMsg{Content: md} }
}

// joinHorizontal joins two blocks of text side by side.
func joinHorizontal(left, right string, height int) string {
	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")

	var b strings.Builder
	for i := 0; i < height; i++ {
		if i > 0 {
			b.WriteString("\n")
		}
		l := ""
		if i < len(leftLines) {
			l = leftLines[i]
		}
		r := ""
		if i < len(rightLines) {
			r = rightLines[i]
		}
		b.WriteString(l)
		b.WriteString("│")
		b.WriteString(r)
	}
	return b.String()
}
