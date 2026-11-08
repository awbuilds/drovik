// Package servers implements saved SSH connection management, credential
// encryption and the connection-test / host-key approval workflow.
package servers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/awbuilds/drovik/backend/internal/audit"
	"github.com/awbuilds/drovik/backend/internal/config"
	"github.com/awbuilds/drovik/backend/internal/secrets"
	"github.com/awbuilds/drovik/backend/internal/sshclient"
	"github.com/awbuilds/drovik/backend/internal/storage"
)

const (
	AuthPassword   = "password"
	AuthPrivateKey = "private_key"

	maxDisplayName = 120
	maxHost        = 253
	maxUsername    = 64
	maxPassword    = 1024
	maxPrivateKey  = 64 * 1024
	maxTags        = 200
	maxNotes       = 2000
)

var (
	ErrHostKeyAlreadyApproved = errors.New("a host key is already approved for this server")
	ErrUnauthorized           = errors.New("server does not belong to this user")
)

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

type CreateInput struct {
	DisplayName string
	Host        string
	Port        int
	Username    string
	AuthMethod  string
	Password    string
	PrivateKey  string
	Passphrase  string
	Tags        string
	Notes       string
}

type UpdateInput struct {
	DisplayName *string
	Host        *string
	Port        *int
	Username    *string
	AuthMethod  *string
	Password    *string
	PrivateKey  *string
	Passphrase  *string
	Tags        *string
	Notes       *string
}

type ConnectionResult struct {
	HostKey      sshclient.HostKeyInfo
	Capabilities sshclient.Capabilities
}

type Service struct {
	repo        *storage.ServerRepository
	users       *storage.UserRepository
	assignments *storage.AssignmentRepository
	known       *storage.KnownHostRepository
	caps        *storage.CapabilitiesRepository
	cipher      *secrets.Cipher
	dialer      *sshclient.Dialer
	audit       *audit.Service
	logger      *slog.Logger
	cfg         *config.Config
}

func NewService(
	cfg *config.Config,
	repo *storage.ServerRepository,
	users *storage.UserRepository,
	assignments *storage.AssignmentRepository,
	known *storage.KnownHostRepository,
	caps *storage.CapabilitiesRepository,
	cipher *secrets.Cipher,
	dialer *sshclient.Dialer,
	auditService *audit.Service,
	logger *slog.Logger,
) *Service {
	return &Service{
		repo:        repo,
		users:       users,
		assignments: assignments,
		known:       known,
		caps:        caps,
		cipher:      cipher,
		dialer:      dialer,
		audit:       auditService,
		logger:      logger,
		cfg:         cfg,
	}
}

func (s *Service) Create(ctx context.Context, ownerID string, input CreateInput) (*storage.Server, error) {
	normalized, err := validateCreate(input)
	if err != nil {
		return nil, err
	}

	id := storage.NewID()
	secret, err := s.encryptSecret(id, normalized)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	server := storage.Server{
		ID:              id,
		OwnerUserID:     ownerID,
		DisplayName:     normalized.DisplayName,
		Host:            normalized.Host,
		Port:            normalized.Port,
		Username:        normalized.Username,
		AuthMethod:      normalized.AuthMethod,
		EncryptedSecret: secret.secret,
		SecretNonce:     secret.nonce,
		SecretVersion:   secrets.CurrentVersion,
		Tags:            normalized.Tags,
		Notes:           normalized.Notes,
		CreatedAt:       now,
		UpdatedAt:       now,
		IsActive:        true,
	}
	if secret.passphrase != "" {
		server.EncryptedPassphrase = secret.passphrase
		server.PassphraseNonce = secret.passphraseNonce
		server.PassphraseVersion = secrets.CurrentVersion
	}

	if err := s.repo.Create(ctx, server); err != nil {
		return nil, err
	}
	s.record(ctx, ownerID, &server.ID, "server.created", "success", "")
	return &server, nil
}

