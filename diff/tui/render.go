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
	addBgStyle        = lipgloss.NewStyle().Background(lipgloss.Color("#142D17"))
	deleteBgStyle     = lipgloss.NewStyle().Background(lipgloss.Color("#2D1414"))
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
	lines            []string
	linesMeta        []lineInfo
	hunkStarts       []int
	commentPositions []commentPos
	numWidth         int
	ext              string
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
		lines:            lines,
		linesMeta:        meta,
		hunkStarts:       hunkStarts,
		commentPositions: positions,
		numWidth:         numWidth,
		ext:              ext,
	}
}

// cursorStyle paints the cursor cell on the focused line. Explicit fg/bg
// (not Reverse()) so it remains visible on top of selectionStyle's background.
var cursorStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("#EEEEEE")).
	Foreground(lipgloss.Color("#1A1A2E"))

// renderFocusedLineWithCursor re-renders a single diff line with the cursor
// drawn at focusedCol (rune index into l.Text). Skips chroma highlighting on
// the focused line so the cursor cell stays predictable. The line background
// is supplied separately via vp.StyleLineFunc.
func renderFocusedLineWithCursor(l diff.ParsedLine, numWidth, focusedCol int) string {
	var prefix string
	bodyStyle := lipgloss.NewStyle()
	switch l.Kind {
	case diff.LineAdd:
		num := numStyle.Render(fmt.Sprintf("%*d", numWidth, l.NewLineNum))
		prefix = num + addStyle.Render(" + ")
	case diff.LineDelete:
		num := numStyle.Render(fmt.Sprintf("%*d", numWidth, l.OldLineNum))
		prefix = num + deleteStyle.Render(" - ")
	default:
		num := numStyle.Render(fmt.Sprintf("%*d", numWidth, l.NewLineNum))
		prefix = num + "   "
	}

	runes := []rune(l.Text)
	if len(runes) == 0 {
		return prefix + cursorStyle.Render(" ")
	}
	col := focusedCol
	if col < 0 {
		col = 0
	}
	if col >= len(runes) {
		col = len(runes) - 1
	}

	before := bodyStyle.Render(string(runes[:col]))
	cursor := cursorStyle.Render(string(runes[col]))
	after := bodyStyle.Render(string(runes[col+1:]))
	return prefix + before + cursor + after
}

