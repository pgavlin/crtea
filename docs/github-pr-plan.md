# Plan: GitHub Pull Request Integration

This plan is organized into phases that can be implemented and shipped
incrementally. Each phase produces a usable increment of functionality.

## Phase 1: GitHub API Client and PR Diff Viewing

**Goal:** `crtea --pr 123` fetches a PR diff from GitHub and opens it in the
existing review UI. Read-only; no comment sync yet.

### 1.1 New package: `github/`

#### `github/types.go`

Define types for GitHub API responses. Keep these minimal — only the fields
crtea needs.

```go
// PullRequest holds PR metadata fetched from the API.
type PullRequest struct {
    Number  int
    Title   string
    Body    string
    State   string // "open", "closed", "merged"
    HTMLURL string
    Author  string // login

    BaseRef string // e.g. "main"
    HeadRef string // e.g. "feature-branch"
    HeadSHA string // latest commit SHA on the PR
}

// ReviewComment is a comment on a specific line in the diff.
type ReviewComment struct {
    ID          int64
    Body        string
    Path        string
    Line        int    // end line (or only line)
    StartLine   int    // 0 if single-line
    Side        string // "LEFT" or "RIGHT"
    StartSide   string
    Author      string
    CreatedAt   time.Time
    InReplyToID int64  // 0 if top-level
}

// Review is a top-level review (body + status).
type Review struct {
    ID     int64
    Body   string
    State  string // "APPROVED", "CHANGES_REQUESTED", "COMMENTED", etc.
    Author string
}

// IssueComment is a general conversation comment on the PR.
type IssueComment struct {
    ID        int64
    Body      string
    Author    string
    CreatedAt time.Time
}
```

#### `github/client.go`

Wrap the `gh` CLI for API access. This avoids adding HTTP dependencies and
inherits the user's existing `gh auth` credentials.

```go
type Client struct {
    Owner string
    Repo  string
}

func NewClient(owner, repo string) *Client

// Read operations (Phase 1)
func (c *Client) GetPullRequest(number int) (*PullRequest, error)
func (c *Client) GetPullRequestDiff(number int) (string, error)
func (c *Client) ListReviews(number int) ([]Review, error)
func (c *Client) ListReviewComments(number int) ([]ReviewComment, error)
func (c *Client) ListIssueComments(number int) ([]IssueComment, error)
func (c *Client) GetAuthenticatedUser() (string, error)

// Write operations (Phase 3)
func (c *Client) CreateReview(number int, body, event string, comments []ReviewCommentDraft) (int64, error)
func (c *Client) CreateReviewComment(number int, draft ReviewCommentDraft) (int64, error)
func (c *Client) ReplyToComment(prNumber int, commentID int64, body string) (int64, error)
func (c *Client) CreateIssueComment(number int, body string) (int64, error)
```

Implementation: each method calls `gh api` via `exec.Command`, parses JSON
output with `encoding/json`. Example:

```go
func (c *Client) GetPullRequestDiff(number int) (string, error) {
    return ghOutput("api",
        fmt.Sprintf("repos/%s/%s/pulls/%d", c.Owner, c.Repo, number),
        "-H", "Accept: application/vnd.github.diff")
}
```

The diff comes back in the same unified format that `vcs.parseDiff` already
handles.

#### `github/remote.go`

Parse the GitHub remote from git config to auto-detect `owner` and `repo`:

```go
// DetectRemote extracts owner/repo from the git remote URL.
func DetectRemote(repoPath string) (owner, repo string, err error)
```

Handles `git@github.com:owner/repo.git`, `https://github.com/owner/repo.git`,
and `gh`-style `github.com/owner/repo` formats.

### 1.2 Model changes

#### `model/session.go`

Add `DiffPullRequest` to `DiffSource`:

```go
const (
    DiffWorkingTree DiffSource = iota
    DiffCommitRange
    DiffPullRequest
)
```

Add `PullRequest` metadata field to `ReviewSession`:

```go
type PRInfo struct {
    Number  int    `json:"number"`
    Title   string `json:"title,omitempty"`
    Body    string `json:"body,omitempty"`
    Author  string `json:"author,omitempty"`
    BaseRef string `json:"base_ref,omitempty"`
    HeadRef string `json:"head_ref,omitempty"`
    HeadSHA string `json:"head_sha,omitempty"`
    HTMLURL string `json:"html_url,omitempty"`
}
```

Add to `ReviewSession`:

```go
PullRequest *PRInfo `json:"pull_request,omitempty"`
```

### 1.3 CLI changes (`main.go`)

Add a `--pr` flag:

```go
&cli.IntFlag{
    Name:    "pr",
    Aliases: []string{"p"},
    Usage:   "review a GitHub pull request by number",
},
```

In `run()`, when `--pr` is set:

