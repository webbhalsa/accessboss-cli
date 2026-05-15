package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	stylePlaceholder  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	styleUpdateBanner = lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Bold(true)
)

type ScopeItem struct {
	Name       string
	Desc       string
	IsDatabase bool
}

func (i ScopeItem) Title() string { return i.Name }
func (i ScopeItem) Description() string {
	if i.IsDatabase {
		return i.Desc + "  [database]"
	}
	return i.Desc
}
func (i ScopeItem) FilterValue() string { return i.Name }

type latestVersionMsg string

type model struct {
	list           list.Model
	allItems       []list.Item
	filter         string
	chosen         string
	quit           bool
	currentVersion string
	latestVersion  string
	checkVersion   func(string) string
}

func (m model) Init() tea.Cmd {
	return m.versionCheckCmd
}

func (m model) versionCheckCmd() tea.Msg {
	return latestVersionMsg(m.checkVersion(m.currentVersion))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case latestVersionMsg:
		m.latestVersion = string(msg)
		return m, nil
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyRunes:
			m.filter += string(msg.Runes)
			m.list.SetItems(applyFilter(m.allItems, m.filter))
			m.list.ResetSelected()
			return m, nil
		case tea.KeyBackspace:
			if runes := []rune(m.filter); len(runes) > 0 {
				m.filter = string(runes[:len(runes)-1])
				m.list.SetItems(applyFilter(m.allItems, m.filter))
				m.list.ResetSelected()
			}
			return m, nil
		case tea.KeyEnter:
			if item, ok := m.list.SelectedItem().(ScopeItem); ok {
				m.chosen = item.Name
				return m, tea.Quit
			}
		case tea.KeyEsc:
			if m.filter != "" {
				m.filter = ""
				m.list.SetItems(m.allItems)
				m.list.ResetSelected()
				return m, nil
			}
			m.quit = true
			return m, tea.Quit
		case tea.KeyCtrlC:
			m.quit = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		// reserve: 1 search + 1 blank + 1 blank + 1 footer = 4 lines
		m.list.SetSize(msg.Width, msg.Height-4)
		return m, nil
	}

	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	var searchLine string
	if m.filter == "" {
		searchLine = stylePlaceholder.Render("  start typing to filter...")
	} else {
		searchLine = "  " + m.filter
	}

	footer := ""
	if m.latestVersion != "" {
		footer = styleUpdateBanner.Render(fmt.Sprintf(
			"  Update available: %s → %s  Run: brew upgrade accessboss",
			m.currentVersion, m.latestVersion,
		))
	}

	return searchLine + "\n\n" + m.list.View() + "\n" + footer
}

func applyFilter(items []list.Item, filter string) []list.Item {
	if filter == "" {
		return items
	}
	lower := strings.ToLower(filter)
	var out []list.Item
	for _, item := range items {
		si := item.(ScopeItem)
		if strings.Contains(strings.ToLower(si.Name), lower) ||
			strings.Contains(strings.ToLower(si.Desc), lower) {
			out = append(out, item)
		}
	}
	return out
}

// PickScope shows an interactive filterable list and returns the selected scope name.
// currentVersion and checkFn are used to display an update banner if a newer version exists.
// Returns an empty string if the user cancelled.
func PickScope(scopes []ScopeItem, currentVersion string, checkFn func(string) string) (string, error) {
	items := make([]list.Item, len(scopes))
	for i, s := range scopes {
		items[i] = s
	}

	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)

	m, err := tea.NewProgram(model{
		list:           l,
		allItems:       items,
		currentVersion: currentVersion,
		checkVersion:   checkFn,
	}, tea.WithAltScreen()).Run()
	if err != nil {
		return "", fmt.Errorf("picker: %w", err)
	}

	result := m.(model)
	if result.quit {
		return "", nil
	}
	return result.chosen, nil
}
