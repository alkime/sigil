---
id: sig-kota
status: open
deps: []
links: []
created: 2026-04-20T16:47:51Z
type: task
priority: 1
assignee: James McKernan
parent: sig-2omg
tags: [sigil, diff, storage]
---
# Storage layer: types + YAML I/O + XDG paths

Core storage primitives for sigil diff: Session/Snapshot/Comment/SessionsIndexEntry types, YAML read/write, XDG-aware base path, flock wrapper for concurrent comments.yaml writes.

## Design

Create new `diff/` package. Files:
- `diff/types.go` — Session, Snapshot, Comment (diff-mode), SessionsIndexEntry
- `diff/storage.go` — base path resolution ($XDG_DATA_HOME/sigil/diffs/ or ~/.local/share/sigil/diffs/), session/snapshot directory lookup, YAML read/write
- `diff/flock.go` — advisory file lock wrapper for comments.yaml writes
- `diff/sessions_index.go` — read/write sessions.yaml at <org>/<repo>/ level

Comment type fields (per brief): id, file, hunk_header, line_hint, side (left|right), context {before[], target, after[]}, body, author, created_at, updated_at, tags[], resolved, orphaned, snapshot_ref.

Session type fields: id, repo, pr_number, pr_title, base_branch, base_sha, head_sha, branch, created_at, updated_at, snapshots [{base, head, observed_at}].

SessionsIndexEntry: {pr_number, path, updated_at}.

No deps beyond gopkg.in/yaml.v3 (already in go.mod) and stdlib.

## Acceptance Criteria

- diff/types.go defines all 4 types with yaml tags matching brief schema
- diff/storage.go: BasePath() returns $XDG_DATA_HOME/sigil/diffs/ or ~/.local/share/sigil/diffs/ fallback; SessionDir(org, repo, pr), SnapshotDir(org, repo, pr, base, head) return deterministic paths
- diff/storage.go: LoadSession, SaveSession, LoadComments, SaveComments with YAML round-trip
- diff/flock.go: WithLock(path, fn) acquires syscall.Flock, runs fn, releases on return or panic
- diff/sessions_index.go: LoadIndex(org, repo), SaveIndex(org, repo, entries)
- Unit tests cover: YAML round-trip, XDG override, flock contention (two goroutines), missing-file creation, partial-write safety
- go vet ./diff/... clean

