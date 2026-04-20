package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
	chroma "github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"

	"github.com/alkime/sigil/diff"
)

var (
	addStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#00CC66"))
	deleteStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444"))
	hunkStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#8888CC")).Bold(true)
	numStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	commentMarkStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAA00"))
	resolvedMarkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#669966"))
	headerStyle       = lipgloss.NewStyle().Background(lipgloss.Color("#1A1A2E")).Foreground(lipgloss.Color("#EEEEEE")).Bold(true)
	fileActiveStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4")).Bold(true)
	fileStyle              = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	prCommentsStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAA00"))
	prCommentsActiveStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAA00")).Bold(true)
	fileNavHintStyle  = lipgloss.NewStyle().Background(lipgloss.Color("#1A1A2E")).Foreground(lipgloss.Color("#AAAAAA")).Bold(true)
	keyHintKeyStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4")).Bold(true)
	keyHintDescStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
	separatorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#333333"))
	selectionStyle    = lipgloss.NewStyle().Background(lipgloss.Color("#3A3A6A"))
	statusMsgStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAA00"))
	orphanBannerStyle = lipgloss.NewStyle().Background(lipgloss.Color("#3A1A00")).Foreground(lipgloss.Color("#FFAA00")).Bold(true)
	helpTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
	contextStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))
)

// lineInfo records what each rendered line in the diff viewport represents.
type lineInfo struct {
	isHunkHeader bool
	isComment    bool
	hunkIdx      int
	lineInHunk   int
	commentID    string
	lineHint     int
	lineKind     diff.LineKind
}

// commentPos records where a comment's inline block appears in the viewport.
type commentPos struct {
	id          string
	renderedIdx int
}

// diffRenderResult holds the rendered content plus navigation metadata.
type diffRenderResult struct {
	content          string
	linesMeta        []lineInfo
	hunkStarts       []int
	commentPositions []commentPos
}

// renderDiff renders a single file's diff into a string with accompanying metadata.
func renderDiff(file diff.ParsedFile, comments []*diff.Comment, width int) diffRenderResult {
	if !file.IsCommentable() && len(file.Hunks) == 0 {
		return diffRenderResult{content: keyHintDescStyle.Render("  (no line context)")}
	}

	maxLine := 0
	for _, h := range file.Hunks {
		for _, l := range h.Lines {
			if int(l.NewLineNum) > maxLine {
				maxLine = int(l.NewLineNum)
			}
			if int(l.OldLineNum) > maxLine {
				maxLine = int(l.OldLineNum)
			}
		}
	}
	numWidth := len(fmt.Sprintf("%d", maxLine))
	if numWidth < 3 {
		numWidth = 3
	}

	ext := filepath.Ext(file.NewPath)
	if ext == "" {
		ext = filepath.Ext(file.OldPath)
	}

	fileComments := filterFileComments(comments, file)

	var lines []string
	var meta []lineInfo
	var hunkStarts []int
	var positions []commentPos

	for hi, hunk := range file.Hunks {
		hunkStarts = append(hunkStarts, len(lines))
		lines = append(lines, hunkStyle.Render(hunk.Header))
		meta = append(meta, lineInfo{isHunkHeader: true, hunkIdx: hi})

		for li, l := range hunk.Lines {
			lineStr := renderOneDiffLine(l, numWidth, ext)
			lines = append(lines, lineStr)
			meta = append(meta, lineInfo{
				hunkIdx:    hi,
				lineInHunk: li,
				lineHint:   diffLineHint(l),
				lineKind:   l.Kind,
			})

			for _, c := range fileComments {
				if commentMatchesLine(c, l) && !c.Orphaned {
					positions = append(positions, commentPos{id: c.ID, renderedIdx: len(lines)})
					lines = append(lines, renderInlineComment(c))
					meta = append(meta, lineInfo{
						isComment:  true,
						commentID:  c.ID,
						hunkIdx:    hi,
						lineInHunk: li,
					})
				}
			}
		}
	}

	return diffRenderResult{
		content:          strings.Join(lines, "\n"),
		linesMeta:        meta,
		hunkStarts:       hunkStarts,
		commentPositions: positions,
	}
}