func (s *Service) List(ctx context.Context, ownerID string) ([]storage.Server, error) {
	user, err := s.users.GetByID(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	if user.Role == "admin" {
		return s.repo.ListAll(ctx)
	}
	return s.repo.ListAccessible(ctx, ownerID)
}

func (s *Service) Get(ctx context.Context, ownerID, id string) (*storage.Server, error) {
	server, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if server.OwnerUserID == ownerID {
		return server, nil
	}
	user, err := s.users.GetByID(ctx, ownerID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}
	if user.Role == "admin" {
		return server, nil
	}
	allowed, err := s.assignments.HasAccess(ctx, ownerID, id)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrUnauthorized
	}
	return server, nil
}

// AccessLevel returns write for owners/admins and the assignment level for an
// assigned user. Callers must treat any error as a denied authorization.
func (s *Service) AccessLevel(ctx context.Context, userID, serverID string) (string, error) {
	server, err := s.repo.GetByID(ctx, serverID)
	if err != nil {
		return "", err
	}
	if server.OwnerUserID == userID {
		return "write", nil
	}
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return "", ErrUnauthorized
		}
		return "", err
	}
	if user.Role == "admin" {
		return "write", nil
	}
	assignment, err := s.assignments.Get(ctx, userID, serverID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return "", ErrUnauthorized
		}
		return "", err
	}
	return assignment.AccessLevel, nil
}

func (s *Service) ListAssignments(ctx context.Context, actorID, serverID string) ([]storage.ServerAssignment, error) {
	if err := s.requireAdmin(ctx, actorID); err != nil {
		return nil, err
	}
	if _, err := s.Get(ctx, actorID, serverID); err != nil {
		return nil, err
	}
	return s.assignments.ListForServer(ctx, serverID)
}

func (s *Service) SetAssignment(ctx context.Context, actorID, serverID, targetUserID, accessLevel string) error {
	if err := s.requireAdmin(ctx, actorID); err != nil {
		return err
	}
	if accessLevel != "read" && accessLevel != "write" {
		return &ValidationError{Field: "access_level", Message: "must be read or write"}
	}
	if _, err := s.Get(ctx, actorID, serverID); err != nil {
		return err
	}
	if _, err := s.users.GetByID(ctx, targetUserID); err != nil {
		return err
	}
	assignment := storage.ServerAssignment{
		UserID: targetUserID, ServerID: serverID, AccessLevel: accessLevel, CreatedAt: time.Now().UTC(),
	}
	if err := s.assignments.Upsert(ctx, assignment); err != nil {
		return err
	}
	s.record(ctx, actorID, &serverID, "server.assignment_updated", "success", "")
	return nil
}

func (s *Service) RemoveAssignment(ctx context.Context, actorID, serverID, targetUserID string) error {
	if err := s.requireAdmin(ctx, actorID); err != nil {
		return err
	}
	if err := s.assignments.Delete(ctx, targetUserID, serverID); err != nil {
		return err
	}
	s.record(ctx, actorID, &serverID, "server.assignment_removed", "success", "")
	return nil
}

func (s *Service) requireAdmin(ctx context.Context, userID string) error {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.Role != "admin" {
		return ErrUnauthorized
	}
	return nil
}

