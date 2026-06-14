package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/filipesteves/nektor/internal/store"
	"github.com/filipesteves/nektor/internal/ui"
	"github.com/filipesteves/nektor/internal/updater"
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add commands from shell history",
	Long: `Opens a filterable list of your shell history. Select entries with Tab,
then confirm with Enter to fill in alias/description metadata for each one.`,
	RunE: runAdd,
}

func runAdd(_ *cobra.Command, _ []string) error {
	cfg, err := store.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	entries, err := ui.LoadHistory(cfg.Shell)
	if err != nil {
		// Non-fatal — user may not have a history file yet.
		fmt.Fprintf(os.Stderr, "Warning: could not read shell history: %v\n", err)
		entries = nil
	}
	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "Shell history is empty or could not be read.")
		return nil
	}

	existing, err := store.LoadCommands()
	if err != nil {
		return fmt.Errorf("loading existing commands: %w", err)
	}

	// Wire the update-availability function so the TUI can read it without
	// importing cmd (which would create a cycle).
	ui.SetAddUpdateAvailableFunc(UpdateAvailable)

	model := ui.NewAddModel(entries, existing)
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	am, ok := finalModel.(ui.AddModel)
	if !ok {
		return nil
	}

	// Handle self-update request (ctrl+u in TUI).
	if am.UpdateRequested != "" {
		fmt.Fprintf(os.Stderr, "Updating to %s...\n", am.UpdateRequested)
		if err := updater.SelfUpdate(am.UpdateRequested); err != nil {
			return fmt.Errorf("self-update failed: %w", err)
		}
		fmt.Fprintln(os.Stderr, "Done. Please restart nektor.")
		return nil
	}

	if am.Cancelled || len(am.Results) == 0 {
		if am.Cancelled {
			fmt.Fprintln(os.Stderr, "Cancelled.")
		}
		return nil
	}

	// Load existing commands once so GenerateAlias can avoid collisions across
	// the batch being saved in this session.
	existingForAlias, _ := store.LoadCommands()

	saved := 0
	skipped := 0
	for _, entry := range am.Results {
		entry.Alias = store.GenerateAlias(entry.Description, existingForAlias)
		if err := store.AddCommand(entry); err != nil {
			fmt.Fprintf(os.Stderr, "Skipping %q: %v\n", entry.Alias, err)
			skipped++
			continue
		}
		existingForAlias = append(existingForAlias, entry) // keep alias pool fresh for next item
		saved++
	}

	fmt.Printf("Saved %d command(s).", saved)
	if skipped > 0 {
		fmt.Printf(" Skipped %d duplicate(s).", skipped)
	}
	fmt.Println()
	return nil
}
