package auth

import (
	"strings"
	"testing"
	"time"
)

func TestTOTPGenerateAndValidate(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}
	key, err := base32NoPadding.DecodeString(secret)
	if err != nil {
		t.Fatalf("decode secret: %v", err)
	}
	now := time.Unix(1_700_000_000, 0)
	counter := now.Unix() / int64(totpPeriod/time.Second)
	code := TOTPCode(key, counter)
	if !ValidateTOTP(secret, code, now) {
		t.Fatal("current code must validate")
	}
	if !ValidateTOTP(secret, code, now.Add(totpPeriod)) {
		t.Fatal("one step of skew must validate")
	}
	if ValidateTOTP(secret, code, now.Add(5*totpPeriod)) {
		t.Fatal("stale code must not validate")
	}
	if ValidateTOTP(secret, "000000", now) {
		t.Fatal("wrong code must not validate")
	}
	if ValidateTOTP("not-base32!", code, now) {
		t.Fatal("invalid secret must not validate")
	}
	if uri := TOTPURI("Drovik", "admin", secret); !strings.HasPrefix(uri, "otpauth://totp/") || !strings.Contains(uri, "issuer=Drovik") {
		t.Fatalf("unexpected URI: %s", uri)
	}
}

func TestRecoveryCodesAreHashedAndUnique(t *testing.T) {
	plain, hashes, err := GenerateRecoveryCodes(8)
	if err != nil {
		t.Fatalf("GenerateRecoveryCodes: %v", err)
	}
	if len(plain) != 8 || len(hashes) != 8 {
		t.Fatalf("unexpected code counts: %d %d", len(plain), len(hashes))
	}
	seen := make(map[string]struct{}, len(plain))
	for index, code := range plain {
		if _, ok := seen[code]; ok {
			t.Fatalf("duplicate recovery code: %s", code)
		}
		seen[code] = struct{}{}
		if hashes[index] != HashRecoveryCode(code) {
			t.Fatalf("hash mismatch for %s", code)
		}
		if HashRecoveryCode(code) == code {
			t.Fatal("recovery codes must not be stored in plaintext")
		}
	}
}
