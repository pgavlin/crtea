package github

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/pgavlin/crtea/provider"
)

// GitHub implements provider.Provider using the gh CLI.
type GitHub struct {
	Owner string
	Repo  string

	// PR GraphQL node ID (for mutations like markReadyForReview)
	prNodeID string

	// Cached state for refresh diffing
	lastHeadSHA    string
	lastCommentIDs map[string]bool
	lastReviewIDs  map[string]bool
	lastConvIDs    map[string]bool
}

// New creates a new GitHub provider for the given owner/repo.
func New(owner, repo string) *GitHub {
	return &GitHub{Owner: owner, Repo: repo}
}

func (g *GitHub) Name() string { return "github" }

func (g *GitHub) apiPath(format string, args ...any) string {
	prefix := fmt.Sprintf("/repos/%s/%s", g.Owner, g.Repo)
	return prefix + fmt.Sprintf(format, args...)
}

// ghAPI calls gh api and returns the raw output.
func ghAPI(args ...string) ([]byte, error) {
	cmdArgs := append([]string{"api"}, args...)
	cmd := exec.Command("gh", cmdArgs...)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			slog.Debug("gh api call failed", "args", args, "stderr", string(ee.Stderr))
			return nil, fmt.Errorf("gh api %s: %s", strings.Join(args, " "), string(ee.Stderr))
		}
		slog.Debug("gh api call failed", "args", args, "error", err)
		return nil, fmt.Errorf("gh api: %w", err)
	}
	return out, nil
}

// ghAPIPaginated calls gh api with pagination and returns all results.
func ghAPIPaginated(endpoint string) ([]json.RawMessage, error) {
	var all []json.RawMessage
	page := 1
	for {
		out, err := ghAPI(endpoint, "--method", "GET",
			"-f", fmt.Sprintf("per_page=%d", 100),
			"-f", fmt.Sprintf("page=%d", page))
		if err != nil {
			return nil, err
		}
		var batch []json.RawMessage
		if err := json.Unmarshal(out, &batch); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}
		if len(batch) == 0 {
			break
		}
		all = append(all, batch...)
		if len(batch) < 100 {
			break
		}
		page++
	}
	return all, nil
}

func (g *GitHub) GetAuthenticatedUser() (string, error) {
	out, err := ghAPI("/user")
	if err != nil {
		return "", err
	}
	var user struct {
		Login string `json:"login"`
	}
	if err := json.Unmarshal(out, &user); err != nil {
		return "", fmt.Errorf("parsing user: %w", err)
	}
	return user.Login, nil
}

func (g *GitHub) GetReviewRequest(id string) (*provider.ReviewRequest, error) {
	out, err := ghAPI(g.apiPath("/pulls/%s", id))
	if err != nil {
		return nil, err
	}
	var pr ghPullRequest
	if err := json.Unmarshal(out, &pr); err != nil {
		return nil, fmt.Errorf("parsing PR: %w", err)
	}
	g.prNodeID = pr.NodeID
	return pr.toProvider(), nil
}

