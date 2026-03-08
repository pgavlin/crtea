// Package ui provides an embeddable Bubble Tea component for interactive code review.
package ui

import (
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/pgavlin/crtea/model"
	"github.com/pgavlin/crtea/persistence"
	"github.com/pgavlin/crtea/provider"
	"github.com/pgavlin/crtea/syntax"
	"github.com/pgavlin/crtea/theme"
	"github.com/pgavlin/crtea/vcs"
)

type inputMode int

const (
	modeNormal inputMode = iota
	modeCommand
	modeSearch
	modeComment
	modeHelp
	modeConfirm
	modeVisualSelect
	modeReview
	modeBug
	modeConversation
)

type focusedPanel int

const (
	panelDiff focusedPanel = iota
	panelFileList
	panelCommitList
	panelConversation
)

type messageLevel int

const (
	messageInfo messageLevel = iota
	messageWarning
	messageError
)

type statusMessage struct {
	text  string
	level messageLevel
}

// DoneMsg is emitted when the component wants to close.
// Session contains the final review state (nil if no review was started).
type DoneMsg struct {
	Session *model.ReviewSession
}

// ClipboardMsg is emitted when the component wants to copy text to the clipboard.
type ClipboardMsg struct {
	Content string
}

// App is the main Bubble Tea model for the code review TUI.
type App struct {
	// Core data
	vcs               vcs.Backend
	vcsInfo           vcs.VcsInfo
	session           *model.ReviewSession
	diffFiles         []model.DiffFile
	combinedDiffFiles []model.DiffFile // original combined diff (from provider), used when all commits enabled
	highlighter       *syntax.Highlighter

	// Phase
	phase          appPhase
	pickerItems    []pickerItem
	pickerCursor   int
	pickerSelected map[int]bool

	// Commit list (review mode)
	reviewCommits    []vcs.CommitInfo
	commitDiffs      map[string][]model.DiffFile
	enabledCommits   map[string]bool
	includesWorkTree bool
	commitCursor     int
	showDescription  bool // show description instead of commit list
	showConversation bool // show conversation panel
	descScroll       int  // scroll offset for description panel
	convScroll       int  // scroll offset for conversation panel

	// UI state
	inputMode    inputMode
	focusedPanel focusedPanel
	showFileList bool

	// Window dimensions
	width  int
	height int

	// Diff view state
	annotations  []annotatedLine
	cursorLine   int
	scrollOffset int
	scrollX      int
	expandedGaps map[gapID][]model.DiffLine

	// File list state
	fileTree       *fileTreeNode
	fileTreeRows   []fileTreeRow
	collapsedDirs  map[string]bool
	fileListCursor int
	fileListScroll int

	// Text input buffers
	commandBuffer string
	searchBuffer  string
	lastSearch    string
	commentBuffer string
	commentCursor int
	commentType   model.CommentType
	commentIsFile bool
	commentLine   int // line number being commented on (0 = file-level)
	commentSide   model.LineSide
	editingID     string // non-empty if editing an existing comment

	// Visual selection
	visualAnchor     int // line number where visual selection started
	visualAnchorSide model.LineSide
	commentLineRange *model.LineRange

	// Reply tracking
	replyToID string // ExternalID of comment being replied to

	// Overall review editor
	reviewBuffer string
	reviewCursor int
	reviewStatus model.ApprovalStatus

	// Conversation editor
	convBuffer string
	convCursor int

	// Bug report editor
	bugBuffer string
	bugCursor int

	// Confirm dialog
	confirmPrompt   string
	confirmCallback func(a *App) tea.Cmd

	// Messages
	message    *statusMessage
	dirty      bool
	quitWarned bool

	// Help screen
	helpScroll int

	// Search highlighting
	searchHighlight string // last successful search pattern (for highlighting matches)

	// Pending key sequences
	pendingCount  string // digit accumulator for {N}G
	pendingPrefix rune   // for pending key sequences like d, z, ;

	// Persistence
	store persistence.Store

	// Theme
	theme theme.Theme

	// Remote provider (nil for local-only reviews)
	provider   provider.Provider
	providerID string // opaque ID for the review request
}

// NewApp creates a new App model.
func NewApp(backend vcs.Backend, files []model.DiffFile, session *model.ReviewSession, th theme.Theme, hl *syntax.Highlighter, store persistence.Store) App {
	var vcsInfo vcs.VcsInfo
	if backend != nil {
		vcsInfo = backend.Info()
	}
	app := App{
		vcs:           backend,
		vcsInfo:       vcsInfo,
		session:       session,
		diffFiles:     files,
		highlighter:   hl,
		inputMode:     modeNormal,
		focusedPanel:  panelDiff,
		showFileList:  true,
		expandedGaps:  make(map[gapID][]model.DiffLine),
		collapsedDirs: make(map[string]bool),
		store:         store,
		theme:         th,
	}
	// Show description by default when there's a description but no commit list.
	if session != nil && session.Description != "" && app.commitListItems() == nil {
		app.showDescription = true
	}
	app.rebuildFileTree()
	app.rebuildAnnotations()
	return app
}

