package tui

import "charm.land/bubbles/v2/key"

// KeyMap holds all key bindings for the diff TUI.
type KeyMap struct {
	Up             key.Binding
	Down           key.Binding
	HunkUp         key.Binding
	HunkDown       key.Binding
	NextFile       key.Binding
	PrevFile       key.Binding
	NextComment    key.Binding
	PrevComment    key.Binding
	Comment        key.Binding
	PRComment      key.Binding
	Resolve        key.Binding
	OrphanCycle    key.Binding
	Quit           key.Binding
	Help           key.Binding
}

// DefaultKeyMap returns j/k, J/K, Tab, n/N, c/C, r, o, q, ? bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:          key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "line up")),
		Down:        key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "line down")),
		HunkUp:      key.NewBinding(key.WithKeys("K"), key.WithHelp("K", "prev hunk")),
		HunkDown:    key.NewBinding(key.WithKeys("J"), key.WithHelp("J", "next hunk")),
		NextFile:    key.NewBinding(key.WithKeys("tab"), key.WithHelp("Tab/S-Tab", "next/prev file")),
		PrevFile:    key.NewBinding(key.WithKeys("shift+tab")),
		NextComment: key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next comment")),
		PrevComment: key.NewBinding(key.WithKeys("N"), key.WithHelp("N", "prev comment")),
		Comment:     key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "add comment")),
		PRComment:   key.NewBinding(key.WithKeys("C"), key.WithHelp("C", "PR-level comment")),
		Resolve:     key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "resolve")),
		OrphanCycle: key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "orphan review")),
		Quit:        key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Help:        key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	}
}
