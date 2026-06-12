package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const DefaultAPIBase = "https://api.weixin.qq.com"

type Config struct {
	DefaultAccount string    `json:"default_account,omitempty"`
	APIBase        string    `json:"api_base,omitempty"`
	APIProxy       string    `json:"api_proxy,omitempty"`
	Accounts       []Account `json:"accounts,omitempty"`
}

type Account struct {
	Alias     string `json:"alias"`
	AppID     string `json:"app_id"`
	AppSecret string `json:"app_secret,omitempty"`
	Source    string `json:"source,omitempty"`
	// Storage reports the at-rest backend that served the secret:
	// "keyring", "encrypted-file", or "" (env-provided).
	Storage string `json:"-"`
}

type diskConfig struct {
	DefaultAccount string        `json:"default_account,omitempty"`
	APIBase        string        `json:"api_base,omitempty"`
	APIProxy       string        `json:"api_proxy,omitempty"`
	Accounts       []diskAccount `json:"accounts,omitempty"`
}

type diskAccount struct {
	Alias         string `json:"alias"`
	AppID         string `json:"app_id"`
	AppSecret     string `json:"app_secret,omitempty"`
	AppSecretEnc  string `json:"app_secret_enc,omitempty"`
	SecretStorage string `json:"secret_storage,omitempty"`
}

type PublicAccount struct {
	Alias            string `json:"alias"`
	AppID            string `json:"app_id"`
	Default          bool   `json:"default"`
	Source           string `json:"source"`
	HasAppSecret     bool   `json:"has_app_secret"`
	SecretStorage    string `json:"secret_storage,omitempty"`
	AppSecretPreview string `json:"app_secret_preview,omitempty"`
}

func Dir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".wechat-mp-cli"
	}
	return filepath.Join(home, ".wechat-mp-cli")
}

func FilePath() string {
	return filepath.Join(Dir(), "config.json")
}

