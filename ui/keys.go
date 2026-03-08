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
	"github.com/pgavlin/crtea/provider"
	"github.com/pgavlin/crtea/vcs"
)

// handleTextInput handles common text editing keys (backspace, shift-enter newline,
// character insertion) for any buffer/cursor pair. Returns true if the key was consumed.
func handleTextInput(key tea.Key, buffer *string, cursor *int) bool {
	switch {
	case key.Code == tea.KeyEnter && key.Mod == tea.ModShift:
		*buffer = (*buffer)[:*cursor] + "\n" + (*buffer)[*cursor:]
		*cursor++
		return true
	case key.Code == tea.KeyBackspace:
		if *cursor > 0 {
			*buffer = (*buffer)[:*cursor-1] + (*buffer)[*cursor:]
			*cursor--
		}
		return true
	default:
		if key.Text != "" {
			*buffer = (*buffer)[:*cursor] + key.Text + (*buffer)[*cursor:]
			*cursor += len(key.Text)
			return true
		}
	}
	return false
}

func (a App) handleKey(msg tea.KeyPressMsg) (App, tea.Cmd) {
	// Clear non-error messages on any keypress; errors persist until Escape
	if a.message != nil && a.message.level != messageError {
		a.clearMessage()
	}

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
	case modeConversation:
		return a.handleConversationKey(msg)
	case modeEditPR:
		return a.handleEditPRKey(msg)
	default:
		return a.handleNormalKey(msg)
	}
}

func (a App) handleNormalKey(msg tea.KeyPressMsg) (App, tea.Cmd) {
	key := msg.Key()

	// Handle pending key sequences
	if a.pendingPrefix != 0 {
		return a.handlePendingKey(msg)
	}

	// Digit accumulation for {N}G
	if key.Text >= "1" && key.Text <= "9" || (key.Text == "0" && a.pendingCount != "") {
		a.pendingCount += key.Text
		return a, nil
	}

	// 0 (when not accumulating digits) resets horizontal scroll
	if key.Text == "0" && a.pendingCount == "" {
		a.scrollX = 0
		return a, nil
	}

	switch {
	// Quit
	case key.Code == 'q' && key.Mod == 0:
		if a.dirty && !a.quitWarned {
			a.quitWarned = true
			a.setMessage("Unsaved changes. Press q again to quit, or :w to save.", messageWarning)
			return a, nil
		}
		return a, func() tea.Msg { return DoneMsg{Session: a.session} }

	case key.Code == tea.KeyEscape:
		a.pendingCount = ""
		a.pendingPrefix = 0
		a.searchHighlight = ""
		a.clearMessage()
		return a, nil

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
	case key.Code == tea.KeyHome:
		a.scrollX = 0

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
		hasConversation := a.showConversation
		switch a.focusedPanel {
		case panelDiff:
			if a.showFileList {
				a.focusedPanel = panelFileList
			} else if hasCommitList {
				a.focusedPanel = panelCommitList
			} else if hasConversation {
				a.focusedPanel = panelConversation
			}
		case panelFileList:
			if hasCommitList {
				a.focusedPanel = panelCommitList
			} else if hasConversation {
				a.focusedPanel = panelConversation
			} else {
				a.focusedPanel = panelDiff
			}
		case panelCommitList:
			if hasConversation {
				a.focusedPanel = panelConversation
			} else {
				a.focusedPanel = panelDiff
			}
		case panelConversation:
			a.focusedPanel = panelDiff
		}

	// Enter: jump to file / toggle dir (file list), compose message (conversation), or toggle expand (diff)
	case key.Code == tea.KeyEnter:
		if a.focusedPanel == panelFileList {
			a.fileListEnter()
		} else if a.focusedPanel == panelConversation {
			cmd := a.postConversationComment()
			return a, cmd
		} else if a.focusedPanel == panelDiff {
			a.toggleExpandAtCursor()
		}

	// Review
	case key.Code == 'r' && key.Mod == 0:
		a.toggleReviewed()

	// Comments
	case key.Code == 'c' && key.Mod == 0:
		if a.focusedPanel == panelConversation {
			cmd := a.postConversationComment()
			return a, cmd
		}
		a.enterLineComment()
	case key.Text == "C":
		a.enterFileComment()
	case key.Code == 'i' && key.Mod == 0:
		a.editCommentAtCursor()
	case key.Code == 'a' && key.Mod == 0:
		a.replyToCommentAtCursor()

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
			a.showConversation = false
			a.descScroll = 0
		}
	case key.Text == "P":
		if a.provider == nil {
			a.setMessage("Conversation requires a remote provider", messageWarning)
		} else if a.session != nil {
			a.showConversation = !a.showConversation
			a.convScroll = 0
			if a.showConversation {
				a.focusedPanel = panelConversation
			} else if a.focusedPanel == panelConversation {
				a.focusedPanel = panelDiff
			}
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
		a.helpScroll = 0
	}

	return a, nil
}