// NewAppWithCommits creates a new App with pre-populated commit list data.
// commits should be in newest-first order. commitDiffs maps commit IDs (and "worktree")
// to their individual diffs. enabledCommits indicates which are initially enabled.
func NewAppWithCommits(backend vcs.Backend, commits []vcs.CommitInfo, commitDiffs map[string][]model.DiffFile, enabledCommits map[string]bool, includesWorkTree bool, session *model.ReviewSession, th theme.Theme, hl *syntax.Highlighter, store persistence.Store) App {
	app := App{
		vcs:              backend,
		vcsInfo:          backend.Info(),
		session:          session,
		highlighter:      hl,
		inputMode:        modeNormal,
		focusedPanel:     panelDiff,
		showFileList:     true,
		expandedGaps:     make(map[gapID][]model.DiffLine),
		collapsedDirs:    make(map[string]bool),
		store:            store,
		theme:            th,
		reviewCommits:    commits,
		commitDiffs:      commitDiffs,
		enabledCommits:   enabledCommits,
		includesWorkTree: includesWorkTree,
	}
	app.diffFiles = app.mergeEnabledDiffs()
	if session != nil {
		for _, f := range app.diffFiles {
			session.GetOrCreateFileReview(f.DisplayPath(), f.Status)
		}
	}
	app.rebuildFileTree()
	app.rebuildAnnotations()
	return app
}

// NewPickerApp creates an App that starts in the commit picker phase.
func NewPickerApp(backend vcs.Backend, th theme.Theme, hl *syntax.Highlighter, store persistence.Store) App {
	commits, _ := backend.GetRecentCommits(0, 30)

	var items []pickerItem
	items = append(items, pickerItem{isWorkingTree: true})
	for _, c := range commits {
		items = append(items, pickerItem{commit: c})
	}

	return App{
		vcs:            backend,
		vcsInfo:        backend.Info(),
		highlighter:    hl,
		store:          store,
		theme:          th,
		phase:          phasePicker,
		pickerItems:    items,
		pickerSelected: make(map[int]bool),
		expandedGaps:   make(map[gapID][]model.DiffLine),
		collapsedDirs:  make(map[string]bool),
	}
}

func (a *App) rebuildFileTree() {
	a.fileTree = buildFileTree(a.diffFiles)
	a.fileTreeRows = flattenTree(a.fileTree, a.collapsedDirs)
}

func (a *App) rebuildAnnotations() {
	a.annotations = buildAnnotations(a.diffFiles, a.session, a.expandedGaps, a.commentWrapWidth())
}

// commentWrapWidth returns the available character width for comment text content.
func (a *App) commentWrapWidth() int {
	w := a.width
	if a.showFileList {
		w -= a.fileListWidth() + 1
	}
	w -= 14 // gutter (10) + box borders "│ " + " │" (4)
	if w < 10 {
		return 0 // disable wrapping if too narrow
	}
	return w
}

// SetProvider sets the remote provider and review request ID.
func (a *App) SetProvider(p provider.Provider, id string) {
	a.provider = p
	a.providerID = id
}

// SetCommits populates the commit list and per-commit diffs for the review.
// commits should be newest-first. All commits are enabled by default.
func (a *App) SetCommits(commits []vcs.CommitInfo, diffs map[string][]model.DiffFile) {
	a.reviewCommits = commits
	a.commitDiffs = diffs
	a.enabledCommits = make(map[string]bool, len(commits))
	for _, c := range commits {
		a.enabledCommits[c.ID] = true
	}
	// Save the current combined diff (from provider) for use when all commits are enabled.
	// mergeEnabledDiffs naively appends hunks and breaks when multiple commits touch the same file.
	a.combinedDiffFiles = a.diffFiles
	// Don't rebuild — all commits are enabled so the combined diff is already correct.
}

// Init implements tea.Model.
func (a App) Init() tea.Cmd {
	return nil
}

// SetSize sets the component's dimensions. Call this from the parent model
// when the available size changes (e.g. in response to tea.WindowSizeMsg).
func (a *App) SetSize(width, height int) {
	a.width = width
	a.height = height
	a.rebuildAnnotations()
}

// Update implements tea.Model.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return a.handleKey(msg)
	default:
		_ = msg
	}
	return &a, nil
}

