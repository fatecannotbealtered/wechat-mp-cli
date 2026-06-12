package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func isolateHome(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}

func TestTokenCacheRoundTrip(t *testing.T) {
	isolateHome(t)
	expires := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second)
	if err := SaveCachedToken("wx123", "secret-token", expires); err != nil {
		t.Fatalf("save: %v", err)
	}

	cached, ok := LoadCachedToken("wx123")
	if !ok {
		t.Fatal("expected cached token")
	}
	if cached.AccessToken != "secret-token" || !cached.ExpiresAt.Equal(expires) {
		t.Fatalf("cached = %+v", cached)
	}

	// The cache file must never hold the token in plaintext.
	raw, err := os.ReadFile(tokenCachePath())
	if err != nil {
		t.Fatalf("read cache: %v", err)
	}
	if strings.Contains(string(raw), "secret-token") {
		t.Fatalf("token stored in plaintext: %s", raw)
	}
}

func TestTokenCachePerApp(t *testing.T) {
	isolateHome(t)
	expires := time.Now().Add(time.Hour)
	_ = SaveCachedToken("wx-a", "token-a", expires)
	_ = SaveCachedToken("wx-b", "token-b", expires)

	a, _ := LoadCachedToken("wx-a")
	b, _ := LoadCachedToken("wx-b")
	if a.AccessToken != "token-a" || b.AccessToken != "token-b" {
		t.Fatalf("a=%+v b=%+v", a, b)
	}
}

func TestClearCachedToken(t *testing.T) {
	isolateHome(t)
	_ = SaveCachedToken("wx123", "bye", time.Now().Add(time.Hour))
	if err := ClearCachedToken("wx123"); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if _, ok := LoadCachedToken("wx123"); ok {
		t.Fatal("token should be gone")
	}
	// Clearing a missing entry is a no-op, not an error.
	if err := ClearCachedToken("never-existed"); err != nil {
		t.Fatalf("clear missing: %v", err)
	}
}
