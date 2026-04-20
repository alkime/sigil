---
id: sig-gz4h
status: open
deps: []
links: []
created: 2026-04-20T16:49:51Z
type: task
priority: 1
assignee: James McKernan
parent: sig-2omg
tags: [sigil, diff, git]
---
# Git + worktree detection

Parse git worktree list --porcelain into a typed model; resolve origin remote to (host, owner, repo); detect non-GitHub remotes for a clean error path.

## Design

New file `diff/worktree.go`. Shell out to git via os/exec.

Functions:
- ListWorktrees(ctx) ([]Worktree, error) — parses porcelain output
- OriginRemote(ctx, cwd) (host, owner, repo string, err error) — parses `git remote get-url origin`; supports github.com https and ssh URLs; returns ErrNotGitHub for non-GH hosts
- CurrentBranch(ctx, cwd) (string, error) — `git rev-parse --abbrev-ref HEAD`; handles detached HEAD

Worktree struct: {Path, Branch, HeadSHA, IsDetached, IsBare}

Parser handles multi-block porcelain: blocks separated by blank lines, each with `worktree <path>`, `HEAD <sha>`, `branch <ref>` OR `detached`, `bare`.

## Acceptance Criteria

- diff/worktree.go implements ListWorktrees, OriginRemote, CurrentBranch
- Porcelain parser handles: multiple worktrees, detached HEAD, bare, linked
- OriginRemote parses https://github.com/..., git@github.com:..., ssh://git@github.com/...
- OriginRemote returns ErrNotGitHub for gitlab.com / bitbucket.org / self-hosted GHE (v1)
- Unit tests with captured porcelain fixtures and injected git binary via PATH

