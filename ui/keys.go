package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/pgavlin/crtea/model"
	"github.com/pgavlin/crtea/output"
	"github.com/pgavlin/crtea/persistence"
)

func (a App) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Clear message on any keypress
	a.clearMessage()

	switch a.inputMode {
	case ModeHelp:
		return a.handleHelpKey(msg)
	case ModeCommand:
		return a.handleCommandKey(msg)
	case ModeSearch:
		return a.handleSearchKey(msg)
	case ModeComment:
		return a.handleCommentKey(msg)
	case ModeConfirm:
		return a.handleConfirmKey(msg)
	case ModeVisualSelect:
		return a.handleVisualKey(msg)
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
			a.setMessage("Unsaved changes. Press q again to quit, or :w to save.", MessageWarning)
			return &a, nil
		}
		return &a, tea.Quit

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
		if a.focusedPanel == PanelDiff {
			if a.scrollX > 0 {
				a.scrollX--
			}
		}
	case key.Code == 'l' && key.Mod == 0, key.Code == tea.KeyRight:
		if a.focusedPanel == PanelDiff {
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
		if a.showFileList {
			if a.focusedPanel == PanelDiff {
				a.focusedPanel = PanelFileList
			} else {
				a.focusedPanel = PanelDiff
			}
		}

	// Enter: jump to file (file list) or toggle expand (diff)
	case key.Code == tea.KeyEnter:
		if a.focusedPanel == PanelFileList {
			a.jumpToFile(a.fileListCursor)
			a.focusedPanel = PanelDiff
		} else if a.focusedPanel == PanelDiff {
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

	// Visual mode
	case key.Code == 'v' && key.Mod == 0:
		a.enterVisualMode()

	// Command/search
	case key.Text == ":":
		a.inputMode = ModeCommand
		a.commandBuffer = ""
	case key.Text == "/":
		a.inputMode = ModeSearch
		a.searchBuffer = ""
	case key.Code == 'n' && key.Mod == 0:
		a.searchNext(true)
	case key.Text == "N":
		a.searchNext(false)

	// Help
	case key.Text == "?":
		a.inputMode = ModeHelp
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
			if !a.showFileList && a.focusedPanel == PanelFileList {
				a.focusedPanel = PanelDiff
			}
		case 'h':
			a.focusedPanel = PanelFileList
		case 'l':
			a.focusedPanel = PanelDiff
		}
	}

	return &a, nil
}

func (a App) handleHelpKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.Key()
	switch {
	case key.Code == tea.KeyEscape, key.Code == 'q', key.Text == "?":
		a.inputMode = ModeNormal
	}
	return &a, nil
}

