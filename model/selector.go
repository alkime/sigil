package model

// ContentBlock represents a block of content (paragraph, heading, etc.)
// identified by blank-line boundaries in the rendered output.
type ContentBlock struct {
	RenderedStart int // first rendered line of this block
	RenderedEnd   int // last rendered line (inclusive)
	SourceStart   int // first source line (0-based in RawLines)
	SourceEnd     int // last source line (0-based, inclusive)
}

// SelectionResult is returned when the user acts on a block.
type SelectionResult struct {
	StartSourceLine int // 0-based index in RawLines
	EndSourceLine   int // 0-based, inclusive
	Span            int // number of source lines
}

// SelectorState tracks block-level cursor position.
type SelectorState struct {
	Blocks      []ContentBlock
	CursorBlock int // index into Blocks
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

// Result returns the source range of the currently focused block.
func (s *SelectorState) Result() *SelectionResult {
	b := s.CurrentBlock()
	if b == nil {
		return nil
	}
	return &SelectionResult{
		StartSourceLine: b.SourceStart,
		EndSourceLine:   b.SourceEnd,
		Span:            b.SourceEnd - b.SourceStart + 1,
	}
}

// IsCursorBlock returns true if the given rendered line is in the focused block.
func (s *SelectorState) IsCursorBlock(renderedLine int) bool {
	b := s.CurrentBlock()
	if b == nil {
		return false
	}
	return renderedLine >= b.RenderedStart && renderedLine <= b.RenderedEnd
}

// JumpToNextCommented moves to the next block that has a comment.
// commentedBlocks maps block index -> true if the block has a comment.
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
