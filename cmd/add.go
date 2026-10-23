package cmd

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/filipesteves/nektor/internal/store"
	"github.com/filipesteves/nektor/internal/ui"
	"github.com/filipesteves/nektor/internal/updater"
)

var addCmd = &cobra.Command{
	Use:   "add [command...]",
	Short: "Add a command directly, or pick commands from shell history",
	Long: `With arguments, saves the given command and prompts for a description:

  nektor add ssh pi@192.168.1.42
  nektor add 'docker ps | grep portainer'   # quote pipes, &&, redirects, etc.

Without arguments, opens a filterable list of your shell history. Select entries
with Tab, then confirm with Enter to fill in a description for each one.`,
	// Everything after "add" is the command to save, so don't let cobra treat
	// its flags (e.g. "ls -la") as nektor flags.
	DisableFlagParsing: true,
	RunE:               runAdd,
}

func runAdd(c *cobra.Command, args []string) error {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		return c.Help()
	}
	if len(args) > 0 {
		return addDirect(strings.Join(args, " "))
	}

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

// addDirect saves a command given on the command line, prompting only for its
// description.
func addDirect(command string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return fmt.Errorf("no command given")
	}

	existing, err := store.LoadCommands()
	if err != nil {
		return fmt.Errorf("loading existing commands: %w", err)
	}
	for _, c := range existing {
		if c.Command == command {
			fmt.Fprintf(os.Stderr, "Already saved as %q.\n", c.Description)
			return nil
		}
	}

	p := tea.NewProgram(ui.NewCommandFormModel(command), tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	fm, ok := finalModel.(ui.FormModel)
	if !ok || fm.Cancelled || !fm.Submitted {
		fmt.Fprintln(os.Stderr, "Cancelled.")
		return nil
	}

	entry := store.Command{
		Alias:       store.GenerateAlias(fm.Result.Description, existing),
		Description: fm.Result.Description,
		Command:     fm.Result.Command,
	}
	if err := store.AddCommand(entry); err != nil {
		return err
	}
	fmt.Printf("Saved %q.\n", entry.Description)
	return nil
}
