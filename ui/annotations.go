package ui

import "github.com/pgavlin/crtea/model"

// AnnotationType categorizes what a rendered line represents.
type AnnotationType int

const (
	AnnFileHeader AnnotationType = iota
	AnnHunkHeader
	AnnDiffLine
	AnnFileComment
	AnnLineComment
	AnnExpander
	AnnBinaryOrEmpty
	AnnSpacing
)

// AnnotatedLine describes what a single screen line maps to in the diff model.
type AnnotatedLine struct {
	Type       AnnotationType
	FileIdx    int
	HunkIdx    int
	LineIdx    int
	OldLineNo  int
	NewLineNo  int
	CommentIdx int
	Side       model.LineSide
}

// GapID identifies a gap between hunks for context expansion.
type GapID struct {
	FileIdx int
	HunkIdx int // gap is before this hunk
}

// BuildAnnotations constructs the list of annotated lines from diff files and session.
func BuildAnnotations(files []model.DiffFile, session *model.ReviewSession) []AnnotatedLine {
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
					annotations = append(annotations, AnnotatedLine{
						Type:       AnnFileComment,
						FileIdx:    fi,
						CommentIdx: ci,
					})
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
			// Expander between hunks (gap)
			if hi > 0 {
				annotations = append(annotations, AnnotatedLine{
					Type:    AnnExpander,
					FileIdx: fi,
					HunkIdx: hi,
				})
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
								annotations = append(annotations, AnnotatedLine{
									Type:       AnnLineComment,
									FileIdx:    fi,
									LineIdx:    li,
									CommentIdx: ci,
									Side:       side,
								})
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
