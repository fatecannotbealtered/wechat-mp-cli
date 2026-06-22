package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Update-notice cache (CLI-SPEC §14): the active-check commands (`update
// --check`, `doctor`) refresh this local cache; every other command may read it
// (and only read it) to piggyback the cached notice onto meta.notices. The
// notice carries no secret, so it is stored as plain JSON. Reads are
// TTL-bounded: an entry older than updateNoticeTTL is treated as absent so a
// stale notice never lingers forever after the user upgrades or the release is
// withdrawn.

const updateNoticeTTL = 24 * time.Hour

// CachedUpdateNotice is the persisted form of the update-available notice. It
// stores the fully-built notice payload plus the CheckedAt timestamp used for
// TTL expiry. Fields beyond CheckedAt are opaque to the cache layer.
type CachedUpdateNotice struct {
	Notice    map[string]any `json:"notice"`
	CheckedAt time.Time      `json:"checked_at"`
}

func updateNoticeCachePath() string {
	return filepath.Join(Dir(), "update-notice.json")
}

// LoadCachedUpdateNotice returns the cached update notice if one is stored and
// not past its TTL. It is strictly read-only and never performs network I/O.
// A missing file, unparseable contents, or an expired entry all return
// (nil, false).
func LoadCachedUpdateNotice() (map[string]any, bool) {
	data, err := os.ReadFile(updateNoticeCachePath())
	if err != nil {
		return nil, false
	}
	var cached CachedUpdateNotice
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, false
	}
	if len(cached.Notice) == 0 {
		return nil, false
	}
	if cached.CheckedAt.IsZero() || time.Since(cached.CheckedAt) > updateNoticeTTL {
		return nil, false
	}
	return cached.Notice, true
}

// SaveCachedUpdateNotice stores the built notice with the current timestamp
// (file mode 0600). A nil or empty notice clears the cache so a "no update
// available" check does not leave a stale notice behind.
func SaveCachedUpdateNotice(notice map[string]any) error {
	if len(notice) == 0 {
		return ClearCachedUpdateNotice()
	}
	if err := os.MkdirAll(Dir(), 0o700); err != nil {
		return err
	}
	out, err := json.MarshalIndent(CachedUpdateNotice{
		Notice:    notice,
		CheckedAt: time.Now().UTC(),
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(updateNoticeCachePath(), out, 0o600)
}

// ClearCachedUpdateNotice removes the cached notice (no error if absent).
func ClearCachedUpdateNotice() error {
	if err := os.Remove(updateNoticeCachePath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
