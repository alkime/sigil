package diff

type Side int

const (
	Left  Side = 0
	Right Side = 1
)

func sideFromString(s string) Side {
	if s == "left" {
		return Left
	}
	return Right
}

func visibleLine(l ParsedLine, side Side) bool {
	switch side {
	case Left:
		return l.Kind == LineContext || l.Kind == LineDelete
	default:
		return l.Kind == LineContext || l.Kind == LineAdd
	}
}

// BuildAnchor extracts a CommentContext for the line at lineIdx in hunk.
// side determines which lines are visible (Left=context+delete, Right=context+add)
// and trims context at hunk edges.
func BuildAnchor(hunk *ParsedHunk, lineIdx int, side Side) CommentContext {
	type vline struct {
		hunkIdx int
		text    string
	}
	var visible []vline
	for i, l := range hunk.Lines {
		if visibleLine(l, side) {
			visible = append(visible, vline{i, l.Text})
		}
	}

	pos := -1
	for i, v := range visible {
		if v.hunkIdx == lineIdx {
			pos = i
			break
		}
	}
	if pos < 0 {
		return CommentContext{Target: hunk.Lines[lineIdx].Text}
	}

	start := pos - 2
	if start < 0 {
		start = 0
	}
	before := make([]string, 0, pos-start)
	for i := start; i < pos; i++ {
		before = append(before, visible[i].text)
	}

	end := pos + 3
	if end > len(visible) {
		end = len(visible)
	}
	after := make([]string, 0, end-pos-1)
	for i := pos + 1; i < end; i++ {
		after = append(after, visible[i].text)
	}

	return CommentContext{Before: before, Target: visible[pos].text, After: after}
}

// ReAnchor searches newDiff for a hunk matching comment's context anchor.
// Returns matched=true and the updated line hint + hunk header on success.
func ReAnchor(comment *Comment, newDiff *ParsedDiff) (matched bool, newLineHint int, hunkHeader string) {
	side := sideFromString(comment.Side)
	ctx := comment.Context

	for _, file := range newDiff.Files {
		if file.NewPath != comment.File && file.OldPath != comment.File {
			continue
		}
		for _, hunk := range file.Hunks {
			type vline struct {
				text       string
				oldLineNum int32
				newLineNum int32
			}
			var visible []vline
			for _, l := range hunk.Lines {
				if visibleLine(l, side) {
					visible = append(visible, vline{l.Text, l.OldLineNum, l.NewLineNum})
				}
			}

			nb := len(ctx.Before)
			for i, vl := range visible {
				if vl.text != ctx.Target {
					continue
				}
				if nb > i {
					continue
				}
				ok := true
				for j, b := range ctx.Before {
					if visible[i-nb+j].text != b {
						ok = false
						break
					}
				}
				if !ok {
					continue
				}
				for j, a := range ctx.After {
					if i+1+j >= len(visible) || visible[i+1+j].text != a {
						ok = false
						break
					}
				}
				if !ok {
					continue
				}
				var hint int
				if side == Left {
					hint = int(vl.oldLineNum)
				} else {
					hint = int(vl.newLineNum)
				}
				return true, hint, hunk.Header
			}
		}
	}
	return false, 0, ""
}

// MarkOrphans calls ReAnchor for each comment, mutates LineHint/HunkHeader/Orphaned in place,
// and returns the IDs of comments that became newly orphaned.
func MarkOrphans(comments []*Comment, newDiff *ParsedDiff) []string {
	var orphanedIDs []string
	for _, c := range comments {
		wasOrphaned := c.Orphaned
		matched, lineHint, hunkHeader := ReAnchor(c, newDiff)
		if matched {
			c.LineHint = lineHint
			c.HunkHeader = hunkHeader
			c.Orphaned = false
		} else {
			if !wasOrphaned {
				orphanedIDs = append(orphanedIDs, c.ID)
			}
			c.Orphaned = true
		}
	}
	return orphanedIDs
}