func Load() (*Config, error) {
	cfg := &Config{APIBase: DefaultAPIBase}
	data, err := os.ReadFile(FilePath())
	if err == nil {
		var disk diskConfig
		if err := json.Unmarshal(data, &disk); err != nil {
			return nil, fmt.Errorf("parsing config %s: %w", FilePath(), err)
		}
		cfg.DefaultAccount = strings.TrimSpace(disk.DefaultAccount)
		if strings.TrimSpace(disk.APIBase) != "" {
			cfg.APIBase = strings.TrimRight(strings.TrimSpace(disk.APIBase), "/")
		}
		cfg.APIProxy = strings.TrimSpace(disk.APIProxy)
		for _, a := range disk.Accounts {
			secret := a.AppSecret
			storage := ""
			switch {
			case a.SecretStorage == storageKeyring:
				secret, err = keyringGet(keyringService, keyringAccountKey(strings.TrimSpace(a.AppID)))
				if err != nil {
					return nil, fmt.Errorf("reading app secret for %s from OS keyring (re-run 'wechat-mp-cli setup account add'): %w", a.Alias, err)
				}
				storage = storageKeyring
			case a.AppSecretEnc != "":
				secret, err = decrypt(a.AppSecretEnc)
				if err != nil {
					return nil, fmt.Errorf("decrypting app secret for %s: %w", a.Alias, err)
				}
				storage = storageEncryptedFile
			}
			cfg.Accounts = append(cfg.Accounts, Account{
				Alias:     strings.TrimSpace(a.Alias),
				AppID:     strings.TrimSpace(a.AppID),
				AppSecret: secret,
				Source:    "config",
				Storage:   storage,
			})
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	applyEnv(cfg)
	if cfg.APIBase == "" {
		cfg.APIBase = DefaultAPIBase
	}
	return cfg, nil
}

func Save(cfg *Config) error {
	if err := os.MkdirAll(Dir(), 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	disk := diskConfig{
		DefaultAccount: strings.TrimSpace(cfg.DefaultAccount),
		APIBase:        strings.TrimRight(strings.TrimSpace(cfg.APIBase), "/"),
		APIProxy:       strings.TrimSpace(cfg.APIProxy),
	}
	if disk.APIBase == "" {
		disk.APIBase = DefaultAPIBase
	}
	for i := range cfg.Accounts {
		a := &cfg.Accounts[i]
		appID := strings.TrimSpace(a.AppID)
		// Keyring three-part pattern (SEC-SPEC §4): secret to the OS keyring,
		// zero-secret config. Machine-bound file encryption only as the
		// visible fallback when no keyring service is available.
		if err := keyringSet(keyringService, keyringAccountKey(appID), a.AppSecret); err == nil {
			a.Storage = storageKeyring
			disk.Accounts = append(disk.Accounts, diskAccount{
				Alias:         strings.TrimSpace(a.Alias),
				AppID:         appID,
				SecretStorage: storageKeyring,
			})
			continue
		}
		secretEnc, err := encrypt(a.AppSecret)
		if err != nil {
			return fmt.Errorf("encrypting app secret for %s: %w", a.Alias, err)
		}
		a.Storage = storageEncryptedFile
		disk.Accounts = append(disk.Accounts, diskAccount{
			Alias:         strings.TrimSpace(a.Alias),
			AppID:         appID,
			AppSecretEnc:  secretEnc,
			SecretStorage: storageEncryptedFile,
		})
	}
	data, err := json.MarshalIndent(disk, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	if err := os.WriteFile(FilePath(), data, 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

func AddOrUpdateAccount(cfg *Config, account Account, makeDefault bool) error {
	account.Alias = strings.TrimSpace(account.Alias)
	account.AppID = strings.TrimSpace(account.AppID)
	account.AppSecret = strings.TrimSpace(account.AppSecret)
	if account.Alias == "" {
		return errors.New("account alias is required")
	}
	if account.AppID == "" {
		return errors.New("app_id is required")
	}
	if account.AppSecret == "" {
		return errors.New("app_secret is required")
	}
	for i, existing := range cfg.Accounts {
		if existing.Alias == account.Alias {
			cfg.Accounts[i] = account
			if makeDefault || cfg.DefaultAccount == "" {
				cfg.DefaultAccount = account.Alias
			}
			return nil
		}
	}
	cfg.Accounts = append(cfg.Accounts, account)
	if makeDefault || cfg.DefaultAccount == "" {
		cfg.DefaultAccount = account.Alias
	}
	return nil
}

func RemoveAccount(cfg *Config, alias string) bool {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return false
	}
	next := cfg.Accounts[:0]
	removed := false
	for _, a := range cfg.Accounts {
		if a.Alias == alias {
			removed = true
			// Best-effort cleanup of the keyring secret and cached token.
			_ = keyringDelete(keyringService, keyringAccountKey(strings.TrimSpace(a.AppID)))
			_ = ClearCachedToken(strings.TrimSpace(a.AppID))
			continue
		}
		next = append(next, a)
	}
	cfg.Accounts = next
	if cfg.DefaultAccount == alias {
		cfg.DefaultAccount = ""
		if len(cfg.Accounts) > 0 {
			cfg.DefaultAccount = cfg.Accounts[0].Alias
		}
	}
	return removed
}

func SetDefault(cfg *Config, alias string) error {
	if _, ok := FindAccount(cfg, alias); !ok {
		return fmt.Errorf("account %q not found", alias)
	}
	cfg.DefaultAccount = alias
	return nil
}

func ResolveAccount(cfg *Config, alias string) (Account, error) {
	if strings.TrimSpace(alias) == "" {
		alias = cfg.DefaultAccount
	}
	if strings.TrimSpace(alias) == "" && len(cfg.Accounts) == 1 {
		return cfg.Accounts[0], nil
	}
	account, ok := FindAccount(cfg, alias)
	if !ok {
		return Account{}, fmt.Errorf("account %q not configured", alias)
	}
	if account.AppID == "" || account.AppSecret == "" {
		return Account{}, fmt.Errorf("account %q is missing app_id or app_secret", alias)
	}
	return account, nil
}

func FindAccount(cfg *Config, alias string) (Account, bool) {
	alias = strings.TrimSpace(alias)
	for _, a := range cfg.Accounts {
		if a.Alias == alias {
			return a, true
		}
	}
	return Account{}, false
}

func PublicAccounts(cfg *Config) []PublicAccount {
	out := make([]PublicAccount, 0, len(cfg.Accounts))
	for _, a := range cfg.Accounts {
		out = append(out, PublicAccount{
			Alias:            a.Alias,
			AppID:            a.AppID,
			Default:          a.Alias == cfg.DefaultAccount,
			Source:           a.Source,
			HasAppSecret:     a.AppSecret != "",
			SecretStorage:    a.Storage,
			AppSecretPreview: previewSecret(a.AppSecret),
		})
	}
	return out
}

func applyEnv(cfg *Config) {
	if base := strings.TrimSpace(os.Getenv("WECHAT_MP_CLI_API_BASE")); base != "" {
		cfg.APIBase = strings.TrimRight(base, "/")
	}
	if proxy := strings.TrimSpace(os.Getenv("WECHAT_MP_CLI_API_PROXY")); proxy != "" {
		cfg.APIProxy = proxy
	}
	appID := firstEnv("WECHAT_MP_CLI_APP_ID", "WECHAT_APP_ID", "WECHAT_APPID")
	appSecret := firstEnv("WECHAT_MP_CLI_APP_SECRET", "WECHAT_APP_SECRET", "WECHAT_APPSECRET")
	if appID == "" && appSecret == "" {
		return
	}
	alias := firstEnv("WECHAT_MP_CLI_ACCOUNT", "WECHAT_ACCOUNT")
	if alias == "" {
		alias = "env"
	}
	account := Account{
		Alias:     alias,
		AppID:     appID,
		AppSecret: appSecret,
		Source:    "env",
	}
	replaced := false
	for i, existing := range cfg.Accounts {
		if existing.Alias == alias {
			cfg.Accounts[i] = account
			replaced = true
			break
		}
	}
	if !replaced {
		cfg.Accounts = append(cfg.Accounts, account)
	}
	if cfg.DefaultAccount == "" || firstEnv("WECHAT_MP_CLI_ACCOUNT", "WECHAT_ACCOUNT") != "" {
		cfg.DefaultAccount = alias
	}
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

func previewSecret(secret string) string {
	if secret == "" {
		return ""
	}
	if len(secret) <= 8 {
		return "***"
	}
	return secret[:4] + "..." + secret[len(secret)-4:]
}

func machineKey() ([]byte, error) {
	home, _ := os.UserHomeDir()
	host, _ := os.Hostname()
	sum := sha256.Sum256([]byte("wechat-mp-cli-config-v1|" + host + "|" + home))
	return sum[:], nil
}

func encrypt(value string) (string, error) {
	key, err := machineKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(value), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func decrypt(encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	key, err := machineKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", errors.New("encrypted value is too short")
	}
	plain, err := gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
