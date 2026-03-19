// Package ui provides an embeddable Bubble Tea component for interactive code review.
package ui

import (
	"log/slog"
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
	modeEditPR
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
// Err is set if the component encountered a fatal error (e.g. provider loading failed).
type DoneMsg struct {
	Session *model.ReviewSession
	Err     error
}

// ClipboardMsg is emitted when the component wants to copy text to the clipboard.
type ClipboardMsg struct {
	Content string
}

// App is the main Bubble Tea model for the code review TUI.
type App struct {
	// Core data
	log               *slog.Logger
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
	visualAnchorFile int // file index where visual selection started
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

	// Loading state (phaseLoading)
	loadingStatus string // current loading step description
	spinnerFrame  int    // index into spinner characters
}

// Option configures an App during construction.
type Option func(*App)

// WithLogger sets the logger. Defaults to slog.Default().
func WithLogger(log *slog.Logger) Option {
	return func(a *App) { a.log = log }
}

// WithHighlighter sets the syntax highlighter.
func WithHighlighter(hl *syntax.Highlighter) Option {
	return func(a *App) { a.highlighter = hl }
}

// WithStore sets the persistence store.
func WithStore(store persistence.Store) Option {
	return func(a *App) { a.store = store }
}

// WithProvider sets the remote provider and review request ID.
func WithProvider(p provider.Provider, id string) Option {
	return func(a *App) {
		a.provider = p
		a.providerID = id
	}
}

// WithCommits populates the commit list and per-commit diffs.
// commits should be newest-first. All commits are enabled by default.
// NOTE: This option must be applied after the App's diffFiles are set
// (i.e. it is intended for use with NewApp, not NewAppWithCommits).
func WithCommits(commits []vcs.CommitInfo, diffs map[string][]model.DiffFile) Option {
	return func(a *App) {
		a.reviewCommits = commits
		a.commitDiffs = diffs
		a.enabledCommits = make(map[string]bool, len(commits))
		for _, c := range commits {
			a.enabledCommits[c.ID] = true
		}
		a.combinedDiffFiles = a.diffFiles
	}
}

// NewApp creates a new App model.
func NewApp(backend vcs.Backend, files []model.DiffFile, session *model.ReviewSession, th theme.Theme, opts ...Option) App {
	var vcsInfo vcs.VcsInfo
	if backend != nil {
		vcsInfo = backend.Info()
	}
	app := App{
		log:           slog.Default(),
		vcs:           backend,
		vcsInfo:       vcsInfo,
		session:       session,
		diffFiles:     files,
		inputMode:     modeNormal,
		focusedPanel:  panelDiff,
		showFileList:  true,
		expandedGaps:  make(map[gapID][]model.DiffLine),
		collapsedDirs: make(map[string]bool),
		theme:         th,
	}
	for _, opt := range opts {
		opt(&app)
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
func NewAppWithCommits(backend vcs.Backend, commits []vcs.CommitInfo, commitDiffs map[string][]model.DiffFile, enabledCommits map[string]bool, includesWorkTree bool, session *model.ReviewSession, th theme.Theme, opts ...Option) App {
	app := App{
		log:              slog.Default(),
		vcs:              backend,
		vcsInfo:          backend.Info(),
		session:          session,
		inputMode:        modeNormal,
		focusedPanel:     panelDiff,
		showFileList:     true,
		expandedGaps:     make(map[gapID][]model.DiffLine),
		collapsedDirs:    make(map[string]bool),
		theme:            th,
		reviewCommits:    commits,
		commitDiffs:      commitDiffs,
		enabledCommits:   enabledCommits,
		includesWorkTree: includesWorkTree,
	}
	for _, opt := range opts {
		opt(&app)
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
func NewPickerApp(backend vcs.Backend, th theme.Theme, opts ...Option) App {
	commits, _ := backend.GetRecentCommits(0, 30)

	var items []pickerItem
	items = append(items, pickerItem{isWorkingTree: true})
	for _, c := range commits {
		items = append(items, pickerItem{commit: c})
	}

	app := App{
		log:            slog.Default(),
		vcs:            backend,
		vcsInfo:        backend.Info(),
		theme:          th,
		phase:          phasePicker,
		pickerItems:    items,
		pickerSelected: make(map[int]bool),
		expandedGaps:   make(map[gapID][]model.DiffLine),
		collapsedDirs:  make(map[string]bool),
	}
	for _, opt := range opts {
		opt(&app)
	}
	return app
}

func (a *App) rebuildFileTree() {
	a.fileTree = buildFileTree(a.diffFiles)
	a.fileTreeRows = flattenTree(a.fileTree, a.collapsedDirs)
	if a.fileListCursor >= len(a.fileTreeRows) {
		a.fileListCursor = len(a.fileTreeRows) - 1
	}
	if a.fileListCursor < 0 {
		a.fileListCursor = 0
	}
	a.fileListScroll = 0
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

// setCommits populates the commit list and per-commit diffs for the review.
// commits should be newest-first. All commits are enabled by default.
func (a *App) setCommits(commits []vcs.CommitInfo, diffs map[string][]model.DiffFile) {
	a.reviewCommits = commits
	a.commitDiffs = diffs
	a.enabledCommits = make(map[string]bool, len(commits))
	for _, c := range commits {
		a.enabledCommits[c.ID] = true
	}
	// Save the current combined diff (from provider) for use when all commits are enabled.
	a.combinedDiffFiles = a.diffFiles
	// Don't rebuild — all commits are enabled so the combined diff is already correct.
}

// Init implements tea.Model.
func (a App) Init() tea.Cmd {
	if a.phase == phaseLoading {
		return tea.Batch(spinnerTick(), a.loadProviderCmd())
	}
	return nil
}

// SetSize sets the component's dimensions. Call this from the parent model
// when the available size changes (e.g. in response to tea.WindowSizeMsg).
func (a *App) SetSize(width, height int) {
	a.width = width
	a.height = height
	a.rebuildAnnotations()
}

// Update handles a tea.Msg and returns the updated App and any command.
func (a App) Update(msg tea.Msg) (App, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if a.phase == phaseLoading {
			if msg.Key().Code == 'q' || msg.Key().Code == tea.KeyEscape {
				return a, func() tea.Msg { return DoneMsg{} }
			}
			return a, nil
		}
		return a.handleKey(msg)
	case spinnerTickMsg:
		if a.phase == phaseLoading {
			a.spinnerFrame++
			return a, spinnerTick()
		}
	case providerLoadedMsg:
		cmd := a.handleProviderLoaded(msg)
		return a, cmd
	}
	return a, nil
}

// View implements tea.Model.
func (a App) View() tea.View {
	v := a.view()
	v.AltScreen = true
	return v
}

func (a App) view() tea.View {
	if a.width == 0 || a.height == 0 {
		return tea.NewView("")
	}

	if a.phase == phaseLoading {
		return tea.NewView(a.renderLoading())
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

	// Editor panes steal height from content area
	var editorPane string
	if a.inputMode == modeBug {
		editorHeight := 8
		if editorHeight > contentHeight-2 {
			editorHeight = contentHeight - 2
		}
		if editorHeight < 3 {
			editorHeight = 3
		}
		contentHeight -= editorHeight + 1
		editorPane = a.renderBugEditor(a.width, editorHeight)
	} else if a.inputMode == modeReview {
		editorHeight := 8
		if editorHeight > contentHeight-2 {
			editorHeight = contentHeight - 2
		}
		if editorHeight < 3 {
			editorHeight = 3
		}
		contentHeight -= editorHeight + 1
		editorPane = a.renderReviewEditor(a.width, editorHeight)
	} else if a.inputMode == modeEditPR {
		editorHeight := 12
		if editorHeight > contentHeight-2 {
			editorHeight = contentHeight - 2
		}
		if editorHeight < 3 {
			editorHeight = 3
		}
		contentHeight -= editorHeight + 1
		editorPane = a.renderPREditor(a.width, editorHeight)
	}

	// Main content area
	if a.inputMode == modeHelp {
		b.WriteString(a.renderHelp(contentHeight))
	} else {
		b.WriteString(a.renderContentArea(contentHeight))
	}

	if editorPane != "" {
		b.WriteString("\n")
		b.WriteString(editorPane)
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

// renderContentArea renders the diff view, optionally with the file list sidebar.
func (a *App) renderContentArea(height int) string {
	if a.showFileList {
		fileListWidth := a.fileListWidth()
		diffWidth := a.width - fileListWidth - 1
		if diffWidth < 1 {
			diffWidth = 1
		}
		fileList := a.renderFileList(fileListWidth, height)
		diffView := a.renderDiffView(diffWidth, height)
		return joinHorizontal(fileList, diffView, height)
	}
	return a.renderDiffView(a.width, height)
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

// markDirty marks the session as having unsaved changes and resets the quit warning.
func (a *App) markDirty() {
	a.dirty = true
	a.quitWarned = false
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

	// Add space for commit message body of the cursored commit
	wrapWidth := a.width - 6 // indent + padding
	if wrapWidth < 10 {
		wrapWidth = 10
	}
	bodyLines := a.currentCommitBodyLines(wrapWidth)
	if len(bodyLines) > 0 {
		h += 1 + len(bodyLines) // blank line + body lines
	}

	if h > 12 {
		h = 12
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

// hasTopPanel returns true if the top panel (commit list or description) is visible.
func (a *App) hasTopPanel() bool {
	return a.topPanelHeight() > 0
}

// currentCommitBodyLines returns the wrapped body lines for the commit at the cursor.
func (a *App) currentCommitBodyLines(wrapWidth int) []string {
	items := a.commitListItems()
	if a.commitCursor < 0 || a.commitCursor >= len(items) {
		return nil
	}
	item := items[a.commitCursor]
	if item.isWorkingTree || item.commit.Body == "" {
		return nil
	}
	return wrapText(strings.TrimSpace(item.commit.Body), wrapWidth)
}

// descriptionLineCount returns the number of wrapped lines in the description.
func (a *App) descriptionLineCount() int {
	if a.session == nil || a.session.Description == "" {
		return 1
	}
	wrapWidth := a.width - 2 // indent
	if wrapWidth < 10 {
		wrapWidth = 10
	}
	return len(wrapText(a.session.Description, wrapWidth))
}

// mergeEnabledDiffs combines per-commit diffs for all enabled commits.
// When multiple commits modify the same file, their hunks are composed
// (rather than naively appended) so that overlapping changes merge correctly.
func (a *App) mergeEnabledDiffs() []model.DiffFile {
	type fileEntry struct {
		file        model.DiffFile
		commitHunks [][]model.DiffHunk // per-commit hunks, oldest-first
		order       int
	}
	fileMap := make(map[string]*fileEntry)
	order := 0

	addDiffs := func(diffs []model.DiffFile) {
		for _, df := range diffs {
			path := df.DisplayPath()
			hunksCopy := make([]model.DiffHunk, len(df.Hunks))
			copy(hunksCopy, df.Hunks)
			if entry, ok := fileMap[path]; ok {
				entry.commitHunks = append(entry.commitHunks, hunksCopy)
			} else {
				fileCopy := df
				fileCopy.Hunks = nil // will be set after composition
				fileMap[path] = &fileEntry{
					file:        fileCopy,
					commitHunks: [][]model.DiffHunk{hunksCopy},
					order:       order,
				}
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
		e.file.Hunks = mergeFileHunks(e.commitHunks)
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
