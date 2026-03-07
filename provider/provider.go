package provider

// Provider defines the interface for remote code review hosts.
type Provider interface {
	// Name returns the provider name (e.g. "github", "gitlab").
	Name() string

	// GetAuthenticatedUser returns the current user's login/username.
	GetAuthenticatedUser() (string, error)

	// GetReviewRequest fetches metadata for a review request (PR, MR, etc.).
	GetReviewRequest(id string) (*ReviewRequest, error)

	// GetDiff fetches the diff for a review request in unified format.
	GetDiff(id string) (string, error)

	// ListReviews fetches existing top-level reviews.
	ListReviews(id string) ([]Review, error)

	// ListComments fetches all inline comments on the review request.
	ListComments(id string) ([]Comment, error)

	// ListConversation fetches general (non-inline) conversation comments.
	ListConversation(id string) ([]ConversationComment, error)

	// SubmitReview posts a review with inline comments.
	SubmitReview(id string, review SubmitReviewRequest) error

	// ReplyToComment posts a reply to an existing inline comment.
	ReplyToComment(id string, commentID string, body string) error

	// PostConversationComment posts a general conversation comment.
	PostConversationComment(id string, body string) error

	// Refresh re-fetches remote state and returns what changed.
	Refresh(id string) (*RefreshResult, error)
}