func (s *Service) Update(ctx context.Context, ownerID, id string, input UpdateInput) (*storage.Server, error) {
	server, err := s.Get(ctx, ownerID, id)
	if err != nil {
		return nil, err
	}

	originalMethod := server.AuthMethod
	methodChanged := false
	hostOrPortChanged := false
	if input.DisplayName != nil {
		server.DisplayName = strings.TrimSpace(*input.DisplayName)
	}
	if input.Host != nil {
		host := normalizeHost(*input.Host)
		if host != server.Host {
			hostOrPortChanged = true
		}
		server.Host = host
	}
	if input.Port != nil {
		if *input.Port != server.Port {
			hostOrPortChanged = true
		}
		server.Port = *input.Port
	}
	if input.Username != nil {
		server.Username = strings.TrimSpace(*input.Username)
	}
	if input.Tags != nil {
		server.Tags = strings.TrimSpace(*input.Tags)
	}
	if input.Notes != nil {
		server.Notes = strings.TrimSpace(*input.Notes)
	}
	if input.AuthMethod != nil {
		server.AuthMethod = *input.AuthMethod
		methodChanged = server.AuthMethod != originalMethod
	}

	credentialChanged := input.Password != nil || input.PrivateKey != nil || input.Passphrase != nil || input.AuthMethod != nil
	if credentialChanged {
		createInput := CreateInput{
			DisplayName: server.DisplayName,
			Host:        server.Host,
			Port:        server.Port,
			Username:    server.Username,
			AuthMethod:  server.AuthMethod,
			Tags:        server.Tags,
			Notes:       server.Notes,
		}
		if methodChanged {
			// A credential is bound to the method it was stored for. Never
			// decrypt an old secret under a newly selected method; require the
			// caller to provide a credential of the new type.
			if input.Password != nil {
				createInput.Password = *input.Password
			}
			if input.PrivateKey != nil {
				createInput.PrivateKey = *input.PrivateKey
			}
			if input.Passphrase != nil {
				createInput.Passphrase = *input.Passphrase
			}
		} else {
			if input.Password != nil {
				createInput.Password = *input.Password
			} else if server.AuthMethod == AuthPassword {
				credentials, err := s.decryptCredentials(server)
				if err != nil {
					return nil, err
				}
				createInput.Password = credentials.Password
			}
			if input.PrivateKey != nil {
				createInput.PrivateKey = *input.PrivateKey
			} else if server.AuthMethod == AuthPrivateKey {
				credentials, err := s.decryptCredentials(server)
				if err != nil {
					return nil, err
				}
				createInput.PrivateKey = string(credentials.PrivateKey)
			}
			if input.Passphrase != nil {
				createInput.Passphrase = *input.Passphrase
			} else {
				credentials, err := s.decryptCredentials(server)
				if err == nil {
					createInput.Passphrase = credentials.Passphrase
				}
			}
		}
		normalized, err := validateCreate(createInput)
		if err != nil {
			return nil, err
		}
		server.DisplayName = normalized.DisplayName
		server.Host = normalized.Host
		server.Port = normalized.Port
		server.Username = normalized.Username
		server.AuthMethod = normalized.AuthMethod
		server.Tags = normalized.Tags
		server.Notes = normalized.Notes

		secret, err := s.encryptSecret(server.ID, normalized)
		if err != nil {
			return nil, err
		}
		server.EncryptedSecret = secret.secret
		server.SecretNonce = secret.nonce
		server.SecretVersion = secrets.CurrentVersion
		server.EncryptedPassphrase = secret.passphrase
		server.PassphraseNonce = secret.passphraseNonce
		server.PassphraseVersion = 0
		if secret.passphrase != "" {
			server.PassphraseVersion = secrets.CurrentVersion
		}
	} else {
		if err := validateMetadata(server); err != nil {
			return nil, err
		}
	}

	server.UpdatedAt = time.Now().UTC()
	if err := s.repo.UpdateByID(ctx, *server); err != nil {
		return nil, err
	}
	if hostOrPortChanged {
		_ = s.known.RevokeActive(ctx, server.ID, server.UpdatedAt)
		_ = s.caps.Delete(ctx, server.ID)
	}
	s.record(ctx, ownerID, &server.ID, "server.updated", "success", "")
	return server, nil
}

func (s *Service) Delete(ctx context.Context, ownerID, id string) error {
	if _, err := s.Get(ctx, ownerID, id); err != nil {
		return err
	}
	if err := s.repo.DeleteByID(ctx, id); err != nil {
		return err
	}
	s.record(ctx, ownerID, &id, "server.deleted", "success", "")
	return nil
}