// renderOneDiffLine renders a single diff line with line number and coloring.
func renderOneDiffLine(l diff.ParsedLine, numWidth int, ext string) string {
	switch l.Kind {
	case diff.LineAdd:
		num := numStyle.Render(fmt.Sprintf("%*d", numWidth, l.NewLineNum))
		return addStyle.Render(num+" + ") + addStyle.Render(l.Text)
	case diff.LineDelete:
		num := numStyle.Render(fmt.Sprintf("%*d", numWidth, l.OldLineNum))
		return deleteStyle.Render(num+" - ") + deleteStyle.Render(l.Text)
	default:
		num := numStyle.Render(fmt.Sprintf("%*d", numWidth, l.NewLineNum))
		text := highlightLine(l.Text, ext)
		return num + "   " + text
	}
}

// renderInlineComment renders an inline comment block below its anchored line.
func renderInlineComment(c *diff.Comment) string {
	firstLine := strings.SplitN(c.Body, "\n", 2)[0]
	var marker string
	if c.Resolved {
		marker = resolvedMarkStyle.Render("  ● ")
	} else {
		marker = commentMarkStyle.Render("  ● ")
	}
	author := commentMarkStyle.Render(c.Author + ": ")
	body := keyHintDescStyle.Render(firstLine)
	return marker + author + body
}

// renderHeader renders the top header bar.
func renderHeader(session *diff.Session, fileCount int) string {
	files := fmt.Sprintf("%d file", fileCount)
	if fileCount != 1 {
		files += "s"
	}
	content := fmt.Sprintf("  sigil diff  ·  %s  ·  PR #%d · %s  ",
		session.Repo, session.PRNumber, files)
	return headerStyle.Render(content)
}

// renderFileList renders the file navigation list.
// fileIdx == -1 means the virtual "PR Comments" entry is active.
const maxVisibleFiles = 5

func renderFileList(files []diff.ParsedFile, fileIdx int, commentCounts map[string]int, width int) string {
	// Build a unified slice of entries: index 0 = PR Comments (virtual), 1..N = files 0..N-1.
	// We use the unified index (uIdx) for windowing.
	total := len(files) + 1 // +1 for the PR Comments entry
	uIdx := fileIdx + 1      // unified index: PR Comments = 0, file i = i+1

	var sb strings.Builder

	// Compute a window of up to maxVisibleFiles centred on uIdx.
	start := uIdx - maxVisibleFiles/2
	if start < 0 {
		start = 0
	}
	end := start + maxVisibleFiles
	if end > total {
		end = total
		start = end - maxVisibleFiles
		if start < 0 {
			start = 0
		}
	}

	for u := start; u < end; u++ {
		if u == 0 {
			// PR Comments virtual entry.
			n := commentCounts[""]
			var dot string
			if n > 0 {
				dot = commentMarkStyle.Render(fmt.Sprintf(" ●%d", n))
			}
			if uIdx == 0 {
				sb.WriteString(prCommentsActiveStyle.Render("  ▸ ◉ PR Comments") + dot)
			} else {
				sb.WriteString(prCommentsStyle.Render("    ◉ PR Comments") + dot)
			}
			sb.WriteByte('\n')
			sb.WriteString(separatorStyle.Render("    · · ·"))
		} else {
			i := u - 1
			f := files[i]
			name := f.NewPath
			if f.IsDelete {
				name = f.OldPath
			}
			added, deleted := fileStats(f)
			var stats string
			if added > 0 || deleted > 0 {
				stats = fmt.Sprintf(" (+%d -%d)", added, deleted)
			}
			if !f.IsCommentable() {
				stats += " (no line context)"
			}

			n := commentCounts[name]
			var dot string
			if n > 0 {
				dot = commentMarkStyle.Render(fmt.Sprintf(" ●%d", n))
			}

			if u == uIdx {
				sb.WriteString(fileActiveStyle.Render("  ▸ "+name+stats) + dot)
			} else {
				sb.WriteString(fileStyle.Render("    "+name+stats) + dot)
			}
		}
		sb.WriteByte('\n')
	}

	if total >= 2 {
		// Display position: PR Comments = 0, files start at 1.
		// Show "PR" for the virtual entry, otherwise the file number.
		var pos string
		if uIdx == 0 {
			pos = fmt.Sprintf("PR/%d files  (Tab/S-Tab to navigate)", len(files))
		} else {
			pos = fmt.Sprintf("%d/%d files  (Tab/S-Tab to navigate)", fileIdx+1, len(files))
		}
		sb.WriteString(fileNavHintStyle.Render("  " + pos))
	} else {
		s := sb.String()
		return strings.TrimRight(s, "\n")
	}
	return sb.String()
}

