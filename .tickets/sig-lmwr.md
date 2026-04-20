---
id: sig-lmwr
status: open
deps: []
links: []
created: 2026-04-20T16:49:51Z
type: task
priority: 1
assignee: James McKernan
parent: sig-2omg
tags: [sigil, diff, parse]
---
# Unified-diff parser (sourcegraph/go-diff)

Parse unified-diff bytes into a typed per-file / per-hunk / per-line model reused by anchoring and TUI rendering. Wire sourcegraph/go-diff.

## Design

New file `diff/parse.go`. Add github.com/sourcegraph/go-diff to go.mod.

Types:
- ParsedDiff { Files []ParsedFile }
- ParsedFile { OldPath, NewPath string; IsBinary, IsRename, IsDelete, IsAdd bool; Hunks []ParsedHunk }
- ParsedHunk { OldStart, OldLines, NewStart, NewLines int; Header string; Lines []ParsedLine }
- ParsedLine { Kind LineKind; OldLineNum, NewLineNum int; Text string }  // LineKind = Context | Add | Delete

Functions:
- Parse(raw []byte) (*ParsedDiff, error)
- (*ParsedFile).IsCommentable() bool — false for binary / rename-without-content / pure delete

## Acceptance Criteria

- sourcegraph/go-diff in go.mod
- Parser emits correct Kind + line numbers per line
- IsCommentable flags binary/rename/delete correctly
- Unit tests with fixture diffs: add, rename, delete, binary, multi-hunk, multi-file
- Trailing newline handling preserved round-trip

