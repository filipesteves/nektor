package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/filipesteves/nektor/internal/store"
	"github.com/filipesteves/nektor/internal/ui"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Set up nektor and the nk shell wrapper",
	Long: `Guides you through choosing an output mode and, for the shell wrapper mode,
appends the 'nk' function to your shell's rc file.`,
	RunE: runInstall,
}

func runInstall(_ *cobra.Command, _ []string) error {
	// Step 1: pick output mode via TUI.
	m := ui.NewInstallModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	im, ok := finalModel.(ui.InstallModel)
	if !ok || im.Cancelled || !im.Confirmed {
		fmt.Fprintln(os.Stderr, "Install cancelled.")
		return nil
	}

	cfg, err := store.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	cfg.OutputMode = string(im.Result)

	// Step 2: copy the binary to ~/.local/bin/nektor.
	if err := installBinary(); err != nil {
		// Non-fatal — the user may not have write permission or may be running
		// from a read-only location. Warn but continue.
		fmt.Fprintf(os.Stderr, "Warning: could not install binary: %v\n", err)
	}

	// Step 3: for shell mode, write the nk wrapper.
	if im.Result == ui.InstallChoiceShell {
		rcPath, err := shellRCPath(cfg.Shell)
		if err != nil {
			return err
		}
		if err := appendShellWrapper(cfg.Shell, rcPath); err != nil {
			return err
		}
		// Ensure ~/.local/bin is in PATH inside the same rc file.
		if err := ensureLocalBinInPath(rcPath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not update PATH in %s: %v\n", rcPath, err)
		}
		fmt.Printf("\nAdded 'nk' function to %s\n", rcPath)
		fmt.Printf("Run: source %s\n(or open a new terminal)\n", rcPath)
	}

	// Step 4: persist config.
	if err := store.SaveConfig(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("\nOutput mode set to: %s\n", cfg.OutputMode)
	fmt.Println("\nSetup complete. Use 'nektor add' to save your first commands.")
	return nil
}

// localBinDir returns the path to ~/.local/bin, creating it if necessary.
func localBinDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding home directory: %w", err)
	}
	dir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating %s: %w", dir, err)
	}
	return dir, nil
}

// installBinary copies the currently running nektor binary to ~/.local/bin/nektor.
func installBinary() error {
	src, err := os.Executable()
	if err != nil {
		return fmt.Errorf("determining executable path: %w", err)
	}
	// Resolve symlinks so we copy the real file.
	src, err = filepath.EvalSymlinks(src)
	if err != nil {
		return fmt.Errorf("resolving executable symlink: %w", err)
	}

	binDir, err := localBinDir()
	if err != nil {
		return err
	}
	dst := filepath.Join(binDir, "nektor")

	// Skip if src == dst (already installed in place).
	if src == dst {
		fmt.Printf("Binary already at %s — skipping copy.\n", dst)
		return nil
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source binary: %w", err)
	}
	defer in.Close()

	// Write to a temp file then rename for atomicity.
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("creating destination file: %w", err)
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return fmt.Errorf("copying binary: %w", err)
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("flushing destination: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("installing binary: %w", err)
	}

	fmt.Printf("Installed binary to %s\n", dst)
	return nil
}

// localBinPathExport is the export line appended to shell rc files.
const localBinPathExport = `export PATH="$HOME/.local/bin:$PATH"`

// localBinPathSentinel is used to detect an existing export in an rc file.
const localBinPathSentinel = `.local/bin`

// ensureLocalBinInPath checks whether ~/.local/bin is already in the process
// $PATH or in the rc file. If not, it appends the export line.
func ensureLocalBinInPath(rcPath string) error {
	// Resolve the exact target directory so the comparison is precise.
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("finding home directory: %w", err)
	}
	localBin := filepath.Join(home, ".local", "bin")

	// If the exact path (or its clean equivalent) is already in the live PATH,
	// nothing to do.  Using filepath.EvalSymlinks avoids false negatives when
	// the same directory is referenced via a symlink.
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		resolved, err := filepath.EvalSymlinks(p)
		if err != nil {
			resolved = filepath.Clean(p)
		}
		if resolved == localBin {
			return nil
		}
	}

	// Check if the rc file already exports it.
	existing, err := os.ReadFile(rcPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("reading %s: %w", rcPath, err)
	}
	if strings.Contains(string(existing), localBinPathSentinel) {
		return nil
	}

	f, err := os.OpenFile(rcPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening %s: %w", rcPath, err)
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, "\n%s\n", localBinPathExport); err != nil {
		return fmt.Errorf("writing PATH export: %w", err)
	}
	fmt.Printf("Added ~/.local/bin to PATH in %s\n", rcPath)
	return nil
}

func shellRCPath(shell string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding home directory: %w", err)
	}
	switch shell {
	case "bash":
		return filepath.Join(home, ".bashrc"), nil
	default:
		return filepath.Join(home, ".zshrc"), nil
	}
}

// nkWrapper returns the shell function text for the given shell.
func nkWrapper(shell string) string {
	if shell == "bash" {
		return `
# nektor - place selected command on the readline buffer
nk() {
  local cmd
  cmd=$(nektor pick)
  if [[ -n "$cmd" ]]; then
    READLINE_LINE="$cmd"
    READLINE_POINT=${#cmd}
  fi
}
`
	}
	// zsh
	return `
# nektor - place selected command on the prompt line
nk() {
  local cmd
  cmd=$(nektor pick)
  if [[ -n "$cmd" ]]; then
    print -z "$cmd"
  fi
}
`
}

// sentinel written alongside the function so we don't append duplicates.
const wrapperSentinel = "# nektor - place selected command"

// appendShellWrapper appends the nk function to rcPath unless it is already present.
func appendShellWrapper(shell, rcPath string) error {
	// Check for existing installation.
	existing, err := os.ReadFile(rcPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("reading %s: %w", rcPath, err)
	}
	if strings.Contains(string(existing), wrapperSentinel) {
		fmt.Printf("'nk' function already present in %s — skipping.\n", rcPath)
		return nil
	}

	f, err := os.OpenFile(rcPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening %s: %w", rcPath, err)
	}
	defer f.Close()

	if _, err := fmt.Fprint(f, nkWrapper(shell)); err != nil {
		return fmt.Errorf("writing to %s: %w", rcPath, err)
	}
	return nil
}