// Connect opens a managed SSH client for a terminal or other long-lived
// operation. The caller owns the returned client and must close it.
func (s *Service) Connect(ctx context.Context, ownerID, id string) (*ssh.Client, *storage.Server, error) {
	server, err := s.Get(ctx, ownerID, id)
	if err != nil {
		return nil, nil, err
	}
	credentials, err := s.decryptCredentials(server)
	if err != nil {
		return nil, server, err
	}
	dialCtx, cancel := context.WithTimeout(ctx, s.cfg.SSHDialTimeout+s.cfg.SSHHandshakeTimeout)
	defer cancel()
	client, _, err := s.dialer.Dial(dialCtx, server, credentials)
	if err != nil {
		s.record(ctx, ownerID, &server.ID, "server.connected", "failure", safeSSHCode(err))
		return nil, server, err
	}
	now := time.Now().UTC()
	if err := s.repo.UpdateLastConnected(ctx, server.ID, now); err != nil {
		s.logger.Warn("update last connected failed", "error", err, "server_id", server.ID)
	}
	s.record(ctx, ownerID, &server.ID, "server.connected", "success", "")
	return client, server, nil
}

func (s *Service) TestConnection(ctx context.Context, ownerID, id string) (*ConnectionResult, error) {
	server, err := s.Get(ctx, ownerID, id)
	if err != nil {
		return nil, err
	}
	credentials, err := s.decryptCredentials(server)
	if err != nil {
		return nil, err
	}

	testCtx, cancel := context.WithTimeout(ctx, s.cfg.SSHDialTimeout+s.cfg.SSHHandshakeTimeout+s.cfg.SSHOperationTimeout)
	defer cancel()

	client, hostKey, err := s.dialer.Dial(testCtx, server, credentials)
	if err != nil {
		s.record(ctx, ownerID, &server.ID, "server.connection_tested", "failure", safeSSHCode(err))
		return nil, err
	}
	defer func() { _ = client.Close() }()

	capabilities := sshclient.DetectCapabilities(testCtx, client)
	data, err := json.Marshal(capabilities)
	if err != nil {
		return nil, fmt.Errorf("encode capabilities: %w", err)
	}
	now := time.Now().UTC()
	if err := s.caps.Upsert(ctx, server.ID, string(data), now); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateLastConnected(ctx, server.ID, now); err != nil {
		s.logger.Warn("update last connected failed", "error", err, "server_id", server.ID)
	}
	s.record(ctx, ownerID, &server.ID, "server.connection_tested", "success", "")
	return &ConnectionResult{HostKey: hostKey, Capabilities: capabilities}, nil
}

func (s *Service) ApproveHostKey(
	ctx context.Context,
	ownerID, id, fingerprint, keyType, publicKey string,
	replace bool,
) error {
	server, err := s.Get(ctx, ownerID, id)
	if err != nil {
		return err
	}

	key, err := sshclient.ParseHostKey(publicKey)
	if err != nil {
		return &ValidationError{Field: "public_key", Message: "host key could not be parsed"}
	}
	actualFingerprint := ssh.FingerprintSHA256(key)
	if actualFingerprint != fingerprint || key.Type() != keyType {
		return &ValidationError{Field: "fingerprint", Message: "fingerprint does not match the presented key"}
	}

	existing, err := s.known.GetActive(ctx, server.ID)
	if err == nil && !replace {
		return ErrHostKeyAlreadyApproved
	}
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return err
	}

	record := storage.KnownHost{
		ID:          storage.NewID(),
		ServerID:    server.ID,
		Host:        server.Host,
		Port:        server.Port,
		KeyType:     key.Type(),
		Fingerprint: actualFingerprint,
		PublicKey:   publicKey,
		ApprovedAt:  time.Now().UTC(),
		ApprovedBy:  &ownerID,
	}
	if err := s.known.Approve(ctx, record, replace); err != nil {
		return err
	}
	action := "server.host_key_approved"
	if existing != nil {
		action = "server.host_key_reapproved"
	}
	s.record(ctx, ownerID, &server.ID, action, "success", "")
	return nil
}

