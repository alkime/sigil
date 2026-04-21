package tui

import "testing"

func TestWordAt(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		col       int
		wantStart int
		wantEnd   int
		wantOK    bool
	}{
		{"start of word", "func main", 0, 0, 3, true},
		{"middle of word", "func main", 2, 0, 3, true},
		{"end of word", "func main", 3, 0, 3, true},
		{"on whitespace", "func main", 4, 0, 0, false},
		{"second word", "func main", 5, 5, 8, true},
		{"on punctuation", "a.b", 1, 0, 0, false},
		{"underscore identifier", "snake_case_var", 7, 0, 13, true},
		{"unicode identifier", "café", 2, 0, 3, true},
		{"empty text", "", 0, 0, 0, false},
		{"col negative", "abc", -1, 0, 0, false},
		{"col past end", "abc", 5, 0, 0, false},
		{"single char word", "a b", 0, 0, 0, true},
		{"digit start", "x123y", 0, 0, 4, true},
		{"digit middle", "x123y", 2, 0, 4, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, ok := WordAt(tt.text, tt.col)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if start != tt.wantStart || end != tt.wantEnd {
				t.Errorf("start=%d end=%d, want start=%d end=%d", start, end, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

func TestWordNext(t *testing.T) {
	tests := []struct {
		name string
		text string
		col  int
		want int
	}{
		{"from word to next word", "func main", 0, 5},
		{"from middle of word", "func main", 2, 5},
		{"from whitespace", "func   main", 4, 7},
		{"already at last word", "abc def", 4, 6}, // clamps to last rune
		{"from end of text", "abc", 2, 2},
		{"empty text", "", 0, 0},
		{"three words", "one two three", 0, 4},
		{"jump from second word", "one two three", 4, 8},
		{"punctuation between words", "foo.bar.baz", 0, 4},
		{"col negative", "abc def", -3, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WordNext(tt.text, tt.col)
			if got != tt.want {
				t.Errorf("WordNext(%q, %d) = %d, want %d", tt.text, tt.col, got, tt.want)
			}
		})
	}
}

func TestWordPrev(t *testing.T) {
	tests := []struct {
		name string
		text string
		col  int
		want int
	}{
		{"middle of word jumps to word start", "func main", 7, 5},
		{"start of word jumps to prev word", "func main", 5, 0},
		{"from end of text", "abc def", 6, 4},
		{"already at start", "abc", 0, 0},
		{"from whitespace", "abc def", 3, 0},
		{"empty text", "", 0, 0},
		{"three words back", "one two three", 8, 4},
		{"from punctuation", "foo.bar", 3, 0},
		{"col past end", "abc def", 99, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WordPrev(tt.text, tt.col)
			if got != tt.want {
				t.Errorf("WordPrev(%q, %d) = %d, want %d", tt.text, tt.col, got, tt.want)
			}
		})
	}
}

func TestWordEnd(t *testing.T) {
	tests := []struct {
		name string
		text string
		col  int
		want int
	}{
		{"start of word goes to end", "func main", 0, 3},
		{"middle of word goes to end", "func main", 2, 3},
		{"end of word jumps to next end", "func main", 3, 8},
		{"on whitespace goes to next word end", "func   main", 4, 10},
		{"already at last word end", "abc def", 6, 6},
		{"empty text", "", 0, 0},
		{"col past end", "abc", 5, 2},
		{"col negative", "abc def", -1, 2},
		{"unicode identifier end", "café x", 0, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WordEnd(tt.text, tt.col)
			if got != tt.want {
				t.Errorf("WordEnd(%q, %d) = %d, want %d", tt.text, tt.col, got, tt.want)
			}
		})
	}
}
