package updater

import (
	"bufio"
	"context"
	"crypto"
	_ "crypto/sha256" // register SHA-256 for crypto.SHA256
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/inconshreveable/go-update"
)

const (
	downloadTimeout  = 60 * time.Second
	checksumTimeout  = 30 * time.Second
	// maxBinarySize caps the response body read during a self-update to 100 MiB.
	// A realistic nektor binary is well under 50 MiB; this prevents an attacker
	// or corrupted CDN from streaming an unbounded payload into update.Apply.
	maxBinarySize = 100 << 20 // 100 MiB
)

// SelfUpdate downloads the release binary for latestVersion, verifies its
// SHA-256 checksum against the goreleaser-generated checksums.txt, and
// atomically replaces the currently running executable. The process must be
// restarted by the caller after a successful return.
//
// The asset naming convention matches the goreleaser config:
//
//	nektor_darwin_arm64
//	nektor_linux_amd64
//	etc.
func SelfUpdate(latestVersion string) error {
	tag, goos, goarch, err := platformInfo(latestVersion)
	if err != nil {
		return err
	}

	assetName := fmt.Sprintf("nektor_%s_%s", goos, goarch)
	assetURL := fmt.Sprintf(
		"https://github.com/filipesteves/nektor/releases/download/%s/%s",
		tag, assetName,
	)

	checksum, err := fetchChecksum(tag, assetName)
	if err != nil {
		return fmt.Errorf("fetching checksum: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	if err != nil {
		return fmt.Errorf("building download request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("downloading release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d for %s", resp.StatusCode, assetURL)
	}

	limited := io.LimitReader(resp.Body, maxBinarySize)
	if err := update.Apply(limited, update.Options{
		Hash:     crypto.SHA256,
		Checksum: checksum,
	}); err != nil {
		return fmt.Errorf("applying update: %w", err)
	}

	return nil
}

// fetchChecksum downloads checksums.txt for the given release tag and returns
// the raw SHA-256 bytes for assetName.
func fetchChecksum(tag, assetName string) ([]byte, error) {
	url := fmt.Sprintf(
		"https://github.com/filipesteves/nektor/releases/download/%s/checksums.txt",
		tag,
	)

	ctx, cancel := context.WithTimeout(context.Background(), checksumTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("checksums.txt returned status %d", resp.StatusCode)
	}

	// checksums.txt format (goreleaser): "<hex-sha256>  <filename>"
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && fields[1] == assetName {
			return hex.DecodeString(fields[0])
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return nil, fmt.Errorf("checksum not found for %s in release %s", assetName, tag)
}

// platformInfo validates the current platform and normalises the version tag.
// Returns (tag, goos, goarch, err).
func platformInfo(version string) (tag, goos, goarch string, err error) {
	tag = strings.TrimSpace(version)
	if tag == "" {
		return "", "", "", fmt.Errorf("empty version string")
	}
	if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}

	goos = runtime.GOOS
	goarch = runtime.GOARCH

	switch goos {
	case "darwin", "linux":
	default:
		return "", "", "", fmt.Errorf(
			"self-update not supported on %s/%s — download manually from https://github.com/filipesteves/nektor/releases",
			goos, goarch,
		)
	}

	return tag, goos, goarch, nil
}
