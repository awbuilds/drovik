// Package testenv assembles a disposable authorized server for integration
// tests. It is test-only support code and is not imported by production paths.
package testenv

import (
	"context"
	"database/sql"
	"encoding/base64"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"path/filepath"
	"strconv"
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

type Env struct {
	Cfg           *config.Config
	DB            *sql.DB
	User          storage.User
	Server        *storage.Server
	ServerService *servers.Service
	Audit         *audit.Service
	Known         *storage.KnownHostRepository
}

// New creates a migrated database, a first admin, and a saved/trusted server
// pointing at the supplied in-process SSH fixture.
func New(dir string, sshServer *testssh.Server) (*Env, error) {
	db, err := storage.Open(filepath.Join(dir, "testenv.db"))
	if err != nil {
		return nil, err
	}
	if err := storage.Migrate(context.Background(), db); err != nil {
		_ = db.Close()
		return nil, err
	}

	now := time.Now().UTC()
	users := storage.NewUserRepository(db)
	user := storage.User{
		ID: storage.NewID(), Username: "admin", PasswordHash: "hash", Role: "admin",
		IsActive: true, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := users.CreateFirstAdmin(context.Background(), user); err != nil {
		_ = db.Close()
		return nil, err
	}

	cipher, err := secrets.New([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	known := storage.NewKnownHostRepository(db)
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
		MonitorRefreshInterval:  10 * time.Millisecond,
		MonitorStaleAfter:       time.Second,
		MonitorTimeout:          5 * time.Second,
		MonitorMaxOutputBytes:   256 * 1024,
		MonitorMaxFilesystems:   64,
		MonitorMaxInterfaces:    64,
		OpsCommandTimeout:       5 * time.Second,
		ProcessMaxEntries:       1000,
		ServiceMaxUnits:         500,
		LogMaxLines:             500,
		LogMaxOutputBytes:       1 << 20,
		AdvancedTimeout:         5 * time.Second,
		AdvancedMaxOutputBytes:  1 << 20,
		DockerMaxContainers:     100,
		PackageMaxEntries:       1000,
		SnippetMaxCommandBytes:  8192,
		MetricsRetention:        time.Hour,
		MetricsMaxSamples:       1000,
	}
	auditService := audit.New(storage.NewAuditRepository(db), logger)
	serverService := servers.NewService(
		cfg,
		storage.NewServerRepository(db),
		users,
		storage.NewAssignmentRepository(db),
		storage.NewTeamRepository(db),
		known,
		storage.NewCapabilitiesRepository(db),
		cipher,
		nil,
		sshclient.NewDialer(cfg, known, logger),
		auditService,
		logger,
	)

	host, portText, err := net.SplitHostPort(sshServer.Addr)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	saved, err := serverService.Create(context.Background(), user.ID, servers.CreateInput{
		DisplayName: "Test host",
		Host:        host,
		Port:        port,
		Username:    "test",
		AuthMethod:  servers.AuthPassword,
		Password:    sshServer.Password,
	})
	if err != nil {
		_ = db.Close()
		return nil, err
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
		_ = db.Close()
		return nil, err
	}

	return &Env{
		Cfg:           cfg,
		DB:            db,
		User:          user,
		Server:        saved,
		ServerService: serverService,
		Audit:         auditService,
		Known:         known,
	}, nil
}

func (e *Env) Close() error {
	if e.DB == nil {
		return nil
	}
	return e.DB.Close()
}
