package ui

// AddModel drives the `nektor add` flow:
//  1. Show a filterable, multi-select list of shell history entries.
//  2. For each selected entry, show the FormModel to capture metadata.
//  3. Return the completed store.Command values via Results.

import (
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/filipesteves/nektor/internal/store"
)

// historyItem implements list.DefaultItem for a shell history entry.
type historyItem struct {
	entry HistoryEntry
}

func (h historyItem) Title() string       { return h.entry.Command }
func (h historyItem) Description() string { return "" }
func (h historyItem) FilterValue() string { return h.entry.Command }

// historyDelegate renders single-line history items with a multi-select indicator.
// It reads selection state from the shared map to avoid needing SetItems on toggle,
// which would reset the list's filter state.
type historyDelegate struct {
	selected map[string]bool
}

func (d historyDelegate) Height() int  { return 1 }
func (d historyDelegate) Spacing() int { return 0 }

func (d historyDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d historyDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	hi, ok := item.(historyItem)
	if !ok {
		return
	}

	cursor := "  "
	if index == m.Index() && m.FilterState() != list.Filtering {
		cursor = "> "
	}

	checkbox := "[ ]"
	if d.selected[hi.entry.Command] {
		checkbox = styleSuccess.Render("[x]")
	}

	line := hi.entry.Command
	if m.Width() > 10 {
		// Truncate to avoid wrapping. Use rune count, not byte length, so we
		// never split a multi-byte UTF-8 character at the cut point.
		maxW := m.Width() - len(cursor) - len(checkbox) - 2
		if maxW > 0 && utf8.RuneCountInString(line) > maxW {
			runes := []rune(line)
			line = string(runes[:maxW-1]) + "…"
		}
	}

	var style lipgloss.Style
	if index == m.Index() && m.FilterState() != list.Filtering {
		style = lipgloss.NewStyle().Foreground(colPrimary)
	} else {
		style = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#dddddd"})
	}

	fmt.Fprintf(w, "%s%s %s", cursor, checkbox, style.Render(line)) //nolint:errcheck
}

// addPhase tracks which step of the add flow is active.
type addPhase int

const (
	phaseSelect addPhase = iota
	phaseForm
	phaseDone
)

// addUpdateAvailableFunc mirrors the hook pattern from pick.go. Assigned by
// cmd/add.go before the program starts to avoid an import cycle.
var addUpdateAvailableFunc func() string

// SetAddUpdateAvailableFunc wires in the update-availability check for the add TUI.
func SetAddUpdateAvailableFunc(f func() string) {
	addUpdateAvailableFunc = f
}

// AddModel is the top-level model for the `nektor add` command.
type AddModel struct {
	phase           addPhase
	histList        list.Model
	selected        map[string]bool // selection state keyed by command string; shared with delegate
	formModel       FormModel
	formQueue       []string // command strings pending form entry
	Results         []store.Command
	Cancelled       bool
	UpdateRequested string          // non-empty when user pressed ctrl+u; holds the version string
	existingCmds    map[string]bool // commands already in the store (by command string)

	width  int
	height int
}

// NewAddModel creates an AddModel from the provided history entries and
// existing commands (used to mark already-saved entries).
func NewAddModel(entries []HistoryEntry, existing []store.Command) AddModel {
	existingMap := make(map[string]bool, len(existing))
	for _, c := range existing {
		existingMap[c.Command] = true
	}

	selected := make(map[string]bool)

	listItems := make([]list.Item, len(entries))
	for i, e := range entries {
		listItems[i] = historyItem{entry: e}
	}

	tabKey := key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "toggle select"),
	)

	// Delegate holds a reference to the shared selected map so it always
	// renders current state without needing SetItems (which resets filter).
	delegate := historyDelegate{selected: selected}
	l := list.New(listItems, delegate, 0, 0)
	l.Title = "Select history entries to save"
	l.Styles.Title = styleTitle
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(true)
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			tabKey,
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm selection")),
		}
	}

	return AddModel{
		phase:        phaseSelect,
		histList:     l,
		selected:     selected,
		existingCmds: existingMap,
	}
}

func (m AddModel) Init() tea.Cmd {
	return nil
}

func (m AddModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.phase {
	case phaseSelect:
		return m.updateSelect(msg)
	case phaseForm:
		return m.updateForm(msg)
	}
	return m, tea.Quit
}

