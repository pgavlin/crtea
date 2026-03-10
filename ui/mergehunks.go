package ui

import (
	"fmt"
	"sort"

	"github.com/pgavlin/crtea/model"
)

type composedLine struct {
	origin    model.LineOrigin
	content   string
	oldLineNo int // in O (original)
	newLineNo int // in B (final)
	spans     []model.StyledSpan
	sortKey   int // for ordering
}

// composeHunkSets composes two sets of hunks: aHunks (O→A) and bHunks (A→B),
// producing hunks representing (O→B). Both inputs must be sorted by their
// respective start positions.
func composeHunkSets(aHunks, bHunks []model.DiffHunk) []model.DiffHunk {
	if len(aHunks) == 0 {
		return bHunks
	}
	if len(bHunks) == 0 {
		return aHunks
	}

	// Build maps of what A does, keyed by intermediate (A-state) line numbers.
	// For each line in A's output (NewLineNo), record where it came from.
	type aLineInfo struct {
		origin    model.LineOrigin // in A's diff
		content   string
		oldLineNo int // line in O (original)
		newLineNo int // line in A (intermediate)
		spans     []model.StyledSpan
	}

	// Map from A-state line number to its info
	aNewLines := map[int]aLineInfo{}
	// A's deletions (lines in O that don't exist in A), keyed by index for removal
	var aDeletions []aLineInfo
	aDelConsumed := map[int]bool{}

	for _, h := range aHunks {
		for _, line := range h.Lines {
			switch line.Origin {
			case model.OriginContext:
				aNewLines[line.NewLineNo] = aLineInfo{
					origin:    model.OriginContext,
					content:   line.Content,
					oldLineNo: line.OldLineNo,
					newLineNo: line.NewLineNo,
					spans:     line.Spans,
				}
			case model.OriginAddition:
				aNewLines[line.NewLineNo] = aLineInfo{
					origin:    model.OriginAddition,
					content:   line.Content,
					oldLineNo: 0,
					newLineNo: line.NewLineNo,
					spans:     line.Spans,
				}
			case model.OriginDeletion:
				aDeletions = append(aDeletions, aLineInfo{
					origin:    model.OriginDeletion,
					content:   line.Content,
					oldLineNo: line.OldLineNo,
					spans:     line.Spans,
				})
			}
		}
	}

	// Now process B's hunks. For each line in B:
	// - B's deletions (OldLineNo in A-state): look up in aNewLines
	// - B's additions (NewLineNo in B-state): new additions
	// - B's context (both line numbers): look up in aNewLines

	var result []composedLine
	consumed := map[int]bool{} // A-state line numbers consumed by B

	for _, h := range bHunks {
		for _, line := range h.Lines {
			switch line.Origin {
			case model.OriginContext:
				// This line exists in both A and B states
				consumed[line.OldLineNo] = true
				if aInfo, ok := aNewLines[line.OldLineNo]; ok {
					if aInfo.origin == model.OriginAddition {
						// Added by A, kept by B → addition in O→B
						result = append(result, composedLine{
							origin:    model.OriginAddition,
							content:   line.Content,
							newLineNo: line.NewLineNo,
							spans:     line.Spans,
							sortKey:   line.NewLineNo,
						})
					} else {
						// Context in A, context in B → context in O→B
						result = append(result, composedLine{
							origin:    model.OriginContext,
							content:   line.Content,
							oldLineNo: aInfo.oldLineNo,
							newLineNo: line.NewLineNo,
							spans:     line.Spans,
							sortKey:   line.NewLineNo,
						})
					}
				} else {
					// Not in A's hunks = unchanged line, context in O→B
					result = append(result, composedLine{
						origin:    model.OriginContext,
						content:   line.Content,
						oldLineNo: line.OldLineNo, // approximate
						newLineNo: line.NewLineNo,
						spans:     line.Spans,
						sortKey:   line.NewLineNo,
					})
				}

			case model.OriginDeletion:
				// B deletes this line from A-state
				consumed[line.OldLineNo] = true
				if aInfo, ok := aNewLines[line.OldLineNo]; ok {
					if aInfo.origin == model.OriginAddition {
						// Added by A, deleted by B → cancels out, omit
					} else {
						// Context in A (exists in O), deleted by B → deletion in O→B
						result = append(result, composedLine{
							origin:    model.OriginDeletion,
							content:   aInfo.content,
							oldLineNo: aInfo.oldLineNo,
							spans:     aInfo.spans,
							sortKey:   line.OldLineNo,
						})
					}
				} else {
					// Not in A's hunks = existed in O, deleted by B → deletion in O→B
					result = append(result, composedLine{
						origin:    model.OriginDeletion,
						content:   line.Content,
						oldLineNo: line.OldLineNo,
						spans:     line.Spans,
						sortKey:   line.OldLineNo,
					})
				}

			case model.OriginAddition:
				// B adds this line. Check if it restores a line that A deleted
				// (same content). If so, the change is a no-op → context line.
				restored := false
				for i, d := range aDeletions {
					if !aDelConsumed[i] && d.content == line.Content {
						aDelConsumed[i] = true
						result = append(result, composedLine{
							origin:    model.OriginContext,
							content:   line.Content,
							oldLineNo: d.oldLineNo,
							newLineNo: line.NewLineNo,
							spans:     line.Spans,
							sortKey:   line.NewLineNo,
						})
						restored = true
						break
					}
				}
				if !restored {
					// Genuine addition in O→B
					result = append(result, composedLine{
						origin:    model.OriginAddition,
						content:   line.Content,
						newLineNo: line.NewLineNo,
						spans:     line.Spans,
						sortKey:   line.NewLineNo,
					})
				}
			}
		}
	}

	// Add A's deletions (lines deleted by A that B doesn't re-add)
	for i, d := range aDeletions {
		if aDelConsumed[i] {
			continue
		}
		result = append(result, composedLine{
			origin:    model.OriginDeletion,
			content:   d.content,
			oldLineNo: d.oldLineNo,
			spans:     d.spans,
			sortKey:   d.oldLineNo,
		})
	}

	// Add A's changes not consumed by B (lines A changed but B's hunks don't cover)
	for lineNo, aInfo := range aNewLines {
		if consumed[lineNo] {
			continue
		}
		switch aInfo.origin {
		case model.OriginAddition:
			// A added it, B doesn't touch it → addition in O→B
			// We need to figure out the NewLineNo in B-state.
			// Since B doesn't touch it, we need to estimate the B-state line number.
			// Use the A-state line number adjusted by B's net offset at that point.
			bNewLineNo := adjustLineForB(lineNo, bHunks)
			result = append(result, composedLine{
				origin:    model.OriginAddition,
				content:   aInfo.content,
				newLineNo: bNewLineNo,
				spans:     aInfo.spans,
				sortKey:   bNewLineNo,
			})
		case model.OriginContext:
			// Context in A, not in B's hunks → context in O→B
			bNewLineNo := adjustLineForB(lineNo, bHunks)
			result = append(result, composedLine{
				origin:    model.OriginContext,
				content:   aInfo.content,
				oldLineNo: aInfo.oldLineNo,
				newLineNo: bNewLineNo,
				spans:     aInfo.spans,
				sortKey:   bNewLineNo,
			})
		}
	}

	// Sort by position; deletions before additions at same position
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].sortKey != result[j].sortKey {
			return result[i].sortKey < result[j].sortKey
		}
		// At same position: deletions < context < additions
		return result[i].origin > result[j].origin
	})

	return groupIntoHunks(result)
}

