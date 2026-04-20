# `sigil diff` — Architecture Brief

## Overview

<!-- @review-ref 0001 -->
`sigil diff` is a local-first, TUI-based code review annotation tool. It renders a PR diff in the terminal and lets the user add inline comments anchored to diff content. Comments persist locally across sessions. Humans add and edit comments through the TUI; agents (e.g. Claude Code) consume them through the same text-output subcommands as sigil's Markdown mode (`get-comments`, `reply-comment`, `resolve-comments`).

It is a subcommand of the broader `sigil` CLI.

---

## Core Design Principles

- **Local-first.** No required network calls during a review session. GH is only touched at session open (PR detection) and optionally at sync time.
- **Git context is the coordination mechanism.** No session IDs need to be passed around — the user (and any agent running in the same repo) resolve to the same session via auto-detection.
- **All state access goes through `sigil diff`.** No direct filesystem reads by consumers. The binary is the interface.
- **Comments outlive snapshots.** Sessions live at the PR level and accumulate diff snapshots as commits are pushed. Comments survive head SHA changes via context anchoring, not line numbers.

---

## Invocation

```
sigil diff [--session <id>] [--draft]
```

### Resolution order

```
sigil diff [--session <id>]
  → if --session: use that session directly
  → else: auto-detect
    → git worktree list --porcelain  (from any worktree context)
    → for each worktree: gh pr list --state open --head <branch>
    → if zero PRs found across all worktrees: error with gh pr create hint
    → if exactly one PR: use it
    → if multiple PRs: interactive picker (branch + PR title + base branch)
    → if existing session matches resolved PR: resume
    → if no session exists: create new session, snapshot diff, open TUI
```

### Flags

<!-- @review-ref 0002 -->
| Flag | Description |
|---|---|
| `--session <id>` | Skip auto-detect, use specific session |
| `--draft` | Include draft PRs in auto-detect (excluded by default) |

