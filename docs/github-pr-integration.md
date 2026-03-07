# GitHub Pull Request Integration

Investigation into the features needed for crtea to interact with GitHub Pull Requests.

## Current Architecture

crtea is a local code review TUI. Its architecture already provides several
building blocks that map well to the GitHub PR review model:

| crtea concept | GitHub equivalent |
|---|---|
| `OverallReview` (body + `ApprovalStatus`) | PR review (body + `APPROVE`/`REQUEST_CHANGES`/`COMMENT`) |
| `Comment` on a line with `LineSide` | Review comment with `path`, `line`, `side` |
| `Comment` with `LineRange` | Multi-line review comment (`start_line`/`line`) |
| File-level `Comment` (`commentIsFile`) | Review comment with `subject_type: "file"` |
| `CommentType` (Note/Issue/Suggestion/...) | No direct equivalent (convention only) |
| `DiffFile` / `DiffHunk` / `DiffLine` | PR diff (unified format, same structure) |

## Gaps

### 1. No concept of review author

**Current state:** `ReviewSession` and `Comment` have no author field. Everything
is implicitly single-user.

**What GitHub provides:** Every review and comment has a `user` object with
`login`, `id`, `avatar_url`, etc. When fetching existing PR reviews, there may be
comments from multiple authors.

**Required changes:**
- Add `Author` field to `Comment` (at minimum `login` string)
- Add `Author` field to `OverallReview` for displaying who left each review
- When displaying fetched PR comments, show the author
- The local user's identity comes from the GitHub token (see gap 3)

### 2. No concept of reviewer identity

**Current state:** No notion of "who am I" in the session or anywhere else.

**What GitHub requires:** The authenticated user is implicit from the API token.
Reviews and comments are attributed to the token owner. The UI needs to
distinguish "my comments" from "others' comments" when displaying an existing
review thread.

