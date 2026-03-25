package model

// ContentBlock represents a block of content (paragraph, heading, etc.)
// identified by blank-line boundaries in the rendered output.
type ContentBlock struct {
	RenderedStart int // first rendered line of this block
	RenderedEnd   int // last rendered line (inclusive)
	SourceStart   int // first source line (0-based in RawLines)
	SourceEnd     int // last source line (0-based, inclusive)
}

// SelectionResult is returned when the user acts on a block or range.
type SelectionResult struct {
	StartSourceLine int // 0-based index in RawLines
	EndSourceLine   int // 0-based, inclusive
	Span            int // number of source lines
}

// SelectorState tracks block-level cursor and multi-block selection.
type SelectorState struct {
	Blocks      []ContentBlock
	CursorBlock int  // index into Blocks
	Selecting   bool // true when multi-block selection is active
	AnchorBlock int  // block where selection started
}

func NewBlockSelector(blocks []ContentBlock) SelectorState {
	return SelectorState{
		Blocks:      blocks,
		CursorBlock: 0,
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

// ToggleSelect starts or cancels multi-block selection.
func (s *SelectorState) ToggleSelect() {
	if s.Selecting {
		s.Selecting = false
		return
	}
	s.Selecting = true
	s.AnchorBlock = s.CursorBlock
}

// CancelSelect exits selection mode.
func (s *SelectorState) CancelSelect() {
	s.Selecting = false
}

// selectionRange returns the ordered (lo, hi) block indices of the current selection.
func (s *SelectorState) selectionRange() (int, int) {
	if !s.Selecting {
		return s.CursorBlock, s.CursorBlock
	}
	lo, hi := s.AnchorBlock, s.CursorBlock
	if lo > hi {
		lo, hi = hi, lo
	}
	return lo, hi
}

// CursorRenderedLine returns the first rendered line of the focused block.
func (s *SelectorState) CursorRenderedLine() int {
	if s.CursorBlock < 0 || s.CursorBlock >= len(s.Blocks) {
		return 0
	}
	return s.Blocks[s.CursorBlock].RenderedStart
}

// CurrentBlock returns the currently focused block, or nil if none.
func (s *SelectorState) CurrentBlock() *ContentBlock {
	if s.CursorBlock < 0 || s.CursorBlock >= len(s.Blocks) {
		return nil
	}
	return &s.Blocks[s.CursorBlock]
}

// Result returns the source range of the selection (single block or multi-block range).
func (s *SelectorState) Result() *SelectionResult {
	if len(s.Blocks) == 0 {
		return nil
	}
	lo, hi := s.selectionRange()
	startSource := s.Blocks[lo].SourceStart
	endSource := s.Blocks[hi].SourceEnd
	return &SelectionResult{
		StartSourceLine: startSource,
		EndSourceLine:   endSource,
		Span:            endSource - startSource + 1,
	}
}

// InSelection returns true if the given rendered line is within the selected range,
// including blank lines between selected blocks.
func (s *SelectorState) InSelection(renderedLine int) bool {
	lo, hi := s.selectionRange()
	if lo >= len(s.Blocks) || hi >= len(s.Blocks) {
		return false
	}
	return renderedLine >= s.Blocks[lo].RenderedStart && renderedLine <= s.Blocks[hi].RenderedEnd
}

// IsCursorBlock returns true if the given rendered line is in the selection range.
// When selecting, this covers all blocks from anchor to cursor.
// When not selecting, just the cursor block.
func (s *SelectorState) IsCursorBlock(renderedLine int) bool {
	return s.InSelection(renderedLine)
}

// JumpToNextCommented moves to the next block that has a comment.
func (s *SelectorState) JumpToNextCommented(commentedBlocks map[int]bool) {
	for i := 1; i < len(s.Blocks); i++ {
		idx := (s.CursorBlock + i) % len(s.Blocks)
		if commentedBlocks[idx] {
			s.CursorBlock = idx
			return
		}
	}
}

// JumpToPrevCommented moves to the previous block that has a comment.
func (s *SelectorState) JumpToPrevCommented(commentedBlocks map[int]bool) {
	for i := 1; i < len(s.Blocks); i++ {
		idx := (s.CursorBlock - i + len(s.Blocks)) % len(s.Blocks)
		if commentedBlocks[idx] {
			s.CursorBlock = idx
			return
		}
	}
}
