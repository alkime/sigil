---
id: sig-cn3r
status: closed
deps: [sig-44hb, sig-1eaj, sig-1d6u, sig-mqtx, sig-8nlr]
links: []
created: 2026-03-25T13:56:28Z
type: task
priority: 2
assignee: James McKernan
parent: sig-hs1z
tags: [cli-subcommands]
---
# Comprehensive CLI subcommand tests

Context: All subcommands need test coverage using temp files with known content.

Tests (14+):
1. TestGetComments_All, _OpenFilter, _ResolvedFilter, _NoComments, _SourceLines
2. TestResolveComments_Single, _Multiple, _NotFound, _NormalizedID
3. TestUnresolveComments
4. TestReplyComment, _PreservesStatus, _NotFound
5. TestInstallSkill

Pattern: writeTempFile, parse, run subcommand with bytes.Buffer, verify output/re-parse.

Key files: Extend cli/cli_test.go

Verification: go test ./cli/... && go test -race ./cli/...