**Required changes:**
- Store the authenticated user's login in the session or app config
- Fetch via [`GET /user`](https://docs.github.com/en/rest/users/users#get-the-authenticated-user)
  at startup
- Use it to visually distinguish own vs. others' comments in the UI

### 3. No GitHub client

**Current state:** The only VCS backend is `GitBackend`, which shells out to `git`
for local operations. There is no HTTP client or API integration.

**What GitHub requires:**
- REST API calls for reviews, comments, and PR metadata
- Authentication via token (PAT, fine-grained PAT, or GitHub App)
- The diff can be fetched via
  [`GET /repos/{owner}/{repo}/pulls/{pull_number}`](https://docs.github.com/en/rest/pulls/pulls#get-a-pull-request)
  with `Accept: application/vnd.github.diff`, which returns the same unified diff
  format that the existing parser already handles

**Approach options:**

| Option | Pros | Cons |
|---|---|---|
| Use `gh` CLI | Zero new deps; auth handled by `gh auth`; `gh api` for raw calls | Requires `gh` installed; subprocess overhead; parsing JSON output |
| Use [`go-github`](https://github.com/google/go-github) | Type-safe; well-maintained; full API coverage | New dependency; need to handle auth separately |
| Raw `net/http` | No deps | Boilerplate; manual JSON handling |

**Recommended:** Use `gh` CLI for a first pass (already available in most dev
environments, handles auth), with an option to add `go-github` later for
performance.

**Required API endpoints:**

| Operation | Endpoint | [Docs](https://docs.github.com/en/rest) |
|---|---|---|
| Get PR diff | `GET /repos/{o}/{r}/pulls/{n}` | [pulls#get](https://docs.github.com/en/rest/pulls/pulls#get-a-pull-request) |
| List PR files | `GET /repos/{o}/{r}/pulls/{n}/files` | [pulls#list-files](https://docs.github.com/en/rest/pulls/pulls#list-pull-requests-files) |
| List reviews | `GET /repos/{o}/{r}/pulls/{n}/reviews` | [reviews#list](https://docs.github.com/en/rest/pulls/reviews#list-reviews-for-a-pull-request) |
| Create review | `POST /repos/{o}/{r}/pulls/{n}/reviews` | [reviews#create](https://docs.github.com/en/rest/pulls/reviews#create-a-review-for-a-pull-request) |
| Submit pending review | `POST /repos/{o}/{r}/pulls/{n}/reviews/{id}/events` | [reviews#submit](https://docs.github.com/en/rest/pulls/reviews#submit-a-review-for-a-pull-request) |
| List review comments | `GET /repos/{o}/{r}/pulls/{n}/comments` | [comments#list](https://docs.github.com/en/rest/pulls/comments#list-review-comments-on-a-pull-request) |
| Create review comment | `POST /repos/{o}/{r}/pulls/{n}/comments` | [comments#create](https://docs.github.com/en/rest/pulls/comments#create-a-review-comment-for-a-pull-request) |
| Reply to comment | `POST /repos/{o}/{r}/pulls/{n}/comments/{id}/replies` | [comments#reply](https://docs.github.com/en/rest/pulls/comments#create-a-reply-for-a-review-comment) |
| List conversation comments | `GET /repos/{o}/{r}/issues/{n}/comments` | [issue-comments#list](https://docs.github.com/en/rest/issues/comments#list-issue-comments) |
| Create conversation comment | `POST /repos/{o}/{r}/issues/{n}/comments` | [issue-comments#create](https://docs.github.com/en/rest/issues/comments#create-an-issue-comment) |
| Get authenticated user | `GET /user` | [users#get-authenticated](https://docs.github.com/en/rest/users/users#get-the-authenticated-user) |

**Authentication requirements:**
- Classic PAT: `repo` scope (private repos) or `public_repo` (public only)
- Fine-grained PAT: "Pull requests" permission set to Read and write
- `GITHUB_TOKEN` in Actions: `pull-requests: write`

### 4. No support for comment replies

**Current state:** Comments are flat lists per line (`map[int][]Comment`). There is
no threading — multiple comments on the same line are independent.

**What GitHub provides:** Review comments can be replies to other comments via
`in_reply_to_id`. Replies form threads displayed together in the PR UI. The
[reply endpoint](https://docs.github.com/en/rest/pulls/comments#create-a-reply-for-a-review-comment)
only needs `body` — all positional info is inherited from the parent.

**Required changes:**
- Add `ReplyToID string` field to `Comment` (maps to GitHub's `in_reply_to_id`)
- Add `ExternalID string` field to `Comment` (GitHub's comment `id`, needed to
  reply to or update remote comments)
- Group comments into threads in the annotation builder and render them as
  connected conversations
- UI support: when cursor is on an existing comment, allow replying (new input
  mode or extending `modeComment`)
- When submitting to GitHub, use the reply endpoint for threaded comments

### 5. No support for general conversation

**Current state:** All comments are anchored to files or lines. There is no way to
leave a comment on the PR itself (not tied to any file or line).

**What GitHub provides:** Two kinds of non-line comments:
1. **Review body** — the top-level text of a review submission (already mapped by
   `OverallReview.Body`)
2. **Issue comments** — general conversation on the PR timeline, via the
   [Issues Comments API](https://docs.github.com/en/rest/issues/comments).
   These are independent of reviews and appear in the PR's conversation tab.

**Required changes:**
- Add a conversation/timeline view showing issue comments alongside review
  activity
- Add a command to post a general PR comment (`:comment` or similar)
- Add `ConversationComments []Comment` to `ReviewSession` (or a separate list)
- Fetch and display existing conversation comments when loading a PR

## Additional Gaps Discovered

### 6. No PR metadata model

**Current state:** `ReviewSession` stores `RepoPath`, `BranchName`, and
`BaseCommit`. There is no concept of a PR number, title, description, author,
base/head refs, or review state.

**Required changes:**
- Add a `PRMetadata` struct or extend `ReviewSession`:
  ```
  PRNumber    int
  PRTitle     string
  PRBody      string
  PRAuthor    string
  BaseRef     string    // e.g. "main"
  HeadRef     string    // e.g. "feature-branch"
  HeadSHA     string    // latest commit on the PR
  HTMLURL     string    // link to PR on GitHub
  ```
- New `DiffSource` value: `DiffPullRequest`

### 7. No remote comment sync

**Current state:** Comments only exist locally. Export is one-way (clipboard
markdown).

**What GitHub requires:** Bidirectional sync:
- **Pull:** fetch existing reviews/comments from GitHub and display them
- **Push:** submit local comments as a GitHub review
- **Update:** edit or delete previously submitted comments
- **Conflict:** handle comments that were modified on GitHub after local fetch

**Required changes:**
- `ExternalID` on `Comment` and `OverallReview` for tracking remote state
- `SyncState` field (local-only / synced / modified / deleted)
- A `:push` or `:submit` command to post the review to GitHub
- A `:pull` or `:refresh` command to fetch remote updates
- Pending review workflow: create review in `PENDING` state, add comments, then
  submit with an event

### 8. No diff position mapping

**Current state:** Comments reference file-relative line numbers (`NewLineNo` /
`OldLineNo`) and `LineSide` (`SideOld`/`SideNew`).

**What GitHub requires:** The preferred API uses `path` + `line` + `side`
(`LEFT`/`RIGHT`), which maps well to the existing model. The deprecated
`position` field (offset from `@@` header) should be avoided.

**Required mapping:**
- `LineSide.SideNew` → `"RIGHT"`, `LineSide.SideOld` → `"LEFT"`
- `LineRange{Start, End}` → `start_line` + `line`
- File-level comments → `subject_type: "file"` with only `path`
- The `commit_id` must be included (use PR's `head.sha`)

This mapping is straightforward and the current model is already close to what
the API expects.

### 9. No concept of review state (pending vs submitted)

**Current state:** Reviews are saved locally and exported to clipboard. There is
no draft/pending/submitted lifecycle.

**What GitHub supports:** A review can be created in `PENDING` state (only visible
to the author), then submitted with an event. This is important for batching
comments into a single review rather than sending them one at a time.

**Required changes:**
- Add `State` field to `ReviewSession` or `OverallReview` (draft/pending/submitted)
- The `:submit` command creates a pending review, attaches all comments, then
  submits it
- Track which comments have been submitted vs are still local drafts

### 10. Suggestion syntax

**Current state:** `CommentType.CommentSuggestion` exists but the content is
plain text.

**What GitHub supports:** Suggestions use a special markdown syntax:
````
```suggestion
replacement code here
```
````

**Required changes:**
- When `CommentType == CommentSuggestion`, wrap content in the suggestion fence
  block before submitting to GitHub
- Optionally, provide a UI for entering suggestions that pre-fills the current
  line content for editing

## Proposed New Packages

| Package | Purpose |
|---|---|
| `github/` | GitHub API client (REST calls, auth, response types) |
| `github/types.go` | GitHub-specific data types (PR, Review, ReviewComment, IssueComment) |
| `github/client.go` | API client (or `gh` CLI wrapper) |
| `github/sync.go` | Bidirectional sync logic (pull/push comments) |
| `github/mapping.go` | Convert between `model.Comment` and GitHub review comments |

## Proposed CLI Changes

```
crtea                           # existing: pick commits from local repo
crtea -r main~5..HEAD           # existing: review revision range
crtea --pr 123                  # new: review GitHub PR #123
crtea --pr owner/repo#123       # new: review PR from specific repo
```

## Summary

The existing architecture handles the hard parts well — unified diff parsing,
line-level comments with side awareness, multi-line ranges, approval statuses,
and session persistence. The main work is:

1. **GitHub API client** — HTTP calls or `gh` wrapper (gap 3)
2. **Comment threading** — `ReplyToID` + thread rendering (gap 4)
3. **Identity** — author on comments, authenticated user (gaps 1-2)
4. **Remote sync** — pull/push comments, pending review lifecycle (gaps 7, 9)
5. **Conversation comments** — PR-level discussion not tied to code (gap 5)
6. **PR metadata** — number, title, refs, URL (gap 6)

The diff format, comment positioning model, and approval statuses require
minimal changes since they already align closely with GitHub's API.
