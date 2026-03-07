// Package syntax provides syntax highlighting for diff content.
package syntax

import (
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"

	"github.com/pgavlin/crtea/model"
)

// Highlighter applies syntax highlighting to diff files.
type Highlighter struct {
	style *chroma.Style
}

// NewHighlighter creates a new highlighter with the given chroma style name.
// Use "monokai" for dark themes, "github" for light themes.
func NewHighlighter(styleName string) *Highlighter {
	style := styles.Get(styleName)
	if style == nil {
		style = styles.Fallback
	}
	return &Highlighter{style: style}
}

// HighlightFile applies syntax highlighting to all lines in a diff file.
func (h *Highlighter) HighlightFile(file *model.DiffFile) {
	path := file.DisplayPath()
	lexer := lexers.Match(path)
	if lexer == nil {
		return
	}
	lexer = chroma.Coalesce(lexer)

	// Collect all line contents for this file, grouped by hunk
	for hi := range file.Hunks {
		hunk := &file.Hunks[hi]
		for li := range hunk.Lines {
			line := &hunk.Lines[li]
			spans := h.highlightLine(lexer, line.Content)
			if spans != nil {
				line.Spans = spans
			}
		}
	}
}

// HighlightFiles applies syntax highlighting to all files.
func (h *Highlighter) HighlightFiles(files []model.DiffFile) {
	for i := range files {
		h.HighlightFile(&files[i])
	}
}

func (h *Highlighter) highlightLine(lexer chroma.Lexer, content string) []model.StyledSpan {
	iterator, err := lexer.Tokenise(nil, content+"\n")
	if err != nil {
		return nil
	}

	var spans []model.StyledSpan
	for _, token := range iterator.Tokens() {
		text := token.Value
		// Strip trailing newline we added
		text = strings.TrimSuffix(text, "\n")
		if text == "" {
			continue
		}

		fg := ""
		entry := h.style.Get(token.Type)
		if entry.Colour.IsSet() {
			fg = fmt.Sprintf("#%06x", int(entry.Colour)&0xFFFFFF)
		}

		spans = append(spans, model.StyledSpan{
			Text: text,
			FG:   fg,
		})
	}

	return spans
}