1. Call `github.DetectRemote(rootPath)` to get owner/repo
2. Create `github.Client{Owner, Repo}`
3. Call `client.GetPullRequest(prNumber)` for metadata
4. Call `client.GetPullRequestDiff(prNumber)` for the diff
5. Parse with existing `vcs.ParseDiff` (export it from the package)
6. Run syntax highlighting
7. Create or load session with `DiffPullRequest` source
8. Populate `session.PullRequest` with PR metadata
9. Set `session.Description` to PR title + body
10. Launch `ui.NewApp(...)` as normal

### 1.4 VCS layer changes

Export `parseDiff` from `vcs/git.go` so the GitHub path can reuse it:

```go
func ParseDiff(input string) []model.DiffFile {
    return parseDiff(input)
}
```

### 1.5 Persistence changes

Update `LoadLatest` to match on PR number when `DiffSource == DiffPullRequest`:

```go
if session.DiffSource == model.DiffPullRequest &&
    session.PullRequest != nil &&
    session.PullRequest.Number == prNumber {
    return &session, nil
}
```

Add `prNumber int` parameter to `LoadLatest` (0 for non-PR sessions).

### 1.6 UI changes

Update `renderStatusBar` to show PR info when available:

```
[git:main] PR #123: Fix login bug                    0/5 reviewed
```

### Files to create
- `github/types.go`
- `github/client.go`
- `github/remote.go`
- `github/remote_test.go`
- `github/client_test.go` (test JSON parsing with fixture data)

### Files to modify
- `model/session.go` — add `DiffPullRequest`, `PRInfo` struct, field on session
- `vcs/git.go` — export `ParseDiff`
- `persistence/session.go` — update `LoadLatest` signature and matching
- `main.go` — add `--pr` flag and GitHub startup path
- `ui/render.go` — PR info in status bar

---

## Phase 2: Author Identity and Fetching Existing Comments

**Goal:** fetch existing reviews and comments from the PR, display them with
author attribution, and distinguish "mine" from "others'".

### 2.1 Model changes

#### `model/comment.go`

Add fields to `Comment`:

```go
Author     string `json:"author,omitempty"`      // GitHub login
ExternalID string `json:"external_id,omitempty"` // GitHub comment ID
ReplyToID  string `json:"reply_to_id,omitempty"` // parent comment external ID
```

Add field to `OverallReview`:

```go
Author     string `json:"author,omitempty"`
ExternalID string `json:"external_id,omitempty"`
```

### 2.2 New package: `github/mapping.go`

Convert between GitHub types and model types:

```go
// ImportReviewComments converts GitHub review comments into model comments,
// grouped by file path and line number.
func ImportReviewComments(
    comments []ReviewComment,
) map[string]*model.FileReview

// ImportReview converts a GitHub review into a model OverallReview.
func ImportReview(review Review) *model.OverallReview
```

Mapping rules:
- `ReviewComment.Side "LEFT"` → `model.SideOld`, `"RIGHT"` → `model.SideNew`
- `ReviewComment.StartLine` + `Line` → `model.LineRange{Start, End}` (when
  StartLine > 0)
- `ReviewComment.InReplyToID` → `Comment.ReplyToID` (as string)
- `ReviewComment.ID` → `Comment.ExternalID` (as string)
- `ReviewComment.Author` → `Comment.Author`
- File-level comments (`subject_type == "file"`) → `FileReview.FileComments`

### 2.3 Session loading with remote comments

Add to the `--pr` startup path (after Phase 1 setup):

1. Call `client.GetAuthenticatedUser()` → store as `session.Reviewer`
2. Call `client.ListReviews(prNumber)` → import as `OverallReview` entries
3. Call `client.ListReviewComments(prNumber)` → import into `FileReview` maps
4. Call `client.ListIssueComments(prNumber)` → store as conversation
5. Merge remote comments into the session, keyed by `ExternalID` to avoid
   duplicates on reload

Add `Reviewer` field to `ReviewSession`:

```go
Reviewer string `json:"reviewer,omitempty"` // authenticated user's login
```

### 2.4 UI changes for author display

#### Comment rendering (`ui/render.go`)

Update `renderCommentLine` to show the author name in the comment badge when
present:

```
╭ Note (@octocat) ───────────────────╮
│ This looks good                    │
╰────────────────────────────────────╯
```

- If `comment.Author == session.Reviewer`, use the existing comment type color
- If `comment.Author != session.Reviewer`, use a distinct "remote" color (e.g.
  `th.FgDim`) and render as read-only

#### Comment editor restrictions

When the cursor is on a remote comment (has `ExternalID` and author !=
reviewer):
- `i` (edit) shows message: "Cannot edit others' comments"
- `dd` (delete) shows message: "Cannot delete others' comments"

### 2.5 Multiple overall reviews