Auto-detect resolves identically for human TUI invocations and agent subcommand invocations (see [Agent Interface](#agent-interface) below) — git context is the shared state.

---

## Storage Layout

Base path: `~/.local/share/sigil/diffs/`  
<!-- @review-ref 0003 -->
(Or `$XDG_DATA_HOME/sigil/diffs/` if that env var is set — follows the [XDG Base Directory spec](https://specifications.freedesktop.org/basedir-spec/), a freedesktop.org convention that lets users relocate app data via an env var. Common for CLI tools on Linux; we respect it cross-platform for consistency.)

```
~/.local/share/sigil/diffs/
  <org>/<repo>/
    sessions.yaml                        # index of all sessions for this repo
    <pr-number>/
      session.yaml                       # PR metadata, current head SHA, timestamps
      snapshots/
        <base-sha>_<head-sha>/
          diff.patch                     # full diff snapshot at this SHA pair
          meta.yaml                      # when observed, branch, head commit message
      comments.yaml                      # all comments; anchor via context not line num
```
<!-- @review-ref 0004 -->

### `session.yaml`

```yaml
id: uuid
repo: org/repo
pr_number: 42
pr_title: "feat: add oidc token refresh"
base_branch: main
base_sha: abc123
head_sha: def456          # updated when new snapshot is observed
branch: feature/oidc
created_at: 2026-04-19T10:00:00Z
updated_at: 2026-04-19T14:30:00Z
snapshots:
  - base: abc123
    head: def456
    observed_at: 2026-04-19T10:00:00Z
  - base: abc123
    head: ghi789
    observed_at: 2026-04-19T14:30:00Z
```

### `comments.yaml`

```yaml
- id: <uuid>
  file: internal/auth/oidc.go
  hunk_header: "@@ -42,7 +42,12 @@"     # orientation hint, not anchor
  line_hint: 47                           # last known line number, informational only
  side: right                             # left=old/deleted, right=new/added
  context:                                # TRUE anchor — survives line number drift
    before:
      - "  if err != nil {"
      - "    return nil, err"
    target: "  return token, nil"         # the line being commented on
    after:
      - "}"
      - ""
  body: "should we be logging the token type here?"
  author: james                           # or "claude-code", etc.
  created_at: 2026-04-19T10:30:00Z
  updated_at: 2026-04-19T10:30:00Z
  tags: [question, needs-discussion]
  resolved: false
  orphaned: false                         # true if context match failed on re-anchor
  snapshot_ref: abc123_def456            # which snapshot this was written against
```

---

## Drift Handling (Head SHA Advances)

When `sigil diff` opens and detects the PR's head SHA has advanced since the last snapshot:

1. Capture new snapshot: `git diff <base>..<new-head>` → write to `snapshots/<new-pair>/`
2. For each existing comment, attempt context re-anchor against new diff:
   - Search new diff for a hunk containing `context.target` with matching `context.before`/`after`
   - If found: update `line_hint`, clear `orphaned`
   - If not found: set `orphaned: true`
3. Update `head_sha` in `session.yaml`
4. Present orphaned comments to user at TUI open (summary banner, navigable)

Context windows of 2 lines before/after are sufficient for most cases. The stored `diff.patch` snapshot means re-anchoring can always be done against the exact diff the comment was written against, even if the branch is later rebased or force-pushed.

---

## TUI Design

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) + [Lip Gloss](https://github.com/charmbracelet/lipgloss).

### Layout

```
┌─────────────────────────────────────────────────────┐
│ sigil diff  ·  org/repo  ·  PR #42  ·  3 files      │  ← header bar
├─────────────────────────────────────────────────────┤
│ ▸ internal/auth/oidc.go (+42 -8)                    │  ← file list / nav
│   internal/auth/token.go (+5 -0)                    │
│   internal/middleware/auth.go (+12 -3)              │
├─────────────────────────────────────────────────────┤
│  42   if err != nil {                               │  ← diff viewport
│  43     return nil, err                             │    (unified, scrollable)
│  44 + return token, nil                             │
│  45   }                                             │
│                                                     │
│  ● james: should we be logging the token type?      │  ← inline comment
│                                                     │
├─────────────────────────────────────────────────────┤
│ [c] comment  [r] resolve  [n] next  [?] help        │  ← key hint bar
└─────────────────────────────────────────────────────┘
```

### Key bindings

| Key | Action |
|---|---|
| `j/k` or `↑/↓` | Navigate lines |
| `J/K` | Jump between hunks |
| `Tab` | Next file |
| `c` | Add comment on current line |
| `r` | Toggle resolve on focused comment |
| `n/N` | Next/prev comment |
| `o` | Next orphaned comment |
| `q` | Quit |
| `?` | Help |

### Comment entry

Opens an inline editor (single or multi-line) at the focused line. Esc cancels. Enter/submit writes to `comments.yaml` immediately — no buffering.

---

## Diff Rendering Stack

| Layer | Library |
|---|---|
| Diff computation | [`go-udiff`](https://github.com/aymanbagabas/go-udiff) (aymanbagabas) |
| Diff parsing (unified) | [`sourcegraph/go-diff`](https://github.com/sourcegraph/go-diff) |
| Syntax highlighting | [`chroma/v2`](https://github.com/alecthomas/chroma) |
| Layout + styling | [`lipgloss`](https://github.com/charmbracelet/lipgloss) |
| Scrollable viewport | [`bubbles/viewport`](https://github.com/charmbracelet/bubbles) |
| Text input | [`bubbles/textarea`](https://github.com/charmbracelet/bubbles) |

`glamour` is not used here — diff rendering is custom. Glamour is for markdown elsewhere in sigil.

---

## Agent Interface

Agents (Claude Code, scripts) interact with `sigil diff` through subcommands that mirror sigil's existing Markdown workflow. Output is plain text, not JSON — matching `sigil get-comments` elsewhere in the CLI, keeping output token-efficient for LLM consumption.

<!-- @review-ref 0005 -->
### Subcommands

```bash
# List comments on the auto-detected session
sigil diff get-comments [--open|--resolved] [--session <id>]

# Reply to an existing comment
sigil diff reply-comment [--session <id>] <comment-id> "..."

# Resolve / unresolve comments
sigil diff resolve-comments [--session <id>] <comment-id> [<comment-id>...]
sigil diff unresolve-comments [--session <id>] <comment-id> [<comment-id>...]
```

Session auto-detection (from git worktree context) is shared with the TUI — no session ID needs to be passed between human and agent invocations.

### Output format

`sigil diff get-comments` emits the same block-text format as the Markdown mode's `get-comments`, with a diff-context quote block per comment (file path + hunk target line, so the agent can locate the anchor without rereading the full diff).

```
=== Comment 0001 [open] ===
File: internal/auth/oidc.go
Hunk target: `  return token, nil`
---
should we be logging the token type here?
---
> @@ -42,7 +42,12 @@
>     if err != nil {
>       return nil, err
> +   return token, nil
>     }
```

### MVP: read-only for agents

**Agents cannot create new comments in the MVP.** Only humans add comments via the TUI; agents consume, reply to, and resolve them. Agent-authored comment creation is deferred (see Future / Deferred) — the initial workflow matches sigil's Markdown mode: human reviews, agent addresses.

---

## Worktree Handling

`sigil diff` calls `git worktree list --porcelain` from whatever directory it's invoked from. This always returns the full list of worktrees for the repo regardless of which worktree you're currently in. Each worktree's branch is checked against open GH PRs. This means:

- Running `sigil diff` from any worktree shows all reviewable PRs across the repo
- The picker (when needed) shows worktree path + branch + PR title so the user can orient

---

## GH Integration

Uses the `gh` CLI as a subprocess. No direct GH API calls, no auth management in sigil itself.

```
gh pr list --repo <org/repo> --state open --head <branch> --json number,title,baseRefName
gh pr diff <number>     # used to get canonical diff if preferred over git diff
```

Requires `gh` to be installed and authenticated. Error message guides user if not.

Future: `sigil diff --sync` posts local comments to GH PR review via `gh api`. Deferred until team workflow requires it.

---

## Future / Deferred

- **Agent-authored comments** — a subcommand (e.g. `sigil diff add-comment`) that lets an LLM propose comments anchored to diff content. Deferred for MVP; the initial workflow is human-writes → agent-addresses, matching sigil's Markdown mode.
- **GH sync** — `sigil diff --sync` posts comments as a GH PR review batch
- **Session cleanup / compaction** — after some threshold of merged or closed sessions accumulates on disk (say, 25+), prune or archive old ones so `sessions.yaml` stays tractable. Not urgent for MVP; revisit once accumulation becomes visible in practice.
- **MCP server mode** — expose `list_comments`, `reply_comment`, `resolve_comment` as MCP tools for richer Claude Code integration (once agent-authored comments land, `add_comment` joins this set).
- **Multi-reviewer** — `author` field already supports it; UI for filtering by author deferred.
- **Comment threads** — reply chains; current model is flat comments only.
- **Draft PR support** — `--draft` flag exists in spec; detection logic deferred.

<!--
@review-backmatter

"0001":
  offset: 1
  span: 2
  comment: "actually i think for MVP we don't want LLMs / Claude Code to add comments. maybe later but for now let's just keep to the standard sigil workflow of get-comment/reply-comment/resolve-comment subs.\n\nREPLY: Good call — aligned with the Markdown workflow. Reworded the overview, core principles, and flags to drop the 'agent writes comments' framing. MVP is now: humans write via TUI, agents consume via get-comments / reply-comment / resolve-comments subcommands. Added 'Agent-authored comments' to Future/Deferred so we don't lose the idea."
  status: resolved

"0002":
  offset: 1
  span: 7
  comment: "In the other sigil commands to get-comments we try to steer clear of JSON since it can waste tokens. I think we should continue with the same thing here. no --json really needed i don't think?\n\nREPLY: Agreed. Dropped --json entirely from the flags table and the invocation example. Rewrote the 'JSON Output' section as an 'Agent Interface' section that uses the same text block format as sigil get-comments elsewhere. Subcommands are sigil diff get-comments / reply-comment / resolve-comments, mirroring Markdown mode."
  status: resolved

"0003":
  offset: 1
  span: 1
  comment: "is XDG_DATA_HOME a common idiom? not familiar with it if so ...\n\nREPLY: Expanded the note. XDG_DATA_HOME is a freedesktop.org Linux convention (XDG Base Directory spec) that lets users relocate app data via an env var — default is ~/.local/share. It's common in Linux CLI tools and harmless on macOS (we fall back to the literal ~/.local/share/sigil/diffs/ when unset). Linked the spec for reference."
  status: resolved

"0004":
  offset: 1
  span: 1
  comment: "the sessions file might grow unweildy after a while, should we include a cleanup / compaction mechanism, like after 25 merged sessions we should basically remove old ones? actually this is probably a v2 sorta thing but worth mentioning and planning for\n\nREPLY: Agreed — v2 concern, but worth capturing. Added 'Session cleanup / compaction' to Future/Deferred with the 25-session threshold idea as a starting point. Flagged as non-urgent; we'll revisit once accumulation is visible in practice."
  status: resolved

"0005":
  offset: 1
  span: 1
  comment: "just want to note again JSON isn't what we want here, let's try to make it more token conscious.\n\nREPLY: Covered by the rewrite for 0002 — the whole --json section is gone, replaced with a plain text-block agent interface that matches sigil get-comments output. No JSON on the read path, no JSON on a write path (since agents aren't writing in MVP)."
  status: resolved

-->