// adjustLineForB converts an A-state line number to a B-state line number
// by accounting for B's insertions and deletions before that point.
func adjustLineForB(aLineNo int, bHunks []model.DiffHunk) int {
	offset := 0
	for _, h := range bHunks {
		if h.OldStart > aLineNo {
			break
		}
		hunkEnd := h.OldStart + h.OldCount
		if aLineNo >= hunkEnd {
			// This hunk is entirely before our line
			offset += h.NewCount - h.OldCount
		}
		// If our line is inside this hunk, the offset is partial but
		// since we know B doesn't touch this line, it must be between hunks.
	}
	return aLineNo + offset
}

// groupIntoHunks converts a sorted list of composed lines into DiffHunks.
// Adjacent lines are grouped together; gaps create separate hunks.
func groupIntoHunks(lines []composedLine) []model.DiffHunk {
	if len(lines) == 0 {
		return nil
	}

	var hunks []model.DiffHunk
	var currentLines []model.DiffLine
	var hunkOldStart, hunkNewStart int
	lastOld, lastNew := 0, 0
	started := false

	flush := func() {
		if len(currentLines) == 0 {
			return
		}
		oldCount := 0
		newCount := 0
		for _, l := range currentLines {
			switch l.Origin {
			case model.OriginContext:
				oldCount++
				newCount++
			case model.OriginDeletion:
				oldCount++
			case model.OriginAddition:
				newCount++
			}
		}
		hunks = append(hunks, model.DiffHunk{
			Header:   fmt.Sprintf("@@ -%d,%d +%d,%d @@", hunkOldStart, oldCount, hunkNewStart, newCount),
			OldStart: hunkOldStart,
			OldCount: oldCount,
			NewStart: hunkNewStart,
			NewCount: newCount,
			Lines:    currentLines,
		})
		currentLines = nil
		started = false
	}

	for _, cl := range lines {
		effectiveOld := cl.oldLineNo
		effectiveNew := cl.newLineNo
		if effectiveOld == 0 {
			effectiveOld = lastOld
		}
		if effectiveNew == 0 {
			effectiveNew = lastNew
		}

		// Start a new hunk if there's a gap (more than 1 line between consecutive lines).
		// For deletions (no NewLineNo) only check old gap; for additions (no OldLineNo) only check new gap.
		// For context lines, check both.
		if started {
			gapOld := effectiveOld - lastOld
			gapNew := effectiveNew - lastNew
			hasGap := false
			switch cl.origin {
			case model.OriginDeletion:
				hasGap = cl.oldLineNo > 0 && lastOld > 0 && gapOld > 1
			case model.OriginAddition:
				hasGap = cl.newLineNo > 0 && lastNew > 0 && gapNew > 1
			default:
				hasGap = (cl.oldLineNo > 0 && lastOld > 0 && gapOld > 1) &&
					(cl.newLineNo > 0 && lastNew > 0 && gapNew > 1)
			}
			if hasGap {
				flush()
			}
		}

		if !started {
			started = true
			hunkOldStart = 0
			hunkNewStart = 0
		}
		if hunkOldStart == 0 && cl.oldLineNo > 0 {
			hunkOldStart = cl.oldLineNo
		}
		if hunkNewStart == 0 && cl.newLineNo > 0 {
			hunkNewStart = cl.newLineNo
		}

		currentLines = append(currentLines, model.DiffLine{
			Origin:    cl.origin,
			Content:   cl.content,
			OldLineNo: cl.oldLineNo,
			NewLineNo: cl.newLineNo,
			Spans:     cl.spans,
		})

		if effectiveOld > 0 {
			lastOld = effectiveOld
		}
		if effectiveNew > 0 {
			lastNew = effectiveNew
		}
	}
	flush()
	return hunks
}

// mergeFileHunks merges hunks from multiple commits (oldest-first) for a single file.
// Each element of commitHunks is one commit's hunks for this file.
func mergeFileHunks(commitHunks [][]model.DiffHunk) []model.DiffHunk {
	if len(commitHunks) == 0 {
		return nil
	}
	accumulated := commitHunks[0]
	for i := 1; i < len(commitHunks); i++ {
		accumulated = composeHunkSets(accumulated, commitHunks[i])
	}
	return accumulated
}