Currently `OverallReview` is a single value. PRs can have multiple reviews from
different people.

Add to `ReviewSession`:

```go
Reviews []OverallReview `json:"reviews,omitempty"`
```

Keep `OverallReview` for the local user's draft. `Reviews` holds fetched remote
reviews for display.

### Files to create
- `github/mapping.go`
- `github/mapping_test.go`

### Files to modify
- `model/comment.go` — add `Author`, `ExternalID`, `ReplyToID`
- `model/session.go` — add `Reviewer`, `Reviews` fields
- `main.go` — fetch and import remote comments on `--pr` startup
- `ui/render.go` — author display in comment badges
- `ui/keys.go` — guard edit/delete on remote comments

---

## Phase 3: Submitting Reviews to GitHub

**Goal:** `:submit` command posts the local review (overall review + all
comments) to the PR as a single GitHub review.

### 3.1 New type for review submission

#### `github/types.go`

```go
type ReviewCommentDraft struct {
    Path      string
    Line      int
    StartLine int    // 0 for single-line
    Side      string // "LEFT" or "RIGHT"
    StartSide string
    Body      string
}
```

### 3.2 Export mapping (`github/mapping.go`)

```go
// ExportComments converts model comments from a session into GitHub review
// comment drafts.
func ExportComments(session *model.ReviewSession) []ReviewCommentDraft
```

Mapping rules:
- `model.SideOld` → `"LEFT"`, `model.SideNew` → `"RIGHT"`
- `model.LineRange` → `StartLine` + `Line`
- `model.CommentSuggestion` → wrap content in `` ```suggestion `` fences
- Skip comments that already have an `ExternalID` (already submitted)
- File-level comments: set `SubjectType: "file"`, omit line fields

### 3.3 Review state tracking

Add to `Comment`:

```go
Submitted bool `json:"submitted,omitempty"`
```

After successful submission, mark all submitted comments and persist the
session. This prevents double-posting on re-submit.

Map `ApprovalStatus` to GitHub events:
- `ApprovalApprove` → `"APPROVE"`
- `ApprovalRequestChanges` → `"REQUEST_CHANGES"`
- `ApprovalNeutral` → `"COMMENT"`

### 3.4 UI: `:submit` command

Add to `executeCommand()`:

```go
case "submit":
    return a.submitToGitHub()