func (s *Service) Capabilities(ctx context.Context, ownerID, id string) (*sshclient.Capabilities, error) {
	if _, err := s.Get(ctx, ownerID, id); err != nil {
		return nil, err
	}
	record, err := s.caps.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	var capabilities sshclient.Capabilities
	if err := json.Unmarshal([]byte(record.Data), &capabilities); err != nil {
		return nil, fmt.Errorf("decode capabilities: %w", err)
	}
	return &capabilities, nil
}

type encryptedSecret struct {
	secret          string
	nonce           string
	passphrase      string
	passphraseNonce string
}

func (s *Service) encryptSecret(id string, input CreateInput) (encryptedSecret, error) {
	plaintext := input.Password
	if input.AuthMethod == AuthPrivateKey {
		plaintext = input.PrivateKey
	}
	encrypted, err := s.cipher.Encrypt([]byte(plaintext), []byte("server:"+id+":secret"))
	if err != nil {
		return encryptedSecret{}, err
	}
	result := encryptedSecret{
		secret: base64.StdEncoding.EncodeToString(encrypted.Ciphertext),
		nonce:  base64.StdEncoding.EncodeToString(encrypted.Nonce),
	}
	if input.Passphrase != "" {
		passphrase, err := s.cipher.Encrypt([]byte(input.Passphrase), []byte("server:"+id+":passphrase"))
		if err != nil {
			return encryptedSecret{}, err
		}
		result.passphrase = base64.StdEncoding.EncodeToString(passphrase.Ciphertext)
		result.passphraseNonce = base64.StdEncoding.EncodeToString(passphrase.Nonce)
	}
	return result, nil
}

