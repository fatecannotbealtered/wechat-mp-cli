package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Token cache (CLI-SPEC §15.1): WeChat access tokens live ~2h, so every
// command reuses the cached token until it nears expiry instead of minting a
// new one per invocation. The token value is stored with the same
// machine-bound encryption as app secrets; expiry stays plaintext so status
// checks never need to decrypt.

type CachedToken struct {
	AccessToken string
	ExpiresAt   time.Time
}

type diskTokenCache struct {
	Tokens map[string]diskCachedToken `json:"tokens"`
}

type diskCachedToken struct {
	TokenEnc  string    `json:"token_enc"`
	ExpiresAt time.Time `json:"expires_at"`
}

func tokenCachePath() string {
	return filepath.Join(Dir(), "token-cache.json")
}

// LoadCachedToken returns the cached token for appID, if one is stored and
// decryptable. Callers decide how much expiry margin they need.
func LoadCachedToken(appID string) (*CachedToken, bool) {
	data, err := os.ReadFile(tokenCachePath())
	if err != nil {
		return nil, false
	}
	var disk diskTokenCache
	if err := json.Unmarshal(data, &disk); err != nil {
		return nil, false
	}
	entry, ok := disk.Tokens[appID]
	if !ok || entry.TokenEnc == "" {
		return nil, false
	}
	token, err := decrypt(entry.TokenEnc)
	if err != nil {
		return nil, false
	}
	return &CachedToken{AccessToken: token, ExpiresAt: entry.ExpiresAt}, true
}

// SaveCachedToken stores the token for appID (file mode 0600).
func SaveCachedToken(appID, token string, expiresAt time.Time) error {
	if err := os.MkdirAll(Dir(), 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	disk := diskTokenCache{Tokens: map[string]diskCachedToken{}}
	if data, err := os.ReadFile(tokenCachePath()); err == nil {
		_ = json.Unmarshal(data, &disk)
		if disk.Tokens == nil {
			disk.Tokens = map[string]diskCachedToken{}
		}
	}
	tokenEnc, err := encrypt(token)
	if err != nil {
		return fmt.Errorf("encrypting token: %w", err)
	}
	disk.Tokens[appID] = diskCachedToken{TokenEnc: tokenEnc, ExpiresAt: expiresAt.UTC()}
	data, err := json.MarshalIndent(disk, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding token cache: %w", err)
	}
	if err := os.WriteFile(tokenCachePath(), data, 0o600); err != nil {
		return fmt.Errorf("writing token cache: %w", err)
	}
	return nil
}

// ClearCachedToken drops the cache entry for appID (account removal/logout).
func ClearCachedToken(appID string) error {
	data, err := os.ReadFile(tokenCachePath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var disk diskTokenCache
	if err := json.Unmarshal(data, &disk); err != nil {
		return nil
	}
	if _, ok := disk.Tokens[appID]; !ok {
		return nil
	}
	delete(disk.Tokens, appID)
	out, err := json.MarshalIndent(disk, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(tokenCachePath(), out, 0o600)
}
