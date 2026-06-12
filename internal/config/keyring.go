package config

import "github.com/zalando/go-keyring"

// Keyring three-part pattern (SEC-SPEC §4): app secrets live in the OS
// keyring (Windows Credential Manager / macOS Keychain / Linux Secret
// Service), keyed per app_id; config.json keeps zero secrets, only the
// storage marker. Machine-bound AES file encryption remains as the fallback
// for environments without a keyring service.
const keyringService = "wechat-mp-cli"

// Seams: tests override these to avoid touching the real OS keyring.
var (
	keyringSet    = keyring.Set
	keyringGet    = keyring.Get
	keyringDelete = keyring.Delete
)

const (
	storageKeyring       = "keyring"
	storageEncryptedFile = "encrypted-file"
)

func keyringAccountKey(appID string) string {
	return "app-secret:" + appID
}
