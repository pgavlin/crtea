package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/pgavlin/crtea/model"
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

	// Theme
	theme theme.Theme
}

// NewApp creates a new App model.
func NewApp(backend vcs.Backend, files []model.DiffFile, session *model.ReviewSession, th theme.Theme, hl *syntax.Highlighter) App {
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
		theme:         th,
	}
	app.rebuildFileTree()
	app.rebuildAnnotations()
	return app
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

	var b strings.Builder

	// Header (1 line)
	b.WriteString(a.renderStatusBar())
	b.WriteString("\n")

	// Main content area
	contentHeight := a.height - 2 // header + footer
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
	w := a.width / 4
	if w < 20 {
		w = 20
	}
	if w > 50 {
		w = 50
	}
	return w
}

func (a *App) diffViewportHeight() int {
	h := a.height - 2
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
