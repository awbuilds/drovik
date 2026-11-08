package monitoring

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"path/filepath"
	"strconv"
	"sync"
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

type monitoringTestEnv struct {
	service *Service
	cfg     *config.Config
	userID  string
	server  *storage.Server
	ssh     *testssh.Server
}

func newMonitoringTestEnv(t *testing.T) *monitoringTestEnv {
	t.Helper()
	sshServer, err := testssh.Start(t.TempDir())
	if err != nil {
		t.Fatalf("testssh.Start: %v", err)
	}
	t.Cleanup(sshServer.Close)

	db, err := storage.Open(filepath.Join(t.TempDir(), "monitoring.db"))
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

	cipher, err := secrets.New([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("secrets.New: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	known := storage.NewKnownHostRepository(db)
	cfg := &config.Config{
		SSHDialTimeout:         5 * time.Second,
		SSHHandshakeTimeout:    5 * time.Second,
		SSHOperationTimeout:    5 * time.Second,
		EgressAllowPrivate:     true,
		EgressAllowedPrefixes:  []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")},
		MonitorRefreshInterval: 20 * time.Millisecond,
		MonitorStaleAfter:      300 * time.Millisecond,
		MonitorTimeout:         5 * time.Second,
		MonitorMaxOutputBytes:  256 * 1024,
		MonitorMaxFilesystems:  64,
		MonitorMaxInterfaces:   64,
	}
	serverService := servers.NewService(
		cfg,
		storage.NewServerRepository(db),
		users,
		storage.NewAssignmentRepository(db),
		known,
		storage.NewCapabilitiesRepository(db),
		cipher,
		nil,
		sshclient.NewDialer(cfg, known, logger),
		audit.New(storage.NewAuditRepository(db), logger),
		logger,
	)

	host, portText, _ := net.SplitHostPort(sshServer.Addr)
	port, _ := strconv.Atoi(portText)
	saved, err := serverService.Create(context.Background(), user.ID, servers.CreateInput{
		DisplayName: "Metrics host",
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
		context.Background(), user.ID, saved.ID,
		info.Fingerprint, info.KeyType, info.PublicKey, false,
	); err != nil {
		t.Fatalf("ApproveHostKey: %v", err)
	}

	return &monitoringTestEnv{
		service: NewService(cfg, serverService, logger),
		cfg:     cfg,
		userID:  user.ID,
		server:  saved,
		ssh:     sshServer,
	}
}

func TestSnapshotFirstSampleAndCache(t *testing.T) {
	env := newMonitoringTestEnv(t)
	ctx := context.Background()

	first, err := env.service.Snapshot(ctx, env.userID, env.server.ID)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if first.Cached {
		t.Fatal("the first snapshot must not be marked cached")
	}
	if first.CPU.UsagePercent != nil {
		t.Fatalf("the first CPU sample must not invent usage: %+v", first.CPU.UsagePercent)
	}
	if first.Host.Hostname != "drovik-test" || first.Memory.TotalBytes == 0 || len(first.Filesystems) != 2 {
		t.Fatalf("unexpected snapshot: %+v", first)
	}
	if !first.CPU.Available || !first.Memory.Available || !first.Load.Available || !first.Uptime.Available {
		t.Fatalf("expected all scalar metrics to be available: %+v", first)
	}
	if !first.FilesystemsAvailable || !first.NetworkAvailable {
		t.Fatalf("expected filesystems and network to be available: %+v", first)
	}

	second, err := env.service.Snapshot(ctx, env.userID, env.server.ID)
	if err != nil {
		t.Fatalf("cached Snapshot: %v", err)
	}
	if !second.Cached {
		t.Fatal("a snapshot inside the refresh interval must be marked cached")
	}
	if !second.CollectedAt.Equal(first.CollectedAt) {
		t.Fatalf("expected the cached sample: %s vs %s", second.CollectedAt, first.CollectedAt)
	}
}

func TestSnapshotCoalescesConcurrentRequests(t *testing.T) {
	env := newMonitoringTestEnv(t)
	ctx := context.Background()

	const callers = 8
	results := make(chan *Snapshot, callers)
	errs := make(chan error, callers)
	var wait sync.WaitGroup
	for index := 0; index < callers; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			snapshot, err := env.service.Snapshot(ctx, env.userID, env.server.ID)
			if err != nil {
				errs <- err
				return
			}
			results <- snapshot
		}()
	}
	wait.Wait()
	close(results)
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent Snapshot: %v", err)
	}
	var collectedAt time.Time
	for snapshot := range results {
		if collectedAt.IsZero() {
			collectedAt = snapshot.CollectedAt
			continue
		}
		if !snapshot.CollectedAt.Equal(collectedAt) {
			t.Fatalf("requests were not coalesced: %s vs %s", snapshot.CollectedAt, collectedAt)
		}
	}
}

func TestSnapshotStaleFallback(t *testing.T) {
	env := newMonitoringTestEnv(t)
	ctx := context.Background()

	first, err := env.service.Snapshot(ctx, env.userID, env.server.ID)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	env.ssh.MetricsUnavailable = true
	time.Sleep(40 * time.Millisecond)

	stale, err := env.service.Snapshot(ctx, env.userID, env.server.ID)
	if err != nil {
		t.Fatalf("stale Snapshot: %v", err)
	}
	if !stale.Stale || !stale.Cached || stale.Warning == "" {
		t.Fatalf("expected a stale snapshot with a warning: %+v", stale)
	}
	if !stale.CollectedAt.Equal(first.CollectedAt) {
		t.Fatal("the stale snapshot must be the previous successful sample")
	}

	time.Sleep(350 * time.Millisecond)
	if _, err := env.service.Snapshot(ctx, env.userID, env.server.ID); !errors.Is(err, ErrMetricsUnavailable) {
		t.Fatalf("expected ErrMetricsUnavailable after the stale window, got %v", err)
	}
}

func TestSnapshotUnavailableAndAuthorization(t *testing.T) {
	env := newMonitoringTestEnv(t)
	env.ssh.MetricsUnavailable = true
	if _, err := env.service.Snapshot(context.Background(), env.userID, env.server.ID); !errors.Is(err, ErrMetricsUnavailable) {
		t.Fatalf("expected ErrMetricsUnavailable, got %v", err)
	}
	if _, err := env.service.Snapshot(context.Background(), storage.NewID(), env.server.ID); !errors.Is(err, servers.ErrUnauthorized) {
		t.Fatalf("expected authorization failure, got %v", err)
	}
}

func TestSnapshotTruncatesFilesystemsAndInterfaces(t *testing.T) {
	env := newMonitoringTestEnv(t)
	env.cfg.MonitorMaxFilesystems = 1
	env.cfg.MonitorMaxInterfaces = 1
	snapshot, err := env.service.Snapshot(context.Background(), env.userID, env.server.ID)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(snapshot.Filesystems) != 1 || !snapshot.FilesystemsTruncated {
		t.Fatalf("filesystem truncation failed: %+v", snapshot.Filesystems)
	}
	if len(snapshot.Network) != 1 || !snapshot.NetworkTruncated {
		t.Fatalf("network truncation failed: %+v", snapshot.Network)
	}
}
