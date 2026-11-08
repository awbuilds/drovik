package httpapi

import (
	"bytes"
	"encoding/base32"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"testing"
	"time"

	"github.com/awbuilds/drovik/backend/internal/auth"
)

func TestMFASetupRequiresStepUp(t *testing.T) {
	app := newTestApp(t)
	setup := app.request(t, http.MethodPost, "/api/v1/setup", setupRequest{
		Username: "admin", Password: "correct horse battery staple",
	}, "")
	if setup.status != http.StatusCreated {
		t.Fatalf("setup: %d %s", setup.status, setup.body)
	}
	csrf := decodeAuth(t, setup.body).CSRFToken
	enrollment := app.request(t, http.MethodPost, "/api/v1/auth/mfa/setup", mfaSetupRequest{
		Password: "correct horse battery staple",
	}, csrf)
	if enrollment.status != http.StatusOK {
		t.Fatalf("MFA setup: %d %s", enrollment.status, enrollment.body)
	}
	var enrollmentBody mfaSetupResponse
	if err := json.Unmarshal(enrollment.body, &enrollmentBody); err != nil {
		t.Fatalf("decode setup: %v", err)
	}
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(enrollmentBody.Secret)
	if err != nil {
		t.Fatalf("decode secret: %v", err)
	}
	enabled := app.request(t, http.MethodPost, "/api/v1/auth/mfa/enable", mfaEnableRequest{
		Code: auth.TOTPCode(key, time.Now().Unix()/30),
	}, csrf)
	if enabled.status != http.StatusOK {
		t.Fatalf("MFA enable: %d %s", enabled.status, enabled.body)
	}

	missing := app.request(t, http.MethodPost, "/api/v1/auth/mfa/setup", mfaSetupRequest{}, csrf)
	if missing.status != http.StatusUnauthorized {
		t.Fatalf("expected missing-password rejection, got %d %s", missing.status, missing.body)
	}
	noCode := app.request(t, http.MethodPost, "/api/v1/auth/mfa/setup", mfaSetupRequest{
		Password: "correct horse battery staple",
	}, csrf)
	if noCode.status != http.StatusUnauthorized {
		t.Fatalf("expected current-factor rejection, got %d %s", noCode.status, noCode.body)
	}
	status := app.request(t, http.MethodGet, "/api/v1/auth/mfa", nil, "")
	if !bytes.Contains(status.body, []byte(`"enabled":true`)) {
		t.Fatalf("a rejected re-enrollment must keep MFA enabled: %s", status.body)
	}
	reset := app.request(t, http.MethodPost, "/api/v1/auth/mfa/setup", mfaSetupRequest{
		Password: "correct horse battery staple",
		Code:     auth.TOTPCode(key, time.Now().Unix()/30),
	}, csrf)
	if reset.status != http.StatusOK {
		t.Fatalf("step-up re-enrollment: %d %s", reset.status, reset.body)
	}
}