func (s *Service) decryptCredentials(server *storage.Server) (sshclient.Credentials, error) {
	nonce, err := base64.StdEncoding.DecodeString(server.SecretNonce)
	if err != nil {
		return sshclient.Credentials{}, fmt.Errorf("decode secret nonce: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(server.EncryptedSecret)
	if err != nil {
		return sshclient.Credentials{}, fmt.Errorf("decode secret: %w", err)
	}
	plaintext, err := s.cipher.Decrypt(secrets.Encrypted{
		Version:    server.SecretVersion,
		Nonce:      nonce,
		Ciphertext: ciphertext,
	}, []byte("server:"+server.ID+":secret"))
	if err != nil {
		return sshclient.Credentials{}, err
	}

	credentials := sshclient.Credentials{AuthMethod: server.AuthMethod}
	switch server.AuthMethod {
	case AuthPassword:
		credentials.Password = string(plaintext)
	case AuthPrivateKey:
		credentials.PrivateKey = plaintext
	default:
		return sshclient.Credentials{}, fmt.Errorf("unsupported authentication method")
	}

	if server.EncryptedPassphrase != "" && server.PassphraseNonce != "" {
		passNonce, err := base64.StdEncoding.DecodeString(server.PassphraseNonce)
		if err != nil {
			return sshclient.Credentials{}, fmt.Errorf("decode passphrase nonce: %w", err)
		}
		passCiphertext, err := base64.StdEncoding.DecodeString(server.EncryptedPassphrase)
		if err != nil {
			return sshclient.Credentials{}, fmt.Errorf("decode passphrase: %w", err)
		}
		passphrase, err := s.cipher.Decrypt(secrets.Encrypted{
			Version:    server.PassphraseVersion,
			Nonce:      passNonce,
			Ciphertext: passCiphertext,
		}, []byte("server:"+server.ID+":passphrase"))
		if err != nil {
			return sshclient.Credentials{}, err
		}
		credentials.Passphrase = string(passphrase)
	}
	return credentials, nil
}

func validateCreate(input CreateInput) (CreateInput, error) {
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Host = normalizeHost(input.Host)
	input.Username = strings.TrimSpace(input.Username)
	input.AuthMethod = strings.TrimSpace(input.AuthMethod)
	input.Tags = strings.TrimSpace(input.Tags)
	input.Notes = strings.TrimSpace(input.Notes)
	if input.Port == 0 {
		input.Port = 22
	}
	if input.DisplayName == "" || len(input.DisplayName) > maxDisplayName {
		return input, &ValidationError{Field: "display_name", Message: "must be 1 to 120 characters"}
	}
	if input.Host == "" || len(input.Host) > maxHost || strings.ContainsAny(input.Host, " \t\r\n/@") {
		return input, &ValidationError{Field: "host", Message: "must be a valid hostname or IP address"}
	}
	if input.Port < 1 || input.Port > 65535 {
		return input, &ValidationError{Field: "port", Message: "must be between 1 and 65535"}
	}
	if input.Username == "" || len(input.Username) > maxUsername || strings.ContainsAny(input.Username, " \t\r\n") {
		return input, &ValidationError{Field: "username", Message: "must be 1 to 64 characters without spaces"}
	}
	if len(input.Tags) > maxTags {
		return input, &ValidationError{Field: "tags", Message: "must be at most 200 characters"}
	}
	if len(input.Notes) > maxNotes {
		return input, &ValidationError{Field: "notes", Message: "must be at most 2000 characters"}
	}
	switch input.AuthMethod {
	case AuthPassword:
		if input.Password == "" || len(input.Password) > maxPassword {
			return input, &ValidationError{Field: "password", Message: "must be 1 to 1024 characters"}
		}
	case AuthPrivateKey:
		if input.PrivateKey == "" || len(input.PrivateKey) > maxPrivateKey {
			return input, &ValidationError{Field: "private_key", Message: "must be a private key up to 64 KiB"}
		}
		if len(input.Passphrase) > maxPassword {
			return input, &ValidationError{Field: "passphrase", Message: "must be at most 1024 characters"}
		}
	default:
		return input, &ValidationError{Field: "auth_method", Message: "must be password or private_key"}
	}
	return input, nil
}

func validateMetadata(server *storage.Server) error {
	if server.DisplayName == "" || len(server.DisplayName) > maxDisplayName {
		return &ValidationError{Field: "display_name", Message: "must be 1 to 120 characters"}
	}
	if server.Host == "" || len(server.Host) > maxHost || strings.ContainsAny(server.Host, " \t\r\n/@") {
		return &ValidationError{Field: "host", Message: "must be a valid hostname or IP address"}
	}
	if server.Port < 1 || server.Port > 65535 {
		return &ValidationError{Field: "port", Message: "must be between 1 and 65535"}
	}
	if server.Username == "" || len(server.Username) > maxUsername {
		return &ValidationError{Field: "username", Message: "must be 1 to 64 characters"}
	}
	return nil
}

func normalizeHost(host string) string {
	host = strings.TrimSpace(host)
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")
	if parsed := net.ParseIP(host); parsed != nil {
		return parsed.String()
	}
	return strings.ToLower(host)
}

func safeSSHCode(err error) string {
	var unknown *sshclient.HostKeyUnknownError
	if errors.As(err, &unknown) {
		return "SSH_HOST_KEY_UNKNOWN"
	}
	var changed *sshclient.HostKeyChangedError
	if errors.As(err, &changed) {
		return "SSH_HOST_KEY_CHANGED"
	}
	if errors.Is(err, sshclient.ErrInvalidCredentials) || errors.Is(err, sshclient.ErrInvalidPrivateKey) {
		return "SSH_AUTH_FAILED"
	}
	return "SSH_CONNECTION_FAILED"
}

func (s *Service) record(ctx context.Context, actorID string, serverID *string, eventType, outcome, code string) {
	var safeCode *string
	if code != "" {
		safeCode = &code
	}
	s.audit.Record(ctx, storage.AuditEvent{
		ActorUserID:   &actorID,
		ServerID:      serverID,
		EventType:     eventType,
		Outcome:       outcome,
		SafeErrorCode: safeCode,
		CreatedAt:     time.Now().UTC(),
	})
}