// renderOneDiffLine renders a single diff line with line number and coloring.
// Add/delete lines get a green/red +/- sign prefix; the body is syntax-highlighted
// like context lines. The subtle add/delete line background is applied by the
// viewport's StyleLineFunc (see model.buildView). The background does not
// fill continuously across every embedded ANSI reset — line numbers and right
// padding show it, but the syntax-colored body and +/- prefix fall back to
// the terminal default between resets. Accepting that for now.
func renderOneDiffLine(l diff.ParsedLine, numWidth int, ext string) string {
	switch l.Kind {
	case diff.LineAdd:
		num := numStyle.Render(fmt.Sprintf("%*d", numWidth, l.NewLineNum))
		text := highlightLine(l.Text, ext)
		return num + addStyle.Render(" + ") + text
	case diff.LineDelete:
		num := numStyle.Render(fmt.Sprintf("%*d", numWidth, l.OldLineNum))
		text := highlightLine(l.Text, ext)
		return num + deleteStyle.Render(" - ") + text
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

func renderFileList(files []diff.ParsedFile, fileIdx int, commentCounts map[string]int, width int, reviewOrder *diff.ReviewOrder, useCustomOrder bool) string {
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
			sb.WriteString(fileStyle.Render("    ─────────────────"))
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
				active := fileActiveStyle.Render("  ▸ "+name+stats) + dot
				sb.WriteString(active)
				if useCustomOrder && reviewOrder != nil {
					if note := reviewOrder.NoteFor(f); note != "" {
						used := lipgloss.Width(active)
						blurb := renderBlurb(note, width-used)
						if blurb != "" {
							sb.WriteString(blurb)
						}
					}
				}
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

// renderPRComments renders all PR-level comments (File == "") and returns
// content + linesMeta so Enter can open the inspect modal on any comment line.
func renderPRComments(comments []*diff.Comment, width int) diffRenderResult {
	var prComments []*diff.Comment
	for _, c := range comments {
		if c.File == "" && !c.Orphaned {
			prComments = append(prComments, c)
		}
	}

	if len(prComments) == 0 {
		return diffRenderResult{content: keyHintDescStyle.Render("  No PR-level comments yet. Press c to add one.")}
	}

	var lines []string
	var meta []lineInfo
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
		commentLine := lineInfo{isComment: true, commentID: c.ID}
		lines = append(lines, header)
		meta = append(meta, commentLine)
		for _, bodyLine := range strings.Split(c.Body, "\n") {
			lines = append(lines, "      "+contextStyle.Render(bodyLine))
			meta = append(meta, commentLine)
		}
		lines = append(lines, "")
		meta = append(meta, lineInfo{})
	}

	return diffRenderResult{content: strings.Join(lines, "\n"), linesMeta: meta}
}

// renderBlurb formats a review-order note for inline display next to the
// active file. Returns "" when there isn't enough room for even a short tail.
// The leading " — " is treated as decoration; when the note must be shortened,
// it ends with a single "…".
func renderBlurb(note string, avail int) string {
	const prefix = " — "
	const minUseful = 8 // prefix + a few chars worth showing
	if avail < minUseful+len(prefix) {
		return ""
	}
	budget := avail - len(prefix)
	runes := []rune(note)
	if len(runes) > budget {
		if budget <= 1 {
			return ""
		}
		runes = append(runes[:budget-1], '…')
	}
	return keyHintDescStyle.Render(prefix + string(runes))
}

// renderKeyBar renders the bottom key hint bar, progressively dropping hints
// from the right when they don't fit within width. Ordered with most essential
// hints first (help + quit) so narrow terminals stay discoverable.
func renderKeyBar(km KeyMap, mode Mode, width int, hasReviewOrder bool) string {
	var hints []string
	switch mode {
	case ModeNormal:
		hints = []string{
			keyHint(km.Help),
			keyHint(km.Quit),
			keyHintRaw("enter/c", "comment"),
			keyHint(km.NextComment),
			keyHintRaw("r/u", "resolve/unresolve"),
			keyHintRaw("gd", "go to def"),
			keyHintRaw("ctrl+o", "jump back"),
			keyHint(km.OrphanCycle),
		}
		if hasReviewOrder {
			hints = append(hints, keyHint(km.ToggleOrder))
		}
	case ModeInspect:
		hints = []string{
			keyHintRaw("ctrl+s", "save"),
			keyHintRaw("esc", "cancel"),
		}
	case ModeOrphan:
		hints = []string{
			keyHintRaw("esc", "exit"),
			keyHintRaw("r", "resolve"),
			keyHintRaw("u", "unresolve"),
			keyHintRaw("o", "next orphan"),
		}
	case ModeHelp:
		hints = []string{keyHintRaw("esc/?/q", "close")}
	case ModeComment:
		hints = []string{
			keyHintRaw("ctrl+s", "submit"),
			keyHintRaw("esc", "cancel"),
		}
	case ModeDefinition:
		hints = []string{
			keyHintRaw("q/esc", "back"),
			keyHintRaw("j/k", "scroll"),
			keyHintRaw("h/l/w/b/e", "cursor"),
			keyHintRaw("gd", "go to def"),
			keyHintRaw("ctrl+o", "jump back"),
		}
	}
	return packHints(hints, width)
}

// packHints left-pads 2 and joins with 2-space separators, dropping entries
// from the right until the total fits in width. width <= 0 means no clipping.
func packHints(hints []string, width int) string {
	const leftPad = 2
	const sep = 2
	if width <= 0 {
		return strings.Repeat(" ", leftPad) + strings.Join(hints, strings.Repeat(" ", sep))
	}
	var out []string
	used := leftPad
	for i, h := range hints {
		add := lipgloss.Width(h)
		if i > 0 {
			add += sep
		}
		if used+add > width {
			break
		}
		out = append(out, h)
		used += add
	}
	return strings.Repeat(" ", leftPad) + strings.Join(out, strings.Repeat(" ", sep))
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
	type group struct {
		title    string
		bindings [][2]string
	}
	groups := []group{
		{
			title: "Navigation",
			bindings: [][2]string{
				{km.Up.Help().Key, km.Up.Help().Desc},
				{km.Down.Help().Key, km.Down.Help().Desc},
				{km.HunkUp.Help().Key, km.HunkUp.Help().Desc},
				{km.HunkDown.Help().Key, km.HunkDown.Help().Desc},
				{km.HalfPageUp.Help().Key, km.HalfPageUp.Help().Desc},
				{km.HalfPageDown.Help().Key, km.HalfPageDown.Help().Desc},
				{km.NextFile.Help().Key, km.NextFile.Help().Desc},
				{km.NextComment.Help().Key, km.NextComment.Help().Desc},
				{km.PrevComment.Help().Key, km.PrevComment.Help().Desc},
			},
		},
		{
			title: "Cursor",
			bindings: [][2]string{
				{km.ColLeft.Help().Key, km.ColLeft.Help().Desc},
				{km.ColRight.Help().Key, km.ColRight.Help().Desc},
				{km.WordNext.Help().Key, km.WordNext.Help().Desc},
				{km.WordPrev.Help().Key, km.WordPrev.Help().Desc},
				{km.WordEnd.Help().Key, km.WordEnd.Help().Desc},
			},
		},
		{
			title: "Definition",
			bindings: [][2]string{
				{km.GoToDef.Help().Key, km.GoToDef.Help().Desc},
				{km.GoToDefAlt.Help().Key, km.GoToDefAlt.Help().Desc},
				{km.JumpBack.Help().Key, km.JumpBack.Help().Desc},
				{"q/esc", "close definition viewer"},
			},
		},
		{
			title: "Actions",
			bindings: [][2]string{
				{km.Comment.Help().Key, km.Comment.Help().Desc},
				{km.Resolve.Help().Key, km.Resolve.Help().Desc},
				{km.OrphanCycle.Help().Key, km.OrphanCycle.Help().Desc},
				{km.ToggleOrder.Help().Key, km.ToggleOrder.Help().Desc},
				{km.Quit.Help().Key, km.Quit.Help().Desc},
			},
		},
	}

	var sb strings.Builder
	sb.WriteString(helpTitleStyle.Render("Key Bindings") + "\n")
	for _, g := range groups {
		sb.WriteString("\n" + helpTitleStyle.Render(g.title) + "\n")
		for _, b := range g.bindings {
			sb.WriteString(keyHintKeyStyle.Width(10).Render(b[0]) + "  " + keyHintDescStyle.Render(b[1]) + "\n")
		}
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
