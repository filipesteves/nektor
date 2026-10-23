package ui

// PickModel is the Bubble Tea model for `nektor pick`. It shows a filterable
// list of saved commands and returns the selected command string via Result.
// Commands can also be edited (e) or deleted (d) in place; changes are written
// to the store immediately.
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

// pickMode tracks what the pick TUI is currently doing.
type pickMode int

const (
	modeBrowse pickMode = iota
	modeEdit
	modeConfirmDelete
)

// PickModel is the model for the pick TUI.
type PickModel struct {
	list            list.Model
	mode            pickMode
	form            FormModel     // active while mode == modeEdit
	target          store.Command // command being edited or deleted
	status          string        // one-line feedback shown under the list
	Result          string        // set when user confirms selection
	UpdateRequested string        // non-empty when user pressed ctrl+u; holds the version string
	quitting        bool
	width           int
	height          int
}

// NewPickModel creates a PickModel from the provided saved commands.
func NewPickModel(commands []store.Command) PickModel {
	delegate := newPickDelegate()
	l := list.New(commandItems(commands), delegate, 0, 0)
	l.Title = "nektor"
	l.Styles.Title = styleTitle
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(true)
	// "d" is a next-page key by default; free it up for delete.
	l.KeyMap.NextPage = key.NewBinding(
		key.WithKeys("right", "l", "pgdown", "f"),
		key.WithHelp("→/l/pgdn", "next page"),
	)
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
			key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
			key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
		}
	}

	return PickModel{list: l}
}

func commandItems(commands []store.Command) []list.Item {
	items := make([]list.Item, len(commands))
	for i, c := range commands {
		items[i] = commandItem{cmd: c}
	}
	return items
}

// reload refreshes the list from the store after an edit or delete. SetItems
// re-applies any active filter, so the user keeps their search.
func (m *PickModel) reload() tea.Cmd {
	commands, err := store.LoadCommands()
	if err != nil {
		m.status = styleDanger.Render("  Reloading commands: " + err.Error())
		return nil
	}
	return m.list.SetItems(commandItems(commands))
}

func (m PickModel) Init() tea.Cmd {
	return nil
}

func (m PickModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = size.Width
		m.height = size.Height
		m.list.SetSize(size.Width, size.Height-2)
	}

	switch m.mode {
	case modeEdit:
		return m.updateEdit(msg)
	case modeConfirmDelete:
		return m.updateConfirmDelete(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Don't capture keys while filtering — the list handles them.
		if m.list.FilterState() == list.Filtering {
			break
		}
		m.status = ""
		switch msg.String() {
		case "e":
			if item, ok := m.list.SelectedItem().(commandItem); ok {
				m.target = item.cmd
				m.form = NewEditFormModel(item.cmd)
				m.form, _ = updateForm(m.form, tea.WindowSizeMsg{Width: m.width, Height: m.height})
				m.mode = modeEdit
				return m, m.form.Init()
			}
		case "d":
			if item, ok := m.list.SelectedItem().(commandItem); ok {
				m.target = item.cmd
				m.mode = modeConfirmDelete
				return m, nil
			}
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

// updateForm forwards msg to the embedded form. The form returns tea.Quit when
// it's submitted or cancelled; callers check its flags instead of running that.
func updateForm(f FormModel, msg tea.Msg) (FormModel, tea.Cmd) {
	updated, cmd := f.Update(msg)
	return updated.(FormModel), cmd
}

func (m PickModel) updateEdit(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.form, cmd = updateForm(m.form, msg)

	switch {
	case m.form.Cancelled:
		m.mode = modeBrowse
		return m, nil
	case m.form.Submitted:
		m.mode = modeBrowse
		r := m.form.Result
		updated := store.Command{
			Alias:       r.Alias,
			Description: r.Description,
			Tags:        r.Tags,
			Command:     r.Command,
		}
		if err := store.UpdateCommand(m.target.Alias, updated); err != nil {
			m.status = styleDanger.Render("  Saving changes: " + err.Error())
			return m, nil
		}
		m.status = styleSuccess.Render(fmt.Sprintf("  Updated %q", updated.Description))
		return m, m.reload()
	}
	return m, cmd
}

func (m PickModel) updateConfirmDelete(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	m.mode = modeBrowse
	if km.String() != "y" && km.String() != "Y" {
		m.status = styleSubtle.Render("  Delete cancelled")
		return m, nil
	}
	if err := store.DeleteCommand(m.target.Alias); err != nil {
		m.status = styleDanger.Render("  Deleting: " + err.Error())
		return m, nil
	}
	m.status = styleSuccess.Render(fmt.Sprintf("  Deleted %q", m.target.Description))
	return m, m.reload()
}

func (m PickModel) View() string {
	if m.quitting {
		return ""
	}

	if m.mode == modeEdit {
		return m.form.View()
	}

	view := m.list.View()

	switch {
	case m.mode == modeConfirmDelete:
		view += "\n" + styleDanger.Render(
			fmt.Sprintf("  Delete %q? [y/N]", m.target.Description),
		)
	case m.status != "":
		view += "\n" + m.status
	}

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
