package files

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/awbuilds/drovik/backend/internal/audit"
	"github.com/awbuilds/drovik/backend/internal/config"
	"github.com/awbuilds/drovik/backend/internal/secrets"
	"github.com/awbuilds/drovik/backend/internal/servers"
	"github.com/awbuilds/drovik/backend/internal/sshclient"
	"github.com/awbuilds/drovik/backend/internal/storage"
	"github.com/awbuilds/drovik/backend/internal/testssh"
)

type filesTestEnv struct {
	service   *Service
	server    *storage.Server
	userID    string
	workDir   string
	sshServer *testssh.Server
}

func newFilesTestEnv(t *testing.T) *filesTestEnv {
	t.Helper()
	workDir := t.TempDir()
	sshServer, err := testssh.Start(workDir)
	if err != nil {
		t.Fatalf("testssh.Start: %v", err)
	}
	t.Cleanup(sshServer.Close)

	db, err := storage.Open(filepath.Join(t.TempDir(), "files.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
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
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	serverRepo := storage.NewServerRepository(db)
	known := storage.NewKnownHostRepository(db)
	caps := storage.NewCapabilitiesRepository(db)
	cfg := &config.Config{
		SSHDialTimeout:          5 * time.Second,
		SSHHandshakeTimeout:     5 * time.Second,
		SSHOperationTimeout:     5 * time.Second,
		EgressAllowPrivate:      true,
		EgressAllowedPrefixes:   []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")},
		SFTPMaxUploadBytes:      1 << 20,
		SFTPMaxEditorBytes:      64 << 10,
		SFTPMaxListEntries:      100,
		SFTPMaxRecursiveEntries: 100,
		SFTPOperationTimeout:    10 * time.Second,
	}
	dialer := sshclient.NewDialer(cfg, known, logger)
	auditService := audit.New(storage.NewAuditRepository(db), logger)
	serverService := servers.NewService(cfg, serverRepo, known, caps, cipher, dialer, auditService, logger)
	filesService := NewService(cfg, serverService, auditService, logger)

	host, portText, _ := net.SplitHostPort(sshServer.Addr)
	port, _ := strconv.Atoi(portText)
	server, err := serverService.Create(context.Background(), user.ID, servers.CreateInput{
		DisplayName: "SFTP test",
		Host:        host,
		Port:        port,
		Username:    "test",
		AuthMethod:  servers.AuthPassword,
		Password:    sshServer.Password,
	})
	if err != nil {
		t.Fatalf("Create server: %v", err)
	}
	info := sshclient.HostKeyInfo{
		KeyType:     sshServer.HostKey.Type(),
		Fingerprint: ssh.FingerprintSHA256(sshServer.HostKey),
		PublicKey:   base64.StdEncoding.EncodeToString(sshServer.HostKey.Marshal()),
	}
	if err := serverService.ApproveHostKey(
		context.Background(), user.ID, server.ID,
		info.Fingerprint, info.KeyType, info.PublicKey, false,
	); err != nil {
		t.Fatalf("ApproveHostKey: %v", err)
	}

	return &filesTestEnv{
		service: filesService, server: server, userID: user.ID, workDir: workDir, sshServer: sshServer,
	}
}

func TestListReadWriteAndStale(t *testing.T) {
	env := newFilesTestEnv(t)
	ctx := context.Background()
	path := filepath.Join(env.workDir, "hello.txt")
	if err := os.WriteFile(path, []byte("hello world\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := os.Mkdir(filepath.Join(env.workDir, "folder"), 0o755); err != nil {
		t.Fatalf("mkdir fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(env.workDir, "folder", "nested.txt"), []byte("nested"), 0o644); err != nil {
		t.Fatalf("nested fixture: %v", err)
	}

	list, err := env.service.List(ctx, env.userID, env.server.ID, env.workDir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list.Entries) < 2 || list.Entries[0].Type != "directory" {
		t.Fatalf("unexpected listing: %+v", list)
	}

	content, err := env.service.ReadFile(ctx, env.userID, env.server.ID, path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if content.Content != "hello world\n" {
		t.Fatalf("unexpected content: %q", content.Content)
	}

	expected := &ExpectedVersion{ModifiedAt: &content.ModifiedAt, Size: &content.Size}
	if err := env.service.WriteFile(ctx, env.userID, env.server.ID, path, "updated\n", expected); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	updated, err := env.service.ReadFile(ctx, env.userID, env.server.ID, path)
	if err != nil || updated.Content != "updated\n" {
		t.Fatalf("ReadFile updated: %v %+v", err, updated)
	}
	if err := env.service.WriteFile(ctx, env.userID, env.server.ID, path, "stale\n", expected); !errors.Is(err, ErrStale) {
		t.Fatalf("expected stale error, got %v", err)
	}
}

func TestBinaryAndSizeLimits(t *testing.T) {
	env := newFilesTestEnv(t)
	ctx := context.Background()
	binaryPath := filepath.Join(env.workDir, "binary.bin")
	if err := os.WriteFile(binaryPath, []byte{0x00, 0x01, 0xff}, 0o644); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	if _, err := env.service.ReadFile(ctx, env.userID, env.server.ID, binaryPath); !errors.Is(err, ErrBinary) {
		t.Fatalf("expected binary error, got %v", err)
	}

	largePath := filepath.Join(env.workDir, "large.txt")
	if err := os.WriteFile(largePath, []byte(strings.Repeat("a", 65<<10)), 0o644); err != nil {
		t.Fatalf("write large: %v", err)
	}
	if _, err := env.service.ReadFile(ctx, env.userID, env.server.ID, largePath); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("expected size error, got %v", err)
	}
}

func TestMkdirTouchRenameDelete(t *testing.T) {
	env := newFilesTestEnv(t)
	ctx := context.Background()
	dir := filepath.Join(env.workDir, "new-folder")
	if err := env.service.Mkdir(ctx, env.userID, env.server.ID, dir); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	file := filepath.Join(dir, "new.txt")
	if err := env.service.Touch(ctx, env.userID, env.server.ID, file); err != nil {
		t.Fatalf("Touch: %v", err)
	}
	if err := env.service.Touch(ctx, env.userID, env.server.ID, file); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
	renamed := filepath.Join(dir, "renamed.txt")
	if err := env.service.Rename(ctx, env.userID, env.server.ID, file, renamed, false); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if err := env.service.Delete(ctx, env.userID, env.server.ID, dir, true); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := env.service.Stat(ctx, env.userID, env.server.ID, dir); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected deleted path, got %v", err)
	}
}

func TestUploadAndDownload(t *testing.T) {
	env := newFilesTestEnv(t)
	ctx := context.Background()
	target := filepath.Join(env.workDir, "uploaded.txt")
	if err := env.service.Upload(ctx, env.userID, env.server.ID, target, strings.NewReader("uploaded content"), 16, false); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	download, err := env.service.OpenDownload(ctx, env.userID, env.server.ID, target)
	if err != nil {
		t.Fatalf("OpenDownload: %v", err)
	}
	defer download.Close()
	data, err := io.ReadAll(download.Reader)
	if err != nil {
		t.Fatalf("read download: %v", err)
	}
	if string(data) != "uploaded content" {
		t.Fatalf("unexpected download: %q", data)
	}
	if err := env.service.Upload(ctx, env.userID, env.server.ID, target, strings.NewReader("x"), 1, false); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected upload conflict, got %v", err)
	}
}

func TestInvalidPathAndSymlink(t *testing.T) {
	env := newFilesTestEnv(t)
	ctx := context.Background()
	if _, err := env.service.List(ctx, env.userID, env.server.ID, "relative/path"); !errors.Is(err, ErrPathInvalid) {
		t.Fatalf("expected invalid path, got %v", err)
	}
	target := filepath.Join(env.workDir, "target.txt")
	if err := os.WriteFile(target, []byte("target"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(env.workDir, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := env.service.ReadFile(ctx, env.userID, env.server.ID, link); !errors.Is(err, ErrSymlink) {
		t.Fatalf("expected symlink error, got %v", err)
	}
}
