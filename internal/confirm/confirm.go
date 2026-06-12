package confirm

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const defaultTTL = 10 * time.Minute

var (
	ErrMalformed = errors.New("malformed confirm token")
	ErrExpired   = errors.New("confirm token expired")
	ErrMismatch  = errors.New("confirm token does not match this operation")

	secretOnce  sync.Once
	secretValue []byte
)

type Claims struct {
	Operation   string `json:"operation"`
	PayloadHash string `json:"payload_hash"`
	ExpiresAt   int64  `json:"expires_at"`
}

func New(operation string, payload any) (string, time.Time, error) {
	return NewWithTTL(operation, payload, defaultTTL)
}

func NewWithTTL(operation string, payload any, ttl time.Duration) (string, time.Time, error) {
	expires := time.Now().Add(ttl).UTC()
	claims := Claims{
		Operation:   operation,
		PayloadHash: payloadHash(payload),
		ExpiresAt:   expires.Unix(),
	}
	body, err := json.Marshal(claims)
	if err != nil {
		return "", time.Time{}, err
	}
	bodyEncoded := base64.RawURLEncoding.EncodeToString(body)
	signature := sign([]byte(bodyEncoded))
	return bodyEncoded + "." + signature, expires, nil
}

func Verify(token string, operation string, payload any) error {
	parts := splitToken(token)
	if len(parts) != 2 {
		return ErrMalformed
	}
	expectedSig := sign([]byte(parts[0]))
	if subtle.ConstantTimeCompare([]byte(expectedSig), []byte(parts[1])) != 1 {
		return ErrMismatch
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ErrMalformed
	}
	var claims Claims
	if err := json.Unmarshal(raw, &claims); err != nil {
		return ErrMalformed
	}
	if time.Now().Unix() > claims.ExpiresAt {
		return ErrExpired
	}
	if claims.Operation != operation || claims.PayloadHash != payloadHash(payload) {
		return ErrMismatch
	}
	return nil
}

func payloadHash(payload any) string {
	data, err := json.Marshal(payload)
	if err != nil {
		data = []byte(fmt.Sprintf("%#v", payload))
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func splitToken(token string) []string {
	for i := 0; i < len(token); i++ {
		if token[i] == '.' {
			return []string{token[:i], token[i+1:]}
		}
	}
	return []string{token}
}

func sign(data []byte) string {
	secret := machineSecret()
	if len(secret) == 0 {
		sum := sha256.Sum256(data)
		return hex.EncodeToString(sum[:])
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}

func machineSecret() []byte {
	secretOnce.Do(func() {
		secretValue = loadOrCreateSecret()
	})
	return secretValue
}

func loadOrCreateSecret() []byte {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil
	}
	path := filepath.Join(home, ".wechat-mp-cli", "confirm.secret")
	if data, err := os.ReadFile(path); err == nil && len(data) >= 32 {
		return data
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil
	}
	if err := os.WriteFile(path, secret, 0o600); err != nil {
		return nil
	}
	return secret
}
