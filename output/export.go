// Package output provides export functionality for review sessions.
package output

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pgavlin/crtea/model"
)

// GenerateMarkdown produces structured markdown from the review session's comments.
func GenerateMarkdown(session *model.ReviewSession) string {
	if session.TotalComments() == 0 && session.OverallReview == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString("# Code Review Comments\n\n")

	// Overall review section
	if session.OverallReview != nil {
		b.WriteString("## Overall Review\n\n")
		b.WriteString(fmt.Sprintf("**Status:** %s\n\n", session.OverallReview.Status.String()))
		if session.OverallReview.Body != "" {
			b.WriteString(session.OverallReview.Body)
			b.WriteString("\n\n---\n\n")
		}
	}

	// Sort file paths for deterministic output
	var paths []string
	for path := range session.Files {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		fr := session.Files[path]
		if !fr.HasComments() {
			continue
		}

		b.WriteString(fmt.Sprintf("## %s\n\n", path))

		// File-level comments
		for _, c := range fr.FileComments {
			writeComment(&b, &c, path, 0, nil)
		}

		// Line comments (sorted by line number)
		var lineNos []int
		for lineNo := range fr.LineComments {
			lineNos = append(lineNos, lineNo)
		}
		sort.Ints(lineNos)

		for _, lineNo := range lineNos {
			for _, c := range fr.LineComments[lineNo] {
				writeComment(&b, &c, path, lineNo, c.LineRange)
			}
		}
	}

	return b.String()
}

func writeComment(b *strings.Builder, c *model.Comment, path string, lineNo int, lineRange *model.LineRange) {
	// Type badge
	b.WriteString(fmt.Sprintf("**%s**", strings.ToUpper(c.Type.String())))

	// Location
	if lineNo > 0 {
		if lineRange != nil {
			b.WriteString(fmt.Sprintf(" (`%s:%d-%d`)", path, lineRange.Start, lineRange.End))
		} else {
			b.WriteString(fmt.Sprintf(" (`%s:%d`)", path, lineNo))
		}
	}
	b.WriteString("\n\n")

	// Content
	b.WriteString(c.Content)
	b.WriteString("\n\n---\n\n")
}
