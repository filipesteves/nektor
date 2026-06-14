package ui

import "github.com/charmbracelet/lipgloss"

// palette — all colours are adaptive so the tool works on both light and dark terminals.
var (
	colPrimary = lipgloss.AdaptiveColor{Light: "#7B2FBE", Dark: "#C084FC"}
	colMuted   = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
	colSuccess = lipgloss.AdaptiveColor{Light: "#059669", Dark: "#34D399"}
	colDanger  = lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#F87171"}
	colSubtle  = lipgloss.AdaptiveColor{Light: "#E5E7EB", Dark: "#374151"}
	colUpdate  = lipgloss.AdaptiveColor{Light: "#92400E", Dark: "#FCD34D"} // warm amber, not alarming
)

var (
	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colPrimary)

	styleSubtle = lipgloss.NewStyle().
			Foreground(colMuted)

	styleSuccess = lipgloss.NewStyle().
			Foreground(colSuccess)

	styleDanger = lipgloss.NewStyle().
			Foreground(colDanger)

	styleTag = lipgloss.NewStyle().
			Foreground(colPrimary).
			Background(colSubtle).
			Padding(0, 1)

	styleFocusedInput = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colPrimary).
				Padding(0, 1)

	styleBlurredInput = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colMuted).
				Padding(0, 1)

	styleLabel = lipgloss.NewStyle().
			Bold(true).
			Foreground(colMuted)

	styleHelp = lipgloss.NewStyle().
			Foreground(colMuted).
			Italic(true)

	styleUpdate = lipgloss.NewStyle().
			Foreground(colUpdate).
			Italic(true)
)
