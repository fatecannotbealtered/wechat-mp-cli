package confirm

import (
	"testing"
	"time"
)

// isolateHome points the consumed-token store (and the machine secret) at a
// throwaway HOME so the real ~/.wechat-mp-cli is never touched.
func isolateHome(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)        // unix
	t.Setenv("USERPROFILE", dir) // windows
}

func TestSingleUseConfirmToken(t *testing.T) {
	isolateHome(t)
	payload := map[string]any{"media_id": "draft-1"}
	token, _, err := New("publish.submit", payload)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	now := time.Now()

	// First use: not yet consumed.
	if IsConsumed(token, now) {
		t.Fatal("IsConsumed() = true before any use, want false")
	}
	MarkConsumed(token, now)

	// Replay: the same token is now rejected.
	if !IsConsumed(token, now) {
		t.Fatal("IsConsumed() = false after MarkConsumed, want true")
	}
}

func TestFingerprintStability(t *testing.T) {
	token := "abc.def"
	first := Fingerprint(token)
	if first != Fingerprint(token) {
		t.Fatal("Fingerprint() is not stable for the same token")
	}
	if len(first) != 16 {
		t.Fatalf("Fingerprint() length = %d, want 16", len(first))
	}
	if Fingerprint("other.token") == first {
		t.Fatal("Fingerprint() collided for different tokens")
	}
}

func TestConsumedPruneExpired(t *testing.T) {
	isolateHome(t)
	payload := map[string]any{"alias": "prod"}
	token, expires, err := New("setup.account.remove", payload)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	MarkConsumed(token, time.Now())

	// Once the token's own expiry has passed, the pruning load drops it and the
	// fingerprint no longer reports as consumed.
	after := expires.Add(time.Second)
	if IsConsumed(token, after) {
		t.Fatal("IsConsumed() = true past token expiry, want false (entry should be pruned)")
	}
}
