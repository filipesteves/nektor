package ui

// InstallModel drives the `nektor install` flow:
// 1. Ask user to pick output mode (shell wrapper or clipboard).
// 2. Return the chosen mode via Result.

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// InstallChoice represents the selected output mode.
type InstallChoice string

const (
	InstallChoiceShell     InstallChoice = "shell"
	InstallChoiceClipboard InstallChoice = "clipboard"
)

// installOptionItem implements list.DefaultItem.
type installOptionItem struct {
	title string
	desc  string
	value InstallChoice
}

func (i installOptionItem) Title() string       { return i.title }
func (i installOptionItem) Description() string { return i.desc }
func (i installOptionItem) FilterValue() string { return i.title }

// InstallModel is the Bubble Tea model for the install TUI.
type InstallModel struct {
	list      list.Model
	Result    InstallChoice
	Confirmed bool
	Cancelled bool
	width     int
	height    int
}

// NewInstallModel creates the install picker model.
func NewInstallModel() InstallModel {
	options := []list.Item{
		installOptionItem{
			title: "Shell wrapper (recommended)",
			desc:  "Adds `nk` function to your shell rc. Places selected command on the prompt line.",
			value: InstallChoiceShell,
		},
		installOptionItem{
			title: "Clipboard",
			desc:  "Copies the selected command to the clipboard. No shell modification needed.",
			value: InstallChoiceClipboard,
		},
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(colPrimary).
		BorderForeground(colPrimary)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(colPrimary)

	l := list.New(options, delegate, 0, 0)
	l.Title = "Choose output mode"
	l.Styles.Title = styleTitle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(true)

	return InstallModel{list: l}
}

func (m InstallModel) Init() tea.Cmd {
	return nil
}

func (m InstallModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-2)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.Cancelled = true
			return m, tea.Quit

		case "enter":
			if item, ok := m.list.SelectedItem().(installOptionItem); ok {
				m.Result = item.value
				m.Confirmed = true
				return m, tea.Quit
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m InstallModel) View() string {
	if m.Cancelled || m.Confirmed {
		return ""
	}
	return m.list.View()
}
