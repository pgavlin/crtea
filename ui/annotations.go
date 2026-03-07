package ui

import (
	"strings"

	"github.com/pgavlin/crtea/model"
)

// AnnotationType categorizes what a rendered line represents.
type AnnotationType int

const (
	AnnFileHeader AnnotationType = iota
	AnnHunkHeader
	AnnDiffLine
	AnnFileComment
	AnnLineComment
	AnnExpander
	AnnExpandedContext
	AnnBinaryOrEmpty
	AnnSpacing
)

// AnnotatedLine describes what a single screen line maps to in the diff model.
type AnnotatedLine struct {
	Type        AnnotationType
	FileIdx     int
	HunkIdx     int
	LineIdx     int
	OldLineNo   int
	NewLineNo   int
	CommentIdx  int
	CommentLine  int // which display line within a comment (0 = header, 1+ = content lines)
	CommentLines int // total display lines for this comment
	Side        model.LineSide
	GapID       GapID // set for AnnExpander and AnnExpandedContext
}

// GapID identifies a gap between hunks for context expansion.
type GapID struct {
	FileIdx int
	HunkIdx int // gap is before this hunk
}

// wrapComment returns the wrapped display lines for a comment's content.
// The result does not include the header line.
func wrapComment(content string, wrapWidth int) []string {
	if wrapWidth <= 0 {
		return strings.Split(content, "\n")
	}
	var result []string
	for _, paragraph := range strings.Split(content, "\n") {
		if len(paragraph) <= wrapWidth {
			result = append(result, paragraph)
			continue
		}
		for len(paragraph) > wrapWidth {
			// Find last space before wrapWidth
			breakAt := wrapWidth
			if idx := strings.LastIndex(paragraph[:wrapWidth], " "); idx > 0 {
				breakAt = idx
			}
			result = append(result, paragraph[:breakAt])
			paragraph = strings.TrimLeft(paragraph[breakAt:], " ")
		}
		if len(paragraph) > 0 {
			result = append(result, paragraph)
		}
	}
	if len(result) == 0 {
		result = []string{""}
	}
	return result
}

// commentDisplayLines returns the number of display lines for a comment:
// top border + N wrapped content lines + bottom border.
func commentDisplayLines(c *model.Comment, wrapWidth int) int {
	return 2 + len(wrapComment(c.Content, wrapWidth)) // top + content + bottom
}

// BuildAnnotations constructs the list of annotated lines from diff files, session, and expanded gaps.
// commentWrapWidth is the available width for comment text (0 means no wrapping).
func BuildAnnotations(files []model.DiffFile, session *model.ReviewSession, expandedGaps map[GapID][]model.DiffLine, commentWrapWidth int) []AnnotatedLine {
	var annotations []AnnotatedLine

	for fi, file := range files {
		// File header
		annotations = append(annotations, AnnotatedLine{
			Type:    AnnFileHeader,
			FileIdx: fi,
		})

		// File-level comments
		if session != nil {
			if fr := session.GetFileReview(file.DisplayPath()); fr != nil {
				for ci := range fr.FileComments {
					total := commentDisplayLines(&fr.FileComments[ci], commentWrapWidth)
					for cl := range total {
						annotations = append(annotations, AnnotatedLine{
							Type:         AnnFileComment,
							FileIdx:      fi,
							CommentIdx:   ci,
							CommentLine:  cl,
							CommentLines: total,
						})
					}
				}
			}
		}

		if file.IsBinary {
			annotations = append(annotations, AnnotatedLine{
				Type:    AnnBinaryOrEmpty,
				FileIdx: fi,
			})
			annotations = append(annotations, AnnotatedLine{Type: AnnSpacing})
			continue
		}

		for hi, hunk := range file.Hunks {
			// Gap between hunks
			if hi > 0 {
				gid := GapID{FileIdx: fi, HunkIdx: hi}
				if expanded, ok := expandedGaps[gid]; ok {
					// Show expanded context lines
					for li, line := range expanded {
						annotations = append(annotations, AnnotatedLine{
							Type:      AnnExpandedContext,
							FileIdx:   fi,
							HunkIdx:   hi,
							LineIdx:   li,
							OldLineNo: line.OldLineNo,
							NewLineNo: line.NewLineNo,
							GapID:     gid,
						})
					}
				} else {
					// Show collapsed expander
					annotations = append(annotations, AnnotatedLine{
						Type:    AnnExpander,
						FileIdx: fi,
						HunkIdx: hi,
						GapID:   gid,
					})
				}
			}

			// Hunk header
			annotations = append(annotations, AnnotatedLine{
				Type:    AnnHunkHeader,
				FileIdx: fi,
				HunkIdx: hi,
			})

			// Diff lines
			for li, line := range hunk.Lines {
				annotations = append(annotations, AnnotatedLine{
					Type:      AnnDiffLine,
					FileIdx:   fi,
					HunkIdx:   hi,
					LineIdx:   li,
					OldLineNo: line.OldLineNo,
					NewLineNo: line.NewLineNo,
				})

				// Line comments
				if session != nil {
					if fr := session.GetFileReview(file.DisplayPath()); fr != nil {
						lineNo := line.NewLineNo
						side := model.SideNew
						if line.Origin == model.OriginDeletion {
							lineNo = line.OldLineNo
							side = model.SideOld
						}
						if comments, ok := fr.LineComments[lineNo]; ok && lineNo > 0 {
							for ci := range comments {
								if comments[ci].Side != side {
									continue
								}
								total := commentDisplayLines(&comments[ci], commentWrapWidth)
								for cl := range total {
									annotations = append(annotations, AnnotatedLine{
										Type:         AnnLineComment,
										FileIdx:      fi,
										HunkIdx:      hi,
										LineIdx:      li,
										OldLineNo:    line.OldLineNo,
										NewLineNo:    line.NewLineNo,
										CommentIdx:   ci,
										CommentLine:  cl,
										CommentLines: total,
										Side:         side,
									})
								}
							}
						}
					}
				}
			}
		}

		// Spacing between files
		annotations = append(annotations, AnnotatedLine{Type: AnnSpacing})
	}

	return annotations
}
