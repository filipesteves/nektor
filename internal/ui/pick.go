package ui

// PickModel is the Bubble Tea model for `nektor pick`. It shows a filterable
// list of saved commands and returns the selected command string via Result.
//
// Usage:
//
//	m := NewPickModel(commands)
//	p := tea.NewProgram(m, tea.WithAltScreen())
//	final, err := p.Run()
//	result := final.(PickModel).Result

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/filipesteves/nektor/internal/store"
)

// commandItem wraps store.Command so it implements list.DefaultItem.
type commandItem struct {
	cmd store.Command
}

func (i commandItem) Title() string       { return i.cmd.Description }
func (i commandItem) FilterValue() string { return i.cmd.Description + " " + i.cmd.Command }
func (i commandItem) Description() string { return i.cmd.Command }

// pickDelegate is a custom delegate that renders each command as a two-line
// block: description on the first line, command on the second. The selected
// item gets a full-width background highlight and a ▶ cursor so it stands out
// clearly against unselected rows.
type pickDelegate struct {
	list.DefaultDelegate
}

func newPickDelegate() pickDelegate {
	d := list.NewDefaultDelegate()
	d.SetHeight(2)
	d.SetSpacing(1)
	return pickDelegate{DefaultDelegate: d}
}

func (d pickDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	ci, ok := item.(commandItem)
	if !ok {
		return
	}

	isSelected := index == m.Index() && m.FilterState() != list.Filtering

	// innerWidth fills the row so the background highlight spans the full line.
	innerWidth := m.Width() - 4
	if innerWidth < 20 {
		innerWidth = 20
	}

	if isSelected {
		titleStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(colPrimary).
			Width(innerWidth).
			PaddingLeft(1)
		cmdStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(colPrimary).
			Width(innerWidth).
			PaddingLeft(1)
		fmt.Fprintf(w, "%s\n%s", //nolint:errcheck
			titleStyle.Render("▶  "+ci.cmd.Description),
			cmdStyle.Render("   $ "+ci.cmd.Command),
		)
	} else {
		titleStyle := lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#111827", Dark: "#F9FAFB"}).
			PaddingLeft(1)
		cmdStyle := lipgloss.NewStyle().
			Foreground(colMuted).
			PaddingLeft(1)
		fmt.Fprintf(w, "%s\n%s", //nolint:errcheck
			titleStyle.Render("   "+ci.cmd.Description),
			cmdStyle.Render("   $ "+ci.cmd.Command),
		)
	}
}

// updateAvailableFunc is a hook so the UI can read cmd.UpdateAvailable without
// a direct import cycle (cmd imports ui; ui must not import cmd).
// Assigned in cmd/pick.go before the program starts.
var updateAvailableFunc func() string

// SetUpdateAvailableFunc wires in a function that returns the current update
// version string (empty when no update is pending).
func SetUpdateAvailableFunc(f func() string) {
	updateAvailableFunc = f
}

// PickModel is the model for the pick TUI.
type PickModel struct {
	list            list.Model
	Result          string // set when user confirms selection
	UpdateRequested string // non-empty when user pressed ctrl+u; holds the version string
	quitting        bool
	width           int
	height          int
}

// NewPickModel creates a PickModel from the provided saved commands.
func NewPickModel(commands []store.Command) PickModel {
	items := make([]list.Item, len(commands))
	for i, c := range commands {
		items[i] = commandItem{cmd: c}
	}

	delegate := newPickDelegate()
	l := list.New(items, delegate, 0, 0)
	l.Title = "nektor"
	l.Styles.Title = styleTitle
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(true)
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
		}
	}

	return PickModel{list: l}
}

func (m PickModel) Init() tea.Cmd {
	return nil
}

func (m PickModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-2)

	case tea.KeyMsg:
		// Don't capture keys while filtering — the list handles them.
		if m.list.FilterState() == list.Filtering {
			break
		}
		switch msg.String() {
		case "enter":
			if item, ok := m.list.SelectedItem().(commandItem); ok {
				m.Result = item.cmd.Command
				m.quitting = true
				return m, tea.Quit
			}
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "ctrl+u":
			if updateAvailableFunc == nil {
				break
			}
			ver := updateAvailableFunc()
			if ver == "" {
				break
			}
			m.quitting = true
			m.UpdateRequested = ver
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m PickModel) View() string {
	if m.quitting {
		return ""
	}

	view := m.list.View()

	// Append update notice footer when an update is cached.
	if updateAvailableFunc != nil {
		if ver := updateAvailableFunc(); ver != "" {
			notice := styleUpdate.Render(
				fmt.Sprintf("  Update available: %s  →  press ctrl+u to update", ver),
			)
			view += "\n" + notice
		}
	}

	return view
}
