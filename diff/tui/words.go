package tui

import "unicode"

// isWordRune reports whether r is part of a vi "word" (\w-class).
func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// WordAt returns the [start, end] rune indices of the word containing col.
// end is inclusive. ok=false when col lies outside text bounds or on a
// non-word rune (whitespace or punctuation).
func WordAt(text string, col int) (start, end int, ok bool) {
	runes := []rune(text)
	if col < 0 || col >= len(runes) {
		return 0, 0, false
	}
	if !isWordRune(runes[col]) {
		return 0, 0, false
	}
	start = col
	for start > 0 && isWordRune(runes[start-1]) {
		start--
	}
	end = col
	for end < len(runes)-1 && isWordRune(runes[end+1]) {
		end++
	}
	return start, end, true
}

// WordNext returns the rune index of the start of the next word after col.
// If no next word exists, returns the index of the last rune in text
// (or 0 if text is empty).
func WordNext(text string, col int) int {
	runes := []rune(text)
	n := len(runes)
	if n == 0 {
		return 0
	}
	if col < 0 {
		col = 0
	}
	i := col
	// First, walk past the current word (if we're on one).
	for i < n && isWordRune(runes[i]) {
		i++
	}
	// Then skip non-word runes to land on the next word start.
	for i < n && !isWordRune(runes[i]) {
		i++
	}
	if i >= n {
		return n - 1
	}
	return i
}

// WordPrev returns the rune index of the start of the previous word.
// If col is in the middle of a word, jumps to the start of the current word.
// If already at start of a word (or before any word), jumps to start of the
// previous word. Returns 0 if no previous word exists.
func WordPrev(text string, col int) int {
	runes := []rune(text)
	n := len(runes)
	if n == 0 {
		return 0
	}
	if col <= 0 {
		return 0
	}
	if col >= n {
		col = n - 1
	}
	i := col
	// If we're inside a word past its start, jump to the start of this word.
	if isWordRune(runes[i]) && i > 0 && isWordRune(runes[i-1]) {
		for i > 0 && isWordRune(runes[i-1]) {
			i--
		}
		return i
	}
	// Otherwise, step back into the previous word.
	i--
	for i >= 0 && !isWordRune(runes[i]) {
		i--
	}
	if i < 0 {
		return 0
	}
	for i > 0 && isWordRune(runes[i-1]) {
		i--
	}
	return i
}

// WordEnd returns the rune index of the end of the current word (inclusive).
// If col is past the end of the current word or on a non-word rune, jumps
// to the end of the next word. Returns the last rune index if no further
// word exists.
func WordEnd(text string, col int) int {
	runes := []rune(text)
	n := len(runes)
	if n == 0 {
		return 0
	}
	if col < 0 {
		col = 0
	}
	if col >= n {
		return n - 1
	}
	i := col
	// If we're already at the end of a word, advance.
	if isWordRune(runes[i]) && (i == n-1 || !isWordRune(runes[i+1])) {
		i++
	}
	// Skip non-word runes.
	for i < n && !isWordRune(runes[i]) {
		i++
	}
	if i >= n {
		return n - 1
	}
	// Walk to the end of this word.
	for i < n-1 && isWordRune(runes[i+1]) {
		i++
	}
	return i
}
