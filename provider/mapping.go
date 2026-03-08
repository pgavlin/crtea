package provider

import (
	"github.com/pgavlin/crtea/model"
)

// ImportComments converts provider comments into model comments,
// grouped by file path. Returns a map of path -> line -> comments.
func ImportComments(comments []Comment) map[string]map[int][]model.Comment {
	result := make(map[string]map[int][]model.Comment)
	for _, c := range comments {
		mc := model.Comment{
			ID:         c.ExternalID,
			Content:    c.Body,
			Type:       model.CommentNote,
			CreatedAt:  c.CreatedAt,
			Author:     c.Author,
			ExternalID: c.ExternalID,
			ReplyToID:  c.ReplyToID,
			IsOutdated: c.IsOutdated,
		}
		switch c.Side {
		case "old":
			mc.Side = model.SideOld
		default:
			mc.Side = model.SideNew
		}
		if c.StartLine > 0 {
			mc.LineRange = &model.LineRange{Start: c.StartLine, End: c.Line}
		}

		if result[c.Path] == nil {
			result[c.Path] = make(map[int][]model.Comment)
		}
		result[c.Path][c.Line] = append(result[c.Path][c.Line], mc)
	}
	return result
}

// ImportReview converts a provider review into a model OverallReview.
func ImportReview(review Review) model.OverallReview {
	var status model.ApprovalStatus
	switch review.State {
	case ReviewApprove:
		status = model.ApprovalApprove
	case ReviewRequestChanges:
		status = model.ApprovalRequestChanges
	default:
		status = model.ApprovalNeutral
	}
	return model.OverallReview{
		Body:       review.Body,
		Status:     status,
		Author:     review.Author,
		ExternalID: review.ExternalID,
	}
}

// ImportConversation converts provider conversation comments into model types.
func ImportConversation(comments []ConversationComment) []model.ConversationComment {
	result := make([]model.ConversationComment, len(comments))
	for i, c := range comments {
		result[i] = model.ConversationComment{
			ExternalID: c.ExternalID,
			Author:     c.Author,
			Body:       c.Body,
			CreatedAt:  c.CreatedAt,
		}
	}
	return result
}

// MergeImportedComments adds remote comments to a file review, deduplicating
// against existing comments by ExternalID. It also replaces submitted local
// comments that match a remote comment (same line, side, author, content),
// which happens when a locally-submitted comment is fetched back from the server.
func MergeImportedComments(fr *model.FileReview, lineComments map[int][]model.Comment) int {
	added := 0
	for line, cs := range lineComments {
		for _, c := range cs {
			// Single pass: skip duplicates by ExternalID and remove submitted
			// local comments that match the incoming remote comment.
			found := false
			n := 0
			for _, existing := range fr.LineComments[line] {
				if existing.ExternalID == c.ExternalID {
					found = true
				}
				if existing.Submitted && existing.ExternalID == "" &&
					existing.Author == c.Author && existing.Content == c.Content && existing.Side == c.Side {
					continue // drop the submitted local duplicate
				}
				fr.LineComments[line][n] = existing
				n++
			}
			fr.LineComments[line] = fr.LineComments[line][:n]

			if !found {
				fr.AddLineComment(line, c)
				added++
			}
		}
	}
	return added
}

// ExportComments converts local model comments into provider comment drafts.
// Only exports comments that have not been submitted and belong to the reviewer.
func ExportComments(session *model.ReviewSession, reviewer string) []CommentDraft {
	var drafts []CommentDraft
	for path, fr := range session.Files {
		for line, comments := range fr.LineComments {
			for _, c := range comments {
				if c.Submitted || (c.Author != "" && c.Author != reviewer) {
					continue
				}
				if c.ExternalID != "" {
					continue // already exists remotely
				}
				draft := CommentDraft{
					Path: path,
					Line: line,
					Body: c.Content,
				}
				switch c.Side {
				case model.SideOld:
					draft.Side = "old"
				default:
					draft.Side = "new"
				}
				if c.LineRange != nil {
					draft.StartLine = c.LineRange.Start
					draft.StartSide = draft.Side
				}
				drafts = append(drafts, draft)
			}
		}
	}
	return drafts
}

// ExportReviewState maps model.ApprovalStatus to provider.ReviewState.
func ExportReviewState(status model.ApprovalStatus) ReviewState {
	switch status {
	case model.ApprovalApprove:
		return ReviewApprove
	case model.ApprovalRequestChanges:
		return ReviewRequestChanges
	default:
		return ReviewComment
	}
}
