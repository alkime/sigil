---
id: sig-44hb
status: closed
deps: [sig-1q2f]
links: []
created: 2026-03-25T13:56:15Z
type: task
priority: 2
assignee: James McKernan
parent: sig-hs1z
tags: [cli-foundation]
---
# CLI foundation tests

Context: Verify Kong parsing routes correctly and NormalizeID works.

Approach:
Create cli/cli_test.go with:
- TestNormalizeID: "1"->"0001", "0001"->"0001", "42"->"0042", "abc"->"abc"
- TestKongParsing: verify empty args routes to TUI, "get-comments file.md" routes to GetComments, invalid subcommand errors

Key files: Create cli/cli_test.go
Follow test patterns from writer/writer_test.go.

Verification: go test ./cli/... passes

