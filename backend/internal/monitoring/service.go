package monitoring

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/awbuilds/drovik/backend/internal/config"
	"github.com/awbuilds/drovik/backend/internal/servers"
	"github.com/awbuilds/drovik/backend/internal/sshclient"
)

type cacheEntry struct {
	snapshot    Snapshot
	sample      rawSample
	collectedAt time.Time
}

// Service collects and caches one snapshot per managed host. Concurrent
// requests for the same host are coalesced, and a short minimum interval
// protects the remote host from request amplification.
type Service struct {
	cfg     *config.Config
	servers *servers.Service
	logger  *slog.Logger
	group   singleflight.Group

	mu      sync.Mutex
	entries map[string]cacheEntry
}

func NewService(cfg *config.Config, serverService *servers.Service, logger *slog.Logger) *Service {
	return &Service{
		cfg:     cfg,
		servers: serverService,
		logger:  logger,
		entries: make(map[string]cacheEntry),
	}
}

func (s *Service) Snapshot(ctx context.Context, ownerID, serverID string) (*Snapshot, error) {
	if _, err := s.servers.Get(ctx, ownerID, serverID); err != nil {
		return nil, err
	}

	s.mu.Lock()
	entry, hasEntry := s.entries[serverID]
	s.mu.Unlock()

	if hasEntry && time.Since(entry.collectedAt) < s.cfg.MonitorRefreshInterval {
		return cachedSnapshot(entry.snapshot), nil
	}

	channel := s.group.DoChan(serverID, func() (any, error) {
		return s.refresh(ownerID, serverID)
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-channel:
		if result.Err != nil {
			if hasEntry && time.Since(entry.collectedAt) <= s.cfg.MonitorStaleAfter {
				stale := entry.snapshot
				stale.Cached = true
				stale.Stale = true
				stale.Warning = "Live metrics could not be refreshed; showing the last successful sample."
				return &stale, nil
			}
			return nil, result.Err
		}
		snapshot, ok := result.Val.(Snapshot)
		if !ok {
			return nil, fmt.Errorf("monitoring returned an unexpected result")
		}
		return &snapshot, nil
	}
}

func (s *Service) refresh(ownerID, serverID string) (Snapshot, error) {
	collectCtx, cancel := context.WithTimeout(context.WithoutCancel(context.Background()), s.cfg.MonitorTimeout)
	defer cancel()

	client, _, err := s.servers.Connect(collectCtx, ownerID, serverID)
	if err != nil {
		return Snapshot{}, err
	}
	defer func() { _ = client.Close() }()

	stdout, _, err := sshclient.RunCommand(collectCtx, client, MetricsCommand, s.cfg.MonitorMaxOutputBytes)
	if err != nil {
		return Snapshot{}, fmt.Errorf("collect host metrics: %w", err)
	}

	s.mu.Lock()
	previous, hadPrevious := s.entries[serverID]
	s.mu.Unlock()

	sample, err := parseSample(stdout, time.Now().UTC())
	if err != nil {
		return Snapshot{}, err
	}

	var previousSample *rawSample
	if hadPrevious {
		previousSample = &previous.sample
	}
	snapshot := computeSnapshot(serverID, previousSample, sample)

	if len(snapshot.Filesystems) > s.cfg.MonitorMaxFilesystems {
		snapshot.Filesystems = snapshot.Filesystems[:s.cfg.MonitorMaxFilesystems]
		snapshot.FilesystemsTruncated = true
	}
	if len(snapshot.Network) > s.cfg.MonitorMaxInterfaces {
		snapshot.Network = snapshot.Network[:s.cfg.MonitorMaxInterfaces]
		snapshot.NetworkTruncated = true
	}

	s.mu.Lock()
	s.entries[serverID] = cacheEntry{
		snapshot:    snapshot,
		sample:      sample,
		collectedAt: snapshot.CollectedAt,
	}
	s.mu.Unlock()

	s.logger.Debug("host metrics collected", "server_id", serverID, "filesystems", len(snapshot.Filesystems), "interfaces", len(snapshot.Network))
	return snapshot, nil
}

func cachedSnapshot(snapshot Snapshot) *Snapshot {
	copy := snapshot
	copy.Cached = true
	return &copy
}