func (a App) handlePendingKey(msg tea.KeyPressMsg) (App, tea.Cmd) {
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
		case 'n':
			a.nextUnreviewedFile()
		case 'p':
			a.prevUnreviewedFile()
		case 'r':
			return a, a.toggleResolveThread()
		}
	}

	return a, nil
}

func (a App) handleHelpKey(msg tea.KeyPressMsg) (App, tea.Cmd) {
	key := msg.Key()
	switch {
	case key.Code == tea.KeyEscape, key.Code == 'q', key.Text == "?":
		a.inputMode = modeNormal
		a.helpScroll = 0
	case key.Code == 'j' && key.Mod == 0, key.Code == tea.KeyDown:
		a.helpScroll++
	case key.Code == 'k' && key.Mod == 0, key.Code == tea.KeyUp:
		a.helpScroll--
		if a.helpScroll < 0 {
			a.helpScroll = 0
		}
	case key.Code == 'd' && key.Mod == tea.ModCtrl:
		a.helpScroll += a.diffViewportHeight() / 2
	case key.Code == 'u' && key.Mod == tea.ModCtrl:
		a.helpScroll -= a.diffViewportHeight() / 2
		if a.helpScroll < 0 {
			a.helpScroll = 0
		}
	case key.Code == 'g' && key.Mod == 0:
		a.helpScroll = 0
	case key.Text == "G":
		a.helpScroll = 9999 // will be clamped in render
	}
	return a, nil
}

func (a App) handleCommandKey(msg tea.KeyPressMsg) (App, tea.Cmd) {
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
		return a, cmd
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
	return a, nil
}