```

`submitToGitHub()`:
1. Verify session has `PullRequest` metadata (error if not a PR review)
2. Collect unsubmitted comments via `github.ExportComments(session)`
3. Map `OverallReview.Status` to GitHub event string
4. Call `client.CreateReview(prNumber, body, event, comments)`
5. On success, mark all comments as `Submitted = true`
6. Store the returned review ID as `OverallReview.ExternalID`
7. Auto-save the session
8. Show status message: "Review submitted to PR #123"

The GitHub client needs to be accessible from the UI. Add it as a field on `App`:

```go
ghClient *github.Client // nil when not reviewing a PR
```

### 3.5 UI: confirmation before submit

Since submitting is a visible-to-others action, use `modeConfirm`:

```
Submit review to PR #123? (Approve, 5 comments) [y/n]
```

### Files to modify
- `github/types.go` — add `ReviewCommentDraft`
- `github/mapping.go` — add `ExportComments`
- `github/client.go` — implement `CreateReview`
- `model/comment.go` — add `Submitted` field
- `ui/app.go` — add `ghClient` field
- `ui/keys.go` — add `:submit` command
- `ui/render.go` — add submit confirmation text
- `main.go` — pass `ghClient` to `NewApp`

---

## Phase 4: Comment Replies and Threading

**Goal:** view and create threaded replies on review comments.

### 4.1 Thread grouping (`ui/annotations.go`)

Update `buildAnnotations` to group comments into threads:

- Comments with the same `ExternalID` as another comment's `ReplyToID` form a
  thread
- Within a thread, comments are ordered by `CreatedAt`
- Render thread as a single connected box:
  ```
  ╭ Note (@alice) ───────────────────╮
  │ Is this intentional?             │
  ├──────────────────────────────────┤
  │ @bob: Yes, it handles the edge  │
  │ case from #456                   │
  ╰──────────────────────────────────╯
  ```
- Each reply shows its author inline

### 4.2 Reply keybinding

Add `a` (answer/reply) keybinding in normal mode:

- When cursor is on a comment that has `ExternalID`, enter `modeComment` with
  `replyToID` set to that comment's `ExternalID`
- The comment editor shows "Reply to @author" in the title
- On save, the new comment gets `ReplyToID` set
- On `:submit`, comments with `ReplyToID` use
  `client.ReplyToComment(prNumber, parentID, body)` instead of being bundled
  into the review

### 4.3 UI state

Add to `App`:

```go
replyToID string // ExternalID of comment being replied to
```

Clear on save/cancel, same as `editingID`.

### Files to modify
- `ui/annotations.go` — thread grouping logic
- `ui/render.go` — threaded comment rendering
- `ui/keys.go` — `a` keybinding, reply handling in submit
- `ui/app.go` — `replyToID` field
- `github/client.go` — `ReplyToComment` implementation

---

## Phase 5: General Conversation

**Goal:** view and post general PR comments (not tied to code lines).

### 5.1 Model changes

Add to `ReviewSession`:

```go
Conversation []ConversationComment `json:"conversation,omitempty"`
```

```go
type ConversationComment struct {
    ExternalID string    `json:"external_id,omitempty"`
    Author     string    `json:"author,omitempty"`
    Body       string    `json:"body"`
    CreatedAt  time.Time `json:"created_at"`
}
```

### 5.2 Conversation view

Add a toggle (like `D` toggles description/commit list) to show the PR
conversation timeline in the top panel area. Display conversation comments
chronologically with author names.

New keybinding: `P` (PR conversation) toggles the conversation panel.

### 5.3 Posting conversation comments

Add `:comment` command that opens a text editor (like `modeReview`) for writing
a general PR comment. On submit, calls
`client.CreateIssueComment(prNumber, body)`.

### Files to modify
- `model/session.go` — add `Conversation`, `ConversationComment`
- `ui/app.go` — conversation panel state
- `ui/render.go` — conversation panel rendering
- `ui/keys.go` — `P` toggle, `:comment` command
- `main.go` — fetch conversation comments on startup
- `github/client.go` — `CreateIssueComment` implementation

---

## Phase 6: Suggestion Syntax

**Goal:** suggestions submitted to GitHub use the ` ```suggestion ``` ` fence
block format.

### 6.1 Export mapping

In `github/mapping.go`, when converting a `CommentSuggestion`:

```go
if comment.Type == model.CommentSuggestion {
    body = "```suggestion\n" + comment.Content + "\n```"
}
```

### 6.2 Suggestion editor UX (optional)

When pressing `c` on a diff line and cycling to `Suggestion` type, pre-fill the
comment buffer with the current line's content so the user can edit it into
their suggested replacement.

### Files to modify
- `github/mapping.go` — suggestion wrapping in `ExportComments`
- `ui/keys.go` — optional: pre-fill suggestion buffer

---

## Phase 7: Refresh and Sync

**Goal:** `:refresh` re-fetches remote state; handle outdated comments
gracefully.

### 7.1 `:refresh` command

1. Re-fetch PR metadata (check for new commits, state changes)
2. Re-fetch all reviews and comments
3. Merge into session: add new remote comments, update changed ones, mark
   deleted ones
4. If `HeadSHA` changed, re-fetch and re-parse the diff
5. Show status: "Refreshed: 3 new comments, 1 updated"

### 7.2 Outdated comment handling

When the PR has new commits after a comment was placed, GitHub marks comments as
"outdated". The API response includes `position: null` for these.

Display outdated comments with a visual indicator (dimmed, with "outdated"
label). Don't prevent viewing them, but warn when replying.

### Files to modify
- `ui/keys.go` — `:refresh` command
- `github/client.go` — re-fetch methods
- `github/mapping.go` — merge logic
- `ui/render.go` — outdated comment styling

---

## Dependency and Risk Summary

| Phase | Dependencies | Risk |
|---|---|---|
| 1 | None (new code + minor model additions) | Low — `gh` CLI may not be installed |
| 2 | Phase 1 | Low — read-only, additive model changes |
| 3 | Phase 1, 2 | Medium — submitting is destructive (visible to others), needs confirmation |
| 4 | Phase 2, 3 | Medium — thread rendering complexity |
| 5 | Phase 1 | Low — mostly new UI, independent of review comments |
| 6 | Phase 3 | Low — small mapping change |
| 7 | Phase 1, 2, 3 | Medium — merge conflicts between local and remote state |

## Testing Strategy

- **`github/client.go`**: test JSON parsing with fixture files (captured `gh api`
  output). Mock `ghOutput` for unit tests.
- **`github/mapping.go`**: pure function tests — given GitHub types, verify
  model types are correct.
- **`github/remote.go`**: test URL parsing for SSH, HTTPS, and `gh`-style
  remotes.
- **Integration**: manual testing against a real PR. Create a test repo with a
  known PR for CI.

## Non-Goals

- **Real-time updates / websockets**: out of scope. Use `:refresh` for manual
  sync.
- **Creating PRs**: crtea is a review tool, not a PR creation tool. Use
  `gh pr create`.
- **Merge / close operations**: use `gh pr merge` or the web UI.
- **Multiple provider support** (GitLab, Bitbucket): design the `github/`
  package cleanly, but don't abstract prematurely. Can be added later behind an
  interface if needed.