func TestMFAEnrollmentAndLogin(t *testing.T) {
	app := newTestApp(t)
	setup := app.request(t, http.MethodPost, "/api/v1/setup", setupRequest{
		Username: "admin", Password: "correct horse battery staple",
	}, "")
	if setup.status != http.StatusCreated {
		t.Fatalf("setup: %d %s", setup.status, setup.body)
	}
	csrf := decodeAuth(t, setup.body).CSRFToken

	status := app.request(t, http.MethodGet, "/api/v1/auth/mfa", nil, "")
	if status.status != http.StatusOK || !bytes.Contains(status.body, []byte(`"enabled":false`)) {
		t.Fatalf("initial MFA status: %d %s", status.status, status.body)
	}
	noPassword := app.request(t, http.MethodPost, "/api/v1/auth/mfa/setup", mfaSetupRequest{}, csrf)
	if noPassword.status != http.StatusUnauthorized {
		t.Fatalf("expected MFA setup password requirement, got %d %s", noPassword.status, noPassword.body)
	}
	setupResponse := app.request(t, http.MethodPost, "/api/v1/auth/mfa/setup", mfaSetupRequest{
		Password: "correct horse battery staple",
	}, csrf)
	if setupResponse.status != http.StatusOK {
		t.Fatalf("MFA setup: %d %s", setupResponse.status, setupResponse.body)
	}
	var enrollment mfaSetupResponse
	if err := json.Unmarshal(setupResponse.body, &enrollment); err != nil {
		t.Fatalf("decode setup: %v", err)
	}
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(enrollment.Secret)
	if err != nil {
		t.Fatalf("decode secret: %v", err)
	}
	code := auth.TOTPCode(key, time.Now().Unix()/30)

	badEnable := app.request(t, http.MethodPost, "/api/v1/auth/mfa/enable", mfaEnableRequest{Code: "000000"}, csrf)
	if badEnable.status != http.StatusUnprocessableEntity {
		t.Fatalf("expected bad code rejection, got %d %s", badEnable.status, badEnable.body)
	}
	enabled := app.request(t, http.MethodPost, "/api/v1/auth/mfa/enable", mfaEnableRequest{Code: code}, csrf)
	if enabled.status != http.StatusOK {
		t.Fatalf("MFA enable: %d %s", enabled.status, enabled.body)
	}
	var enrollmentResult mfaEnableResponse
	if err := json.Unmarshal(enabled.body, &enrollmentResult); err != nil {
		t.Fatalf("decode enable: %v", err)
	}
	if len(enrollmentResult.RecoveryCodes) != 8 {
		t.Fatalf("expected recovery codes: %+v", enrollmentResult)
	}

	logout := app.request(t, http.MethodPost, "/api/v1/auth/logout", nil, csrf)
	if logout.status != http.StatusNoContent {
		t.Fatalf("logout: %d %s", logout.status, logout.body)
	}

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	noCode := requestAs(t, app, client, http.MethodPost, "/api/v1/auth/login", loginRequest{
		Username: "admin", Password: "correct horse battery staple",
	}, "")
	if noCode.status != http.StatusUnauthorized || decodeError(t, noCode.body).Error.Code != CodeAuthMFARequired {
		t.Fatalf("expected MFA challenge, got %d %s", noCode.status, noCode.body)
	}
	wrong := requestAs(t, app, client, http.MethodPost, "/api/v1/auth/login", loginRequest{
		Username: "admin", Password: "correct horse battery staple", TOTP: "000000",
	}, "")
	if wrong.status != http.StatusUnauthorized || decodeError(t, wrong.body).Error.Code != CodeAuthMFAInvalid {
		t.Fatalf("expected invalid MFA code, got %d %s", wrong.status, wrong.body)
	}
	good := requestAs(t, app, client, http.MethodPost, "/api/v1/auth/login", loginRequest{
		Username: "admin", Password: "correct horse battery staple",
		TOTP: auth.TOTPCode(key, time.Now().Unix()/30),
	}, "")
	if good.status != http.StatusOK {
		t.Fatalf("expected MFA login success, got %d %s", good.status, good.body)
	}
	goodCSRF := decodeAuth(t, good.body).CSRFToken
	if logout := requestAs(t, app, client, http.MethodPost, "/api/v1/auth/logout", nil, goodCSRF); logout.status != http.StatusNoContent {
		t.Fatalf("logout after MFA: %d %s", logout.status, logout.body)
	}

	recovery := requestAs(t, app, client, http.MethodPost, "/api/v1/auth/login", loginRequest{
		Username: "admin", Password: "correct horse battery staple", TOTP: enrollmentResult.RecoveryCodes[0],
	}, "")
	if recovery.status != http.StatusOK {
		t.Fatalf("expected recovery-code login, got %d %s", recovery.status, recovery.body)
	}
	recoveryCSRF := decodeAuth(t, recovery.body).CSRFToken
	if reuse := requestAs(t, app, client, http.MethodPost, "/api/v1/auth/login", loginRequest{
		Username: "admin", Password: "correct horse battery staple", TOTP: enrollmentResult.RecoveryCodes[0],
	}, ""); reuse.status != http.StatusUnauthorized {
		t.Fatalf("expected recovery code to be single-use, got %d %s", reuse.status, reuse.body)
	}
	// Disabling requires the password.
	disabled := requestAs(t, app, client, http.MethodPost, "/api/v1/auth/mfa/disable", mfaDisableRequest{
		Password: "correct horse battery staple",
	}, recoveryCSRF)
	if disabled.status != http.StatusNoContent {
		t.Fatalf("disable MFA: %d %s", disabled.status, disabled.body)
	}
}
