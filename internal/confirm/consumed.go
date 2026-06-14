package confirm

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Single-use confirm tokens: once a confirm token has been accepted to drive a
// write, its fingerprint is recorded so the SAME token cannot drive a second
// write. This gives agents safe-retry semantics — a confirmed write that times
// out cannot be blindly replayed; the retry is rejected and the agent must
// re-run --dry-run (which reveals the now-current state). WeChat has no reliable
// upstream resource version to bind against, so single-use IS the safe-retry
// mechanism here. The store lives at ~/.wechat-mp-cli/confirm-consumed.json
// (0600) and is pruned of expired entries on every access.
//
// Degrades gracefully: if the store cannot be read or written (no HOME, disk
// error), the write is never blocked — single-use simply cannot be guaranteed
// on that host.
var consumedMu sync.Mutex

func consumedPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", err
	}
	return filepath.Join(home, ".wechat-mp-cli", "confirm-consumed.json"), nil
}

// Fingerprint is a short, non-reversible id for a confirm token: the first 16
// hex chars of sha256(token). Stable for a given token, decoupled from the
// token's contents so the store never holds the signing material.
func Fingerprint(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])[:16]
}

// tokenExpiry decodes the token's Claims to recover its expiry (unix seconds).
// The body is the part before the '.'; signature is not verified here because
// the caller has already run Verify before marking a token consumed.
func tokenExpiry(token string) (int64, bool) {
	parts := splitToken(token)
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return 0, false
	}
	var claims Claims
	if err := json.Unmarshal(raw, &claims); err != nil {
		return 0, false
	}
	return claims.ExpiresAt, true
}

func loadConsumed(path string, now time.Time) map[string]int64 {
	out := map[string]int64{}
	data, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	var stored map[string]int64
	if json.Unmarshal(data, &stored) != nil {
		return out
	}
	// Drop expired entries so the file cannot grow without bound.
	for fp, exp := range stored {
		if exp > now.Unix() {
			out[fp] = exp
		}
	}
	return out
}

// IsConsumed reports whether this token has already driven a write. A storage
// failure reports false (cannot check → do not block the operation).
func IsConsumed(token string, now time.Time) bool {
	path, err := consumedPath()
	if err != nil {
		return false
	}
	consumedMu.Lock()
	defer consumedMu.Unlock()
	tokens := loadConsumed(path, now)
	_, ok := tokens[Fingerprint(token)]
	return ok
}

// MarkConsumed records the token as used until its own expiry. Best effort: a
// storage failure does not block the write (single-use simply cannot be
// guaranteed on that host). Must be called BEFORE the write executes so a
// crash mid-write still rejects the replay.
func MarkConsumed(token string, now time.Time) {
	path, err := consumedPath()
	if err != nil {
		return
	}
	expiry, ok := tokenExpiry(token)
	if !ok {
		// Fall back to the default TTL window so a malformed-expiry token still
		// gets recorded and pruned eventually rather than living forever.
		expiry = now.Add(defaultTTL).Unix()
	}
	consumedMu.Lock()
	defer consumedMu.Unlock()
	tokens := loadConsumed(path, now)
	tokens[Fingerprint(token)] = expiry
	data, err := json.Marshal(tokens)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}
