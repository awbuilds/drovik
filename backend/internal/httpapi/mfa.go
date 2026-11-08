package httpapi

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/awbuilds/drovik/backend/internal/auth"
	"github.com/awbuilds/drovik/backend/internal/secrets"
)

type mfaStatusResponse struct {
	Enabled bool `json:"enabled"`
}

type mfaSetupResponse struct {
	Secret string `json:"secret"`
	URI    string `json:"uri"`
}

type mfaEnableRequest struct {
	Code string `json:"code"`
}

type mfaEnableResponse struct {
	RecoveryCodes []string `json:"recovery_codes"`
}

type mfaDisableRequest struct {
	Password string `json:"password"`
}

func (s *Server) handleMFAStatus(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	_, enabled, err := s.mfa.Get(r.Context(), user.ID)
	if err != nil {
		s.logger.Error("MFA status failed", "error", err, "request_id", RequestID(r.Context()))
		writeError(w, r, NewAPIError(http.StatusInternalServerError, CodeInternal, "An internal error occurred."))
		return
	}
	writeJSON(w, http.StatusOK, mfaStatusResponse{Enabled: enabled})
}

func (s *Server) handleMFASetup(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		s.logger.Error("MFA secret generation failed", "error", err, "request_id", RequestID(r.Context()))
		writeError(w, r, NewAPIError(http.StatusInternalServerError, CodeInternal, "An internal error occurred."))
		return
	}
	encrypted, err := s.encryptMFASecret(user.ID, secret)
	if err != nil {
		s.logger.Error("MFA secret encryption failed", "error", err, "request_id", RequestID(r.Context()))
		writeError(w, r, NewAPIError(http.StatusInternalServerError, CodeInternal, "An internal error occurred."))
		return
	}
	if err := s.mfa.SetSecret(r.Context(), user.ID, encrypted); err != nil {
		s.logger.Error("MFA secret store failed", "error", err, "request_id", RequestID(r.Context()))
		writeError(w, r, NewAPIError(http.StatusInternalServerError, CodeInternal, "An internal error occurred."))
		return
	}
	writeJSON(w, http.StatusOK, mfaSetupResponse{
		Secret: secret,
		URI:    auth.TOTPURI("Drovik", user.Username, secret),
	})
}

func (s *Server) handleMFAEnable(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	var request mfaEnableRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	encrypted, enabled, err := s.mfa.Get(r.Context(), user.ID)
	if err != nil || enabled || encrypted == "" {
		writeError(w, r, NewAPIError(http.StatusConflict, CodeConflict, "Start MFA setup before enabling it."))
		return
	}
	secret, err := s.decryptMFASecret(user.ID, encrypted)
	if err != nil {
		s.logger.Error("MFA secret decrypt failed", "error", err, "request_id", RequestID(r.Context()))
		writeError(w, r, NewAPIError(http.StatusInternalServerError, CodeInternal, "An internal error occurred."))
		return
	}
	if !auth.ValidateTOTP(secret, request.Code, time.Now()) {
		writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeValidation, "The authenticator code is invalid."))
		return
	}
	if err := s.mfa.SetEnabled(r.Context(), user.ID, true); err != nil {
		s.logger.Error("MFA enable failed", "error", err, "request_id", RequestID(r.Context()))
		writeError(w, r, NewAPIError(http.StatusInternalServerError, CodeInternal, "An internal error occurred."))
		return
	}
	plain, hashes, err := auth.GenerateRecoveryCodes(8)
	if err != nil {
		s.logger.Error("MFA recovery code generation failed", "error", err, "request_id", RequestID(r.Context()))
		writeError(w, r, NewAPIError(http.StatusInternalServerError, CodeInternal, "An internal error occurred."))
		return
	}
	if err := s.mfa.CreateRecoveryCodes(r.Context(), user.ID, hashes); err != nil {
		s.logger.Error("MFA recovery code store failed", "error", err, "request_id", RequestID(r.Context()))
		writeError(w, r, NewAPIError(http.StatusInternalServerError, CodeInternal, "An internal error occurred."))
		return
	}
	s.record(r, user.ID, "auth.mfa_enabled", user.Username, "success", "")
	writeJSON(w, http.StatusOK, mfaEnableResponse{RecoveryCodes: plain})
}

func (s *Server) handleMFADisable(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	var request mfaDisableRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	valid, err := auth.VerifyPassword(request.Password, user.PasswordHash)
	if err != nil || !valid {
		writeError(w, r, NewAPIError(http.StatusUnauthorized, CodeAuthInvalidCredentials, "The password is invalid."))
		return
	}
	if err := s.mfa.Clear(r.Context(), user.ID); err != nil {
		s.logger.Error("MFA disable failed", "error", err, "request_id", RequestID(r.Context()))
		writeError(w, r, NewAPIError(http.StatusInternalServerError, CodeInternal, "An internal error occurred."))
		return
	}
	s.record(r, user.ID, "auth.mfa_disabled", user.Username, "success", "")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) verifyMFACode(ctx context.Context, encrypted, userID, code string) bool {
	secret, err := s.decryptMFASecret(userID, encrypted)
	if err != nil {
		s.logger.Error("MFA secret decrypt failed", "error", err, "user_id", userID)
		return false
	}
	if auth.ValidateTOTP(secret, code, time.Now()) {
		return true
	}
	consumed, err := s.mfa.ConsumeRecoveryCode(ctx, userID, auth.HashRecoveryCode(code))
	if err != nil {
		s.logger.Error("MFA recovery code check failed", "error", err, "user_id", userID)
		return false
	}
	return consumed
}

func (s *Server) encryptMFASecret(userID, secret string) (string, error) {
	encrypted, err := s.cipher.Encrypt([]byte(secret), []byte("user:"+userID+":mfa"))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%s:%s", encrypted.Version,
		base64.StdEncoding.EncodeToString(encrypted.Nonce),
		base64.StdEncoding.EncodeToString(encrypted.Ciphertext)), nil
}

func (s *Server) decryptMFASecret(userID, stored string) (string, error) {
	parts := strings.SplitN(stored, ":", 3)
	if len(parts) != 3 {
		return "", errors.New("malformed MFA secret")
	}
	version, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", errors.New("malformed MFA secret version")
	}
	nonce, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", errors.New("malformed MFA secret nonce")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return "", errors.New("malformed MFA secret ciphertext")
	}
	plain, err := s.cipher.Decrypt(secrets.Encrypted{
		Version: version, Nonce: nonce, Ciphertext: ciphertext,
	}, []byte("user:"+userID+":mfa"))
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
