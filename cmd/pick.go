package cmd

import (
	"fmt"
	"os"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/filipesteves/nektor/internal/store"
	"github.com/filipesteves/nektor/internal/ui"
	"github.com/filipesteves/nektor/internal/updater"
)

var pickCmd = &cobra.Command{
	Use:    "pick",
	Short:  "Open the command picker TUI (called by the nk shell wrapper)",
	Hidden: true, // internal command; users interact via the `nk` shell wrapper
	RunE:   runPick,
}

func runPick(_ *cobra.Command, _ []string) error {
	commands, err := store.LoadCommands()
	if err != nil {
		return fmt.Errorf("loading commands: %w", err)
	}
	if len(commands) == 0 {
		fmt.Fprintln(os.Stderr, "No commands saved yet. Run 'nektor add' to add some.")
		return nil
	}

	cfg, err := store.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Wire the update-availability function so the TUI can read it without
	// importing cmd (which would create a cycle).
	ui.SetUpdateAvailableFunc(UpdateAvailable)

	model := ui.NewPickModel(commands)

	// nektor pick writes the selected command to stdout which the nk shell
	// wrapper captures. We must therefore:
	//   - render the TUI to stderr (so it doesn't pollute the captured stdout)
	//   - output ONLY the command string to stdout on success
	// stdout is captured by the nk shell wrapper (cmd=$(nektor pick)), so it
	// is a pipe rather than a TTY. By default lipgloss probes os.Stdout for
	// colour support; override the global renderer to use os.Stderr (the real
	// TTY) so background colours aren't stripped.
	lipgloss.SetDefaultRenderer(lipgloss.NewRenderer(os.Stderr))

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithOutput(os.Stderr),
	)

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	pm, ok := finalModel.(ui.PickModel)
	if !ok {
		return nil
	}

	// Handle self-update request (ctrl+u in TUI).
	if pm.UpdateRequested != "" {
		return doSelfUpdate(pm.UpdateRequested)
	}

	if pm.Result == "" {
		return nil // user cancelled
	}

	switch cfg.OutputMode {
	case "clipboard":
		if err := clipboard.WriteAll(pm.Result); err != nil {
			// Clipboard may not be available on headless systems — print to stdout
			// as fallback so the shell wrapper still works.
			fmt.Fprint(os.Stderr, "Clipboard unavailable, falling back to stdout: ")
			fmt.Println(pm.Result)
			return nil
		}
		fmt.Fprintln(os.Stderr, "Copied to clipboard.")
	default:
		// Shell mode: print command to stdout. The nk wrapper captures this and
		// places it on the readline buffer.
		fmt.Println(pm.Result)
	}
	return nil
}

// doSelfUpdate performs the in-place binary replacement and prints status.
func doSelfUpdate(version string) error {
	fmt.Fprintf(os.Stderr, "Updating to %s...\n", version)
	if err := updater.SelfUpdate(version); err != nil {
		return fmt.Errorf("self-update failed: %w", err)
	}
	fmt.Fprintln(os.Stderr, "Done. Please restart nektor.")
	return nil
}
