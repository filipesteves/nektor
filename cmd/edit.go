package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/filipesteves/nektor/internal/store"
	"github.com/filipesteves/nektor/internal/ui"
)

var editCmd = &cobra.Command{
	Use:   "edit [alias]",
	Short: "Edit a saved command (or open commands.toml in $EDITOR)",
	Long: `Without an argument, opens ~/.config/nektor/commands.toml in $EDITOR.
With an alias argument, opens an interactive form pre-populated with that command's values.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runEdit,
}

func runEdit(_ *cobra.Command, args []string) error {
	if len(args) == 0 {
		return openInEditor()
	}
	return editByAlias(args[0])
}

func openInEditor() error {
	path, err := store.CommandsPath()
	if err != nil {
		return err
	}

	// Ensure the file exists so $EDITOR doesn't error on a missing file.
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := store.SaveCommands(nil); err != nil {
			return fmt.Errorf("creating commands file: %w", err)
		}
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}

	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor exited with error: %w", err)
	}
	return nil
}

func editByAlias(alias string) error {
	cmd, err := store.GetCommand(alias)
	if err != nil {
		return err
	}

	model := ui.NewEditFormModel(cmd)
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, runErr := p.Run()
	if runErr != nil {
		return fmt.Errorf("TUI error: %w", runErr)
	}

	fm, ok := finalModel.(ui.FormModel)
	if !ok || fm.Cancelled || !fm.Submitted {
		fmt.Fprintln(os.Stderr, "Edit cancelled.")
		return nil
	}

	r := fm.Result
	updated := store.Command{
		Alias:       r.Alias,
		Description: r.Description,
		Tags:        r.Tags,
		Command:     r.Command,
	}

	if err := store.UpdateCommand(alias, updated); err != nil {
		return fmt.Errorf("saving changes: %w", err)
	}

	fmt.Printf("Updated %q.\n", updated.Alias)
	return nil
}
