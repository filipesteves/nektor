package ui

// FormModel is a generic multi-field text form used for adding and editing commands.
// It cycles focus through alias, description, tags, and command fields.

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/filipesteves/nektor/internal/store"
)

// FormResult holds the values submitted from FormModel.
type FormResult struct {
	Alias       string
	Description string
	Tags        []string
	Command     string
}

const (
	fieldDescription = iota
	fieldCommand
	fieldCount
)

// FormModel is the Bubble Tea model for the add/edit form.
type FormModel struct {
	inputs    [fieldCount]textinput.Model
	focused   int
	Submitted bool
	Cancelled bool
	Result    FormResult
	title     string
	width     int
	height    int
}

// NewFormModel creates a blank form for adding a new command.
func NewFormModel(title string) FormModel {
	return newFormModel(title, FormResult{})
}

// NewEditFormModel creates a pre-populated form for editing an existing command.
func NewEditFormModel(cmd store.Command) FormModel {
	return newFormModel("Edit command", FormResult{
		Alias:       cmd.Alias,
		Description: cmd.Description,
		Tags:        cmd.Tags,
		Command:     cmd.Command,
	})
}

func newFormModel(title string, prefill FormResult) FormModel {
	m := FormModel{title: title}

	placeholders := [fieldCount]string{
		"Expose local dev server via ngrok",
		"ngrok http --url=example.ngrok-free.app 5173",
	}

	for i := 0; i < fieldCount; i++ {
		t := textinput.New()
		t.Placeholder = placeholders[i]
		t.CharLimit = 256
		m.inputs[i] = t
	}

	// Pre-populate for edit.
	m.inputs[fieldDescription].SetValue(prefill.Description)
	m.inputs[fieldCommand].SetValue(prefill.Command)

	// Focus the first field.
	m.inputs[fieldDescription].Focus()

	return m
}

func (m FormModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m FormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Resize inputs to fit.
		w := m.inputWidth()
		for i := range m.inputs {
			m.inputs[i].Width = w
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.Cancelled = true
			return m, tea.Quit

		case "esc":
			m.Cancelled = true
			return m, tea.Quit

		case "tab", "down":
			m.inputs[m.focused].Blur()
			m.focused = (m.focused + 1) % fieldCount
			m.inputs[m.focused].Focus()
			return m, nil

		case "shift+tab", "up":
			m.inputs[m.focused].Blur()
			m.focused = (m.focused - 1 + fieldCount) % fieldCount
			m.inputs[m.focused].Focus()
			return m, nil

		case "enter":
			// From last field, submit; otherwise advance.
			if m.focused == fieldCommand {
				description := strings.TrimSpace(m.inputs[fieldDescription].Value())
				command := strings.TrimSpace(m.inputs[fieldCommand].Value())
				if command == "" {
					// Stay on form — required field missing.
					return m, nil
				}
				m.Result = FormResult{
					Description: description,
					Command:     command,
				}
				m.Submitted = true
				return m, tea.Quit
			}
			// Advance focus.
			m.inputs[m.focused].Blur()
			m.focused = (m.focused + 1) % fieldCount
			m.inputs[m.focused].Focus()
			return m, nil
		}
	}

	// Route key events to the focused input.
	var cmd tea.Cmd
	m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
	return m, cmd
}

func (m FormModel) View() string {
	labels := [fieldCount]string{
		"Description",
		"Command (required)",
	}

	var b strings.Builder
	b.WriteString(styleTitle.Render(m.title))
	b.WriteString("\n\n")

	w := m.inputWidth()
	if w <= 0 {
		w = 60
	}

	for i := 0; i < fieldCount; i++ {
		b.WriteString(styleLabel.Render(labels[i]))
		b.WriteString("\n")

		inp := m.inputs[i]
		inp.Width = w
		var box lipgloss.Style
		if i == m.focused {
			box = styleFocusedInput
		} else {
			box = styleBlurredInput
		}
		b.WriteString(box.Render(inp.View()))
		b.WriteString("\n\n")
	}

	help := "tab/↓ next • shift+tab/↑ prev • enter confirm • esc cancel"
	if m.focused != fieldCommand {
		help = "tab/↓ next field • enter advance • esc cancel"
	}
	b.WriteString(styleHelp.Render(help))
	return b.String()
}

func (m FormModel) inputWidth() int {
	if m.width > 10 {
		w := m.width - 6 // account for border+padding
		if w > 80 {
			return 80
		}
		return w
	}
	return 60
}

