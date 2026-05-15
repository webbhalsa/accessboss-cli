package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var styleDurationCursor = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)

type durationItem struct {
	label string
	value string
}

var durationOptions = []durationItem{
	{"1 hour", "1-hour"},
	{"4 hours", "4-hours"},
	{"24 hours", "24-hours"},
}

type durationModel struct {
	cursor int
	chosen string
	quit   bool
}

func (m durationModel) Init() tea.Cmd { return nil }

func (m durationModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(durationOptions)-1 {
				m.cursor++
			}
		case "enter", " ":
			m.chosen = durationOptions[m.cursor].value
			return m, tea.Quit
		case "ctrl+c", "esc":
			m.quit = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m durationModel) View() string {
	s := "\n  How long do you need access?\n\n"
	for i, opt := range durationOptions {
		if i == m.cursor {
			s += styleDurationCursor.Render("  ❯ "+opt.label) + "\n"
		} else {
			s += "    " + opt.label + "\n"
		}
	}
	return s + "\n"
}

// PickDuration shows an inline duration picker and returns the selected value ("1h", "4h", "24h").
// Returns an empty string if the user cancelled.
func PickDuration() (string, error) {
	m, err := tea.NewProgram(durationModel{}).Run()
	if err != nil {
		return "", fmt.Errorf("duration picker: %w", err)
	}
	result := m.(durationModel)
	if result.quit {
		return "", nil
	}
	return result.chosen, nil
}
