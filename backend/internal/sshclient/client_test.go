package sshclient

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

	"github.com/awbuilds/drovik/backend/internal/config"
	"github.com/awbuilds/drovik/backend/internal/storage"
	"github.com/awbuilds/drovik/backend/internal/testssh"
)

type sshTestEnv struct {
	dialer *Dialer
	known  *storage.KnownHostRepository
	server *storage.Server
	cfg    *config.Config
}

func newSSHTestEnv(t *testing.T, sshServer *testssh.Server) *sshTestEnv {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "ssh.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := storage.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	users := storage.NewUserRepository(db)
	now := time.Now().UTC()
	user := storage.User{
		ID: storage.NewID(), Username: "admin", PasswordHash: "hash", Role: "admin",
		IsActive: true, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := users.CreateFirstAdmin(context.Background(), user); err != nil {
		t.Fatalf("CreateFirstAdmin: %v", err)
	}
	host, portText, err := net.SplitHostPort(sshServer.Addr)
	if err != nil {
		t.Fatalf("SplitHostPort: %v", err)
	}
	port, _ := strconv.Atoi(portText)
	servers := storage.NewServerRepository(db)
	server := storage.Server{
		ID: storage.NewID(), OwnerUserID: user.ID, DisplayName: "test",
		Host: host, Port: port, Username: "test", AuthMethod: "password",
		EncryptedSecret: "x", SecretNonce: "x", SecretVersion: 1,
		CreatedAt: now, UpdatedAt: now, IsActive: true,
	}
	if err := servers.Create(context.Background(), server); err != nil {
		t.Fatalf("Create server: %v", err)
	}
	cfg := &config.Config{
		SSHDialTimeout:        5 * time.Second,
		SSHHandshakeTimeout:   5 * time.Second,
		SSHOperationTimeout:   5 * time.Second,
		EgressAllowPrivate:    true,
		EgressAllowedPrefixes: []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")},
	}
	known := storage.NewKnownHostRepository(db)
	dialer := NewDialer(cfg, known, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return &sshTestEnv{dialer: dialer, known: known, server: &server, cfg: cfg}
}

func approveTestHost(t *testing.T, env *sshTestEnv, sshServer *testssh.Server) {
	t.Helper()
	info := HostKeyInfo{
		KeyType:     sshServer.HostKey.Type(),
		Fingerprint: ssh.FingerprintSHA256(sshServer.HostKey),
		PublicKey:   base64.StdEncoding.EncodeToString(sshServer.HostKey.Marshal()),
	}
	err := env.known.Approve(context.Background(), storage.KnownHost{
		ID: storage.NewID(), ServerID: env.server.ID, Host: env.server.Host, Port: env.server.Port,
		KeyType: info.KeyType, Fingerprint: info.Fingerprint, PublicKey: info.PublicKey,
		ApprovedAt: time.Now().UTC(),
	}, false)
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}
}

func TestDialUnknownHostKey(t *testing.T) {
	sshServer, err := testssh.Start(t.TempDir())
	if err != nil {
		t.Fatalf("testssh.Start: %v", err)
	}
	defer sshServer.Close()
	env := newSSHTestEnv(t, sshServer)

	client, info, err := env.dialer.Dial(context.Background(), env.server, Credentials{
		AuthMethod: "password",
		Password:   sshServer.Password,
	})
	if client != nil {
		_ = client.Close()
	}
	var unknown *HostKeyUnknownError
	if !errors.As(err, &unknown) {
		t.Fatalf("expected HostKeyUnknownError, got %v", err)
	}
	expected := ssh.FingerprintSHA256(sshServer.HostKey)
	if unknown.Info.Fingerprint != expected || info.Fingerprint != expected {
		t.Fatalf("unexpected fingerprint: %q / %q", unknown.Info.Fingerprint, info.Fingerprint)
	}
}

func TestDialPasswordAuthenticatedHost(t *testing.T) {
	sshServer, err := testssh.Start(t.TempDir())
	if err != nil {
		t.Fatalf("testssh.Start: %v", err)
	}
	defer sshServer.Close()
	env := newSSHTestEnv(t, sshServer)
	approveTestHost(t, env, sshServer)

	client, info, err := env.dialer.Dial(context.Background(), env.server, Credentials{
		AuthMethod: "password",
		Password:   sshServer.Password,
	})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer func() { _ = client.Close() }()
	if info.Fingerprint != ssh.FingerprintSHA256(sshServer.HostKey) {
		t.Fatalf("unexpected host key info: %+v", info)
	}

	stdout, _, err := RunCommand(context.Background(), client, "uname -s", 1024)
	if err != nil || stdout != "Linux" {
		t.Fatalf("RunCommand: %q %v", stdout, err)
	}

	capabilities := DetectCapabilities(context.Background(), client)
	if capabilities.OS != "Linux" || !capabilities.Systemd || !capabilities.SFTP {
		t.Fatalf("unexpected capabilities: %+v", capabilities)
	}
	if capabilities.Distribution != "Drovik Test Linux" {
		t.Fatalf("unexpected distribution: %q", capabilities.Distribution)
	}
}

func TestDialPrivateKeyAuthenticatedHost(t *testing.T) {
	sshServer, err := testssh.Start(t.TempDir())
	if err != nil {
		t.Fatalf("testssh.Start: %v", err)
	}
	defer sshServer.Close()
	env := newSSHTestEnv(t, sshServer)
	approveTestHost(t, env, sshServer)

	block, err := ssh.MarshalPrivateKey(sshServer.ClientPrivateKey, "")
	if err != nil {
		t.Fatalf("MarshalPrivateKey: %v", err)
	}
	privateKey := pem.EncodeToMemory(block)
	client, _, err := env.dialer.Dial(context.Background(), env.server, Credentials{
		AuthMethod: "private_key",
		PrivateKey: privateKey,
	})
	if err != nil {
		t.Fatalf("Dial with private key: %v", err)
	}
	_ = client.Close()
}

func TestDialChangedHostKey(t *testing.T) {
	first, err := testssh.Start(t.TempDir())
	if err != nil {
		t.Fatalf("testssh.Start: %v", err)
	}
	defer first.Close()
	env := newSSHTestEnv(t, first)
	approveTestHost(t, env, first)

	second, err := testssh.Start(t.TempDir())
	if err != nil {
		t.Fatalf("testssh.Start second: %v", err)
	}
	defer second.Close()
	_, portText, _ := net.SplitHostPort(second.Addr)
	port, _ := strconv.Atoi(portText)
	env.server.Port = port

	client, _, err := env.dialer.Dial(context.Background(), env.server, Credentials{
		AuthMethod: "password",
		Password:   second.Password,
	})
	if client != nil {
		_ = client.Close()
	}
	var changed *HostKeyChangedError
	if !errors.As(err, &changed) {
		t.Fatalf("expected HostKeyChangedError, got %v", err)
	}
	if changed.Presented.Fingerprint == changed.Previous.Fingerprint {
		t.Fatalf("expected different fingerprints: %+v", changed)
	}
}
