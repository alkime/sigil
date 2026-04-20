# sigil diff — PR Review Walkthrough

A typical review session from invocation to agent handoff.

## Prerequisites

- `gh` CLI installed and authenticated (`gh auth login`)
- An open GitHub PR on the current branch

## Invocation

```
$ cd my-repo
$ git checkout feature/oidc
$ sigil diff
```

If a single open PR is found, the TUI opens immediately. If multiple PRs are
found across worktrees, a picker lets you select which one to review.

## TUI Layout

```
┌─────────────────────────────────────────────────────────────────┐
│  sigil diff  ·  org/repo  ·  PR #42  ·  3 files                │  ← header
├─────────────────────────────────────────────────────────────────┤
│    ◉ PR Comments                                                │  ← virtual entry
│    ─────────────────                                            │
│  ▸ internal/auth/oidc.go (+42 -8)                               │  ← file list
│    internal/auth/token.go (+5 -0)                               │
│    internal/middleware/auth.go (+12 -3)                         │
│  1/3 files  (Tab/S-Tab to navigate)                             │
├─────────────────────────────────────────────────────────────────┤
│   42     if err != nil {                                        │  ← diff viewport
│   43     return nil, err                                        │  (focused line has
│   44  +  return token, nil                                      │   subtle bg highlight)
│   45     }                                                      │
│                                                                 │
│   ●  james: should we be logging the token type?               │  ← inline comment
├─────────────────────────────────────────────────────────────────┤
│  [enter/c] comment  [r/u] resolve/unresolve  [n] next  [?] help │  ← key bar
└─────────────────────────────────────────────────────────────────┘
```

The focused line is highlighted with a subtle full-row background. Inline
comments appear below their anchored line with `●`.

## Keybindings

| Key | Action |
|-----|--------|
| `j` / `↓`, `k` / `↑` | Move focused line |
| `J` / `K` | Jump to next / prev hunk |
| `Tab` / `Shift+Tab` | Cycle to next / prev file (includes PR Comments entry) |
| `n` / `N` | Jump to next / prev comment |
| `c` | Add inline comment on focused line; in PR Comments view, add a PR-level comment |
| `Enter` | Open inspect modal for comment under cursor |
| `r` / `u` | Resolve / unresolve focused comment |
| `o` | Review orphaned comments |
| `?` | Toggle keybinding help |
| `q` | Quit |

## Adding a Comment

1. Navigate to the line you want to comment on with `j`/`k`.
2. Press `c` to open the comment textarea.
3. Type your comment (multi-line supported).
4. Press `Ctrl+S` to submit or `Esc` to cancel.

The comment is immediately written to `comments.yaml` under a file lock.

```
  New Comment
  ┌────────────────────────────────────────────────────┐
  │ should we log the token type here for debugging?   │
  │                                                    │
  └────────────────────────────────────────────────────┘
  Ctrl+S to submit  ·  Esc to cancel
```

## PR-Level Comments

Not all feedback belongs on a specific line. The file list always includes a
virtual **PR Comments** entry (shown with `◉`) above the file list. Navigate
to it with `Shift+Tab` from the first file, or `Tab` from the last.

While the PR Comments view is active, press `c` to add a top-level comment
with no file or line anchor. Existing PR-level comments are listed in the
viewport; press `Enter` on any one to edit it.

PR-level comments appear in `sigil diff get-comments` output with an empty
`File` field and are never marked orphaned.

## Orphaned Comments

When the PR branch is force-pushed or rebased and a comment's anchor no longer
matches the new diff, it becomes orphaned. An orange banner appears at the top:

```
  ⚠  2 orphaned comment(s) — press o to review
```

Press `o` to open the orphan view. It shows the original hunk context (loaded
from the stored snapshot) alongside the comment body:

```
  Orphaned Comment 1/2
  File: internal/auth/oidc.go  ·  Author: james

  should we log the token type here for debugging?

  Original context:
  @@ -42,7 +42,12 @@
   42     if err != nil {
   43       return nil, err
  +44     return token, nil
   45     }
```

Press `r` to resolve, `u` to unresolve, `o` to cycle to the next orphan,
`Esc` to return to the main diff view.

## Agent Interface

After adding comments, hand the PR back to an agent:

```bash
# Read all open comments (token-efficient plain text)
sigil diff get-comments --open
```

Output format:
```
=== Comment <id> [open] ===
File: internal/auth/oidc.go
Hunk target: `  return token, nil`
---
should we log the token type here for debugging?
---
> @@ -42,7 +42,12 @@
>     if err != nil {
>       return nil, err
> +   return token, nil
>     }
```

After the agent addresses a comment:

```bash
sigil diff reply-comment <id> "Added token type logging in the next commit"
sigil diff resolve-comments <id>
```

## Session Storage

Sessions are stored at `$XDG_DATA_HOME/sigil/diffs/` (default:
`~/.local/share/sigil/diffs/`). Each PR gets its own directory:

```
~/.local/share/sigil/diffs/
  org/repo/
    sessions.yaml
    42/
      session.yaml          # PR metadata + snapshot history
      comments.yaml         # all comments with context anchors
      snapshots/
        <base>_<head>/
          diff.patch        # full diff at that SHA pair
          meta.yaml
```

Comments survive head SHA changes because they are anchored to surrounding
code context (2 lines before/after the target line), not to line numbers.

## Flags

```
sigil diff [--session <id>] [--draft]

  --session, -s    Skip auto-detect; load this session ID directly
  --draft          Include draft PRs in auto-detect (excluded by default)
```
