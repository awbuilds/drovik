package servers

import (
	"context"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/awbuilds/drovik/backend/internal/audit"
	"github.com/awbuilds/drovik/backend/internal/config"
	"github.com/awbuilds/drovik/backend/internal/secrets"
	"github.com/awbuilds/drovik/backend/internal/sshclient"
	"github.com/awbuilds/drovik/backend/internal/storage"
	"github.com/awbuilds/drovik/backend/internal/testssh"
	"github.com/awbuilds/drovik/backend/internal/vault"
)

type serviceTestEnv struct {
	service     *Service
	users       storage.User
	userRepo    *storage.UserRepository
	servers     *storage.ServerRepository
	assignments *storage.AssignmentRepository
	known       *storage.KnownHostRepository
	cfg         *config.Config
}

func newServiceTestEnv(t *testing.T, sshServer *testssh.Server) *serviceTestEnv {
	t.Helper()
	return newServiceTestEnvWith(t, sshServer, nil)
}

func newServiceTestEnvWith(t *testing.T, sshServer *testssh.Server, vaultClient *vault.Client) *serviceTestEnv {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "servers.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := storage.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	now := time.Now().UTC()
	users := storage.NewUserRepository(db)
	user := storage.User{
		ID: storage.NewID(), Username: "admin", PasswordHash: "hash", Role: "admin",
		IsActive: true, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := users.CreateFirstAdmin(context.Background(), user); err != nil {
		t.Fatalf("CreateFirstAdmin: %v", err)
	}

	key := []byte("0123456789abcdef0123456789abcdef")
	cipher, err := secrets.New(key)
	if err != nil {
		t.Fatalf("secrets.New: %v", err)
	}
	serverRepo := storage.NewServerRepository(db)
	known := storage.NewKnownHostRepository(db)
	caps := storage.NewCapabilitiesRepository(db)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{
		SSHDialTimeout:        5 * time.Second,
		SSHHandshakeTimeout:   5 * time.Second,
		SSHOperationTimeout:   5 * time.Second,
		EgressAllowPrivate:    true,
		EgressAllowedPrefixes: []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")},
	}
	dialer := sshclient.NewDialer(cfg, known, logger)
	auditService := audit.New(storage.NewAuditRepository(db), logger)
	assignments := storage.NewAssignmentRepository(db)
	service := NewService(cfg, serverRepo, users, assignments, storage.NewTeamRepository(db), known, caps, cipher, vaultClient, dialer, auditService, logger)
	return &serviceTestEnv{
		service: service, users: user, userRepo: users, servers: serverRepo,
		assignments: assignments, known: known, cfg: cfg,
	}
}

func testCreateInput(t *testing.T, sshServer *testssh.Server) CreateInput {
	t.Helper()
	host, portText, err := net.SplitHostPort(sshServer.Addr)
	if err != nil {
		t.Fatalf("SplitHostPort: %v", err)
	}
	port, _ := strconv.Atoi(portText)
	return CreateInput{
		DisplayName: "Test host",
		Host:        host,
		Port:        port,
		Username:    "test",
		AuthMethod:  AuthPassword,
		Password:    sshServer.Password,
	}
}

func TestCreateEncryptsCredential(t *testing.T) {
	sshServer, err := testssh.Start(t.TempDir())
	if err != nil {
		t.Fatalf("testssh.Start: %v", err)
	}
	defer sshServer.Close()
	env := newServiceTestEnv(t, sshServer)

	server, err := env.service.Create(context.Background(), env.users.ID, testCreateInput(t, sshServer))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if server.EncryptedSecret == "" || server.SecretNonce == "" {
		t.Fatal("credential was not encrypted")
	}
	if server.EncryptedSecret == sshServer.Password {
		t.Fatal("credential stored in plaintext")
	}
	stored, err := env.servers.GetByID(context.Background(), server.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if stored.EncryptedSecret == sshServer.Password {
		t.Fatal("stored credential is plaintext")
	}
}

func TestCreateValidation(t *testing.T) {
	sshServer, _ := testssh.Start(t.TempDir())
	defer sshServer.Close()
	env := newServiceTestEnv(t, sshServer)

	cases := []CreateInput{
		{DisplayName: "", Host: "example.test", Port: 22, Username: "ops", AuthMethod: AuthPassword, Password: "x"},
		{DisplayName: "x", Host: "bad host", Port: 22, Username: "ops", AuthMethod: AuthPassword, Password: "x"},
		{DisplayName: "x", Host: "example.test", Port: 70000, Username: "ops", AuthMethod: AuthPassword, Password: "x"},
		{DisplayName: "x", Host: "example.test", Port: 22, Username: "bad user", AuthMethod: AuthPassword, Password: "x"},
		{DisplayName: "x", Host: "example.test", Port: 22, Username: "ops", AuthMethod: "agent", Password: "x"},
	}
	for _, input := range cases {
		if _, err := env.service.Create(context.Background(), env.users.ID, input); err == nil {
			t.Fatalf("expected validation error for %+v", input)
		}
	}
}

func TestHostKeyApprovalAndConnection(t *testing.T) {
	sshServer, err := testssh.Start(t.TempDir())
	if err != nil {
		t.Fatalf("testssh.Start: %v", err)
	}
	defer sshServer.Close()
	env := newServiceTestEnv(t, sshServer)
	input := testCreateInput(t, sshServer)
	server, err := env.service.Create(context.Background(), env.users.ID, input)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = env.service.TestConnection(context.Background(), env.users.ID, server.ID)
	var unknown *sshclient.HostKeyUnknownError
	if !errors.As(err, &unknown) {
		t.Fatalf("expected unknown host key, got %v", err)
	}
	if unknown.Info.Fingerprint != ssh.FingerprintSHA256(sshServer.HostKey) {
		t.Fatalf("unexpected fingerprint: %s", unknown.Info.Fingerprint)
	}

	if err := env.service.ApproveHostKey(
		context.Background(), env.users.ID, server.ID,
		unknown.Info.Fingerprint, unknown.Info.KeyType, unknown.Info.PublicKey, false,
	); err != nil {
		t.Fatalf("ApproveHostKey: %v", err)
	}
	if err := env.service.ApproveHostKey(
		context.Background(), env.users.ID, server.ID,
		unknown.Info.Fingerprint, unknown.Info.KeyType, unknown.Info.PublicKey, false,
	); !errors.Is(err, ErrHostKeyAlreadyApproved) {
		t.Fatalf("expected already-approved error, got %v", err)
	}

	result, err := env.service.TestConnection(context.Background(), env.users.ID, server.ID)
	if err != nil {
		t.Fatalf("TestConnection: %v", err)
	}
	if result.Capabilities.OS != "Linux" || !result.Capabilities.SFTP {
		t.Fatalf("unexpected capabilities: %+v", result.Capabilities)
	}
	capabilities, err := env.service.Capabilities(context.Background(), env.users.ID, server.ID)
	if err != nil || capabilities.OS != "Linux" {
		t.Fatalf("Capabilities: %v %+v", err, capabilities)
	}
}

func TestWrongPasswordFails(t *testing.T) {
	sshServer, err := testssh.Start(t.TempDir())
	if err != nil {
		t.Fatalf("testssh.Start: %v", err)
	}
	defer sshServer.Close()
	env := newServiceTestEnv(t, sshServer)
	input := testCreateInput(t, sshServer)
	input.Password = "wrong-password"
	server, err := env.service.Create(context.Background(), env.users.ID, input)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	info := sshclient.HostKeyInfo{
		KeyType:     sshServer.HostKey.Type(),
		Fingerprint: ssh.FingerprintSHA256(sshServer.HostKey),
		PublicKey:   base64.StdEncoding.EncodeToString(sshServer.HostKey.Marshal()),
	}
	if err := env.service.ApproveHostKey(
		context.Background(), env.users.ID, server.ID,
		info.Fingerprint, info.KeyType, info.PublicKey, false,
	); err != nil {
		t.Fatalf("ApproveHostKey: %v", err)
	}
	_, err = env.service.TestConnection(context.Background(), env.users.ID, server.ID)
	if err == nil {
		t.Fatal("expected authentication failure")
	}
}

func TestUpdateHostRevokesTrust(t *testing.T) {
	sshServer, err := testssh.Start(t.TempDir())
	if err != nil {
		t.Fatalf("testssh.Start: %v", err)
	}
	defer sshServer.Close()
	env := newServiceTestEnv(t, sshServer)
	input := testCreateInput(t, sshServer)
	server, err := env.service.Create(context.Background(), env.users.ID, input)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	info := sshclient.HostKeyInfo{
		KeyType:     sshServer.HostKey.Type(),
		Fingerprint: ssh.FingerprintSHA256(sshServer.HostKey),
		PublicKey:   base64.StdEncoding.EncodeToString(sshServer.HostKey.Marshal()),
	}
	if err := env.service.ApproveHostKey(
		context.Background(), env.users.ID, server.ID,
		info.Fingerprint, info.KeyType, info.PublicKey, false,
	); err != nil {
		t.Fatalf("ApproveHostKey: %v", err)
	}

	newHost := "localhost"
	updated, err := env.service.Update(context.Background(), env.users.ID, server.ID, UpdateInput{Host: &newHost})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Host != newHost {
		t.Fatalf("host not updated: %s", updated.Host)
	}
	if _, err := env.known.GetActive(context.Background(), server.ID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("expected trust to be revoked, got %v", err)
	}
}

func TestDeleteServer(t *testing.T) {
	sshServer, err := testssh.Start(t.TempDir())
	if err != nil {
		t.Fatalf("testssh.Start: %v", err)
	}
	defer sshServer.Close()
	env := newServiceTestEnv(t, sshServer)
	server, err := env.service.Create(context.Background(), env.users.ID, testCreateInput(t, sshServer))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := env.service.Delete(context.Background(), env.users.ID, server.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := env.service.Get(context.Background(), env.users.ID, server.ID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestAuthMethodChangeRequiresNewCredential(t *testing.T) {
	sshServer, err := testssh.Start(t.TempDir())
	if err != nil {
		t.Fatalf("testssh.Start: %v", err)
	}
	defer sshServer.Close()
	env := newServiceTestEnv(t, sshServer)

	input := testCreateInput(t, sshServer)
	input.AuthMethod = AuthPrivateKey
	block, err := ssh.MarshalPrivateKey(sshServer.ClientPrivateKey, "")
	if err != nil {
		t.Fatalf("MarshalPrivateKey: %v", err)
	}
	input.PrivateKey = string(pem.EncodeToMemory(block))
	input.Password = ""
	server, err := env.service.Create(context.Background(), env.users.ID, input)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	password := AuthPassword
	if _, err := env.service.Update(context.Background(), env.users.ID, server.ID, UpdateInput{AuthMethod: &password}); err == nil {
		t.Fatal("expected a method change without a new password to be rejected")
	}
	stored, err := env.servers.GetByID(context.Background(), server.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if stored.AuthMethod != AuthPrivateKey {
		t.Fatalf("auth method changed despite rejected update: %s", stored.AuthMethod)
	}

	newPassword := "a-new-password-for-the-host"
	if _, err := env.service.Update(context.Background(), env.users.ID, server.ID, UpdateInput{
		AuthMethod: &password,
		Password:   &newPassword,
	}); err != nil {
		t.Fatalf("Update with new password: %v", err)
	}
}

func TestServerAccessLevels(t *testing.T) {
	sshServer, err := testssh.Start(t.TempDir())
	if err != nil {
		t.Fatalf("testssh.Start: %v", err)
	}
	defer sshServer.Close()
	env := newServiceTestEnv(t, sshServer)
	ctx := context.Background()
	server, err := env.service.Create(ctx, env.users.ID, testCreateInput(t, sshServer))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	operator := storage.User{
		ID: storage.NewID(), Username: "operator", PasswordHash: "hash", Role: "operator",
		IsActive: true, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := env.userRepo.Create(ctx, operator); err != nil {
		t.Fatalf("Create operator: %v", err)
	}
	if err := env.assignments.Upsert(ctx, storage.ServerAssignment{
		UserID: operator.ID, ServerID: server.ID, AccessLevel: "read", CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("Assign read: %v", err)
	}
	level, err := env.service.AccessLevel(ctx, operator.ID, server.ID)
	if err != nil || level != "read" {
		t.Fatalf("AccessLevel read: %v %s", err, level)
	}
	if _, err := env.service.Get(ctx, operator.ID, server.ID); err != nil {
		t.Fatalf("Get assigned server: %v", err)
	}
	list, err := env.service.List(ctx, operator.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("List assigned: %v %+v", err, list)
	}
	if err := env.assignments.Upsert(ctx, storage.ServerAssignment{
		UserID: operator.ID, ServerID: server.ID, AccessLevel: "write", CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("Assign write: %v", err)
	}
	level, err = env.service.AccessLevel(ctx, operator.ID, server.ID)
	if err != nil || level != "write" {
		t.Fatalf("AccessLevel write: %v %s", err, level)
	}
	if level, err := env.service.AccessLevel(ctx, env.users.ID, server.ID); err != nil || level != "write" {
		t.Fatalf("admin AccessLevel: %v %s", err, level)
	}
}
