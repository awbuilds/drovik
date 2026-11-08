package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // RFC 6238 TOTP mandates HMAC-SHA1 interoperability
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	totpDigits = 6
	totpPeriod = 30 * time.Second
	totpWindow = 1
)

var base32NoPadding = base32.StdEncoding.WithPadding(base32.NoPadding)

// GenerateTOTPSecret returns a 160-bit base32 secret suitable for RFC 6238.
func GenerateTOTPSecret() (string, error) {
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate TOTP secret: %w", err)
	}
	return base32NoPadding.EncodeToString(raw), nil
}

func TOTPURI(issuer, account, secret string) string {
	query := url.Values{}
	query.Set("secret", secret)
	query.Set("issuer", issuer)
	query.Set("algorithm", "SHA1")
	query.Set("digits", "6")
	query.Set("period", "30")
	label := url.PathEscape(issuer + ":" + account)
	return "otpauth://totp/" + label + "?" + query.Encode()
}

// ValidateTOTP accepts the current step and one step of clock skew in either
// direction.
func ValidateTOTP(secret, code string, now time.Time) bool {
	code = strings.TrimSpace(code)
	if len(code) != totpDigits {
		return false
	}
	key, err := base32NoPadding.DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil || len(key) == 0 {
		return false
	}
	counter := now.Unix() / int64(totpPeriod/time.Second)
	for offset := -totpWindow; offset <= totpWindow; offset++ {
		if subtle.ConstantTimeCompare([]byte(TOTPCode(key, counter+int64(offset))), []byte(code)) == 1 {
			return true
		}
	}
	return false
}

func TOTPCode(key []byte, counter int64) string {
	var buffer [8]byte
	// Counter is a non-negative Unix time step.
	binary.BigEndian.PutUint64(buffer[:], uint64(counter)) //nolint:gosec // non-negative time step
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(buffer[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	binaryCode := (uint32(sum[offset])&0x7f)<<24 |
		uint32(sum[offset+1])<<16 |
		uint32(sum[offset+2])<<8 |
		uint32(sum[offset+3])
	return fmt.Sprintf("%06d", binaryCode%1_000_000)
}

func GenerateRecoveryCodes(count int) ([]string, []string, error) {
	plain := make([]string, 0, count)
	hashes := make([]string, 0, count)
	for index := 0; index < count; index++ {
		raw := make([]byte, 6)
		if _, err := rand.Read(raw); err != nil {
			return nil, nil, fmt.Errorf("generate recovery code: %w", err)
		}
		code := strings.ToUpper(hex.EncodeToString(raw))
		formatted := code[0:4] + "-" + code[4:8] + "-" + code[8:12]
		plain = append(plain, formatted)
		hashes = append(hashes, HashRecoveryCode(formatted))
	}
	return plain, hashes, nil
}

func HashRecoveryCode(code string) string {
	normalized := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(code), "-", ""))
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}