func (a App) handleCommandKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.Key()
	switch {
	case key.Code == tea.KeyEscape:
		a.inputMode = ModeNormal
		a.commandBuffer = ""
	case key.Code == tea.KeyEnter:
		cmd := a.executeCommand(a.commandBuffer)
		a.inputMode = ModeNormal
		a.commandBuffer = ""
		return &a, cmd
	case key.Code == tea.KeyBackspace:
		if len(a.commandBuffer) > 0 {
			a.commandBuffer = a.commandBuffer[:len(a.commandBuffer)-1]
		} else {
			a.inputMode = ModeNormal
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
		a.inputMode = ModeNormal
		a.searchBuffer = ""
	case key.Code == tea.KeyEnter:
		a.lastSearch = a.searchBuffer
		a.inputMode = ModeNormal
		a.searchNext(true)
	case key.Code == tea.KeyBackspace:
		if len(a.searchBuffer) > 0 {
			a.searchBuffer = a.searchBuffer[:len(a.searchBuffer)-1]
		} else {
			a.inputMode = ModeNormal
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
		a.inputMode = ModeNormal
		a.commentBuffer = ""
		a.editingID = ""
		a.commentLineRange = nil
	case key.Code == tea.KeyEnter && key.Mod == tea.ModShift:
		// Shift-Enter: newline in comment
		a.commentBuffer = a.commentBuffer[:a.commentCursor] + "\n" + a.commentBuffer[a.commentCursor:]
		a.commentCursor++
	case key.Code == tea.KeyEnter:
		a.saveComment()
		a.inputMode = ModeNormal
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
		a.inputMode = ModeNormal
		// Execute confirmed action
	case key.Code == tea.KeyEscape, key.Code == 'n', key.Code == 'N':
		a.inputMode = ModeNormal
	}
	return &a, nil
}

func (a App) handleVisualKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.Key()
	switch {
	case key.Code == tea.KeyEscape, key.Code == 'v':
		a.inputMode = ModeNormal
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
	if a.focusedPanel == PanelFileList {
		a.fileListCursor += n
		if a.fileListCursor >= len(a.diffFiles) {
			a.fileListCursor = len(a.diffFiles) - 1
		}
		if a.fileListCursor < 0 {
			a.fileListCursor = 0
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
	if a.focusedPanel == PanelFileList {
		a.fileListCursor -= n
		if a.fileListCursor < 0 {
			a.fileListCursor = 0
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
		if ann.Type == AnnFileHeader && ann.FileIdx == fileIdx {
			a.cursorLine = i
			a.ensureCursorVisible()
			return
		}
	}
}

func (a *App) jumpToSourceLine(lineNo int) {
	// Find the annotation matching this source line number
	for i, ann := range a.annotations {
		if ann.Type == AnnDiffLine && ann.NewLineNo == lineNo {
			a.cursorLine = i
			a.ensureCursorVisible()
			return
		}
	}
}

func (a *App) nextFile() {
	currentFile := a.currentFileIdx()
	for i := a.cursorLine + 1; i < len(a.annotations); i++ {
		if a.annotations[i].Type == AnnFileHeader && a.annotations[i].FileIdx > currentFile {
			a.cursorLine = i
			a.ensureCursorVisible()
			return
		}
	}
}

func (a *App) prevFile() {
	currentFile := a.currentFileIdx()
	for i := a.cursorLine - 1; i >= 0; i-- {
		if a.annotations[i].Type == AnnFileHeader && a.annotations[i].FileIdx < currentFile {
			a.cursorLine = i
			a.ensureCursorVisible()
			return
		}
	}
}

func (a *App) nextHunk() {
	for i := a.cursorLine + 1; i < len(a.annotations); i++ {
		if a.annotations[i].Type == AnnHunkHeader {
			a.cursorLine = i
			a.ensureCursorVisible()
			return
		}
	}
}

func (a *App) prevHunk() {
	for i := a.cursorLine - 1; i >= 0; i-- {
		if a.annotations[i].Type == AnnHunkHeader {
			a.cursorLine = i
			a.ensureCursorVisible()
			return
		}
	}
}

// Context expansion

func (a *App) toggleExpandAtCursor() {
	if a.cursorLine < 0 || a.cursorLine >= len(a.annotations) {
		return
	}
	ann := a.annotations[a.cursorLine]

	switch ann.Type {
	case AnnExpander:
		a.expandGap(ann.GapID)
	case AnnExpandedContext:
		a.collapseGap(ann.GapID)
	}
}

func (a *App) expandGap(gid GapID) {
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
		a.setMessage("Failed to expand context: "+err.Error(), MessageError)
		return
	}

	a.expandedGaps[gid] = lines
	a.rebuildAnnotations()
}

func (a *App) collapseGap(gid GapID) {
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
		a.setMessage(fmt.Sprintf("Marked %s as reviewed", path), MessageInfo)
	} else {
		a.setMessage(fmt.Sprintf("Unmarked %s as reviewed", path), MessageInfo)
	}
}

// Comment actions

func (a *App) enterLineComment() {
	if a.cursorLine < 0 || a.cursorLine >= len(a.annotations) {
		return
	}
	ann := a.annotations[a.cursorLine]
	if ann.Type != AnnDiffLine {
		a.setMessage("Move cursor to a diff line to comment", MessageWarning)
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
	a.inputMode = ModeComment
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
	a.inputMode = ModeComment
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
	case AnnFileComment:
		if ann.CommentIdx < len(fr.FileComments) {
			c := fr.FileComments[ann.CommentIdx]
			a.commentBuffer = c.Content
			a.commentCursor = len(c.Content)
			a.commentType = c.Type
			a.commentIsFile = true
			a.editingID = c.ID
			a.inputMode = ModeComment
		}
	case AnnLineComment:
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
			a.inputMode = ModeComment
		}
	default:
		a.setMessage("Move cursor to a comment to edit", MessageWarning)
	}
}

func (a *App) getCommentLineNo(ann AnnotatedLine) int {
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
	case AnnFileComment:
		if ann.CommentIdx < len(fr.FileComments) {
			fr.FileComments = append(fr.FileComments[:ann.CommentIdx], fr.FileComments[ann.CommentIdx+1:]...)
			a.dirty = true
			a.rebuildAnnotations()
			a.setMessage("Comment deleted", MessageInfo)
		}
	case AnnLineComment:
		lineNo := a.getCommentLineNo(ann)
		if comments, ok := fr.LineComments[lineNo]; ok && ann.CommentIdx < len(comments) {
			fr.LineComments[lineNo] = append(comments[:ann.CommentIdx], comments[ann.CommentIdx+1:]...)
			if len(fr.LineComments[lineNo]) == 0 {
				delete(fr.LineComments, lineNo)
			}
			a.dirty = true
			a.rebuildAnnotations()
			a.setMessage("Comment deleted", MessageInfo)
		}
	default:
		a.setMessage("Move cursor to a comment to delete", MessageWarning)
	}
}

