package ui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/charmbracelet/x/ansi"
	"github.com/pgavlin/crtea/model"
)

// annotationType categorizes what a rendered line represents.
type annotationType int

const (
	annFileHeader annotationType = iota
	annHunkHeader
	annDiffLine
	annFileComment
	annLineComment
	annExpander
	annExpandedContext
	annBinaryOrEmpty
	annSpacing
)

// annotatedLine describes what a single screen line maps to in the diff model.
type annotatedLine struct {
	Type         annotationType
	FileIdx      int
	HunkIdx      int
	LineIdx      int
	OldLineNo    int
	NewLineNo    int
	CommentIdx   int
	CommentLine  int // which display line within a comment (0 = header, 1+ = content lines)
	CommentLines int // total display lines for this comment
	Side         model.LineSide
	gapID        gapID // set for annExpander and annExpandedContext

	// Thread rendering hints
	IsReply       bool // this comment is a reply (has ReplyToID)
	HasReplyAfter bool // the next comment is a reply to this one's thread
}

// gapID identifies a gap between hunks for context expansion.
type gapID struct {
	FileIdx int
	HunkIdx int // gap is before this hunk
}

// wrapComment returns the wrapped display lines for a comment's content.
// The result does not include the header line.
func wrapComment(content string, wrapWidth int) []string {
	// Normalize \r\n (Windows/GitHub) and stray \r to \n before splitting.
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	if wrapWidth <= 0 {
		return strings.Split(content, "\n")
	}
	var result []string
	for _, paragraph := range strings.Split(content, "\n") {
		if lipgloss.Width(paragraph) <= wrapWidth {
			result = append(result, paragraph)
			continue
		}
		for lipgloss.Width(paragraph) > wrapWidth {
			// Find last space within visual wrapWidth
			truncated := ansi.Truncate(paragraph, wrapWidth, "")
			breakAt := strings.LastIndex(truncated, " ")
			if breakAt <= 0 {
				breakAt = len(truncated)
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

// buildAnnotations constructs the list of annotated lines from diff files, session, and expanded gaps.
// commentWrapWidth is the available width for comment text (0 means no wrapping).
func buildAnnotations(files []model.DiffFile, session *model.ReviewSession, expandedGaps map[gapID][]model.DiffLine, commentWrapWidth int) []annotatedLine {
	var annotations []annotatedLine

	for fi, file := range files {
		// File header
		annotations = append(annotations, annotatedLine{
			Type:    annFileHeader,
			FileIdx: fi,
		})

		// File-level comments
		if session != nil {
			if fr := session.GetFileReview(file.DisplayPath()); fr != nil {
				for ci := range fr.FileComments {
					total := commentDisplayLines(&fr.FileComments[ci], commentWrapWidth)
					for cl := range total {
						annotations = append(annotations, annotatedLine{
							Type:         annFileComment,
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
			annotations = append(annotations, annotatedLine{
				Type:    annBinaryOrEmpty,
				FileIdx: fi,
			})
			annotations = append(annotations, annotatedLine{Type: annSpacing})
			continue
		}

		for hi, hunk := range file.Hunks {
			// Gap between hunks
			if hi > 0 {
				gid := gapID{FileIdx: fi, HunkIdx: hi}
				if expanded, ok := expandedGaps[gid]; ok {
					// Show expanded context lines
					for li, line := range expanded {
						annotations = append(annotations, annotatedLine{
							Type:      annExpandedContext,
							FileIdx:   fi,
							HunkIdx:   hi,
							LineIdx:   li,
							OldLineNo: line.OldLineNo,
							NewLineNo: line.NewLineNo,
							gapID:     gid,
						})
					}
				} else {
					// Show collapsed expander
					annotations = append(annotations, annotatedLine{
						Type:    annExpander,
						FileIdx: fi,
						HunkIdx: hi,
						gapID:   gid,
					})
				}
			}

			// Hunk header
			annotations = append(annotations, annotatedLine{
				Type:    annHunkHeader,
				FileIdx: fi,
				HunkIdx: hi,
			})

			// Diff lines
			for li, line := range hunk.Lines {
				annotations = append(annotations, annotatedLine{
					Type:      annDiffLine,
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
								isReply := comments[ci].ReplyToID != ""
								hasReplyAfter := false
								for nci := ci + 1; nci < len(comments); nci++ {
									if comments[nci].Side == side {
										hasReplyAfter = comments[nci].ReplyToID != ""
										break
									}
								}
								total := commentDisplayLines(&comments[ci], commentWrapWidth)
								displayTotal := total
								if hasReplyAfter {
									displayTotal = total - 1 // skip bottom border; next reply continues the box
								}
								for cl := range displayTotal {
									annotations = append(annotations, annotatedLine{
										Type:          annLineComment,
										FileIdx:       fi,
										HunkIdx:       hi,
										LineIdx:       li,
										OldLineNo:     line.OldLineNo,
										NewLineNo:     line.NewLineNo,
										CommentIdx:    ci,
										CommentLine:   cl,
										CommentLines:  displayTotal,
										Side:          side,
										IsReply:       isReply,
										HasReplyAfter: hasReplyAfter,
									})
								}
							}
						}
					}
				}
			}
		}

		// Spacing between files
		annotations = append(annotations, annotatedLine{Type: annSpacing})
	}

	return annotations
}
