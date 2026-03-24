package model

// ContentBlock represents a block of content (paragraph, heading, etc.)
// identified by blank-line boundaries in the rendered output.
type ContentBlock struct {
	RenderedStart int // first rendered line of this block
	RenderedEnd   int // last rendered line (inclusive)
	SourceStart   int // first source line (0-based in RawLines)
	SourceEnd     int // last source line (0-based, inclusive)
}

// SelectorState tracks block-level selection in select mode.
type SelectorState struct {
	Blocks       []ContentBlock // all navigable blocks
	CursorBlock  int            // index into Blocks
	StartBlock   int            // first selected block (-1 if not yet set)
}

// SelectionResult is returned when the user completes a selection.
type SelectionResult struct {
	StartSourceLine int // 0-based index in RawLines
	EndSourceLine   int // 0-based, inclusive
	Span            int // number of source lines
}

func NewBlockSelector(blocks []ContentBlock) SelectorState {
	return SelectorState{
		Blocks:      blocks,
		CursorBlock: 0,
		StartBlock:  -1,
	}
}

func (s *SelectorState) MoveDown() {
	if s.CursorBlock < len(s.Blocks)-1 {
		s.CursorBlock++
	}
}

func (s *SelectorState) MoveUp() {
	if s.CursorBlock > 0 {
		s.CursorBlock--
	}
}

// CursorRenderedLine returns the first rendered line of the focused block.
func (s *SelectorState) CursorRenderedLine() int {
	if s.CursorBlock < 0 || s.CursorBlock >= len(s.Blocks) {
		return 0
	}
	return s.Blocks[s.CursorBlock].RenderedStart
}

// Confirm handles Enter press. Returns a SelectionResult if selection is complete, nil otherwise.
func (s *SelectorState) Confirm() *SelectionResult {
	if len(s.Blocks) == 0 {
		return nil
	}

	if s.StartBlock < 0 {
		// First Enter: set start block
		s.StartBlock = s.CursorBlock
		return nil
	}

	// Second Enter: complete selection across block range
	lo, hi := s.StartBlock, s.CursorBlock
	if lo > hi {
		lo, hi = hi, lo
	}

	startSource := s.Blocks[lo].SourceStart
	endSource := s.Blocks[hi].SourceEnd

	return &SelectionResult{
		StartSourceLine: startSource,
		EndSourceLine:   endSource,
		Span:            endSource - startSource + 1,
	}
}

// InSelection returns true if the given rendered line is within the current selection range.
func (s *SelectorState) InSelection(renderedLine int) bool {
	if len(s.Blocks) == 0 {
		return false
	}

	lo, hi := s.CursorBlock, s.CursorBlock
	if s.StartBlock >= 0 {
		lo, hi = s.StartBlock, s.CursorBlock
		if lo > hi {
			lo, hi = hi, lo
		}
	}

	for i := lo; i <= hi && i < len(s.Blocks); i++ {
		b := s.Blocks[i]
		if renderedLine >= b.RenderedStart && renderedLine <= b.RenderedEnd {
			return true
		}
	}
	return false
}

// IsCursorBlock returns true if the given rendered line is in the currently focused block.
func (s *SelectorState) IsCursorBlock(renderedLine int) bool {
	if s.CursorBlock < 0 || s.CursorBlock >= len(s.Blocks) {
		return false
	}
	b := s.Blocks[s.CursorBlock]
	return renderedLine >= b.RenderedStart && renderedLine <= b.RenderedEnd
}