func (g *GitHub) GetDiff(id string) (string, error) {
	out, err := ghAPI(g.apiPath("/pulls/%s", id),
		"-H", "Accept: application/vnd.github.diff")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (g *GitHub) ListCommits(id string) ([]provider.Commit, error) {
	raw, err := ghAPIPaginated(g.apiPath("/pulls/%s/commits", id))
	if err != nil {
		return nil, err
	}
	var commits []provider.Commit
	for _, r := range raw {
		var gc ghCommit
		if err := json.Unmarshal(r, &gc); err != nil {
			slog.Debug("skipping malformed commit", "error", err)
			continue
		}
		commits = append(commits, gc.toProvider())
	}
	// GitHub returns oldest-first; reverse to newest-first
	for i, j := 0, len(commits)-1; i < j; i, j = i+1, j-1 {
		commits[i], commits[j] = commits[j], commits[i]
	}
	return commits, nil
}

func (g *GitHub) GetCommitDiff(id string, commitID string) (string, error) {
	out, err := ghAPI(g.apiPath("/commits/%s", commitID),
		"-H", "Accept: application/vnd.github.diff")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (g *GitHub) ListReviews(id string) ([]provider.Review, error) {
	raw, err := ghAPIPaginated(g.apiPath("/pulls/%s/reviews", id))
	if err != nil {
		return nil, err
	}
	var reviews []provider.Review
	for _, r := range raw {
		var gr ghReview
		if err := json.Unmarshal(r, &gr); err != nil {
			slog.Debug("skipping malformed review", "error", err)
			continue
		}
		reviews = append(reviews, gr.toProvider())
	}
	return reviews, nil
}

func (g *GitHub) ListComments(id string) ([]provider.Comment, error) {
	raw, err := ghAPIPaginated(g.apiPath("/pulls/%s/comments", id))
	if err != nil {
		return nil, err
	}
	var comments []provider.Comment
	for _, r := range raw {
		var gc ghComment
		if err := json.Unmarshal(r, &gc); err != nil {
			slog.Debug("skipping malformed comment", "error", err)
			continue
		}
		comments = append(comments, gc.toProvider())
	}

	// Fetch thread resolution status via GraphQL and apply to comments.
	threads, err := g.fetchThreadResolution(id)
	if err != nil {
		slog.Warn("failed to fetch thread resolution status", "id", id, "error", err)
	}
	if err == nil {
		// Build map from root comment ID to thread info
		rootThreads := make(map[string]threadInfo)
		for commentID, info := range threads {
			rootThreads[commentID] = info
		}
		// First pass: set thread info on root comments
		for i := range comments {
			if info, ok := rootThreads[comments[i].ExternalID]; ok {
				comments[i].ThreadID = info.threadID
				comments[i].IsResolved = info.isResolved
			}
		}
		// Second pass: propagate to replies
		threadByRoot := make(map[string]threadInfo)
		for i := range comments {
			if comments[i].ThreadID != "" {
				threadByRoot[comments[i].ExternalID] = threadInfo{
					threadID:   comments[i].ThreadID,
					isResolved: comments[i].IsResolved,
				}
			}
		}
		for i := range comments {
			if comments[i].ThreadID == "" && comments[i].ReplyToID != "" {
				if info, ok := threadByRoot[comments[i].ReplyToID]; ok {
					comments[i].ThreadID = info.threadID
					comments[i].IsResolved = info.isResolved
				}
			}
		}
	}

	return comments, nil
}

func (g *GitHub) ListConversation(id string) ([]provider.ConversationComment, error) {
	raw, err := ghAPIPaginated(g.apiPath("/issues/%s/comments", id))
	if err != nil {
		return nil, err
	}
	var comments []provider.ConversationComment
	for _, r := range raw {
		var gc ghIssueComment
		if err := json.Unmarshal(r, &gc); err != nil {
			slog.Debug("skipping malformed conversation comment", "error", err)
			continue
		}
		comments = append(comments, gc.toProvider())
	}
	return comments, nil
}

func (g *GitHub) SubmitReview(id string, review provider.SubmitReviewRequest) error {
	body := map[string]any{
		"event": exportReviewEvent(review.State),
		"body":  review.Body,
	}
	if len(review.Comments) > 0 {
		var ghComments []map[string]any
		for _, c := range review.Comments {
			gc := map[string]any{
				"path": c.Path,
				"body": c.Body,
				"side": exportSide(c.Side),
			}
			if c.Line > 0 {
				gc["line"] = c.Line
			}
			if c.StartLine > 0 {
				gc["start_line"] = c.StartLine
				gc["start_side"] = exportSide(c.StartSide)
			}
			ghComments = append(ghComments, gc)
		}
		body["comments"] = ghComments
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	cmd := exec.Command("gh", "api", g.apiPath("/pulls/%s/reviews", id),
		"--method", "POST", "--input", "-")
	cmd.Stdin = strings.NewReader(string(payload))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("submitting review: %s", string(out))
	}
	return nil
}

func (g *GitHub) ReplyToComment(id string, commentID string, body string) error {
	payload, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return err
	}
	cmd := exec.Command("gh", "api",
		g.apiPath("/pulls/%s/comments/%s/replies", id, commentID),
		"--method", "POST", "--input", "-")
	cmd.Stdin = strings.NewReader(string(payload))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("replying to comment: %s", string(out))
	}
	return nil
}

func (g *GitHub) PostConversationComment(id string, body string) error {
	payload, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return err
	}
	cmd := exec.Command("gh", "api",
		g.apiPath("/issues/%s/comments", id),
		"--method", "POST", "--input", "-")
	cmd.Stdin = strings.NewReader(string(payload))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("posting comment: %s", string(out))
	}
	return nil
}