// View implements tea.Model.
func (a App) View() tea.View {
	if a.width == 0 || a.height == 0 {
		return tea.NewView("")
	}

	if a.phase == phasePicker {
		return tea.NewView(a.renderPicker())
	}

	var b strings.Builder

	// Header (1 line)
	b.WriteString(a.renderStatusBar())
	b.WriteString("\n")

	// Top panel (commit list or description)
	contentHeight := a.height - 2 // header + footer
	tpHeight := a.topPanelHeight()
	if tpHeight > 0 {
		if a.showDescription {
			b.WriteString(a.renderDescription(a.width, tpHeight))
		} else {
			b.WriteString(a.renderCommitList(a.width, tpHeight))
		}
		b.WriteString("\n")
		contentHeight -= tpHeight
	}
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Conversation panel (bottom half of content area)
	convHeight := a.conversationPanelHeight(contentHeight)
	if convHeight > 0 {
		contentHeight -= convHeight
	}

	// Bug report editor pane (steals height from content area)
	if a.inputMode == modeBug {
		bugHeight := 8
		if bugHeight > contentHeight-2 {
			bugHeight = contentHeight - 2
		}
		if bugHeight < 3 {
			bugHeight = 3
		}
		contentHeight -= bugHeight + 1
		if a.showFileList {
			fileListWidth := a.fileListWidth()
			diffWidth := a.width - fileListWidth - 1
			if diffWidth < 1 {
				diffWidth = 1
			}
			fileList := a.renderFileList(fileListWidth, contentHeight)
			diffView := a.renderDiffView(diffWidth, contentHeight)
			b.WriteString(joinHorizontal(fileList, diffView, contentHeight))
		} else {
			b.WriteString(a.renderDiffView(a.width, contentHeight))
		}
		b.WriteString("\n")
		b.WriteString(a.renderBugEditor(a.width, bugHeight))
	} else if a.inputMode == modeReview {
		reviewHeight := 8
		if reviewHeight > contentHeight-2 {
			reviewHeight = contentHeight - 2
		}
		if reviewHeight < 3 {
			reviewHeight = 3
		}
		contentHeight -= reviewHeight + 1 // +1 for separator newline
		// Render content first, then review pane below
		if a.showFileList {
			fileListWidth := a.fileListWidth()
			diffWidth := a.width - fileListWidth - 1
			if diffWidth < 1 {
				diffWidth = 1
			}
			fileList := a.renderFileList(fileListWidth, contentHeight)
			diffView := a.renderDiffView(diffWidth, contentHeight)
			b.WriteString(joinHorizontal(fileList, diffView, contentHeight))
		} else {
			b.WriteString(a.renderDiffView(a.width, contentHeight))
		}
		b.WriteString("\n")
		b.WriteString(a.renderReviewEditor(a.width, reviewHeight))
	} else if a.inputMode == modeHelp {
		b.WriteString(a.renderHelp(contentHeight))
	} else if a.showFileList {
		fileListWidth := a.fileListWidth()
		diffWidth := a.width - fileListWidth - 1 // -1 for separator
		if diffWidth < 1 {
			diffWidth = 1
		}
		fileList := a.renderFileList(fileListWidth, contentHeight)
		diffView := a.renderDiffView(diffWidth, contentHeight)
		b.WriteString(joinHorizontal(fileList, diffView, contentHeight))
	} else {
		b.WriteString(a.renderDiffView(a.width, contentHeight))
	}

	// Conversation panel (below diff content)
	if convHeight > 0 {
		b.WriteString("\n")
		b.WriteString(a.renderConversation(a.width, convHeight, a.focusedPanel == panelConversation))
	}

	// Footer (1 line)
	b.WriteString("\n")
	b.WriteString(a.renderFooter())

	return tea.NewView(b.String())
}

// captureScreen renders the current screen as it would appear in normal mode.
func (a *App) captureScreen() string {
	saved := a.inputMode
	a.inputMode = modeNormal
	view := a.View()
	a.inputMode = saved
	return view.Content
}

func (a *App) fileListWidth() int {
	w := a.width / 5
	if w < 20 {
		w = 20
	}
	if w > 40 {
		w = 40
	}
	return w
}

func (a *App) diffViewportHeight() int {
	h := a.height - 2 - a.topPanelHeight()
	if h < 1 {
		h = 1
	}
	return h
}

// currentFileIdx returns the file index at the current cursor position.
func (a *App) currentFileIdx() int {
	if a.cursorLine < 0 || a.cursorLine >= len(a.annotations) {
		return -1
	}
	return a.annotations[a.cursorLine].FileIdx
}

// currentFilePath returns the display path of the file at the cursor.
func (a *App) currentFilePath() string {
	idx := a.currentFileIdx()
	if idx < 0 || idx >= len(a.diffFiles) {
		return ""
	}
	return a.diffFiles[idx].DisplayPath()
}

// setMessage sets a status message.
func (a *App) setMessage(text string, level messageLevel) {
	a.message = &statusMessage{text: text, level: level}
}

