package diff

import (
	"fmt"
	"strings"

	godiff "github.com/sourcegraph/go-diff/diff"
)

type LineKind int

const (
	LineContext LineKind = iota
	LineAdd
	LineDelete
)

type ParsedLine struct {
	Kind       LineKind
	OldLineNum int32
	NewLineNum int32
	Text       string
}

type ParsedHunk struct {
	OldStart int32
	OldLines int32
	NewStart int32
	NewLines int32
	Header   string
	Lines    []ParsedLine
}

type ParsedFile struct {
	OldPath  string
	NewPath  string
	IsBinary bool
	IsRename bool
	IsDelete bool
	IsAdd    bool
	Hunks    []ParsedHunk
}

type ParsedDiff struct {
	Files []ParsedFile
}

func (f *ParsedFile) IsCommentable() bool {
	if f.IsBinary {
		return false
	}
	if f.IsDelete {
		return false
	}
	if f.IsRename && len(f.Hunks) == 0 {
		return false
	}
	return true
}

func Parse(raw []byte) (*ParsedDiff, error) {
	files, err := godiff.ParseMultiFileDiff(raw)
	if err != nil {
		return nil, fmt.Errorf("parsing diff: %w", err)
	}

	pd := &ParsedDiff{Files: make([]ParsedFile, 0, len(files))}
	for _, f := range files {
		pf, err := convertFile(f)
		if err != nil {
			return nil, err
		}
		pd.Files = append(pd.Files, pf)
	}
	return pd, nil
}

func convertFile(f *godiff.FileDiff) (ParsedFile, error) {
	pf := ParsedFile{
		OldPath:  stripGitPrefix(f.OrigName, "a/"),
		NewPath:  stripGitPrefix(f.NewName, "b/"),
		IsAdd:    f.OrigName == "/dev/null",
		IsDelete: f.NewName == "/dev/null",
	}

	for _, ext := range f.Extended {
		lower := strings.ToLower(ext)
		if strings.Contains(lower, "binary files") || strings.Contains(ext, "GIT binary patch") {
			pf.IsBinary = true
		}
		if strings.HasPrefix(ext, "rename from ") {
			pf.IsRename = true
		}
	}

	pf.Hunks = make([]ParsedHunk, 0, len(f.Hunks))
	for _, h := range f.Hunks {
		ph, err := convertHunk(h)
		if err != nil {
			return pf, err
		}
		pf.Hunks = append(pf.Hunks, ph)
	}
	return pf, nil
}

func convertHunk(h *godiff.Hunk) (ParsedHunk, error) {
	ph := ParsedHunk{
		OldStart: h.OrigStartLine,
		OldLines: h.OrigLines,
		NewStart: h.NewStartLine,
		NewLines: h.NewLines,
	}

	if h.Section != "" {
		ph.Header = fmt.Sprintf("@@ -%d,%d +%d,%d @@ %s", h.OrigStartLine, h.OrigLines, h.NewStartLine, h.NewLines, h.Section)
	} else {
		ph.Header = fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.OrigStartLine, h.OrigLines, h.NewStartLine, h.NewLines)
	}

	oldLine := h.OrigStartLine
	newLine := h.NewStartLine

	body := string(h.Body)
	lines := strings.Split(body, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		prefix := line[0]
		text := line[1:]

		switch prefix {
		case ' ':
			ph.Lines = append(ph.Lines, ParsedLine{Kind: LineContext, OldLineNum: oldLine, NewLineNum: newLine, Text: text})
			oldLine++
			newLine++
		case '-':
			ph.Lines = append(ph.Lines, ParsedLine{Kind: LineDelete, OldLineNum: oldLine, NewLineNum: 0, Text: text})
			oldLine++
		case '+':
			ph.Lines = append(ph.Lines, ParsedLine{Kind: LineAdd, OldLineNum: 0, NewLineNum: newLine, Text: text})
			newLine++
		case '\\':
			// "\ No newline at end of file" — skip
		default:
			return ph, fmt.Errorf("unexpected hunk line prefix %q in: %q", prefix, line)
		}
	}
	return ph, nil
}

func stripGitPrefix(name, prefix string) string {
	if name == "/dev/null" {
		return name
	}
	return strings.TrimPrefix(name, prefix)
}