// Visual mode

func (a *App) enterVisualMode() {
	if a.cursorLine < 0 || a.cursorLine >= len(a.annotations) {
		return
	}
	ann := a.annotations[a.cursorLine]
	if ann.Type != AnnDiffLine {
		a.setMessage("Move cursor to a diff line for visual selection", MessageWarning)
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
	a.inputMode = ModeVisualSelect
}

func (a *App) enterCommentFromVisual() {
	if a.cursorLine < 0 || a.cursorLine >= len(a.annotations) {
		return
	}
	ann := a.annotations[a.cursorLine]
	if ann.Type != AnnDiffLine {
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
	a.inputMode = ModeComment
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
		case AnnFileHeader:
			if ann.FileIdx >= 0 && ann.FileIdx < len(a.diffFiles) {
				return strings.Contains(strings.ToLower(a.diffFiles[ann.FileIdx].DisplayPath()), pattern)
			}
		case AnnDiffLine:
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
				a.setMessage("Search wrapped to top", MessageInfo)
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
				a.setMessage("Search wrapped to bottom", MessageInfo)
				return
			}
		}
	}

	a.setMessage("Pattern not found: "+a.lastSearch, MessageWarning)
}

// Command execution

func (a *App) executeCommand(cmd string) tea.Cmd {
	cmd = strings.TrimSpace(cmd)
	switch cmd {
	case "q", "quit":
		if a.dirty && !a.quitWarned {
			a.quitWarned = true
			a.setMessage("Unsaved changes. Use :q! to force quit, or :w to save.", MessageWarning)
			return nil
		}
		return tea.Quit
	case "q!", "quit!":
		return tea.Quit
	case "w", "write":
		if _, err := persistence.Save(a.session); err != nil {
			a.setMessage("Save failed: "+err.Error(), MessageError)
		} else {
			a.setMessage("Session saved", MessageInfo)
			a.dirty = false
		}
		return nil
	case "x", "wq":
		persistence.Save(a.session)
		a.dirty = false
		return tea.Quit
	case "e", "reload":
		files, err := a.vcs.GetWorkingTreeDiff()
		if err != nil {
			a.setMessage("Reload failed: "+err.Error(), MessageError)
			return nil
		}
		a.diffFiles = files
		a.rebuildAnnotations()
		a.setMessage(fmt.Sprintf("Reloaded %d files", len(files)), MessageInfo)
		return nil
	case "clip", "export":
		return a.exportComments()
	case "clear":
		for _, fr := range a.session.Files {
			fr.FileComments = nil
			fr.LineComments = make(map[int][]model.Comment)
		}
		a.dirty = true
		a.rebuildAnnotations()
		a.setMessage("All comments cleared", MessageInfo)
		return nil
	}

	if strings.HasPrefix(cmd, "set ") {
		opt := strings.TrimPrefix(cmd, "set ")
		switch opt {
		case "wrap":
			a.setMessage("Line wrapping enabled", MessageInfo)
		case "nowrap":
			a.setMessage("Line wrapping disabled", MessageInfo)
		default:
			a.setMessage("Unknown option: "+opt, MessageWarning)
		}
		return nil
	}

	a.setMessage("Unknown command: "+cmd, MessageWarning)
	return nil
}

func (a *App) exportComments() tea.Cmd {
	total := a.session.TotalComments()
	if total == 0 {
		a.setMessage("No comments to export", MessageWarning)
		return nil
	}
	md := output.GenerateMarkdown(a.session)
	a.setMessage(fmt.Sprintf("Exported %d comments to clipboard", total), MessageInfo)
	return tea.SetClipboard(md)
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