func (g *GitHub) EditComment(id string, commentID string, body string) error {
	payload, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return err
	}
	cmd := exec.Command("gh", "api",
		g.apiPath("/pulls/comments/%s", commentID),
		"--method", "PATCH", "--input", "-")
	cmd.Stdin = strings.NewReader(string(payload))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("editing comment: %s", string(out))
	}
	return nil
}

func (g *GitHub) DeleteComment(id string, commentID string) error {
	_, err := ghAPI(g.apiPath("/pulls/comments/%s", commentID), "--method", "DELETE")
	if err != nil {
		return fmt.Errorf("deleting comment: %w", err)
	}
	return nil
}

func (g *GitHub) ResolveThread(id string, threadID string) error {
	query := `mutation($threadId: ID!) { resolveReviewThread(input: {threadId: $threadId}) { thread { isResolved } } }`
	_, err := ghAPI("graphql", "-f", "query="+query, "-f", "threadId="+threadID)
	if err != nil {
		return fmt.Errorf("resolving thread: %w", err)
	}
	return nil
}

func (g *GitHub) UnresolveThread(id string, threadID string) error {
	query := `mutation($threadId: ID!) { unresolveReviewThread(input: {threadId: $threadId}) { thread { isResolved } } }`
	_, err := ghAPI("graphql", "-f", "query="+query, "-f", "threadId="+threadID)
	if err != nil {
		return fmt.Errorf("unresolving thread: %w", err)
	}
	return nil
}

func (g *GitHub) MarkReadyForReview(id string) error {
	if g.prNodeID == "" {
		return fmt.Errorf("PR node ID not available")
	}
	query := `mutation($prId: ID!) { markPullRequestReadyForReview(input: {pullRequestId: $prId}) { pullRequest { isDraft } } }`
	_, err := ghAPI("graphql", "-f", "query="+query, "-f", "prId="+g.prNodeID)
	if err != nil {
		return fmt.Errorf("marking ready: %w", err)
	}
	return nil
}

func (g *GitHub) UpdateReviewRequest(id string, title string, body string) error {
	payload, err := json.Marshal(map[string]string{"title": title, "body": body})
	if err != nil {
		return err
	}
	cmd := exec.Command("gh", "api",
		g.apiPath("/pulls/%s", id),
		"--method", "PATCH", "--input", "-")
	cmd.Stdin = strings.NewReader(string(payload))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("updating PR: %s", string(out))
	}
	return nil
}

// fetchThreadResolution queries GraphQL for review thread resolution status.
// Returns a map of comment database ID (string) to threadInfo.
type threadInfo struct {
	threadID   string
	isResolved bool
}

