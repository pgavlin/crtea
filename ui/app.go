package ui

import (
	"fmt"
	"io"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/pgavlin/crtea/model"
	"github.com/pgavlin/crtea/output"
	"github.com/pgavlin/crtea/persistence"
	"github.com/pgavlin/crtea/syntax"
	"github.com/pgavlin/crtea/theme"
	"github.com/pgavlin/crtea/vcs"
)

// InputMode tracks what kind of input the app is currently processing.
type InputMode int

const (
	ModeNormal InputMode = iota
	ModeCommand
	ModeSearch
	ModeComment
	ModeHelp
	ModeConfirm
	ModeVisualSelect
)

// FocusedPanel tracks which panel has focus.
type FocusedPanel int

const (
	PanelDiff FocusedPanel = iota
	PanelFileList
	PanelCommitList
)

// MessageLevel indicates the severity of a status message.
type MessageLevel int

const (
	MessageInfo MessageLevel = iota
	MessageWarning
	MessageError
)

// StatusMessage holds a message to display in the status bar.
type StatusMessage struct {
	Text  string
	Level MessageLevel
}

// App is the main Bubble Tea model for the code review TUI.
type App struct {
	// Core data
	vcs         vcs.Backend
	vcsInfo     vcs.VcsInfo
	session     *model.ReviewSession
	diffFiles   []model.DiffFile
	highlighter *syntax.Highlighter

	// Phase
	phase          AppPhase
	pickerItems    []PickerItem
	pickerCursor   int
	pickerSelected map[int]bool

	// Commit list (review mode)
	reviewCommits    []vcs.CommitInfo
	commitDiffs      map[string][]model.DiffFile
	enabledCommits   map[string]bool
	includesWorkTree bool
	commitCursor     int

	// UI state
	inputMode    InputMode
	focusedPanel FocusedPanel
	showFileList bool

	// Window dimensions
	width  int
	height int

	// Diff view state
	annotations  []AnnotatedLine
	cursorLine   int
	scrollOffset int
	scrollX      int
	expandedGaps map[GapID][]model.DiffLine

	// File list state
	fileTree       *FileTreeNode
	fileTreeRows   []FileTreeRow
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

	// Messages
	message    *StatusMessage
	dirty      bool
	quitWarned bool

	// Pending key sequences
	pendingCount  string // digit accumulator for {N}G
	pendingPrefix rune   // for pending key sequences like d, z, ;

	// Persistence
	store persistence.Store

	// Theme
	theme theme.Theme
}

// NewApp creates a new App model.
func NewApp(backend vcs.Backend, files []model.DiffFile, session *model.ReviewSession, th theme.Theme, hl *syntax.Highlighter, store persistence.Store) App {
	app := App{
		vcs:           backend,
		vcsInfo:       backend.Info(),
		session:       session,
		diffFiles:     files,
		highlighter:   hl,
		inputMode:     ModeNormal,
		focusedPanel:  PanelDiff,
		showFileList:  true,
		expandedGaps:  make(map[GapID][]model.DiffLine),
		collapsedDirs: make(map[string]bool),
		store:         store,
		theme:         th,
	}
	app.rebuildFileTree()
	app.rebuildAnnotations()
	return app
}

// NewPickerApp creates an App that starts in the commit picker phase.
func NewPickerApp(backend vcs.Backend, th theme.Theme, hl *syntax.Highlighter, store persistence.Store) App {
	commits, _ := backend.GetRecentCommits(0, 30)

	var items []PickerItem
	items = append(items, PickerItem{IsWorkingTree: true})
	for _, c := range commits {
		items = append(items, PickerItem{Commit: c})
	}

	return App{
		vcs:            backend,
		vcsInfo:        backend.Info(),
		highlighter:    hl,
		store:          store,
		theme:          th,
		phase:          PhasePicker,
		pickerItems:    items,
		pickerSelected: make(map[int]bool),
		expandedGaps:   make(map[GapID][]model.DiffLine),
		collapsedDirs:  make(map[string]bool),
	}
}

func (a *App) rebuildFileTree() {
	a.fileTree = buildFileTree(a.diffFiles)
	a.fileTreeRows = flattenTree(a.fileTree, a.collapsedDirs)
}

func (a *App) rebuildAnnotations() {
	a.annotations = BuildAnnotations(a.diffFiles, a.session, a.expandedGaps, a.commentWrapWidth())
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

// Init implements tea.Model.
func (a App) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.rebuildAnnotations()
		return &a, nil

	case tea.KeyPressMsg:
		return a.handleKey(msg)
	}
	return &a, nil
}

// View implements tea.Model.
func (a App) View() tea.View {
	if a.width == 0 || a.height == 0 {
		return tea.NewView("")
	}

	if a.phase == PhasePicker {
		return tea.NewView(a.renderPicker())
	}

	var b strings.Builder

	// Header (1 line)
	b.WriteString(a.renderStatusBar())
	b.WriteString("\n")

	// Commit list panel
	contentHeight := a.height - 2 // header + footer
	clHeight := a.commitListHeight()
	if clHeight > 0 {
		b.WriteString(a.renderCommitList(a.width, clHeight))
		b.WriteString("\n")
		contentHeight -= clHeight
	}
	if contentHeight < 1 {
		contentHeight = 1
	}

	if a.inputMode == ModeHelp {
		b.WriteString(a.renderHelp(contentHeight))
	} else if a.showFileList {
		fileListWidth := a.fileListWidth()
		diffWidth := a.width - fileListWidth - 1 // -1 for separator
		fileList := a.renderFileList(fileListWidth, contentHeight)
		diffView := a.renderDiffView(diffWidth, contentHeight)
		b.WriteString(joinHorizontal(fileList, diffView, contentHeight))
	} else {
		b.WriteString(a.renderDiffView(a.width, contentHeight))
	}

	// Footer (1 line)
	b.WriteString("\n")
	b.WriteString(a.renderFooter())

	return tea.NewView(b.String())
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
	h := a.height - 2 - a.commitListHeight()
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
func (a *App) setMessage(text string, level MessageLevel) {
	a.message = &StatusMessage{Text: text, Level: level}
}

// clearMessage clears the status message.
func (a *App) clearMessage() {
	a.message = nil
}

// ExportMarkdown writes the session's comments as markdown to the given writer.
func (a *App) ExportMarkdown(w io.Writer) {
	if a.session != nil && a.session.TotalComments() > 0 {
		fmt.Fprint(w, output.GenerateMarkdown(a.session))
	}
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

// commitListHeight returns the height of the commit list panel (0 if hidden).
func (a *App) commitListHeight() int {
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

// rebuildFromCommits merges enabled diffs and rebuilds the view.
func (a *App) rebuildFromCommits() {
	a.diffFiles = a.mergeEnabledDiffs()
	if a.session != nil {
		for _, f := range a.diffFiles {
			a.session.GetOrCreateFileReview(f.DisplayPath(), f.Status)
		}
	}
	a.expandedGaps = make(map[GapID][]model.DiffLine)
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
