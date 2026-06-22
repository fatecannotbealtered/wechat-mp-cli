package cmd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fatecannotbealtered/wechat-mp-cli/internal/config"
)

// writeCacheFile writes a CachedUpdateNotice straight to the on-disk cache path,
// bypassing SaveCachedUpdateNotice's "now" stamp so a test can plant a stale
// CheckedAt.
func writeCacheFile(t *testing.T, entry config.CachedUpdateNotice) error {
	t.Helper()
	out, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(config.Dir(), "update-notice.json")
	if err := os.MkdirAll(config.Dir(), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o600)
}

// isolateConfigDir points config.Dir() at a temp dir so the update-notice cache
// in these tests never touches the developer's real ~/.wechat-mp-cli.
func isolateConfigDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}

// failingRoundTripper fails the test if any HTTP request is attempted. It is the
// network seam for "this path makes no network call".
type failingRoundTripper struct{ t *testing.T }

func (f failingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	f.t.Fatalf("unexpected network call to %s — the meta.notices path must be cache-only", req.URL)
	return nil, nil
}

// runCommandCapture drives an arbitrary command through the real CLI boundary
// and returns the parsed envelope (ok/data/meta).
func runCommandCapture(t *testing.T, args ...string) map[string]any {
	t.Helper()
	updateCheck = false
	updateTargetVersion = ""
	dryRun = false
	confirmFlag = ""
	forceMode = false
	compactJSON = false
	formatMode = formatJSON

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	origStdout := os.Stdout
	os.Stdout = w

	rootCmd.SetArgs(args)
	_ = ExecuteContext(context.Background())
	_ = w.Close()
	os.Stdout = origStdout

	out, _ := io.ReadAll(r)
	_ = r.Close()

	var env map[string]any
	if err := json.Unmarshal(out, &env); err != nil {
		t.Fatalf("output is not a single JSON envelope (%v): %s", err, out)
	}
	return env
}

func metaNotices(t *testing.T, env map[string]any) []any {
	t.Helper()
	meta, _ := env["meta"].(map[string]any)
	if meta == nil {
		t.Fatalf("envelope has no meta: %v", env)
	}
	notices, _ := meta["notices"].([]any)
	return notices
}

// TestMetaNotices_PresentFromCache_NoNetwork proves that an arbitrary
// (non-update) command surfaces the cached update notice on meta.notices, and
// that the path makes ZERO network calls (the HTTP seam fails the test if used).
func TestMetaNotices_PresentFromCache_NoNetwork(t *testing.T) {
	isolateConfigDir(t)

	notice := map[string]any{
		"type":            "update_available",
		"severity":        "warning",
		"current_version": "1.0.0",
		"latest_version":  "2.0.0",
	}
	if err := config.SaveCachedUpdateNotice(notice); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	// Wire the HTTP seam to a transport that fails on any request, proving the
	// business command's meta.notices path is cache-only.
	origClient := updateBinaryHTTPClient
	updateBinaryHTTPClient = &http.Client{Transport: failingRoundTripper{t: t}}
	defer func() { updateBinaryHTTPClient = origClient }()

	env := runCommandCapture(t, "context")
	notices := metaNotices(t, env)
	if len(notices) != 1 {
		t.Fatalf("expected exactly one meta.notice from cache, got %v", notices)
	}
	got, _ := notices[0].(map[string]any)
	if got["type"] != "update_available" || got["severity"] != "warning" {
		t.Fatalf("cached notice not surfaced verbatim: %v", got)
	}
}

// TestMetaNotices_AbsentWhenCacheEmpty proves meta.notices is omitted entirely
// when the cache holds nothing (omitempty).
func TestMetaNotices_AbsentWhenCacheEmpty(t *testing.T) {
	isolateConfigDir(t)

	env := runCommandCapture(t, "context")
	meta, _ := env["meta"].(map[string]any)
	if meta == nil {
		t.Fatalf("envelope has no meta: %v", env)
	}
	if _, present := meta["notices"]; present {
		t.Fatalf("meta.notices must be absent when cache is empty, got %v", meta["notices"])
	}
}

// TestMetaNotices_AbsentWhenCacheExpired proves an entry past its TTL is treated
// as absent, so a stale notice never lingers after upgrade/withdrawal.
func TestMetaNotices_AbsentWhenCacheExpired(t *testing.T) {
	isolateConfigDir(t)

	// Write an expired entry directly to the cache file.
	stale := config.CachedUpdateNotice{
		Notice:    map[string]any{"type": "update_available"},
		CheckedAt: time.Now().Add(-48 * time.Hour).UTC(),
	}
	if _, ok := config.LoadCachedUpdateNotice(); ok {
		t.Fatal("precondition: cache should start empty")
	}
	if err := writeCacheFile(t, stale); err != nil {
		t.Fatalf("seed expired cache: %v", err)
	}

	env := runCommandCapture(t, "context")
	meta, _ := env["meta"].(map[string]any)
	if _, present := meta["notices"]; present {
		t.Fatalf("expired cache must yield no meta.notices, got %v", meta["notices"])
	}
}

const testChangelog = `# Changelog

## [Unreleased]

## [1.2.0] - 2026-06-20

### Added
- a feature

## [1.1.0] - 2026-06-10

### Security
- patched an auth bypass

## [1.0.1] - 2026-06-01

### Fixed
- a small bug
`

// TestSeverity_SecurityEntryInDelta proves a delta containing a Security entry
// grades as "warning".
func TestSeverity_SecurityEntryInDelta(t *testing.T) {
	// running 1.0.1 -> latest 1.2.0 includes the 1.1.0 Security block.
	if got := severityFromDelta(testChangelog, "1.0.1", "1.2.0"); got != "warning" {
		t.Fatalf("security entry in delta should be warning, got %q", got)
	}
}

// TestSeverity_MajorBump proves a major version bump grades as "warning" even
// with no security entry in the delta.
func TestSeverity_MajorBump(t *testing.T) {
	noSecurity := `## [2.0.0] - 2026-06-20

### Changed
- breaking change
`
	if got := severityFromDelta(noSecurity, "1.9.9", "2.0.0"); got != "warning" {
		t.Fatalf("major bump should be warning, got %q", got)
	}
}

// TestSeverity_PlainPatch proves a routine patch with no security entry grades
// as "info".
func TestSeverity_PlainPatch(t *testing.T) {
	// running 1.1.0 -> latest 1.2.0 skips the 1.1.0 Security block (not in delta).
	if got := severityFromDelta(testChangelog, "1.1.0", "1.2.0"); got != "info" {
		t.Fatalf("plain minor/patch should be info, got %q", got)
	}
}
