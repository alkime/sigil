package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/alkime/sigil/diff"
)

// renderPicker renders the PR picker overlay.
func renderPicker(candidates []diff.PRCandidate, selected int, width, height int) string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
	activeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EEEEEE")).Bold(true)
	inactiveStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Select PR to review") + "\n\n")
	for i, c := range candidates {
		line := fmt.Sprintf("[%s] PR #%d: %s  (base: %s, worktree: %s)",
			c.Branch, c.PRNumber, c.PRTitle, c.BaseRefName, c.WorktreePath)
		if i == selected {
			sb.WriteString(activeStyle.Render("  ▶ "+line) + "\n")
		} else {
			sb.WriteString(inactiveStyle.Render("    "+line) + "\n")
		}
	}
	sb.WriteString("\n" + keyHintDescStyle.Render("  j/k navigate  ·  Enter select  ·  q cancel"))

	boxWidth := 80
	if width-4 < boxWidth {
		boxWidth = width - 4
	}
	if boxWidth < 30 {
		boxWidth = 30
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Width(boxWidth).
		Render(sb.String())

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

// pickerModel is a minimal Bubbletea model for the PR picker.
type pickerModel struct {
	candidates []diff.PRCandidate
	selected   int
	chosen     *diff.PRCandidate
	width      int
	height     int
}

func (m pickerModel) Init() tea.Cmd { return nil }

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyPressMsg:
		switch msg.String() {
		case "j", "down":
			if m.selected < len(m.candidates)-1 {
				m.selected++
			}
		case "k", "up":
			if m.selected > 0 {
				m.selected--
			}
		case "enter":
			c := m.candidates[m.selected]
			m.chosen = &c
			return m, tea.Quit
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m pickerModel) View() tea.View {
	w, h := m.width, m.height
	if w == 0 {
		w = 80
	}
	if h == 0 {
		h = 24
	}
	content := renderPicker(m.candidates, m.selected, w, h)
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

// RunPicker runs the interactive PR picker and returns the chosen candidate.
// Returns an error if the user cancels.
func RunPicker(candidates []diff.PRCandidate) (diff.PRCandidate, error) {
	m := pickerModel{candidates: candidates}
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return diff.PRCandidate{}, fmt.Errorf("picker: %w", err)
	}
	pm := result.(pickerModel)
	if pm.chosen == nil {
		return diff.PRCandidate{}, fmt.Errorf("no candidate selected")
	}
	return *pm.chosen, nil
}

var _ tea.Model = pickerModel{}