// renderPRComments renders all PR-level comments (File == "") into a viewport string.
func renderPRComments(comments []*diff.Comment, width int) string {
	var prComments []*diff.Comment
	for _, c := range comments {
		if c.File == "" && !c.Orphaned {
			prComments = append(prComments, c)
		}
	}

	if len(prComments) == 0 {
		return keyHintDescStyle.Render("  No PR-level comments yet. Press C to add one.")
	}

	var lines []string
	for _, c := range prComments {
		var marker string
		if c.Resolved {
			marker = resolvedMarkStyle.Render("  ● ")
		} else {
			marker = commentMarkStyle.Render("  ● ")
		}
		author := commentMarkStyle.Render(c.Author + ": ")
		status := ""
		if c.Resolved {
			status = resolvedMarkStyle.Render(" [resolved]")
		}
		header := marker + author + keyHintDescStyle.Render(c.CreatedAt.Format("2006-01-02 15:04")) + status
		lines = append(lines, header)
		for _, bodyLine := range strings.Split(c.Body, "\n") {
			lines = append(lines, "      "+contextStyle.Render(bodyLine))
		}
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// renderKeyBar renders the bottom key hint bar.
func renderKeyBar(km KeyMap, mode Mode) string {
	var hints []string
	switch mode {
	case ModeNormal:
		hints = []string{
			keyHintRaw("enter/c", "comment"),
			keyHint(km.PRComment),
			keyHintRaw("r/u", "resolve/unresolve"),
			keyHint(km.NextComment),
			keyHint(km.PrevComment),
			keyHint(km.OrphanCycle),
			keyHint(km.Help),
			keyHint(km.Quit),
		}
	case ModeInspect:
		hints = []string{
			keyHintRaw("ctrl+s", "save"),
			keyHintRaw("esc", "cancel"),
		}
	case ModeOrphan:
		hints = []string{
			keyHintRaw("r", "resolve"),
			keyHintRaw("u", "unresolve"),
			keyHintRaw("o", "next orphan"),
			keyHintRaw("esc", "exit"),
		}
	case ModeHelp:
		hints = []string{keyHintRaw("esc/?/q", "close")}
	case ModeComment:
		hints = []string{
			keyHintRaw("ctrl+s", "submit"),
			keyHintRaw("esc", "cancel"),
		}
	}
	return "  " + strings.Join(hints, "  ")
}

func keyHint(b key.Binding) string {
	h := b.Help()
	return "[" + keyHintKeyStyle.Render(h.Key) + "] " + keyHintDescStyle.Render(h.Desc)
}

func keyHintRaw(k, d string) string {
	return "[" + keyHintKeyStyle.Render(k) + "] " + keyHintDescStyle.Render(d)
}

// renderInspectModal renders an editable view of an existing comment as a centered modal.
func renderInspectModal(c *diff.Comment, ta textarea.Model, hunkVP viewport.Model, hunkFocused bool, width, height int) string {
	if c == nil {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
			keyHintDescStyle.Render("comment not found"))
	}

	statusStr := "open"
	statusColor := lipgloss.Color("#FF8800")
	if c.Resolved {
		statusStr = "resolved"
		statusColor = lipgloss.Color("#00CC66")
	}
	titleStyle := lipgloss.NewStyle().Bold(true)
	statusStyle := lipgloss.NewStyle().Bold(true).Foreground(statusColor)
	title := titleStyle.Render(fmt.Sprintf("Comment · %s", c.File)) + "  " + statusStyle.Render(statusStr)
	meta := keyHintDescStyle.Render(fmt.Sprintf("by %s · %s", c.Author, c.CreatedAt.Format("2006-01-02 15:04")))

	sepColor := lipgloss.Color("#555555")
	sepFocused := lipgloss.Color("#7D56F4")
	sepW := min(width-10, 74)
	hunkSepColor := sepColor
	taSepColor := sepColor
	if hunkFocused {
		hunkSepColor = sepFocused
	} else {
		taSepColor = sepFocused
	}
	hunkSep := lipgloss.NewStyle().Foreground(hunkSepColor).Render(strings.Repeat("─", sepW))
	taSep := lipgloss.NewStyle().Foreground(taSepColor).Render(strings.Repeat("─", sepW))
	bottomSep := lipgloss.NewStyle().Foreground(sepColor).Render(strings.Repeat("─", sepW))

	tabHint := "  [Tab] scroll diff"
	footer := keyHintDescStyle.Render("[Ctrl+S] Save  [Esc] cancel" + tabHint)

	content := strings.Join([]string{title, meta, hunkSep, hunkVP.View(), taSep, ta.View(), bottomSep, footer}, "\n")

	modalWidth := min(width-4, 80)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Padding(1, 2).
		Width(modalWidth).
		Render(content)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

// renderCommentModal renders the new-comment input as a centered floating modal.
func renderCommentModal(ta textarea.Model, width, height int) string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF8800")).Render("New Comment")
	footer := keyHintDescStyle.Render("[Ctrl+S] Submit  [Enter] Newline  [Esc] Cancel")
	sep := lipgloss.NewStyle().Foreground(lipgloss.Color("#555555")).Render(strings.Repeat("─", min(width-10, 74)))

	content := strings.Join([]string{title, "", ta.View(), "", sep, footer}, "\n")

	modalWidth := min(width-4, 80)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Padding(1, 2).
		Width(modalWidth).
		Render(content)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

// renderHelp renders the help overlay.
func renderHelp(km KeyMap, width, height int) string {
	bindings := [][2]string{
		{km.Up.Help().Key, km.Up.Help().Desc},
		{km.Down.Help().Key, km.Down.Help().Desc},
		{km.HunkUp.Help().Key, km.HunkUp.Help().Desc},
		{km.HunkDown.Help().Key, km.HunkDown.Help().Desc},
		{km.NextFile.Help().Key, km.NextFile.Help().Desc},
		{km.NextComment.Help().Key, km.NextComment.Help().Desc},
		{km.PrevComment.Help().Key, km.PrevComment.Help().Desc},
		{km.Comment.Help().Key, km.Comment.Help().Desc},
		{km.PRComment.Help().Key, km.PRComment.Help().Desc},
		{km.Resolve.Help().Key, km.Resolve.Help().Desc},
		{km.OrphanCycle.Help().Key, km.OrphanCycle.Help().Desc},
		{km.Quit.Help().Key, km.Quit.Help().Desc},
	}

	var sb strings.Builder
	sb.WriteString(helpTitleStyle.Render("Key Bindings") + "\n\n")
	for _, b := range bindings {
		sb.WriteString(keyHintKeyStyle.Width(10).Render(b[0]) + "  " + keyHintDescStyle.Render(b[1]) + "\n")
	}
	sb.WriteString("\n" + keyHintDescStyle.Render("[?/esc/q] close"))

	boxWidth := 50
	if width-4 < boxWidth {
		boxWidth = width - 4
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Width(boxWidth).
		Render(sb.String())

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

// highlightLine applies chroma syntax highlighting to one line of code.
// Returns the original text on any failure (plain-text fallback).
func highlightLine(text, ext string) string {
	if ext == "" || text == "" {
		return text
	}
	lexer := lexers.Match("file" + ext)
	if lexer == nil {
		return text
	}
	lexer = chroma.Coalesce(lexer)
	style := styles.Get("monokai")
	if style == nil {
		return text
	}
	formatter := formatters.Get("terminal16m")
	if formatter == nil {
		return text
	}
	var buf strings.Builder
	iter, err := lexer.Tokenise(nil, text+"\n")
	if err != nil {
		return text
	}
	if err := formatter.Format(&buf, style, iter); err != nil {
		return text
	}
	return strings.TrimRight(buf.String(), "\n")
}

// filterFileComments returns comments belonging to the given file.
func filterFileComments(comments []*diff.Comment, file diff.ParsedFile) []*diff.Comment {
	var result []*diff.Comment
	for _, c := range comments {
		if c.File == file.NewPath || c.File == file.OldPath {
			result = append(result, c)
		}
	}
	return result
}

// commentMatchesLine reports whether a comment is anchored to a specific diff line.
func commentMatchesLine(c *diff.Comment, l diff.ParsedLine) bool {
	if c.LineHint <= 0 {
		return false
	}
	switch c.Side {
	case "left":
		return l.OldLineNum > 0 && int(l.OldLineNum) == c.LineHint
	default:
		return l.NewLineNum > 0 && int(l.NewLineNum) == c.LineHint
	}
}

// diffLineHint returns the relevant line number for a diff line.
func diffLineHint(l diff.ParsedLine) int {
	if l.NewLineNum > 0 {
		return int(l.NewLineNum)
	}
	return int(l.OldLineNum)
}

// fileStats counts added/deleted lines in a ParsedFile.
func fileStats(file diff.ParsedFile) (added, deleted int) {
	for _, h := range file.Hunks {
		for _, l := range h.Lines {
			switch l.Kind {
			case diff.LineAdd:
				added++
			case diff.LineDelete:
				deleted++
			}
		}
	}
	return
}
