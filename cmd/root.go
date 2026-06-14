// Package cmd defines all cobra CLI commands for nektor.
package cmd

import (
	"fmt"
	"os"
	"sync/atomic"

	"github.com/filipesteves/nektor/internal/store"
	"github.com/filipesteves/nektor/internal/updater"
	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags "-X github.com/filipesteves/nektor/cmd.Version=vX.Y.Z".
// It defaults to "dev" for local builds.
var Version = "dev"

// updateAvailable holds the cached latest version string (or empty) as an
// atomically-swapped pointer so the background goroutine in checkUpdates can
// write it safely while the main goroutine (or TUI) reads it.
var updateAvailable atomic.Pointer[string]

// UpdateAvailable returns the cached latest version string if a newer release
// was detected, or "" if no update is pending. Safe for concurrent use.
func UpdateAvailable() string {
	if p := updateAvailable.Load(); p != nil {
		return *p
	}
	return ""
}

func setUpdateAvailable(version string) {
	updateAvailable.Store(&version)
}

var rootCmd = &cobra.Command{
	Use:   "nektor",
	Short: "A personal command recall tool",
	Long: `nektor lets you save infrequently-used shell commands (SSH, maintenance scripts,
service restarts, etc.) and quickly recall them via a fuzzy-searchable TUI.

Use 'nektor install' to set up the 'nk' shell wrapper for the best experience.`,
	// Don't print usage on arbitrary errors — only on incorrect invocations.
	SilenceUsage: true,
	// PersistentPreRun runs before every sub-command.
	PersistentPreRun: func(_ *cobra.Command, _ []string) {
		checkUpdates()
	},
}

// checkUpdates reads the cached latest version and decides whether to surface
// an update notice. It also fires an async background fetch so the cache is
// warm for the next run.
func checkUpdates() {
	cfg, err := store.LoadConfig()
	if err != nil {
		return
	}

	// Show update notice based on last-run cached result.
	if cfg.LatestVersion != "" && updater.IsNewer(cfg.LatestVersion, Version) {
		setUpdateAvailable(cfg.LatestVersion)
	}

	// Fire background fetch — result is written to config for the next startup.
	if store.ShouldCheckForUpdate(cfg) {
		go func() {
			latest, hasUpdate, err := updater.CheckForUpdate(Version)
			if err != nil {
				return
			}
			_ = store.SaveUpdateCheckResult(latest)
			// Also update the in-process var so the current session benefits
			// if the TUI hasn't launched yet. The atomic store makes this safe.
			if hasUpdate {
				setUpdateAvailable(latest)
			}
		}()
	}
}

// Execute is the entry point called from main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Version = Version
	rootCmd.AddCommand(pickCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(editCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(installCmd)
}
