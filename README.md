# Sigil

A terminal-based Markdown review tool. Read rendered Markdown, navigate by content blocks, and add inline review comments — all persisted within the file itself.

Designed for human-in-the-loop review of LLM-generated content: review Markdown in the TUI, add comments, hand the file back to the LLM with "address all open review comments."

## Install

### Prebuilt binaries

Grab a tarball for your platform from the [releases page](https://github.com/alkime/sigil/releases/latest). Archives are named `sigil_<version>_<os>_<arch>.tar.gz` and ship for `darwin`/`linux` × `amd64`/`arm64`.

```bash
# example: macOS Apple Silicon
tar -xzf sigil_<version>_darwin_arm64.tar.gz
cd sigil_<version>_darwin_arm64
./INSTALL.sh
```

`INSTALL.sh` verifies the bundled `sigil.sha256`, strips the macOS quarantine attribute (Gatekeeper won't prompt), and moves the binary into `~/.local/bin` if it exists. If that directory doesn't exist, it leaves the binary in place and prints a one-liner to finish the job.

<details>
<summary>Manual install (skip the script)</summary>

```bash
tar -xzf sigil_<version>_darwin_arm64.tar.gz
shasum -a 256 -c sigil.sha256          # verify
xattr -d com.apple.quarantine sigil    # macOS only — clear Gatekeeper
mv sigil ~/.local/bin/                 # or anywhere on PATH
```

`checksums.txt` on the release page covers the archives themselves; the bundled `sigil.sha256` covers the extracted binary.
</details>

### From source

```bash
go install github.com/alkime/sigil@latest
```

Or:

```bash
git clone https://github.com/alkime/sigil
cd sigil
go build .
```

## Usage

### Interactive review (TUI)

```bash
sigil <file.md>
```

Opens the file in a Glamour-rendered viewport where you can navigate by content block, select line ranges, and attach review comments. Press `?` in-app for the current keybindings.

| Key | Action |
|-----|--------|
| `j` / `↓`, `k` / `↑` | Next / previous block |
| `n` / `N` | Next / previous comment |
| `x` | Toggle multi-block selection range |
| `Enter` | Edit existing comment or add new one |
| `r` | Resolve / reopen comment |
| `d` | Delete resolved comment |
| `Ctrl+d` / `Ctrl+u` | Half-page down / up |
| `Shift+J` / `Shift+K` | Half-page down / up |
| `g` / `G` | Top / bottom of file |
| `?` | Toggle keybinding help |
| `q` | Quit |

In the comment modal: `Ctrl+S` saves, `Esc` cancels.

### Scripted / LLM-driven commands

Sigil also exposes its review state over a CLI so LLMs and scripts can read and update comments without the TUI:

```bash
# Read comments as JSON (optionally filter by status)
sigil get-comments file.md
sigil get-comments --open file.md
sigil get-comments --resolved file.md

# Mark comments resolved / unresolved by ID
sigil resolve-comments file.md 1 2 3
sigil unresolve-comments file.md 1

# Append a reply to a comment thread
sigil reply-comment file.md 1 "Fixed — see updated wording."

# Print version
sigil --version
```

IDs can be passed as plain integers (`1`) or zero-padded (`0001`) — both resolve to the same comment.

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

## PR Review with sigil diff

Review GitHub pull requests directly in the terminal. Comments are stored locally and survive force-pushes via context anchoring.

```bash
cd my-repo
git checkout feature/x

sigil diff                                          # open TUI for the open PR on this branch
sigil diff get-comments                             # plain-text dump for LLM consumption
sigil diff get-comments --open                      # only unresolved comments
sigil diff reply-comment <id> "fixed in next push"  # append a reply
sigil diff resolve-comments <id>                    # mark resolved
sigil diff unresolve-comments <id>                  # reopen
```

### TUI keybindings

| Key | Action |
|-----|--------|
| `j` / `↓`, `k` / `↑` | Navigate lines |
| `J` / `K` | Jump between hunks |
| `Tab` / `Shift+Tab` | Next / prev file (cycles through a virtual **PR Comments** entry too) |
| `n` / `N` | Next / prev comment |
| `c` | Add comment on focused line; in the PR Comments view, adds a PR-level comment |
| `Enter` | Open / edit the comment under the cursor |
| `r` / `u` | Resolve / unresolve focused comment |
| `o` | Cycle through orphaned comments |
| `?` | Toggle keybinding help |
| `q` | Quit |

### Requirements

`gh` CLI must be installed and authenticated (`gh auth login`). Comments are stored under `$XDG_DATA_HOME/sigil/diffs/` (default: `~/.local/share/sigil/diffs/`).

### LLM agent workflow

```
Human: sigil diff          → reviews PR, adds comments with c
Agent:  sigil diff get-comments → reads comments, addresses them
Agent:  sigil diff reply-comment <id> "done"
Agent:  sigil diff resolve-comments <id>
```

## LLM Workflow

1. LLM generates a Markdown document
2. Human reviews it in sigil, adding comments on blocks that need work
3. Human hands the file back to the LLM: *"I've commented on @design.md with /sigil, you can use this address them."*
4. LLM uses the skill's CLI sub-commands to reads open comments, updates the content, and resolve/reply when done
5. Repeat until satisfied

The format is designed to be natively consumable by LLMs — the comments are structured, machine-readable, and co-located with the content they reference. LLMs can drive the loop programmatically using the CLI subcommands above (`get-comments`, `reply-comment`, `resolve-comments`).

### Installing the skill

Sigil ships with a built-in skill document describing how LLM agents should use it. Print it to stdout and install it wherever your agent expects skill files:

```bash
# Claude Code, for example
mkdir -p ~/.claude/skills/sigil
sigil generate-skill > ~/.claude/skills/sigil/SKILL.md
```

Or just pipe it to a project-local file:

```bash
sigil generate-skill > SKILL.md
```

The skill covers the full CLI surface, the comment format, and the canonical review loop.

## License

MIT