func (m AddModel) updateSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.histList.SetSize(msg.Width, msg.Height-2)

	case tea.KeyMsg:
		if m.histList.FilterState() == list.Filtering {
			break // let list handle it
		}
		switch msg.String() {
		case "ctrl+c", "q":
			m.Cancelled = true
			return m, tea.Quit

		case "ctrl+u":
			if addUpdateAvailableFunc == nil {
				break
			}
			ver := addUpdateAvailableFunc()
			if ver == "" {
				break
			}
			m.UpdateRequested = ver
			m.Cancelled = true
			return m, tea.Quit

		case "tab":
			// Toggle selection on the current visible item by command string.
			// We update the shared selected map — the delegate reads from it
			// directly, so no SetItems call is needed and filter state is preserved.
			visible := m.histList.VisibleItems()
			idx := m.histList.Index()
			if idx >= 0 && idx < len(visible) {
				if hi, ok := visible[idx].(historyItem); ok {
					m.selected[hi.entry.Command] = !m.selected[hi.entry.Command]
				}
			}
			// Route a synthetic Down key through the list's own Update so the
			// paginator and cursor state are managed consistently.
			var listCmd tea.Cmd
			m.histList, listCmd = m.histList.Update(tea.KeyMsg{Type: tea.KeyDown})
			return m, listCmd

		case "enter":
			// Collect selected commands (in list order).
			queue := make([]string, 0)
			for _, it := range m.histList.Items() {
				hi, ok := it.(historyItem)
				if !ok {
					continue
				}
				if m.selected[hi.entry.Command] && !m.existingCmds[hi.entry.Command] {
					queue = append(queue, hi.entry.Command)
				}
			}
			if len(queue) == 0 {
				// Nothing selected — treat current visible item as implicit selection.
				idx := m.histList.Index()
				visible := m.histList.VisibleItems()
				if idx >= 0 && idx < len(visible) {
					if hi, ok := visible[idx].(historyItem); ok {
						if !m.existingCmds[hi.entry.Command] {
							queue = append(queue, hi.entry.Command)
						}
					}
				}
			}
			if len(queue) == 0 {
				return m, nil
			}
			m.formQueue = queue
			m.phase = phaseForm
			m.formModel = NewFormModel(fmt.Sprintf("Save command (1 of %d)", len(m.formQueue)))
			m.formModel.inputs[fieldCommand].SetValue(m.formQueue[0])
			m.formModel.inputs[fieldCommand].CursorEnd()
			m.formModel.width = m.width
			m.formModel.height = m.height
			return m, m.formModel.Init()
		}
	}

	var cmd tea.Cmd
	m.histList, cmd = m.histList.Update(msg)
	return m, cmd
}

func (m AddModel) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		// Handle global cancel.
		if msg.String() == "ctrl+c" {
			m.Cancelled = true
			return m, tea.Quit
		}
	}

	updated, cmd := m.formModel.Update(msg)
	fm, ok := updated.(FormModel)
	if !ok {
		return m, cmd
	}
	m.formModel = fm

	if m.formModel.Cancelled {
		m.Cancelled = true
		return m, tea.Quit
	}

	if m.formModel.Submitted {
		r := m.formModel.Result
		m.Results = append(m.Results, store.Command{
			Description: r.Description,
			Command:     r.Command,
		})

		// Advance to next queued command, or finish.
		m.formQueue = m.formQueue[1:]
		if len(m.formQueue) == 0 {
			m.phase = phaseDone
			return m, tea.Quit
		}

		saved := len(m.Results)
		total := saved + len(m.formQueue)
		m.formModel = NewFormModel(fmt.Sprintf("Save command (%d of %d)", saved+1, total))
		m.formModel.inputs[fieldCommand].SetValue(m.formQueue[0])
		m.formModel.inputs[fieldCommand].CursorEnd()
		m.formModel.width = m.width
		m.formModel.height = m.height
		return m, m.formModel.Init()
	}

	return m, cmd
}

func (m AddModel) View() string {
	switch m.phase {
	case phaseSelect:
		selected := 0
		for _, v := range m.selected {
			if v {
				selected++
			}
		}
		view := m.histList.View()
		if selected > 0 {
			view += "\n" + styleSuccess.Render(fmt.Sprintf("  %d selected — press enter to continue", selected))
		}
		// Append update notice footer when an update is cached.
		if addUpdateAvailableFunc != nil {
			if ver := addUpdateAvailableFunc(); ver != "" {
				view += "\n" + styleUpdate.Render(
					fmt.Sprintf("  Update available: %s  →  press ctrl+u to update", ver),
				)
			}
		}
		return view
	case phaseForm:
		return m.formModel.View()
	}
	return ""
}

