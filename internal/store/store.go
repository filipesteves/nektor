// Package store handles reading and writing nektor's TOML configuration files.
package store

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/BurntSushi/toml"
)

// Command represents a saved shell command entry.
type Command struct {
	Alias       string   `toml:"alias"`
	Description string   `toml:"description"`
	Tags        []string `toml:"tags"`
	Command     string   `toml:"command"`
}

// CommandFile is the top-level structure of commands.toml.
type CommandFile struct {
	Commands []Command `toml:"commands"`
}

// Config holds user preferences from config.toml.
type Config struct {
	OutputMode      string    `toml:"output_mode"`       // "shell" or "clipboard"
	Shell           string    `toml:"shell"`             // "zsh" or "bash"
	LastUpdateCheck time.Time `toml:"last_update_check"` // zero value = never checked
	LatestVersion   string    `toml:"latest_version"`    // cached from last GitHub check
}

// updateCheckInterval is how long to wait between live GitHub API calls.
const updateCheckInterval = 24 * time.Hour

// ShouldCheckForUpdate returns true if enough time has passed since the last
// update check that we should make a new network request.
func ShouldCheckForUpdate(cfg Config) bool {
	return cfg.LastUpdateCheck.IsZero() || time.Since(cfg.LastUpdateCheck) > updateCheckInterval
}

// SaveUpdateCheckResult persists the latest version string and the current
// timestamp to config.toml so the next startup can read it without a network call.
func SaveUpdateCheckResult(latestVersion string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	cfg.LastUpdateCheck = time.Now()
	cfg.LatestVersion = latestVersion
	return SaveConfig(cfg)
}

// DefaultConfig returns sensible defaults for a new config.
func DefaultConfig() Config {
	shell := "zsh"
	if s := os.Getenv("SHELL"); s != "" {
		base := filepath.Base(s)
		if base == "bash" {
			shell = "bash"
		}
	}
	return Config{
		OutputMode: "shell",
		Shell:      shell,
	}
}

// ConfigDir returns the path to the nektor config directory (~/.config/nektor),
// creating it if it does not exist.
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	dir := filepath.Join(home, ".config", "nektor")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("could not create config directory: %w", err)
	}
	return dir, nil
}

// CommandsPath returns the absolute path to commands.toml.
func CommandsPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "commands.toml"), nil
}

// ConfigPath returns the absolute path to config.toml.
func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

// LoadCommands reads commands.toml and returns all saved commands. If the file
// does not exist it returns an empty slice — not an error.
func LoadCommands() ([]Command, error) {
	path, err := CommandsPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading commands file: %w", err)
	}

	var cf CommandFile
	if _, err := toml.NewDecoder(bytes.NewReader(data)).Decode(&cf); err != nil {
		return nil, fmt.Errorf("parsing commands file: %w", err)
	}
	return cf.Commands, nil
}

// SaveCommands writes the full list of commands to commands.toml, replacing
// any existing content. The write is atomic: data is first written to a
// temporary file and then renamed into place to avoid partial-write corruption.
func SaveCommands(commands []Command) error {
	path, err := CommandsPath()
	if err != nil {
		return err
	}

	cf := CommandFile{Commands: commands}
	return atomicWriteTOML(path, cf)
}

// AddCommand appends a new command to commands.toml. It returns an error if a
// command with the same alias or same command string already exists.
func AddCommand(cmd Command) error {
	existing, err := LoadCommands()
	if err != nil {
		return err
	}
	for _, c := range existing {
		if c.Alias == cmd.Alias {
			return fmt.Errorf("an entry with alias %q already exists", cmd.Alias)
		}
		if c.Command == cmd.Command {
			return fmt.Errorf("command already saved under alias %q", c.Alias)
		}
	}
	existing = append(existing, cmd)
	return SaveCommands(existing)
}

// UpdateCommand replaces the command that matches alias with the provided value.
// It returns an error if no command with that alias is found.
func UpdateCommand(alias string, updated Command) error {
	existing, err := LoadCommands()
	if err != nil {
		return err
	}
	found := false
	for i, c := range existing {
		if c.Alias == alias {
			existing[i] = updated
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("no command with alias %q found", alias)
	}
	return SaveCommands(existing)
}

// DeleteCommand removes the command with the given alias. Returns an error if
// no such command exists.
func DeleteCommand(alias string) error {
	existing, err := LoadCommands()
	if err != nil {
		return err
	}
	// Use a fresh slice — reusing existing[:0] would alias the backing array and
	// overwrite entries that haven't been examined yet during the append loop.
	filtered := make([]Command, 0, len(existing))
	found := false
	for _, c := range existing {
		if c.Alias == alias {
			found = true
			continue
		}
		filtered = append(filtered, c)
	}
	if !found {
		return fmt.Errorf("no command with alias %q found", alias)
	}
	return SaveCommands(filtered)
}

// GenerateAlias creates a unique kebab-case alias from a description string.
// If the slugified description collides with an existing alias it appends -2, -3, etc.
func GenerateAlias(description string, existing []Command) string {
	slug := slugify(description)
	if slug == "" {
		slug = "command"
	}
	used := make(map[string]bool, len(existing))
	for _, c := range existing {
		used[c.Alias] = true
	}
	if !used[slug] {
		return slug
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", slug, i)
		if !used[candidate] {
			return candidate
		}
	}
}

// slugify converts a string to a lowercase kebab-case identifier.
func slugify(s string) string {
	var b strings.Builder
	lastWasDash := true // avoid leading dash
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastWasDash = false
		} else if !lastWasDash {
			b.WriteByte('-')
			lastWasDash = true
		}
	}
	result := strings.TrimRight(b.String(), "-")
	return result
}

// GetCommand returns the command with the given alias, or an error if not found.
func GetCommand(alias string) (Command, error) {
	commands, err := LoadCommands()
	if err != nil {
		return Command{}, err
	}
	for _, c := range commands {
		if c.Alias == alias {
			return c, nil
		}
	}
	return Command{}, fmt.Errorf("no command with alias %q found", alias)
}

// LoadConfig reads config.toml. If it does not exist, returns defaults.
func LoadConfig() (Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return DefaultConfig(), err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultConfig(), nil
		}
		return DefaultConfig(), fmt.Errorf("reading config file: %w", err)
	}

	cfg := DefaultConfig()
	if _, err := toml.NewDecoder(bytes.NewReader(data)).Decode(&cfg); err != nil {
		return DefaultConfig(), fmt.Errorf("parsing config file: %w", err)
	}
	return cfg, nil
}

// SaveConfig writes the provided Config to config.toml atomically.
func SaveConfig(cfg Config) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	return atomicWriteTOML(path, cfg)
}

// atomicWriteTOML encodes v as TOML and writes it to path atomically by writing
// to a sibling .tmp file first, then renaming. This prevents a crash mid-write
// from leaving a partially-written (corrupt) file.
func atomicWriteTOML(path string, v any) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}

	if err := toml.NewEncoder(f).Encode(v); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("encoding TOML: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("flushing temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}
