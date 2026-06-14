package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/filipesteves/nektor/internal/store"
)

var deleteCmd = &cobra.Command{
	Use:     "delete <alias>",
	Aliases: []string{"rm", "remove"},
	Short:   "Delete a saved command by alias",
	Args:    cobra.ExactArgs(1),
	RunE:    runDelete,
}

func runDelete(_ *cobra.Command, args []string) error {
	alias := args[0]

	cmd, err := store.GetCommand(alias)
	if err != nil {
		return err
	}

	fmt.Printf("Alias:   %s\n", cmd.Alias)
	fmt.Printf("Desc:    %s\n", cmd.Description)
	if len(cmd.Tags) > 0 {
		fmt.Printf("Tags:    %s\n", strings.Join(cmd.Tags, ", "))
	}
	fmt.Printf("Command: %s\n", cmd.Command)
	fmt.Printf("\nDelete %q? [y/N] ", alias)

	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	if strings.ToLower(strings.TrimSpace(answer)) != "y" {
		fmt.Println("Aborted.")
		return nil
	}

	if err := store.DeleteCommand(alias); err != nil {
		return fmt.Errorf("deleting command: %w", err)
	}

	fmt.Printf("Deleted %q.\n", alias)
	return nil
}
