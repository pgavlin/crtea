# Gaps: GitHub PR Lifecycle vs. crtea Data Model

## 1. PR State Transitions

GitHub supports: **open → closed**, **open → merged**, **closed → reopened**

- crtea reads `ReviewRequest.State` ("open"/"closed"/"merged") but never displays it or reacts to it
- No handling for when a PR gets merged/closed while reviewing — the user keeps working on stale state
- No ability to close, reopen, or merge a PR from the TUI

## 2. Review Request / Re-request Review

GitHub supports: **requesting reviewers** and **re-requesting review** after changes

- crtea has no concept of "requested reviewers" or "review requested from you"
- No way to see who is assigned as a reviewer
- No way to request or re-request review from someone

## 3. Conversation Thread Resolution

GitHub supports: **resolve/unresolve** on inline comment threads

- crtea has no `Resolved` / `IsResolved` field on comments or threads
- No ability to resolve or unresolve a conversation thread
- Can't see which threads are resolved vs. unresolved

## 4. Comment Editing and Deletion

GitHub supports: **editing** and **deleting** your own comments after submission

- crtea marks comments as `Submitted` and then treats them as read-only
- No way to edit a submitted comment on the remote
- No way to delete a submitted comment
- The provider interface has no `EditComment` or `DeleteComment` methods

## 5. Reactions / Emoji

GitHub supports: **reactions** (+1, -1, laugh, heart, etc.) on comments and reviews

- Not modeled at all — no reaction types, no ability to add/view reactions

## 6. Draft PRs

GitHub supports: **draft** pull requests that can be **marked ready for review**

- `ReviewRequest` has no `IsDraft` field
- No ability to mark a draft PR as ready for review

## 7. Labels, Milestones, Assignees

GitHub supports rich PR metadata: **labels**, **milestones**, **assignees**, **projects**

- None of these are in `ReviewRequest` or anywhere in the model
- No ability to add/remove labels, assign reviewers, etc.

## 8. PR Description Editing

GitHub supports: editing the **PR title and body** after creation

- `session.Description` is populated from the PR once, then never synced back
- No way to update the PR title/body from the TUI

## 9. Suggested Changes (GitHub-native)

GitHub has a specific **suggested changes** syntax (`` ```suggestion ``) that can be **applied as commits**

- crtea has `CommentSuggestion` type but it's purely local metadata — not exported to GitHub's suggestion format
- No ability to apply a suggestion (commit the proposed change)

## 10. Check Runs / CI Status

GitHub PRs have **status checks** and **CI/CD integrations**

- Not modeled at all — can't see if CI is passing/failing
- Can't see required checks or their status

## 11. Commit-Level Comments

GitHub supports comments on **specific commits** (separate from PR review comments)

- crtea fetches commit diffs but doesn't support commit-level comments
- The provider interface has `GetCommitDiff` but no `ListCommitComments`

## 12. File-Level "Viewed" State

GitHub tracks **viewed/not-viewed** state per file in the PR UI

- crtea has `FileReview.Reviewed` locally but this is never synced to/from GitHub
- GitHub's "viewed" state is separate from crtea's — they can drift

## 13. Branch Updates / Force Pushes

GitHub tracks when the **head branch is updated** (new commits pushed, force push, rebase)

- `Refresh` detects `HeadSHA` changes and re-fetches the diff
- But there's no model for tracking which comments became outdated due to the push — GitHub's `position`/`original_position` mapping is partially handled but the outdated state isn't surfaced prominently

## 14. Multiple Review Rounds

GitHub implicitly supports review rounds through sequential reviews

- crtea accumulates reviews in `session.Reviews` but doesn't group them by round
- No concept of "changes since last review" or "files changed since my last review"

## Priority

| Priority | Gap | Impact |
|----------|-----|--------|
| High | Thread resolution | Can't mark conversations as resolved — a core PR workflow |
| High | CI/check status | Can't see if PR is ready to merge |
| High | Draft PR state | Can't tell if PR is draft or mark it ready |
| Medium | Comment editing | Can't fix typos in submitted comments |
| Medium | Suggested changes format | `CommentSuggestion` doesn't produce GitHub-compatible suggestions |
| Medium | Requested reviewers | Can't see who should review |
| Low | Reactions | Nice-to-have, not blocking workflow |
| Low | Labels/milestones | Metadata management, rarely needed in review flow |