func (a App) handleSearchKey(msg tea.KeyPressMsg) (App, tea.Cmd) {
	key := msg.Key()
	switch {
	case key.Code == tea.KeyEscape:
		a.inputMode = modeNormal
		a.searchBuffer = ""
	case key.Code == tea.KeyEnter:
		a.lastSearch = a.searchBuffer
		a.searchHighlight = a.searchBuffer
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
	return a, nil
}

func (a App) handleCommentKey(msg tea.KeyPressMsg) (App, tea.Cmd) {
	key := msg.Key()
	switch {
	case key.Code == tea.KeyEscape:
		a.inputMode = modeNormal
		a.commentBuffer = ""
		a.editingID = ""
		a.commentLineRange = nil
	case key.Code == tea.KeyEnter && key.Mod == 0:
		a.saveComment()
		a.inputMode = modeNormal
	case key.Code == tea.KeyTab:
		a.commentType = a.commentType.Next()
	default:
		handleTextInput(key, &a.commentBuffer, &a.commentCursor)
	}
	return a, nil
}

func (a App) handleConfirmKey(msg tea.KeyPressMsg) (App, tea.Cmd) {
	key := msg.Key()
	switch {
	case key.Code == 'y', key.Code == 'Y':
		a.inputMode = modeNormal
		if a.confirmCallback != nil {
			cb := a.confirmCallback
			a.confirmCallback = nil
			a.confirmPrompt = ""
			return a, cb(&a)
		}
	case key.Code == tea.KeyEscape, key.Code == 'n', key.Code == 'N':
		a.inputMode = modeNormal
		a.confirmCallback = nil
		a.confirmPrompt = ""
	}
	return a, nil
}

func (a App) handleVisualKey(msg tea.KeyPressMsg) (App, tea.Cmd) {
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
	return a, nil
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
		a.ensureFileListCursorVisible()
		return
	}
	if a.focusedPanel == panelConversation {
		a.convScroll += n
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
		a.ensureFileListCursorVisible()
		return
	}
	if a.focusedPanel == panelConversation {
		a.convScroll -= n
		if a.convScroll < 0 {
			a.convScroll = 0
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

func (a *App) fileListVisibleRows(height int) int {
	// header takes 1 line
	vis := height - 1
	if vis < 1 {
		vis = 1
	}
	return vis
}

func (a *App) ensureFileListCursorVisible() {
	vis := a.fileListVisibleRows(a.diffViewportHeight())
	if a.fileListCursor < a.fileListScroll {
		a.fileListScroll = a.fileListCursor
	}
	if a.fileListCursor >= a.fileListScroll+vis {
		a.fileListScroll = a.fileListCursor - vis + 1
	}
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
	if a.focusedPanel == panelFileList {
		a.fileListCursor = 0
		a.fileListScroll = 0
		return
	}
	a.cursorLine = 0
	a.scrollOffset = 0
}

func (a *App) jumpToEnd() {
	if a.focusedPanel == panelFileList {
		if len(a.fileTreeRows) > 0 {
			a.fileListCursor = len(a.fileTreeRows) - 1
			a.ensureFileListCursorVisible()
		}
		return
	}
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

func (a *App) nextUnreviewedFile() {
	currentFile := a.currentFileIdx()
	for i := a.cursorLine + 1; i < len(a.annotations); i++ {
		ann := a.annotations[i]
		if ann.Type == annFileHeader && ann.FileIdx > currentFile {
			path := a.diffFiles[ann.FileIdx].DisplayPath()
			fr := a.session.GetFileReview(path)
			if fr == nil || !fr.Reviewed {
				a.cursorLine = i
				a.ensureCursorVisible()
				return
			}
		}
	}
	// Wrap around from start
	for i := 0; i <= a.cursorLine && i < len(a.annotations); i++ {
		ann := a.annotations[i]
		if ann.Type == annFileHeader {
			path := a.diffFiles[ann.FileIdx].DisplayPath()
			fr := a.session.GetFileReview(path)
			if fr == nil || !fr.Reviewed {
				a.cursorLine = i
				a.ensureCursorVisible()
				a.setMessage("Search wrapped to top", messageInfo)
				return
			}
		}
	}
	a.setMessage("All files reviewed", messageInfo)
}

func (a *App) prevUnreviewedFile() {
	currentFile := a.currentFileIdx()
	for i := a.cursorLine - 1; i >= 0; i-- {
		ann := a.annotations[i]
		if ann.Type == annFileHeader && ann.FileIdx < currentFile {
			path := a.diffFiles[ann.FileIdx].DisplayPath()
			fr := a.session.GetFileReview(path)
			if fr == nil || !fr.Reviewed {
				a.cursorLine = i
				a.ensureCursorVisible()
				return
			}
		}
	}
	// Wrap around from bottom
	for i := len(a.annotations) - 1; i > a.cursorLine; i-- {
		ann := a.annotations[i]
		if ann.Type == annFileHeader {
			path := a.diffFiles[ann.FileIdx].DisplayPath()
			fr := a.session.GetFileReview(path)
			if fr == nil || !fr.Reviewed {
				a.cursorLine = i
				a.ensureCursorVisible()
				a.setMessage("Search wrapped to bottom", messageInfo)
				return
			}
		}
	}
	a.setMessage("All files reviewed", messageInfo)
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

	if a.vcs == nil {
		a.setMessage("Context expansion requires a VCS backend", messageWarning)
		return
	}
	lines, err := a.vcs.FetchContextLines(file.DisplayPath(), file.Status, startLine, endLine)
	if err != nil {
		a.log.Error("failed to expand context", "file", file.DisplayPath(), "error", err)
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
	a.markDirty()
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

// isOthersComment returns true if the comment belongs to another user.
func (a *App) isOthersComment(c *model.Comment) bool {
	return c.Author != "" && a.session != nil && a.session.Reviewer != "" && c.Author != a.session.Reviewer
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
			if a.isOthersComment(&c) {
				a.setMessage("Cannot edit others' comments", messageWarning)
				return
			}
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
			if a.isOthersComment(&c) {
				a.setMessage("Cannot edit others' comments", messageWarning)
				return
			}
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

func (a *App) replyToCommentAtCursor() {
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

	var comment *model.Comment
	var lineNo int
	var side model.LineSide
	switch ann.Type {
	case annLineComment:
		lineNo = a.getCommentLineNo(ann)
		if comments, ok := fr.LineComments[lineNo]; ok && ann.CommentIdx < len(comments) {
			comment = &comments[ann.CommentIdx]
			side = comment.Side
		}
	default:
		a.setMessage("Move cursor to a comment to reply", messageWarning)
		return
	}

	if comment == nil {
		return
	}

	// Determine the thread root ExternalID
	replyTo := comment.ExternalID
	if replyTo == "" {
		a.setMessage("Can only reply to remote comments", messageWarning)
		return
	}

	// Move cursor to the last annotation line of the last comment in this thread
	// so the editor appears after the entire thread, not mid-comment.
	lastThreadLine := a.cursorLine
	for j := a.cursorLine + 1; j < len(a.annotations); j++ {
		next := a.annotations[j]
		if next.Type != annLineComment {
			break
		}
		// Still same line number and side?
		nextLineNo := next.NewLineNo
		if next.Side == model.SideOld {
			nextLineNo = next.OldLineNo
		}
		if nextLineNo != lineNo || next.Side != side {
			break
		}
		lastThreadLine = j
	}
	a.cursorLine = lastThreadLine

	a.replyToID = replyTo
	a.commentBuffer = ""
	a.commentCursor = 0
	a.commentType = model.CommentNote
	a.commentIsFile = false
	a.commentLine = lineNo
	a.commentSide = side
	a.editingID = ""
	a.inputMode = modeComment
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
					if c.ExternalID != "" && a.provider != nil {
						if err := a.provider.EditComment(a.providerID, c.ExternalID, a.commentBuffer); err != nil {
							a.log.Error("remote comment edit failed", "commentID", c.ExternalID, "error", err)
							a.setMessage("Remote edit failed: "+err.Error(), messageError)
							return
						}
					}
					fr.FileComments[i].Content = a.commentBuffer
					fr.FileComments[i].Type = a.commentType
					break
				}
			}
		} else {
			if comments, ok := fr.LineComments[a.commentLine]; ok {
				for i, c := range comments {
					if c.ID == a.editingID {
						if c.ExternalID != "" && a.provider != nil {
							if err := a.provider.EditComment(a.providerID, c.ExternalID, a.commentBuffer); err != nil {
								a.log.Error("remote comment edit failed", "commentID", c.ExternalID, "error", err)
								a.setMessage("Remote edit failed: "+err.Error(), messageError)
								return
							}
						}
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
		if a.replyToID != "" {
			comment.ReplyToID = a.replyToID
		}
		if a.session != nil && a.session.Reviewer != "" {
			comment.Author = a.session.Reviewer
		}

		if a.commentIsFile {
			fr.AddFileComment(comment)
		} else {
			fr.AddLineComment(a.commentLine, comment)
		}
	}

	a.markDirty()
	a.commentBuffer = ""
	a.editingID = ""
	a.replyToID = ""
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
			c := fr.FileComments[ann.CommentIdx]
			if a.isOthersComment(&c) {
				a.setMessage("Cannot delete others' comments", messageWarning)
				return
			}
			if c.ExternalID != "" && a.provider != nil {
				if err := a.provider.DeleteComment(a.providerID, c.ExternalID); err != nil {
					a.log.Error("remote comment delete failed", "commentID", c.ExternalID, "error", err)
					a.setMessage("Remote delete failed: "+err.Error(), messageError)
					return
				}
			}
			fr.FileComments = append(fr.FileComments[:ann.CommentIdx], fr.FileComments[ann.CommentIdx+1:]...)
			a.markDirty()
			a.rebuildAnnotations()
			a.setMessage("Comment deleted", messageInfo)
		}
	case annLineComment:
		lineNo := a.getCommentLineNo(ann)
		if comments, ok := fr.LineComments[lineNo]; ok && ann.CommentIdx < len(comments) {
			c := comments[ann.CommentIdx]
			if a.isOthersComment(&c) {
				a.setMessage("Cannot delete others' comments", messageWarning)
				return
			}
			if c.ExternalID != "" && a.provider != nil {
				if err := a.provider.DeleteComment(a.providerID, c.ExternalID); err != nil {
					a.log.Error("remote comment delete failed", "commentID", c.ExternalID, "error", err)
					a.setMessage("Remote delete failed: "+err.Error(), messageError)
					return
				}
			}
			fr.LineComments[lineNo] = append(comments[:ann.CommentIdx], comments[ann.CommentIdx+1:]...)
			if len(fr.LineComments[lineNo]) == 0 {
				delete(fr.LineComments, lineNo)
			}
			a.markDirty()
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

func (a App) handleReviewKey(msg tea.KeyPressMsg) (App, tea.Cmd) {
	key := msg.Key()
	switch {
	case key.Code == tea.KeyEscape:
		a.inputMode = modeNormal
		a.reviewBuffer = ""
	case key.Code == tea.KeyEnter && key.Mod == 0:
		a.saveOverallReview()
		a.inputMode = modeNormal
	case key.Code == tea.KeyTab:
		a.reviewStatus = a.reviewStatus.Next()
	default:
		handleTextInput(key, &a.reviewBuffer, &a.reviewCursor)
	}
	return a, nil
}

func (a App) handleEditPRKey(msg tea.KeyPressMsg) (App, tea.Cmd) {
	key := msg.Key()
	switch {
	case key.Code == tea.KeyEscape:
		a.inputMode = modeNormal
		a.reviewBuffer = ""
	case key.Code == tea.KeyEnter && key.Mod == 0:
		cmd := a.saveEditPR()
		a.inputMode = modeNormal
		a.reviewBuffer = ""
		return a, cmd
	default:
		handleTextInput(key, &a.reviewBuffer, &a.reviewCursor)
	}
	return a, nil
}

func (a *App) saveConversationComment() {
	body := strings.TrimSpace(a.convBuffer)
	a.convBuffer = ""
	a.convCursor = 0
	if body == "" {
		return
	}
	if a.provider != nil {
		if err := a.provider.PostConversationComment(a.providerID, body); err != nil {
			a.log.Error("failed to post conversation comment", "error", err)
			a.setMessage("Post failed: "+err.Error(), messageError)
			return
		}
	}
	cc := model.ConversationComment{
		Author:    a.session.Reviewer,
		Body:      body,
		CreatedAt: time.Now(),
	}
	a.session.Conversation = append(a.session.Conversation, cc)
	a.markDirty()
	a.setMessage("Comment posted", messageInfo)
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
	a.markDirty()
	a.reviewBuffer = ""
	a.setMessage("Overall review saved", messageInfo)
}

// Bug report

func (a *App) enterBugReport() {
	a.bugBuffer = ""
	a.bugCursor = 0
	a.inputMode = modeBug
}

func (a App) handleBugKey(msg tea.KeyPressMsg) (App, tea.Cmd) {
	key := msg.Key()
	switch {
	case key.Code == tea.KeyEscape:
		a.inputMode = modeNormal
		a.bugBuffer = ""
	case key.Code == tea.KeyEnter && key.Mod == 0:
		return a.submitBugReport()
	default:
		handleTextInput(key, &a.bugBuffer, &a.bugCursor)
	}
	return a, nil
}

func (a App) handleConversationKey(msg tea.KeyPressMsg) (App, tea.Cmd) {
	key := msg.Key()
	switch {
	case key.Code == tea.KeyEscape:
		a.inputMode = modeNormal
		a.convBuffer = ""
		a.convCursor = 0
	case key.Code == tea.KeyEnter && key.Mod == 0:
		a.saveConversationComment()
		a.inputMode = modeNormal
	default:
		handleTextInput(key, &a.convBuffer, &a.convCursor)
	}
	return a, nil
}

func (a App) submitBugReport() (App, tea.Cmd) {
	body := strings.TrimSpace(a.bugBuffer)
	if body == "" {
		a.inputMode = modeNormal
		a.bugBuffer = ""
		return a, nil
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
		a.log.Error("failed to write bug report", "error", err)
		a.setMessage("Bug report failed: "+err.Error(), messageError)
		a.inputMode = modeNormal
		a.bugBuffer = ""
		return a, nil
	}

	a.inputMode = modeNormal
	a.bugBuffer = ""
	a.setMessage(fmt.Sprintf("Bug report saved to %s", path), messageInfo)
	return a, func() tea.Msg { return ClipboardMsg{Content: path} }
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
		if a.store == nil {
			a.setMessage("No store configured", messageWarning)
		} else if _, err := a.store.Save(a.session); err != nil {
			a.log.Error("failed to save session", "error", err)
			a.setMessage("Save failed: "+err.Error(), messageError)
		} else {
			a.setMessage("Session saved", messageInfo)
			a.dirty = false
		}
		return nil
	case "x", "wq":
		if a.store != nil {
			a.store.Save(a.session)
		}
		a.dirty = false
		return func() tea.Msg { return DoneMsg{Session: a.session} }
	case "e", "reload":
		if a.vcs == nil {
			a.setMessage("Reload requires a VCS backend", messageWarning)
			return nil
		}
		files, err := a.vcs.GetWorkingTreeDiff()
		if err != nil {
			a.log.Error("failed to reload working tree diff", "error", err)
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
	case "submit":
		return a.submitToProvider()
	case "refresh":
		return a.refreshFromProvider()
	case "comment":
		return a.postConversationComment()
	case "ready":
		return a.markReadyForReview()
	case "edit-pr", "desc":
		return a.editPRDescription()
	case "clear":
		// Count draft comments first
		draftCount := 0
		for _, fr := range a.session.Files {
			for _, c := range fr.FileComments {
				if !c.Submitted && c.ExternalID == "" {
					draftCount++
				}
			}
			for _, comments := range fr.LineComments {
				for _, c := range comments {
					if !c.Submitted && c.ExternalID == "" {
						draftCount++
					}
				}
			}
		}
		if a.session.OverallReview != nil && a.session.OverallReview.ExternalID == "" {
			draftCount++
		}
		if draftCount == 0 {
			a.setMessage("No draft comments to clear", messageInfo)
			return nil
		}
		a.confirmPrompt = "Clear draft comments?"
		a.confirmCallback = func(a *App) tea.Cmd {
			cleared := 0
			for _, fr := range a.session.Files {
				kept := fr.FileComments[:0]
				for _, c := range fr.FileComments {
					if c.Submitted || c.ExternalID != "" {
						kept = append(kept, c)
					} else {
						cleared++
					}
				}
				if len(kept) == 0 {
					fr.FileComments = nil
				} else {
					fr.FileComments = kept
				}
				for line, comments := range fr.LineComments {
					var keptLine []model.Comment
					for _, c := range comments {
						if c.Submitted || c.ExternalID != "" {
							keptLine = append(keptLine, c)
						} else {
							cleared++
						}
					}
					if len(keptLine) == 0 {
						delete(fr.LineComments, line)
					} else {
						fr.LineComments[line] = keptLine
					}
				}
			}
			if a.session.OverallReview != nil && a.session.OverallReview.ExternalID == "" {
				a.session.OverallReview = nil
				cleared++
			}
			a.markDirty()
			a.rebuildAnnotations()
			a.setMessage(fmt.Sprintf("Cleared %d draft comment(s)", cleared), messageInfo)
			return nil
		}
		a.inputMode = modeConfirm
		return nil
	}

	a.setMessage("Unknown command: "+cmd, messageWarning)
	return nil
}

func (a *App) postConversationComment() tea.Cmd {
	if a.provider == nil {
		a.setMessage("Not connected to a remote provider", messageWarning)
		return nil
	}
	a.convBuffer = ""
	a.convCursor = 0
	a.inputMode = modeConversation
	a.focusedPanel = panelConversation
	if !a.showConversation {
		a.showConversation = true
		a.convScroll = 0
	}
	return nil
}

func (a *App) submitToProvider() tea.Cmd {
	if a.provider == nil {
		a.setMessage("Not connected to a remote provider", messageWarning)
		return nil
	}

	drafts := provider.ExportComments(a.session, a.session.Reviewer)
	state := provider.ExportReviewState(model.ApprovalNeutral)
	body := ""
	if a.session.OverallReview != nil {
		state = provider.ExportReviewState(a.session.OverallReview.Status)
		body = a.session.OverallReview.Body
	}

	statusText := "Comment"
	switch state {
	case provider.ReviewApprove:
		statusText = "Approve"
	case provider.ReviewRequestChanges:
		statusText = "Request Changes"
	}

	prompt := fmt.Sprintf("Submit review to %s #%s? (%s, %d comments)",
		a.session.Provider.Name, a.session.Provider.ID, statusText, len(drafts))

	a.confirmPrompt = prompt
	a.confirmCallback = func(a *App) tea.Cmd {
		req := provider.SubmitReviewRequest{
			Body:     body,
			State:    state,
			Comments: drafts,
		}
		if err := a.provider.SubmitReview(a.providerID, req); err != nil {
			a.log.Error("failed to submit review", "state", state, "error", err)
			a.setMessage("Submit failed: "+err.Error(), messageError)
			return nil
		}

		// Mark only the exported comments as submitted
		exported := make(map[string]bool, len(drafts))
		for _, d := range drafts {
			// Key by path+line+side+body to match exactly what was exported
			exported[d.Path+"\x00"+strconv.Itoa(d.Line)+"\x00"+d.Side+"\x00"+d.Body] = true
		}
		for _, fr := range a.session.Files {
			for line, comments := range fr.LineComments {
				for i := range comments {
					c := &comments[i]
					if c.Submitted || c.ExternalID != "" {
						continue
					}
					side := "new"
					if c.Side == model.SideOld {
						side = "old"
					}
					key := fr.Path + "\x00" + strconv.Itoa(line) + "\x00" + side + "\x00" + c.Content
					if exported[key] {
						c.Submitted = true
					}
				}
				fr.LineComments[line] = comments
			}
		}

		// Auto-save
		if a.store != nil {
			a.store.Save(a.session)
		}
		a.dirty = false
		a.setMessage(fmt.Sprintf("Review submitted to %s #%s", a.session.Provider.Name, a.session.Provider.ID), messageInfo)
		return nil
	}
	a.inputMode = modeConfirm
	return nil
}

func (a *App) refreshFromProvider() tea.Cmd {
	if a.provider == nil {
		a.setMessage("Not connected to a remote provider", messageWarning)
		return nil
	}

	result, err := a.provider.Refresh(a.providerID)
	if err != nil {
		a.log.Error("failed to refresh from provider", "error", err)
		a.setMessage("Refresh failed: "+err.Error(), messageError)
		return nil
	}

	newCount := 0

	// Import new reviews
	if len(result.NewReviews) > 0 {
		for _, r := range result.NewReviews {
			a.session.Reviews = append(a.session.Reviews, provider.ImportReview(r))
		}
		newCount += len(result.NewReviews)
	}

	// Import new comments
	if len(result.NewComments) > 0 {
		imported := provider.ImportComments(result.NewComments)
		for path, lineComments := range imported {
			fr := a.session.GetOrCreateFileReview(path, model.FileModified)
			newCount += provider.MergeImportedComments(fr, lineComments)
		}
	}

	// Import new conversation
	if len(result.NewConversation) > 0 {
		a.session.Conversation = append(a.session.Conversation, provider.ImportConversation(result.NewConversation)...)
		newCount += len(result.NewConversation)
	}

	// Update outdated status from full comment list
	if len(result.AllComments) > 0 {
		outdated := make(map[string]bool, len(result.AllComments))
		for _, c := range result.AllComments {
			if c.IsOutdated {
				outdated[c.ExternalID] = true
			}
		}
		for _, fr := range a.session.Files {
			for line, comments := range fr.LineComments {
				changed := false
				for i := range comments {
					if comments[i].ExternalID != "" {
						was := comments[i].IsOutdated
						comments[i].IsOutdated = outdated[comments[i].ExternalID]
						if was != comments[i].IsOutdated {
							changed = true
						}
					}
				}
				if changed {
					fr.LineComments[line] = comments
				}
			}
		}
	}

	// Re-parse diff if changed
	if result.DiffChanged && result.Diff != "" {
		files := vcs.ParseDiff(result.Diff)
		if a.highlighter != nil {
			a.highlighter.HighlightFiles(files)
		}
		a.diffFiles = files
		for _, f := range files {
			a.session.GetOrCreateFileReview(f.DisplayPath(), f.Status)
		}
		a.rebuildFileTree()
	}

	a.rebuildAnnotations()
	if a.store != nil {
		if _, err := a.store.Save(a.session); err == nil {
			a.dirty = false
		}
	}
	a.setMessage(fmt.Sprintf("Refreshed: %d new items", newCount), messageInfo)
	return nil
}

func (a *App) toggleResolveThread() tea.Cmd {
	if a.provider == nil {
		a.setMessage("Not connected to a remote provider", messageWarning)
		return nil
	}
	if a.cursorLine < 0 || a.cursorLine >= len(a.annotations) {
		return nil
	}
	ann := a.annotations[a.cursorLine]
	if ann.Type != annLineComment && ann.Type != annFileComment {
		a.setMessage("Move cursor to a comment to resolve/unresolve", messageWarning)
		return nil
	}

	var comment *model.Comment
	path := a.currentFilePath()
	if path == "" {
		return nil
	}
	fr := a.session.GetFileReview(path)
	if fr == nil {
		return nil
	}

	switch ann.Type {
	case annFileComment:
		if ann.CommentIdx < len(fr.FileComments) {
			comment = &fr.FileComments[ann.CommentIdx]
		}
	case annLineComment:
		lineNo := a.getCommentLineNo(ann)
		if comments, ok := fr.LineComments[lineNo]; ok && ann.CommentIdx < len(comments) {
			comment = &comments[ann.CommentIdx]
			defer func() { fr.LineComments[lineNo] = comments }()
		}
	}

	if comment == nil {
		return nil
	}
	if comment.ThreadID == "" {
		a.setMessage("Comment has no thread to resolve", messageWarning)
		return nil
	}

	if comment.IsResolved {
		if err := a.provider.UnresolveThread(a.providerID, comment.ThreadID); err != nil {
			a.log.Error("failed to unresolve thread", "threadID", comment.ThreadID, "error", err)
			a.setMessage("Unresolve failed: "+err.Error(), messageError)
			return nil
		}
		a.setThreadResolved(comment.ThreadID, false)
		a.setMessage("Thread unresolved", messageInfo)
	} else {
		if err := a.provider.ResolveThread(a.providerID, comment.ThreadID); err != nil {
			a.log.Error("failed to resolve thread", "threadID", comment.ThreadID, "error", err)
			a.setMessage("Resolve failed: "+err.Error(), messageError)
			return nil
		}
		a.setThreadResolved(comment.ThreadID, true)
		a.setMessage("Thread resolved", messageInfo)
	}
	a.rebuildAnnotations()
	return nil
}

// setThreadResolved updates IsResolved on all comments sharing the given
// thread ID across every file in the session.
func (a *App) setThreadResolved(threadID string, resolved bool) {
	if a.session == nil {
		return
	}
	for _, fr := range a.session.Files {
		for i := range fr.FileComments {
			if fr.FileComments[i].ThreadID == threadID {
				fr.FileComments[i].IsResolved = resolved
			}
		}
		for lineNo, comments := range fr.LineComments {
			for i := range comments {
				if comments[i].ThreadID == threadID {
					comments[i].IsResolved = resolved
				}
			}
			fr.LineComments[lineNo] = comments
		}
	}
}

func (a *App) markReadyForReview() tea.Cmd {
	if a.provider == nil {
		a.setMessage("Not connected to a remote provider", messageWarning)
		return nil
	}
	if a.session == nil || !a.session.IsDraft {
		a.setMessage("PR is not a draft", messageWarning)
		return nil
	}
	a.confirmPrompt = "Mark PR as ready for review?"
	a.confirmCallback = func(a *App) tea.Cmd {
		if err := a.provider.MarkReadyForReview(a.providerID); err != nil {
			a.log.Error("failed to mark PR as ready", "error", err)
			a.setMessage("Failed: "+err.Error(), messageError)
			return nil
		}
		a.session.IsDraft = false
		a.setMessage("PR marked as ready for review", messageInfo)
		return nil
	}
	a.inputMode = modeConfirm
	return nil
}

func (a *App) editPRDescription() tea.Cmd {
	if a.provider == nil {
		a.setMessage("Not connected to a remote provider", messageWarning)
		return nil
	}
	if a.session == nil || a.session.Provider == nil {
		a.setMessage("No PR session", messageWarning)
		return nil
	}

	// Parse title and body from description
	desc := a.session.Description
	parts := strings.SplitN(desc, "\n\n", 2)
	title := parts[0]
	body := ""
	if len(parts) > 1 {
		body = parts[1]
	}

	// Use the review buffer/mode to edit the title
	a.reviewBuffer = title + "\n\n" + body
	a.reviewCursor = len(a.reviewBuffer)
	a.inputMode = modeEditPR
	return nil
}

func (a *App) saveEditPR() tea.Cmd {
	if a.provider == nil {
		return nil
	}
	text := strings.TrimSpace(a.reviewBuffer)
	if text == "" {
		a.setMessage("Title cannot be empty", messageWarning)
		return nil
	}

	parts := strings.SplitN(text, "\n\n", 2)
	title := strings.TrimSpace(parts[0])
	body := ""
	if len(parts) > 1 {
		body = strings.TrimSpace(parts[1])
	}

	if err := a.provider.UpdateReviewRequest(a.providerID, title, body); err != nil {
		a.log.Error("failed to update PR", "error", err)
		a.setMessage("Update failed: "+err.Error(), messageError)
		return nil
	}

	if body != "" {
		a.session.Description = title + "\n\n" + body
	} else {
		a.session.Description = title
	}
	a.markDirty()
	a.setMessage("PR description updated", messageInfo)
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
