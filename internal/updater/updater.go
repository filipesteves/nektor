// Package updater handles checking for and applying nektor self-updates.
package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	githubReleasesURL = "https://api.github.com/repos/filipesteves/nektor/releases/latest"
	checkTimeout      = 3 * time.Second
)

// githubRelease is the subset of the GitHub API response we care about.
type githubRelease struct {
	TagName string `json:"tag_name"`
}

// CheckForUpdate calls the GitHub Releases API and reports whether a version
// newer than currentVersion is available. Any error (network failure, rate
// limiting, malformed response) is returned quietly — callers should treat
// errors as "no update information available" rather than a hard failure.
func CheckForUpdate(currentVersion string) (latestVersion string, hasUpdate bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubReleasesURL, nil)
	if err != nil {
		return "", false, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("fetching releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", false, fmt.Errorf("decoding response: %w", err)
	}

	latest := release.TagName
	if latest == "" {
		return "", false, fmt.Errorf("empty tag_name in response")
	}

	return latest, IsNewer(latest, currentVersion), nil
}

// IsNewer reports whether candidate is a strictly greater semver than baseline.
// Both strings are expected in "vX.Y.Z" form; anything that cannot be compared
// as semver returns false so we never falsely claim an update is available.
func IsNewer(candidate, baseline string) bool {
	c := normalizeSemver(candidate)
	b := normalizeSemver(baseline)

	// "dev" or empty baselines mean the binary was built locally — skip update
	// prompts so developers aren't nagged on every run.
	if b == "" || b == "dev" {
		return false
	}
	if c == "" {
		return false
	}

	return compareSemver(c, b) > 0
}

// normalizeSemver strips a leading "v" and returns the version string, or ""
// if the result is empty or the special "dev" sentinel.
func normalizeSemver(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	return v
}

// compareSemver compares two "X.Y.Z" version strings.
// Returns -1, 0, or 1.  Non-numeric segments are treated as 0.
func compareSemver(a, b string) int {
	aParts := splitParts(a)
	bParts := splitParts(b)

	for i := 0; i < 3; i++ {
		av := partAt(aParts, i)
		bv := partAt(bParts, i)
		if av != bv {
			if av > bv {
				return 1
			}
			return -1
		}
	}
	return 0
}

func splitParts(v string) []string {
	// Only keep the core version, strip pre-release/build metadata.
	v = strings.SplitN(v, "-", 2)[0]
	v = strings.SplitN(v, "+", 2)[0]
	return strings.Split(v, ".")
}

func partAt(parts []string, i int) int {
	if i >= len(parts) {
		return 0
	}
	// strconv.Atoi handles overflow and non-numeric input cleanly; on error we
	// treat the segment as 0 so a malformed version never panics or wraps.
	n, err := strconv.Atoi(parts[i])
	if err != nil || n < 0 {
		return 0
	}
	return n
}
