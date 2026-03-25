# Sigil

A terminal-based Markdown review tool. Read rendered Markdown, navigate by content blocks, and add inline review comments — all persisted within the file itself.

Designed for human-in-the-loop review of LLM-generated content: review Markdown in the TUI, add comments, hand the file back to the LLM with "address all open review comments."

## Install

```bash
go install github.com/alkime/sigil@latest
```

Or build from source:

```bash
git clone https://github.com/alkime/sigil
cd sigil
go build .
```

## Usage

```bash
sigil <file.md>
```

### Keybindings

| Key | Action |
|-----|--------|
| `j` / `k` | Navigate between content blocks |
| `n` / `N` | Jump to next / previous comment |
| `Enter` | Edit existing comment or add new one |
| `r` | Resolve / reopen comment |
| `d` | Delete comment (resolves first, then confirms) |
| `Ctrl+S` | Save comment text (in edit modal) |
| `Esc` | Close modal / cancel |
| `d` / `u` | Half-page down / up (when not on comment) |
| `g` / `G` | Top / bottom of file |
| `?` | Show keybinding help |
| `q` | Quit |

## Comment Format

Comments are stored as structured data within the Markdown file using HTML comments, invisible to standard renderers.

### Inline Ref Markers

```markdown
<!-- @review-ref 0001 -->
```

Placed on the line above the content being commented on. IDs are zero-padded 4-digit integers.

### Backmatter Block

A YAML block inside an HTML comment at the end of the file:

```markdown
<!--
@review-backmatter

"0001":
  offset: 1
  span: 4
  comment: "This undersells the OAuth complexity."
  status: open
-->
```

| Field | Type | Description |
|-------|------|-------------|
| `offset` | int | Lines below the ref marker where the highlight starts |
| `span` | int | Number of source lines the comment covers |
| `comment` | string | The review comment text |
| `status` | string | `open` or `resolved` |

### Full Example

```markdown
# Architecture Design

<!-- @review-ref 0001 -->
The system uses a simple token-based auth flow
where users authenticate via a shared secret
that is passed in the Authorization header
on every request.

<!-- @review-ref 0002 -->
## Database Schema

We use a single `users` table with no indexes.

## Deployment

Standard Docker-based deployment to fly.io.

<!--
@review-backmatter

"0001":
  offset: 1
  span: 4
  comment: "This undersells the OAuth complexity. Expand with redirect flow details."
  status: open

"0002":
  offset: 1
  span: 1
  comment: "Missing the indexes discussion entirely."
  status: open
-->
```

## LLM Workflow

1. LLM generates a Markdown document
2. Human reviews it in sigil, adding comments on blocks that need work
3. Human hands the file back to the LLM: *"Address all open review comments"*
4. LLM reads the ref markers and backmatter, updates the content, resolves comments
5. Repeat until satisfied

The format is designed to be natively consumable by LLMs — the comments are structured, machine-readable, and co-located with the content they reference.

## License

MIT
