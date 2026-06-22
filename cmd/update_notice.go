package cmd

import (
	"strings"

	project "github.com/fatecannotbealtered/wechat-mp-cli"
	"github.com/fatecannotbealtered/wechat-mp-cli/internal/config"
	"github.com/fatecannotbealtered/wechat-mp-cli/internal/output"
)

// Update-notice plumbing (CLI-SPEC §14). Three responsibilities:
//
//  1. updateNoticesFromRelease builds the update-available notice, with its
//     severity graded from the embedded CHANGELOG delta between the running
//     version and the latest. Active-check commands (update --check / doctor)
//     build it, surface it in their `data`, and persist it to the local cache.
//  2. readCachedUpdateNotices reads that cache (read-only, TTL-bounded, no
//     network) so any command can piggyback it onto meta.notices.
//  3. The init() below wires output.UpdateNoticesProvider to (2), via a
//     func-pointer hook so package output never imports package cmd.

func init() {
	output.UpdateNoticesProvider = readCachedUpdateNotices
}

// readCachedUpdateNotices returns the cached update notice as a one-element
// slice, or nil when the cache is empty/expired. It performs ZERO network I/O —
// it only reads the local TTL-bounded cache.
func readCachedUpdateNotices() []any {
	notice, ok := config.LoadCachedUpdateNotice()
	if !ok {
		return nil
	}
	return []any{notice}
}

// updateNoticesFromRelease builds the update-available notice for the running
// version vs. the latest release, computing severity from the CHANGELOG delta.
// It returns nil when no update is available. The result is the canonical notice
// payload surfaced in active-check command data AND stored in the cache.
func updateNoticesFromRelease(currentVersion, latestVersion, releaseURL, checkedAt, recommendedCommand, installMethod string) map[string]any {
	cmp, ok := compareVersions(currentVersion, latestVersion)
	if !ok || cmp >= 0 {
		return nil
	}
	notice := map[string]any{
		"type":                "update_available",
		"severity":            changelogDeltaSeverity(currentVersion, latestVersion),
		"current_version":     normalizeVersion(currentVersion),
		"latest_version":      normalizeVersion(latestVersion),
		"install_method":      installMethod,
		"recommended_command": recommendedCommand,
		"checked_at":          checkedAt,
		"next_steps": []string{
			recommendedCommand,
			"wechat-mp-cli changelog --since " + normalizeVersion(currentVersion),
		},
	}
	if releaseURL != "" {
		notice["release_url"] = releaseURL
	}
	return notice
}

// changelogDeltaSeverity grades the update severity from the embedded CHANGELOG
// delta between current and latest (CLI-SPEC §14):
//   - "warning" when any version in the delta has a non-empty Security category,
//     OR latest's major > current's major;
//   - "info" otherwise.
//
// "critical" is reserved and never emitted here.
func changelogDeltaSeverity(currentVersion, latestVersion string) string {
	return severityFromDelta(project.ChangelogMarkdown, currentVersion, latestVersion)
}

// severityFromDelta is changelogDeltaSeverity with the changelog body injected,
// so the grading logic is exercised directly in tests without depending on the
// embedded CHANGELOG.md.
func severityFromDelta(markdown, currentVersion, latestVersion string) string {
	if majorBump(currentVersion, latestVersion) {
		return "warning"
	}
	for _, e := range changelogEntriesSince(markdown, currentVersion) {
		if e.hasSecurity {
			return "warning"
		}
	}
	return "info"
}

// majorBump reports whether latest's major (first semver component) is greater
// than current's. Unparseable versions yield false (no false-positive warning).
func majorBump(currentVersion, latestVersion string) bool {
	cur, ok := parseVersion(currentVersion)
	if !ok {
		return false
	}
	latest, ok := parseVersion(latestVersion)
	if !ok {
		return false
	}
	return latest[0] > cur[0]
}

// changelogEntry is one parsed CHANGELOG version block.
type changelogEntry struct {
	version     string
	hasSecurity bool
}

// changelogEntriesSince parses the embedded CHANGELOG and returns the entries
// strictly newer than sinceVersion (the delta the user would receive on update).
// The "[Unreleased]" block carries no version and is skipped.
func changelogEntriesSince(markdown, sinceVersion string) []changelogEntry {
	var out []changelogEntry
	for _, e := range parseChangelogEntries(markdown) {
		if cmp, ok := compareVersions(sinceVersion, e.version); ok && cmp < 0 {
			out = append(out, e)
		}
	}
	return out
}

// parseChangelogEntries parses "## [version] - date" version blocks and flags
// each block that contains a non-empty "### Security" category. Headings whose
// bracketed token is not a parseable semver (e.g. "[Unreleased]") are skipped.
func parseChangelogEntries(markdown string) []changelogEntry {
	var (
		entries []changelogEntry
		cur     *changelogEntry
	)
	flush := func() {
		if cur != nil {
			entries = append(entries, *cur)
			cur = nil
		}
	}
	for _, line := range strings.Split(markdown, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "## "):
			flush()
			ver, ok := changelogHeadingVersion(trimmed)
			if ok {
				cur = &changelogEntry{version: ver}
			}
		case cur != nil && strings.HasPrefix(trimmed, "### "):
			category := strings.TrimSpace(strings.TrimPrefix(trimmed, "### "))
			if strings.EqualFold(category, "Security") {
				cur.hasSecurity = true
			}
		}
	}
	flush()
	return entries
}

// changelogHeadingVersion extracts the version from a "## [x.y.z] - date"
// heading. Returns (version, true) only when the bracketed token parses as a
// semver, so "## [Unreleased]" is rejected.
func changelogHeadingVersion(heading string) (string, bool) {
	open := strings.Index(heading, "[")
	closeIdx := strings.Index(heading, "]")
	if open < 0 || closeIdx <= open {
		return "", false
	}
	token := strings.TrimSpace(heading[open+1 : closeIdx])
	if _, ok := parseVersion(token); !ok {
		return "", false
	}
	return token, true
}