func (g *GitHub) fetchThreadResolution(prNumber string) (map[string]threadInfo, error) {
	query := fmt.Sprintf(`query {
		repository(owner: %q, name: %q) {
			pullRequest(number: %s) {
				reviewThreads(first: 100) {
					nodes {
						id
						isResolved
						comments(first: 1) {
							nodes { databaseId }
						}
					}
				}
			}
		}
	}`, g.Owner, g.Repo, prNumber)

	out, err := ghAPI("graphql", "-f", "query="+query)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data struct {
			Repository struct {
				PullRequest struct {
					ReviewThreads struct {
						Nodes []struct {
							ID         string `json:"id"`
							IsResolved bool   `json:"isResolved"`
							Comments   struct {
								Nodes []struct {
									DatabaseID int `json:"databaseId"`
								} `json:"nodes"`
							} `json:"comments"`
						} `json:"nodes"`
					} `json:"reviewThreads"`
				} `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("parsing thread resolution: %w", err)
	}

	result := make(map[string]threadInfo)
	for _, thread := range resp.Data.Repository.PullRequest.ReviewThreads.Nodes {
		for _, comment := range thread.Comments.Nodes {
			result[strconv.Itoa(comment.DatabaseID)] = threadInfo{
				threadID:   thread.ID,
				isResolved: thread.IsResolved,
			}
		}
	}
	return result, nil
}

// Seed records the initial state of the review so that the first Refresh
// correctly reports only items that appeared after startup.
func (g *GitHub) Seed(rr *provider.ReviewRequest, comments []provider.Comment, reviews []provider.Review, conv []provider.ConversationComment) {
	g.lastHeadSHA = rr.HeadSHA

	g.lastCommentIDs = make(map[string]bool, len(comments))
	for _, c := range comments {
		g.lastCommentIDs[c.ExternalID] = true
	}

	g.lastReviewIDs = make(map[string]bool, len(reviews))
	for _, r := range reviews {
		g.lastReviewIDs[r.ExternalID] = true
	}

	g.lastConvIDs = make(map[string]bool, len(conv))
	for _, c := range conv {
		g.lastConvIDs[c.ExternalID] = true
	}
}

func (g *GitHub) Refresh(id string) (*provider.RefreshResult, error) {
	result := &provider.RefreshResult{}

	// Re-fetch PR metadata
	rr, err := g.GetReviewRequest(id)
	if err != nil {
		return nil, err
	}
	result.Request = rr

	// Check if diff changed (head SHA differs)
	if g.lastHeadSHA != "" && rr.HeadSHA != g.lastHeadSHA {
		result.DiffChanged = true
		diff, err := g.GetDiff(id)
		if err != nil {
			return nil, err
		}
		result.Diff = diff
	}
	g.lastHeadSHA = rr.HeadSHA

	// Find new comments
	comments, err := g.ListComments(id)
	if err != nil {
		slog.Warn("refresh: failed to list comments", "id", id, "error", err)
	}
	if err == nil {
		result.AllComments = comments
		newIDs := make(map[string]bool, len(comments))
		for _, c := range comments {
			newIDs[c.ExternalID] = true
			if g.lastCommentIDs != nil && !g.lastCommentIDs[c.ExternalID] {
				result.NewComments = append(result.NewComments, c)
			}
		}
		g.lastCommentIDs = newIDs
	}

	// Find new reviews
	reviews, err := g.ListReviews(id)
	if err != nil {
		slog.Warn("refresh: failed to list reviews", "id", id, "error", err)
	}
	if err == nil {
		newIDs := make(map[string]bool, len(reviews))
		for _, r := range reviews {
			newIDs[r.ExternalID] = true
			if g.lastReviewIDs != nil && !g.lastReviewIDs[r.ExternalID] {
				result.NewReviews = append(result.NewReviews, r)
			}
		}
		g.lastReviewIDs = newIDs
	}

	// Find new conversation comments
	conv, err := g.ListConversation(id)
	if err != nil {
		slog.Warn("refresh: failed to list conversation", "id", id, "error", err)
	}
	if err == nil {
		newIDs := make(map[string]bool, len(conv))
		for _, c := range conv {
			newIDs[c.ExternalID] = true
			if g.lastConvIDs != nil && !g.lastConvIDs[c.ExternalID] {
				result.NewConversation = append(result.NewConversation, c)
			}
		}
		g.lastConvIDs = newIDs
	}

	return result, nil
}

// exportSide maps provider side ("new"/"old") to GitHub side ("RIGHT"/"LEFT").
func exportSide(side string) string {
	switch side {
	case "new":
		return "RIGHT"
	case "old":
		return "LEFT"
	default:
		return "RIGHT"
	}
}

// exportReviewEvent maps provider.ReviewState to a GitHub review event string.
func exportReviewEvent(state provider.ReviewState) string {
	switch state {
	case provider.ReviewApprove:
		return "APPROVE"
	case provider.ReviewRequestChanges:
		return "REQUEST_CHANGES"
	default:
		return "COMMENT"
	}
}

// --- GitHub API response types ---

type ghPullRequest struct {
	Number  int    `json:"number"`
	NodeID  string `json:"node_id"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	State   string `json:"state"`
	Draft   bool   `json:"draft"`
	User    ghUser `json:"user"`
	HTMLURL string `json:"html_url"`
	Base    struct {
		Ref string `json:"ref"`
	} `json:"base"`
	Head struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"head"`
	Merged bool `json:"merged"`
}

func (pr *ghPullRequest) toProvider() *provider.ReviewRequest {
	state := pr.State
	if pr.Merged {
		state = "merged"
	}
	return &provider.ReviewRequest{
		ID:      strconv.Itoa(pr.Number),
		Title:   pr.Title,
		Body:    pr.Body,
		State:   state,
		IsDraft: pr.Draft,
		Author:  pr.User.Login,
		URL:     pr.HTMLURL,
		BaseRef: pr.Base.Ref,
		HeadRef: pr.Head.Ref,
		HeadSHA: pr.Head.SHA,
	}
}

type ghUser struct {
	Login string `json:"login"`
}

type ghReview struct {
	ID        int    `json:"id"`
	User      ghUser `json:"user"`
	Body      string `json:"body"`
	State     string `json:"state"`
	CreatedAt string `json:"submitted_at"`
}

func (r *ghReview) toProvider() provider.Review {
	var state provider.ReviewState
	switch r.State {
	case "APPROVED":
		state = provider.ReviewApprove
	case "CHANGES_REQUESTED":
		state = provider.ReviewRequestChanges
	default:
		state = provider.ReviewComment
	}
	t, _ := time.Parse(time.RFC3339, r.CreatedAt)
	return provider.Review{
		ExternalID: strconv.Itoa(r.ID),
		Author:     r.User.Login,
		Body:       r.Body,
		State:      state,
		CreatedAt:  t,
	}
}

type ghComment struct {
	ID                int    `json:"id"`
	User              ghUser `json:"user"`
	Body              string `json:"body"`
	Path              string `json:"path"`
	Line              *int   `json:"line"`
	OriginalLine      *int   `json:"original_line"`
	StartLine         *int   `json:"start_line"`
	OriginalStartLine *int   `json:"original_start_line"`
	Side              string `json:"side"`
	StartSide         string `json:"start_side"`
	InReplyToID       *int   `json:"in_reply_to_id"`
	CreatedAt         string `json:"created_at"`
	Position          *int   `json:"position"` // nil when outdated
	OriginalPosition  *int   `json:"original_position"`
}

func (c *ghComment) toProvider() provider.Comment {
	t, _ := time.Parse(time.RFC3339, c.CreatedAt)

	line := 0
	if c.Line != nil {
		line = *c.Line
	} else if c.OriginalLine != nil {
		line = *c.OriginalLine
	}

	startLine := 0
	if c.StartLine != nil {
		startLine = *c.StartLine
	} else if c.OriginalStartLine != nil {
		startLine = *c.OriginalStartLine
	}

	side := strings.ToLower(c.Side)
	if side == "right" {
		side = "new"
	} else if side == "left" {
		side = "old"
	}

	startSide := strings.ToLower(c.StartSide)
	if startSide == "right" {
		startSide = "new"
	} else if startSide == "left" {
		startSide = "old"
	}

	replyToID := ""
	if c.InReplyToID != nil {
		replyToID = strconv.Itoa(*c.InReplyToID)
	}

	// A comment is outdated when its position is null (GitHub sets position
	// to null when the diff has changed and the line no longer exists).
	isOutdated := c.Position == nil && c.OriginalPosition != nil

	return provider.Comment{
		ExternalID: strconv.Itoa(c.ID),
		Author:     c.User.Login,
		Body:       c.Body,
		Path:       c.Path,
		Line:       line,
		StartLine:  startLine,
		Side:       side,
		StartSide:  startSide,
		ReplyToID:  replyToID,
		CreatedAt:  t,
		IsOutdated: isOutdated,
	}
}

type ghCommit struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Name string `json:"name"`
			Date string `json:"date"`
		} `json:"author"`
	} `json:"commit"`
	Author ghUser `json:"author"`
}

func (c *ghCommit) toProvider() provider.Commit {
	t, _ := time.Parse(time.RFC3339, c.Commit.Author.Date)
	summary := c.Commit.Message
	if idx := strings.Index(summary, "\n"); idx >= 0 {
		summary = summary[:idx]
	}
	shortID := c.SHA
	if len(shortID) > 7 {
		shortID = shortID[:7]
	}
	author := c.Author.Login
	if author == "" {
		author = c.Commit.Author.Name
	}
	return provider.Commit{
		ID:      c.SHA,
		ShortID: shortID,
		Summary: summary,
		Author:  author,
		Time:    t,
	}
}

type ghIssueComment struct {
	ID        int    `json:"id"`
	User      ghUser `json:"user"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

func (c *ghIssueComment) toProvider() provider.ConversationComment {
	t, _ := time.Parse(time.RFC3339, c.CreatedAt)
	return provider.ConversationComment{
		ExternalID: strconv.Itoa(c.ID),
		Author:     c.User.Login,
		Body:       c.Body,
		CreatedAt:  t,
	}
}
