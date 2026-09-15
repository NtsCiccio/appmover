// Package update checks GitHub Releases for a newer AppMover version than
// the one currently running.
package update

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const releasesURL = "https://api.github.com/repos/NtsCiccio/appmover/releases/latest"

// Result describes the outcome of a successful check.
type Result struct {
	Available bool
	Version   string // latest release version, without a leading "v"
	URL       string // release page to open, e.g. for the user to download it
}

type ghRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// Check queries the GitHub releases API for the latest AppMover release
// and reports whether it's newer than currentVersion. currentVersion may
// have a leading "v" or not; a currentVersion of "" or "dev" (the default
// for a build that didn't set internal/version.Version) always reports no
// update available, since there's nothing meaningful to compare against.
func Check(currentVersion string) (Result, error) {
	if currentVersion == "" || currentVersion == "dev" {
		return Result{}, nil
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(releasesURL)
	if err != nil {
		return Result{}, fmt.Errorf("fetching %s: %w", releasesURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("fetching %s: unexpected status %s", releasesURL, resp.Status)
	}

	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return Result{}, fmt.Errorf("decoding release info: %w", err)
	}

	latest := strings.TrimPrefix(rel.TagName, "v")
	current := strings.TrimPrefix(currentVersion, "v")

	return Result{
		Available: isNewer(latest, current),
		Version:   latest,
		URL:       rel.HTMLURL,
	}, nil
}

// isNewer reports whether latest is a greater dotted version than current
// (e.g. "1.10.0" > "1.9.2"), comparing components numerically rather than
// lexicographically. A non-numeric component compares as 0, and a version
// with fewer components is padded with zeros, so a malformed version never
// causes an error — it just compares as if unset.
func isNewer(latest, current string) bool {
	lp := strings.Split(latest, ".")
	cp := strings.Split(current, ".")

	for i := 0; i < len(lp) || i < len(cp); i++ {
		var lv, cv int
		if i < len(lp) {
			lv, _ = strconv.Atoi(lp[i])
		}
		if i < len(cp) {
			cv, _ = strconv.Atoi(cp[i])
		}
		if lv != cv {
			return lv > cv
		}
	}
	return false
}