// clearMessage clears the status message.
func (a *App) clearMessage() {
	a.message = nil
}

// commitListItems returns the ordered list of entries for the commit list panel.
func (a *App) commitListItems() []commitListEntry {
	var items []commitListEntry
	if a.includesWorkTree {
		items = append(items, commitListEntry{key: worktreeKey, isWorkingTree: true})
	}
	for _, c := range a.reviewCommits {
		items = append(items, commitListEntry{key: c.ID, commit: c})
	}
	return items
}

// conversationPanelHeight returns the height of the conversation panel (0 if hidden).
func (a *App) conversationPanelHeight(contentHeight int) int {
	if !a.showConversation {
		return 0
	}
	h := contentHeight / 2
	if h < 3 {
		h = 3
	}
	return h
}

// topPanelHeight returns the height of the top panel (commit list or description, 0 if hidden).
func (a *App) topPanelHeight() int {
	if a.showDescription {
		h := a.descriptionLineCount() + 1 // content + separator
		if h > 12 {
			h = 12
		}
		if h < 3 {
			h = 3
		}
		return h
	}
	items := a.commitListItems()
	if len(items) == 0 {
		return 0
	}
	h := len(items) + 1 // items + separator
	if h > 8 {
		h = 8
	}
	return h
}

// commitListHeight returns the height of the commit list panel (0 if hidden).
func (a *App) commitListHeight() int {
	if a.showDescription {
		return 0
	}
	return a.topPanelHeight()
}

// descriptionLineCount returns the number of wrapped lines in the description.
func (a *App) descriptionLineCount() int {
	if a.session == nil || a.session.Description == "" {
		return 1
	}
	return len(strings.Split(a.session.Description, "\n"))
}

// mergeEnabledDiffs combines per-commit diffs for all enabled commits.
func (a *App) mergeEnabledDiffs() []model.DiffFile {
	type fileEntry struct {
		file  model.DiffFile
		order int
	}
	fileMap := make(map[string]*fileEntry)
	order := 0

	addDiffs := func(diffs []model.DiffFile) {
		for _, df := range diffs {
			path := df.DisplayPath()
			if entry, ok := fileMap[path]; ok {
				entry.file.Hunks = append(entry.file.Hunks, df.Hunks...)
			} else {
				fileCopy := df
				fileCopy.Hunks = make([]model.DiffHunk, len(df.Hunks))
				copy(fileCopy.Hunks, df.Hunks)
				fileMap[path] = &fileEntry{file: fileCopy, order: order}
				order++
			}
		}
	}

	// Process commits oldest-first (reviewCommits is newest-first)
	for i := len(a.reviewCommits) - 1; i >= 0; i-- {
		c := a.reviewCommits[i]
		if !a.enabledCommits[c.ID] {
			continue
		}
		addDiffs(a.commitDiffs[c.ID])
	}

	// Working tree last
	if a.enabledCommits[worktreeKey] {
		addDiffs(a.commitDiffs[worktreeKey])
	}

	entries := make([]*fileEntry, 0, len(fileMap))
	for _, e := range fileMap {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].order < entries[j].order
	})

	result := make([]model.DiffFile, len(entries))
	for i, e := range entries {
		result[i] = e.file
	}
	return result
}

// allCommitsEnabled returns true if every commit (and worktree, if present) is enabled.
func (a *App) allCommitsEnabled() bool {
	for _, c := range a.reviewCommits {
		if !a.enabledCommits[c.ID] {
			return false
		}
	}
	if _, ok := a.commitDiffs[worktreeKey]; ok && !a.enabledCommits[worktreeKey] {
		return false
	}
	return true
}

// rebuildFromCommits merges enabled diffs and rebuilds the view.
func (a *App) rebuildFromCommits() {
	if a.combinedDiffFiles != nil && a.allCommitsEnabled() {
		a.diffFiles = a.combinedDiffFiles
	} else {
		a.diffFiles = a.mergeEnabledDiffs()
	}
	if a.session != nil {
		for _, f := range a.diffFiles {
			a.session.GetOrCreateFileReview(f.DisplayPath(), f.Status)
		}
	}
	a.expandedGaps = make(map[gapID][]model.DiffLine)
	a.rebuildFileTree()
	a.rebuildAnnotations()
	if a.cursorLine >= len(a.annotations) {
		a.cursorLine = 0
		a.scrollOffset = 0
	}
}

// toggleCommitAtCursor toggles the commit at the commit list cursor.
func (a *App) toggleCommitAtCursor() {
	items := a.commitListItems()
	if a.commitCursor < 0 || a.commitCursor >= len(items) {
		return
	}
	key := items[a.commitCursor].key
	if a.enabledCommits[key] {
		delete(a.enabledCommits, key)
	} else {
		a.enabledCommits[key] = true
	}
	a.rebuildFromCommits()
}
