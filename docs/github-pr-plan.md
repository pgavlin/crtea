# Plan: Remote Code Review Provider Integration

This plan is organized into phases that can be implemented and shipped
incrementally. Each phase produces a usable increment of functionality.

A key architectural goal is **provider abstraction**: the interface between the
app and any remote code review host (GitHub, GitLab, Bitbucket, etc.) is a
clean Go interface. All model, mapping, UI, and CLI work is implemented against
this abstract interface first. The concrete GitHub provider comes last — it
simply implements the interface and plugs in. All phases before the final one
can be developed and tested with a mock provider.

---

## Phase 1: Provider Interface and Model Foundation

**Goal:** define the abstraction boundary, provider-neutral types, mapping
layer, and all model changes needed for remote reviews.

### 1.1 New package: `provider/`

#### `provider/provider.go`

```go
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
```

The `id` parameter is an opaque string whose meaning is provider-specific (e.g.
"123" for GitHub PR #123, "!456" for GitLab MR !456). The CLI flag determines
which provider is used and what the id means.

#### `provider/types.go`

Provider-neutral types used by the interface:

```go
// ReviewRequest holds metadata for a PR / MR / etc.
type ReviewRequest struct {
    ID           string
    Title        string
    Body         string
    State        string // "open", "closed", "merged"
    Author       string
    URL          string // web URL
    BaseRef      string
    HeadRef      string
    HeadSHA      string
    ProviderMeta map[string]string // provider-specific metadata
}

// Review is a top-level review (body + status).
type Review struct {
    ExternalID string
    Author     string
    Body       string
    State      ReviewState
    CreatedAt  time.Time
}

type ReviewState int

const (
    ReviewComment ReviewState = iota
    ReviewApprove
    ReviewRequestChanges
)

// Comment is an inline comment on a specific file/line.
type Comment struct {
    ExternalID string
    Author     string
    Body       string
    Path       string
    Line       int
    StartLine  int    // 0 for single-line
    Side       string // "old" or "new"
    StartSide  string
    ReplyToID  string // empty if top-level
    CreatedAt  time.Time
    IsOutdated bool
}

// ConversationComment is a general comment not tied to code.
type ConversationComment struct {
    ExternalID string
    Author     string
    Body       string
    CreatedAt  time.Time
}

// SubmitReviewRequest is the payload for submitting a review.
type SubmitReviewRequest struct {
    Body     string
    State    ReviewState
    Comments []CommentDraft
}

// CommentDraft is a new inline comment to submit.
type CommentDraft struct {
    Path      string
    Line      int
    StartLine int
    Side      string
    StartSide string
    Body      string
}

// RefreshResult describes what changed on refresh.
type RefreshResult struct {
    Request         *ReviewRequest
    NewComments     []Comment
    NewReviews      []Review
    NewConversation []ConversationComment
    DiffChanged     bool   // true if the diff changed (new commits pushed)
    Diff            string // new diff content, if DiffChanged
}
```

#### `provider/mapping.go`

Convert between `provider` types and `model` types. This is the only place
where both packages are imported together:

```go
// ImportComments converts provider comments into model comments,
// grouped by file path.
func ImportComments(comments []Comment) map[string][]model.Comment

// ImportReview converts a provider review into a model OverallReview.
func ImportReview(review Review) model.OverallReview

// ExportComments converts local model comments into provider comment drafts.
// Skips comments that are already submitted.
func ExportComments(session *model.ReviewSession, reviewer string) []CommentDraft

// ExportReviewState maps model.ApprovalStatus to provider.ReviewState.
func ExportReviewState(status model.ApprovalStatus) ReviewState
```

Mapping rules (provider-neutral):
- `Side "old"` → `model.SideOld`, `"new"` → `model.SideNew`
- `StartLine` + `Line` → `model.LineRange{Start, End}`
- `ReplyToID` → `model.Comment.ReplyToID`
- `ExternalID` → `model.Comment.ExternalID`
- `CommentSuggestion` content is passed through as-is; provider-specific
  wrapping (e.g. GitHub suggestion fences) is each provider's responsibility

### 1.2 Model changes

#### `model/comment.go`

Add fields to `Comment`:

```go
Author     string `json:"author,omitempty"`
ExternalID string `json:"external_id,omitempty"`
ReplyToID  string `json:"reply_to_id,omitempty"`
Submitted  bool   `json:"submitted,omitempty"`
```

#### `model/session.go`

Add `DiffPullRequest` to `DiffSource`. Add new fields:

```go
type ProviderInfo struct {
    Name string `json:"name"`          // "github", "gitlab", etc.
    ID   string `json:"id"`            // "123" for PR #123
    URL  string `json:"url,omitempty"` // web URL
}
```

Add to `ReviewSession`:

```go
Provider *ProviderInfo  `json:"provider,omitempty"`
Reviewer string         `json:"reviewer,omitempty"`
Reviews  []OverallReview `json:"reviews,omitempty"`
```

Add `Author` and `ExternalID` to `OverallReview`:

```go
Author     string `json:"author,omitempty"`
ExternalID string `json:"external_id,omitempty"`
```

### 1.3 VCS layer changes

Export `parseDiff` from `vcs/git.go` so the provider path can reuse it:

```go
func ParseDiff(input string) []model.DiffFile {
    return parseDiff(input)
}
```

### 1.4 Persistence changes

Update `LoadLatest` to match on provider info when
`DiffSource == DiffPullRequest`.

### Files to create
- `provider/provider.go` — interface definition
- `provider/types.go` — provider-neutral types
- `provider/mapping.go` — bidirectional conversion
- `provider/mapping_test.go`

### Files to modify
- `model/comment.go` — add `Author`, `ExternalID`, `ReplyToID`, `Submitted`
- `model/session.go` — add `DiffPullRequest`, `ProviderInfo`, `Reviewer`,
  `Reviews`, fields on `OverallReview`
- `vcs/git.go` — export `ParseDiff`
- `persistence/session.go` — update `LoadLatest` matching

---

## Phase 2: CLI and UI Plumbing for Remote Reviews

**Goal:** wire the provider interface into the CLI and UI so that any provider
can be plugged in. Uses a mock/stub provider for testing.

### 2.1 CLI changes (`main.go`)

Add a `--pr` flag:

```go
&cli.StringFlag{
    Name:    "pr",
    Aliases: []string{"p"},
    Usage:   "review a pull/merge request (e.g. 123, owner/repo#123)",
},
```

In `run()`, when `--pr` is set:

1. Detect the provider and create a `provider.Provider` (Phase 7 implements
   the actual detection; for now, accept a provider as a function parameter or
   use a stub)
2. Call `provider.GetReviewRequest(id)` for metadata
3. Call `provider.GetDiff(id)` for the diff
4. Parse with `vcs.ParseDiff`
5. Run syntax highlighting
6. Create or load session with `DiffPullRequest` source
7. Populate `session.Provider` and `session.Description`
8. Pass the `provider.Provider` to `NewApp`
9. Launch as normal

### 2.2 UI: provider field on App

Add `provider provider.Provider` field to `App` (nil for local-only reviews).
Add constructor variant or option to set it.

Update `renderStatusBar` to show provider info when available:

```
[git:main] PR #123: Fix login bug                    0/5 reviewed
```

### Files to modify
- `main.go` — add `--pr` flag and provider startup path
- `ui/app.go` — add `provider` field
- `ui/render.go` — provider info in status bar

---

## Phase 3: Displaying Remote Comments and Author Attribution

**Goal:** display fetched comments with author names, distinguish "mine" from
"others'", guard editing of others' comments.

### 3.1 Comment import on startup

In the `--pr` startup path (after Phase 2 setup):

1. Call `provider.GetAuthenticatedUser()` → store as `session.Reviewer`
2. Call `provider.ListReviews(id)` → import via `provider.ImportReview`
3. Call `provider.ListComments(id)` → import via `provider.ImportComments`
4. Merge remote comments into the session, keyed by `ExternalID` to avoid
   duplicates on reload

### 3.2 UI: author display

Update `renderCommentLine` to show author in the comment badge when present:

```
╭ Note (@octocat) ───────────────────╮
│ This looks good                    │
╰────────────────────────────────────╯
```

- Own comments (`Author == Reviewer`): use existing comment type color
- Others' comments: use `th.FgDim`, render as read-only

### 3.3 Edit/delete guards

When cursor is on a remote comment (has `ExternalID` and `Author != Reviewer`):
- `i` (edit) → "Cannot edit others' comments"
- `dd` (delete) → "Cannot delete others' comments"

### Files to modify
- `provider/mapping.go` — implement `ImportComments`, `ImportReview`
- `main.go` — fetch and import remote comments on startup
- `ui/render.go` — author display in comment badges
- `ui/keys.go` — guard edit/delete on remote comments

---

## Phase 4: Submitting Reviews

**Goal:** `:submit` command posts the local review to the remote provider.

### 4.1 Export mapping

Implement `provider.ExportComments` and `ExportReviewState` in
`provider/mapping.go`.

The mapping layer produces `CommentDraft` with raw content. Each provider's
`SubmitReview` implementation handles provider-specific formatting (e.g.
GitHub wraps `CommentSuggestion` in `` ```suggestion `` fences).

### 4.2 UI: `:submit` command

Add to `executeCommand()`. The flow:

1. Verify session has `Provider` metadata and `app.provider` is set
2. Collect unsubmitted comments via `provider.ExportComments`
3. Map `OverallReview.Status` via `ExportReviewState`
4. Show confirmation via `modeConfirm`:
   ```
   Submit review to PR #123? (Approve, 5 comments) [y/n]
   ```
5. Call `provider.SubmitReview(id, request)`
6. Mark comments as `Submitted = true`
7. Auto-save session
8. Show status: "Review submitted to PR #123"

### Files to modify
- `provider/mapping.go` — implement `ExportComments`, `ExportReviewState`
- `model/comment.go` — use `Submitted` field (added in Phase 1)
- `ui/keys.go` — add `:submit` command, confirmation flow
- `ui/render.go` — submit confirmation text

---

## Phase 5: Comment Replies and Threading

**Goal:** view and create threaded replies on review comments.

### 5.1 Thread grouping (`ui/annotations.go`)

Update `buildAnnotations` to group comments into threads:

- Comments sharing the same parent `ExternalID` form a thread
- Within a thread, order by `CreatedAt`
- Render as a single connected box:
  ```
  ╭ Note (@alice) ───────────────────╮
  │ Is this intentional?             │
  ├──────────────────────────────────┤
  │ @bob: Yes, it handles the edge  │
  │ case from #456                   │
  ╰──────────────────────────────────╯
  ```

### 5.2 Reply keybinding

Add `a` (answer/reply) in normal mode:

- On a comment with `ExternalID`: enter `modeComment` with `replyToID` set
- Comment editor shows "Reply to @author" in the title
- On `:submit`, replies use `provider.ReplyToComment` instead of being bundled
  into the review

### 5.3 App state

Add to `App`:

```go
replyToID string // ExternalID of comment being replied to
```

Clear on save/cancel, same as `editingID`.

### Files to modify
- `ui/annotations.go` — thread grouping
- `ui/render.go` — threaded comment rendering
- `ui/keys.go` — `a` keybinding, reply handling
- `ui/app.go` — `replyToID` field

---

## Phase 6: General Conversation and Refresh

**Goal:** view/post general conversation comments; refresh remote state.

### 6.1 Model changes

Add to `ReviewSession`:

```go
type ConversationComment struct {
    ExternalID string    `json:"external_id,omitempty"`
    Author     string    `json:"author,omitempty"`
    Body       string    `json:"body"`
    CreatedAt  time.Time `json:"created_at"`
}

Conversation []ConversationComment `json:"conversation,omitempty"`
```

### 6.2 Conversation view

Toggle with `P` to show the conversation timeline in the top panel. Display
chronologically with author names.

### 6.3 Posting

Add `:comment` command → text editor (like `modeReview`) →
`provider.PostConversationComment`.

### 6.4 `:refresh` command

Uses `provider.Refresh(id)` which returns a `RefreshResult`:

1. Merge new comments/reviews into session
2. If `DiffChanged`, re-parse and rebuild the view
3. Show status: "Refreshed: 3 new comments, 1 updated"

### 6.5 Outdated comments

Display outdated comments (`IsOutdated`) with dimmed styling and "outdated"
label. Warn when replying to outdated comments.

### Files to modify
- `model/session.go` — add `ConversationComment`, `Conversation` field
- `ui/app.go` — conversation panel state
- `ui/render.go` — conversation rendering, outdated comment styling
- `ui/keys.go` — `P` toggle, `:comment` command, `:refresh` command
- `main.go` — fetch conversation on startup

---

## Phase 7: Mock Provider Demo App

**Goal:** build a standalone demo binary with a mock provider that returns
realistic canned data. This allows full UX exploration of all remote review
features locally, without any real API, before implementing a real provider.

### 7.1 New package: `provider/mock/`

#### `provider/mock/mock.go`

A complete `provider.Provider` implementation backed by in-memory data:

```go
type Mock struct {
    user         string
    request      provider.ReviewRequest
    diff         string
    reviews      []provider.Review
    comments     []provider.Comment
    conversation []provider.ConversationComment

    // Mutable state for write operations
    submitted    []provider.SubmitReviewRequest
    replies      []replyRecord
    posted       []string
}

func New() *Mock // pre-populated with realistic sample data

func (m *Mock) Name() string { return "mock" }
func (m *Mock) GetAuthenticatedUser() (string, error)
func (m *Mock) GetReviewRequest(id string) (*provider.ReviewRequest, error)
func (m *Mock) GetDiff(id string) (string, error)
func (m *Mock) ListReviews(id string) ([]provider.Review, error)
func (m *Mock) ListComments(id string) ([]provider.Comment, error)
func (m *Mock) ListConversation(id string) ([]provider.ConversationComment, error)
func (m *Mock) SubmitReview(id string, review provider.SubmitReviewRequest) error
func (m *Mock) ReplyToComment(id string, commentID string, body string) error
func (m *Mock) PostConversationComment(id string, body string) error
func (m *Mock) Refresh(id string) (*provider.RefreshResult, error)
```

Write operations (`SubmitReview`, `ReplyToComment`, `PostConversationComment`)
record what was submitted in memory and return success. `Refresh` returns a
`RefreshResult` that includes any comments/replies created since the last
refresh, simulating real bidirectional sync.

#### Canned data

`New()` populates the mock with a realistic scenario:

- **Review request:** PR #42 "Add user authentication middleware" by `@alice`,
  open, base `main` → head `feature/auth`, 3 files changed
- **Diff:** a realistic unified diff with ~3 files (e.g. `auth/middleware.go`
  added, `server.go` modified, `auth/middleware_test.go` added), containing
  additions, deletions, context lines, and multiple hunks
- **Reviews:**
  - `@bob`: "REQUEST_CHANGES" — "A few things to address before merging"
  - `@carol`: "COMMENT" — "Looking good overall, minor nits"
- **Inline comments (with threads):**
  - `@bob` on `auth/middleware.go:25` (new side): "This should validate the
    token expiry" (ExternalID: "c1")
    - `@alice` reply: "Good catch, I'll add that check" (ReplyToID: "c1")
    - `@bob` reply: "Thanks! Also consider the clock skew case" (ReplyToID: "c1")
  - `@carol` on `auth/middleware.go:40` (new side): "Nit: consider extracting
    this into a helper" (ExternalID: "c4")
  - `@bob` on `server.go:112` (old side): "Was this intentional? The old
    handler had rate limiting" (ExternalID: "c5")
  - An outdated comment: `@carol` on `auth/middleware.go:10`, marked
    `IsOutdated: true`: "Typo in package doc" (ExternalID: "c6")
- **Conversation:**
  - `@alice`: "Ready for review! This adds JWT-based auth middleware."
  - `@bob`: "I'll take a look this afternoon."
  - `@carol`: "Reviewing now."
- **Authenticated user:** `@you` (so own comments are distinguishable)

### 7.2 Demo binary: `cmd/crtea-demo/main.go`

A minimal main that wires the mock provider into the existing app:

```go
func main() {
    mock := mock.New()

    // Parse the mock diff into DiffFiles
    files := vcs.ParseDiff(mock.GetDiff("42"))
    highlighter.HighlightFiles(files)

    // Build session from mock data
    session := // ... create session, import reviews/comments via mapping

    app := ui.NewApp(backend, files, session, th, highlighter, store)
    app.SetProvider(mock, "42")

    // Run as normal
}
```

This binary requires no git repo, no network, no `gh` CLI. Run it with:

```
go run ./cmd/crtea-demo
```

It exercises the full UX: viewing the diff, seeing threaded comments from
multiple authors, author attribution, outdated comment styling, the
conversation panel, `:submit` (records in memory and shows success), `:refresh`
(returns any locally-submitted data back), and reply workflows.

### 7.3 What to validate

Use the demo to verify before moving to a real provider:

- [ ] Author names display correctly in comment badges
- [ ] Own vs others' comments are visually distinct
- [ ] Edit/delete guards work on others' comments
- [ ] Threaded comments render as connected boxes
- [ ] `a` (reply) works on comments with `ExternalID`
- [ ] Outdated comments show dimmed with label
- [ ] Conversation panel (`P` toggle) displays timeline
- [ ] `:submit` shows confirmation, reports success
- [ ] `:refresh` shows "N new comments" status
- [ ] `:comment` posts to conversation
- [ ] Provider info shows in status bar ("mock PR #42: Add user auth...")

### Files to create
- `provider/mock/mock.go` — mock provider with canned data
- `cmd/crtea-demo/main.go` — demo binary

---

## Phase 8: GitHub Provider

**Goal:** implement the `provider.Provider` interface for GitHub. This is the
only phase that introduces GitHub-specific code.

### 8.1 New package: `provider/github/`

#### `provider/github/github.go`

Implements `provider.Provider` by wrapping the `gh` CLI:

```go
type GitHub struct {
    Owner string
    Repo  string
}

func New(owner, repo string) *GitHub

func (g *GitHub) Name() string { return "github" }
func (g *GitHub) GetAuthenticatedUser() (string, error)
func (g *GitHub) GetReviewRequest(id string) (*provider.ReviewRequest, error)
func (g *GitHub) GetDiff(id string) (string, error)
func (g *GitHub) ListReviews(id string) ([]provider.Review, error)
func (g *GitHub) ListComments(id string) ([]provider.Comment, error)
func (g *GitHub) ListConversation(id string) ([]provider.ConversationComment, error)
func (g *GitHub) SubmitReview(id string, review provider.SubmitReviewRequest) error
func (g *GitHub) ReplyToComment(id string, commentID string, body string) error
func (g *GitHub) PostConversationComment(id string, body string) error
func (g *GitHub) Refresh(id string) (*provider.RefreshResult, error)
```

Each method calls `gh api` via `exec.Command`, parses JSON with
`encoding/json`. The diff comes back in unified format that `vcs.ParseDiff`
already handles.

**Suggestion formatting:** `SubmitReview` scans `CommentDraft` bodies for
suggestion markers and wraps them in `` ```suggestion `` fences before posting.

**API mapping:**

| Provider method | GitHub API endpoint |
|---|---|
| `GetReviewRequest` | `GET /repos/{o}/{r}/pulls/{n}` |
| `GetDiff` | `GET /repos/{o}/{r}/pulls/{n}` with `Accept: application/vnd.github.diff` |
| `ListReviews` | `GET /repos/{o}/{r}/pulls/{n}/reviews` |
| `ListComments` | `GET /repos/{o}/{r}/pulls/{n}/comments` |
| `ListConversation` | `GET /repos/{o}/{r}/issues/{n}/comments` |
| `SubmitReview` | `POST /repos/{o}/{r}/pulls/{n}/reviews` |
| `ReplyToComment` | `POST /repos/{o}/{r}/pulls/{n}/comments/{id}/replies` |
| `PostConversationComment` | `POST /repos/{o}/{r}/issues/{n}/comments` |
| `GetAuthenticatedUser` | `GET /user` |

**Review state mapping:**
- `ReviewApprove` → `"APPROVE"`
- `ReviewRequestChanges` → `"REQUEST_CHANGES"`
- `ReviewComment` → `"COMMENT"`

#### `provider/github/remote.go`

```go
// DetectRemote extracts owner/repo from the git remote URL.
func DetectRemote(repoPath string) (owner, repo string, err error)
```

Handles `git@github.com:owner/repo.git`, `https://github.com/owner/repo.git`,
and `gh`-style formats.

### 8.2 CLI: provider detection

Wire up provider detection in `main.go`:

1. When `--pr` is set, call `github.DetectRemote(rootPath)` to get owner/repo
2. Create `github.New(owner, repo)` as the `provider.Provider`
3. Pass to the existing startup path from Phase 2

Future providers add detection here (e.g. detect GitLab remote, create
`gitlab.New(...)`).

### Files to create
- `provider/github/github.go`
- `provider/github/remote.go`
- `provider/github/remote_test.go`
- `provider/github/github_test.go` (test JSON parsing with fixture data)

### Files to modify
- `main.go` — wire GitHub provider detection into `--pr` flag

---

## Adding a New Provider

To add a new provider (e.g. GitLab):

1. Create `provider/gitlab/gitlab.go` implementing `provider.Provider`
2. Add remote detection in `provider/gitlab/remote.go`
3. Add a CLI flag (e.g. `--mr`) or auto-detect from the git remote
4. Register the provider in `main.go`

No changes needed to `model/`, `ui/`, or `provider/mapping.go`. The mapping
layer works with the provider-neutral types. Provider-specific formatting
(like suggestion syntax) lives in the provider implementation.

---

## Dependency and Risk Summary

| Phase | Dependencies | Risk |
|---|---|---|
| 1 | None (new types, interfaces, model fields) | Low — design work |
| 2 | Phase 1 | Low — CLI plumbing, testable with mock |
| 3 | Phase 1, 2 | Low — read-only UI changes |
| 4 | Phase 1, 2, 3 | Medium — visible to others, needs confirmation |
| 5 | Phase 3 | Medium — thread rendering complexity |
| 6 | Phase 1, 2 | Low — mostly new UI, independent of inline comments |
| 7 | Phase 1–6 | Low — canned data, exercises all features end-to-end |
| 8 | Phase 1–7 | Low — pure implementation of existing interface |

## Testing Strategy

- **Phases 1–6**: unit tests with a mock `provider.Provider` that returns
  canned data. All UI/model/mapping logic can be fully tested without any API.
- **`provider/mapping.go`**: pure function tests — given provider types, verify
  model types are correct (and vice versa).
- **Phase 7 (`provider/mock/`)**: the mock provider doubles as both the demo
  app backend and a reusable test fixture. The demo binary is the manual
  integration test — use it to validate every UX feature end-to-end.
- **Phase 8 (`provider/github/`)**: test JSON parsing with fixture files
  (captured `gh api` output). Mock `ghOutput` for unit tests.
- **`provider/github/remote.go`**: test URL parsing for SSH, HTTPS, and
  `gh`-style remotes.
- **Integration**: manual testing against a real PR. Create a test repo with a
  known PR for CI.

## Non-Goals

- **Real-time updates / websockets**: out of scope. Use `:refresh` for manual
  sync.
- **Creating PRs/MRs**: crtea is a review tool. Use `gh pr create` / `glab mr
  create`.
- **Merge / close operations**: use the provider's CLI or web UI.
