package ui

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// HistoryEntry represents a single shell history entry.
type HistoryEntry struct {
	Command string
}

// LoadHistory reads shell history from the appropriate history file, returning
// deduplicated entries in reverse order (most recent first).
func LoadHistory(shell string) ([]HistoryEntry, error) {
	path, err := historyPath(shell)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening history file %s: %w", path, err)
	}
	defer f.Close()

	var raw []string
	scanner := bufio.NewScanner(f)
	// History files can be large; increase buffer.
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, len(buf))

	for scanner.Scan() {
		line := scanner.Text()
		cmd := parseHistoryLine(shell, line)
		if cmd != "" {
			raw = append(raw, cmd)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading history: %w", err)
	}

	// Deduplicate and reverse — most recent first.
	seen := make(map[string]bool)
	out := make([]HistoryEntry, 0, len(raw))
	for i := len(raw) - 1; i >= 0; i-- {
		cmd := raw[i]
		if seen[cmd] {
			continue
		}
		seen[cmd] = true
		out = append(out, HistoryEntry{Command: cmd})
	}
	return out, nil
}

func historyPath(shell string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding home directory: %w", err)
	}

	// Respect explicit history file environment variables.
	if hf := os.Getenv("HISTFILE"); hf != "" {
		return hf, nil
	}

	switch shell {
	case "bash":
		return filepath.Join(home, ".bash_history"), nil
	default: // zsh
		return filepath.Join(home, ".zsh_history"), nil
	}
}

// parseHistoryLine handles both plain history lines and zsh's extended format.
// Zsh extended format: `: timestamp:duration;command`
func parseHistoryLine(shell, line string) string {
	if shell != "zsh" && shell != "" {
		return strings.TrimSpace(line)
	}

	// Zsh extended_history format starts with ': '.
	if strings.HasPrefix(line, ": ") {
		idx := strings.Index(line, ";")
		if idx != -1 && idx+1 < len(line) {
			return strings.TrimSpace(line[idx+1:])
		}
		return ""
	}
	return strings.TrimSpace(line)
}
