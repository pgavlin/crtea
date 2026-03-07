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
