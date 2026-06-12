package config

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestLoadAppliesEnvAccount(t *testing.T) {
	t.Setenv("WECHAT_MP_CLI_ACCOUNT", "prod")
	t.Setenv("WECHAT_MP_CLI_APP_ID", "wx123")
	t.Setenv("WECHAT_MP_CLI_APP_SECRET", "secret123")
	t.Setenv("WECHAT_MP_CLI_API_BASE", "https://example.test/")
	t.Setenv("WECHAT_MP_CLI_API_PROXY", "socks5://127.0.0.1:1080")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.APIBase != "https://example.test" {
		t.Fatalf("APIBase = %q", cfg.APIBase)
	}
	if cfg.APIProxy != "socks5://127.0.0.1:1080" {
		t.Fatalf("APIProxy = %q", cfg.APIProxy)
	}
	account, err := ResolveAccount(cfg, "")
	if err != nil {
		t.Fatalf("ResolveAccount() error = %v", err)
	}
	if account.Alias != "prod" || account.AppID != "wx123" || account.AppSecret != "secret123" {
		t.Fatalf("account = %#v", account)
	}
}

func TestPublicAccountsRedactsSecret(t *testing.T) {
	cfg := &Config{
		DefaultAccount: "prod",
		Accounts: []Account{{
			Alias:     "prod",
			AppID:     "wx123",
			AppSecret: "abcdefghijklmnop",
			Source:    "config",
		}},
	}
	got := PublicAccounts(cfg)
	if len(got) != 1 {
		t.Fatalf("len(PublicAccounts) = %d", len(got))
	}
	if got[0].AppSecretPreview == "abcdefghijklmnop" || got[0].AppSecretPreview == "" {
		t.Fatalf("secret preview was not redacted: %q", got[0].AppSecretPreview)
	}
}

func TestLoadKeepsPartialEnvAccountForDoctor(t *testing.T) {
	t.Setenv("WECHAT_MP_CLI_ACCOUNT", "partial")
	t.Setenv("WECHAT_MP_CLI_APP_ID", "wx123")
	t.Setenv("WECHAT_MP_CLI_APP_SECRET", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	account, ok := FindAccount(cfg, "partial")
	if !ok {
		t.Fatal("partial env account missing")
	}
	if account.AppID != "wx123" || account.AppSecret != "" {
		t.Fatalf("account = %#v", account)
	}
}

func TestMain(m *testing.M) {
	// Tests must never touch the real OS keyring.
	keyring.MockInit()
	dir, err := os.MkdirTemp("", "wechat-mp-cli-config-test-*")
	if err != nil {
		panic(err)
	}
	oldHome := os.Getenv("HOME")
	oldUserProfile := os.Getenv("USERPROFILE")
	os.Setenv("HOME", dir)
	os.Setenv("USERPROFILE", dir)
	code := m.Run()
	os.Setenv("HOME", oldHome)
	os.Setenv("USERPROFILE", oldUserProfile)
	os.Exit(code)
}

// TestSaveUsesKeyringAndKeepsConfigSecretFree covers the keyring three-part
// pattern (SEC-SPEC §4): secret in the keyring, zero-secret config file with
// a visible storage marker, and Load round-tripping through the keyring.
func TestSaveUsesKeyringAndKeepsConfigSecretFree(t *testing.T) {
	resetConfigDirForKeyringTest(t)
	cfg := &Config{Accounts: []Account{{Alias: "main", AppID: "wx-keyring", AppSecret: "kr-s3cret"}}}
	if err := Save(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	raw, err := os.ReadFile(FilePath())
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if strings.Contains(string(raw), "kr-s3cret") {
		t.Fatalf("config file must not contain the secret: %s", raw)
	}
	if !strings.Contains(string(raw), `"secret_storage": "keyring"`) {
		t.Fatalf("config should declare the keyring backend: %s", raw)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Accounts) != 1 || loaded.Accounts[0].AppSecret != "kr-s3cret" || loaded.Accounts[0].Storage != "keyring" {
		t.Fatalf("loaded = %+v", loaded.Accounts)
	}
}

// TestSaveFallsBackToEncryptedFile covers the degradation path: no keyring
// service available -> machine-bound encrypted file, marker visible.
func TestSaveFallsBackToEncryptedFile(t *testing.T) {
	resetConfigDirForKeyringTest(t)
	origSet := keyringSet
	keyringSet = func(string, string, string) error { return errors.New("no keyring service") }
	defer func() { keyringSet = origSet }()

	cfg := &Config{Accounts: []Account{{Alias: "main", AppID: "wx-fb", AppSecret: "fb-secret"}}}
	if err := Save(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	raw, _ := os.ReadFile(FilePath())
	if strings.Contains(string(raw), "fb-secret") {
		t.Fatalf("config file must not contain the secret: %s", raw)
	}
	if !strings.Contains(string(raw), `"secret_storage": "encrypted-file"`) {
		t.Fatalf("config should declare the fallback backend: %s", raw)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Accounts[0].AppSecret != "fb-secret" || loaded.Accounts[0].Storage != "encrypted-file" {
		t.Fatalf("loaded = %+v", loaded.Accounts)
	}
}

// TestRemoveAccountClearsKeyring: removing an account must drop its secret.
func TestRemoveAccountClearsKeyring(t *testing.T) {
	resetConfigDirForKeyringTest(t)
	cfg := &Config{Accounts: []Account{{Alias: "gone", AppID: "wx-gone", AppSecret: "bye"}}}
	if err := Save(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	if !RemoveAccount(cfg, "gone") {
		t.Fatal("expected removal")
	}
	if _, err := keyringGet(keyringService, keyringAccountKey("wx-gone")); err == nil {
		t.Fatal("keyring secret should be gone after RemoveAccount")
	}
}

func resetConfigDirForKeyringTest(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}
