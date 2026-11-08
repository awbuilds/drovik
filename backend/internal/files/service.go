// Package files provides bounded SFTP file operations for managed servers.
package files

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/awbuilds/drovik/backend/internal/audit"
	"github.com/awbuilds/drovik/backend/internal/config"
	"github.com/awbuilds/drovik/backend/internal/servers"
	"github.com/awbuilds/drovik/backend/internal/storage"
)

var (
	ErrPathInvalid     = errors.New("remote path is invalid")
	ErrSymlink         = errors.New("symlink operations are not allowed for this action")
	ErrIsDirectory     = errors.New("path is a directory")
	ErrTooLarge        = errors.New("file is too large")
	ErrBinary          = errors.New("file is binary or not valid UTF-8")
	ErrStale           = errors.New("remote file changed since it was opened")
	ErrConflict        = errors.New("remote path already exists")
	ErrSFTPUnavailable = errors.New("SFTP subsystem is unavailable")
	ErrPermission      = errors.New("remote permission denied")
	ErrNotFound        = errors.New("remote path not found")
)

type Entry struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Type       string    `json:"type"`
	Size       int64     `json:"size"`
	Mode       string    `json:"mode"`
	ModifiedAt time.Time `json:"modified_at"`
	UID        uint32    `json:"uid,omitempty"`
	GID        uint32    `json:"gid,omitempty"`
}

type ListResult struct {
	Path      string  `json:"path"`
	Entries   []Entry `json:"entries"`
	Truncated bool    `json:"truncated"`
}

type ExpectedVersion struct {
	ModifiedAt *time.Time
	Size       *int64
}

type Content struct {
	Path       string    `json:"path"`
	Content    string    `json:"content"`
	Size       int64     `json:"size"`
	Mode       string    `json:"mode"`
	ModifiedAt time.Time `json:"modified_at"`
}

type Download struct {
	Reader     io.ReadCloser
	Name       string
	Size       int64
	ModifiedAt time.Time
	close      func()
}

func (d *Download) Close() {
	if d.close != nil {
		d.close()
	}
}

type Service struct {
	cfg     *config.Config
	servers *servers.Service
	audit   *audit.Service
	logger  *slog.Logger
}

func NewService(cfg *config.Config, serverService *servers.Service, auditService *audit.Service, logger *slog.Logger) *Service {
	return &Service{cfg: cfg, servers: serverService, audit: auditService, logger: logger}
}

func (s *Service) List(ctx context.Context, ownerID, serverID, rawPath string) (*ListResult, error) {
	var result *ListResult
	err := s.withClient(ctx, ownerID, serverID, func(client *sftp.Client) error {
		directory, err := resolvePath(client, rawPath)
		if err != nil {
			return err
		}
		entries, err := client.ReadDir(directory)
		if err != nil {
			return mapSFTPError(err)
		}
		sort.SliceStable(entries, func(i, j int) bool {
			if entries[i].IsDir() != entries[j].IsDir() {
				return entries[i].IsDir()
			}
			return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
		})
		result = &ListResult{Path: directory, Entries: make([]Entry, 0, min(len(entries), s.cfg.SFTPMaxListEntries))}
		for _, info := range entries {
			if len(result.Entries) >= s.cfg.SFTPMaxListEntries {
				result.Truncated = true
				break
			}
			result.Entries = append(result.Entries, toEntry(directory, info))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) Stat(ctx context.Context, ownerID, serverID, rawPath string) (*Entry, error) {
	var entry *Entry
	err := s.withClient(ctx, ownerID, serverID, func(client *sftp.Client) error {
		clean, err := resolveRequiredPath(rawPath)
		if err != nil {
			return err
		}
		info, err := client.Lstat(clean)
		if err != nil {
			return mapSFTPError(err)
		}
		item := toEntry(path.Dir(clean), info)
		item.Path = clean
		entry = &item
		return nil
	})
	return entry, err
}

func (s *Service) ReadFile(ctx context.Context, ownerID, serverID, rawPath string) (*Content, error) {
	var content *Content
	err := s.withClient(ctx, ownerID, serverID, func(client *sftp.Client) error {
		clean, err := resolveRequiredPath(rawPath)
		if err != nil {
			return err
		}
		info, err := client.Lstat(clean)
		if err != nil {
			return mapSFTPError(err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return ErrSymlink
		}
		if info.IsDir() {
			return ErrIsDirectory
		}
		if info.Size() > s.cfg.SFTPMaxEditorBytes {
			return ErrTooLarge
		}
		file, err := client.Open(clean)
		if err != nil {
			return mapSFTPError(err)
		}
		defer func() { _ = file.Close() }()
		data, err := io.ReadAll(io.LimitReader(file, s.cfg.SFTPMaxEditorBytes+1))
		if err != nil {
			return fmt.Errorf("read remote file: %w", err)
		}
		if int64(len(data)) > s.cfg.SFTPMaxEditorBytes {
			return ErrTooLarge
		}
		if bytes.ContainsRune(data, 0) || !utf8.Valid(data) {
			return ErrBinary
		}
		content = &Content{
			Path:       clean,
			Content:    string(data),
			Size:       info.Size(),
			Mode:       info.Mode().String(),
			ModifiedAt: info.ModTime().UTC(),
		}
		return nil
	})
	return content, err
}

func (s *Service) WriteFile(ctx context.Context, ownerID, serverID, rawPath, text string, expected *ExpectedVersion) error {
	if int64(len(text)) > s.cfg.SFTPMaxEditorBytes {
		return ErrTooLarge
	}
	if !utf8.ValidString(text) {
		return ErrBinary
	}
	return s.withClient(ctx, ownerID, serverID, func(client *sftp.Client) error {
		clean, err := resolveRequiredPath(rawPath)
		if err != nil {
			return err
		}
		if clean == "/" {
			return ErrPathInvalid
		}
		var mode os.FileMode = 0o644
		if info, err := client.Lstat(clean); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return ErrSymlink
			}
			if expected != nil {
				if expected.ModifiedAt != nil && !info.ModTime().UTC().Equal(expected.ModifiedAt.UTC()) {
					return ErrStale
				}
				if expected.Size != nil && info.Size() != *expected.Size {
					return ErrStale
				}
			}
			mode = info.Mode().Perm()
		} else if !errors.Is(err, os.ErrNotExist) {
			return mapSFTPError(err)
		}

		temp, err := s.tempPath(clean)
		if err != nil {
			return err
		}
		file, err := client.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
		if err != nil {
			return mapSFTPError(err)
		}
		// Keep the empty staging file private while content is being written,
		// then apply the final mode before the atomic rename.
		if err := client.Chmod(temp, 0o600); err != nil {
			_ = file.Close()
			_ = client.Remove(temp)
			return fmt.Errorf("secure staging file: %w", err)
		}
		if _, err := file.Write([]byte(text)); err != nil {
			_ = file.Close()
			_ = client.Remove(temp)
			return fmt.Errorf("write remote file: %w", err)
		}
		if err := file.Close(); err != nil {
			_ = client.Remove(temp)
			return fmt.Errorf("close remote file: %w", err)
		}
		if err := client.Chmod(temp, mode); err != nil {
			_ = client.Remove(temp)
			return fmt.Errorf("preserve file mode: %w", err)
		}
		if err := replaceFile(client, temp, clean); err != nil {
			_ = client.Remove(temp)
			return err
		}
		s.record(ctx, ownerID, serverID, "file.write", clean, "success", "")
		return nil
	})
}

func (s *Service) Mkdir(ctx context.Context, ownerID, serverID, rawPath string) error {
	return s.withClient(ctx, ownerID, serverID, func(client *sftp.Client) error {
		clean, err := resolveRequiredPath(rawPath)
		if err != nil || clean == "/" {
			return ErrPathInvalid
		}
		if err := client.Mkdir(clean); err != nil {
			return mapSFTPError(err)
		}
		s.record(ctx, ownerID, serverID, "file.mkdir", clean, "success", "")
		return nil
	})
}

func (s *Service) Touch(ctx context.Context, ownerID, serverID, rawPath string) error {
	return s.withClient(ctx, ownerID, serverID, func(client *sftp.Client) error {
		clean, err := resolveRequiredPath(rawPath)
		if err != nil || clean == "/" {
			return ErrPathInvalid
		}
		if _, err := client.Lstat(clean); err == nil {
			return ErrConflict
		} else if !errors.Is(err, os.ErrNotExist) {
			return mapSFTPError(err)
		}
		file, err := client.OpenFile(clean, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
		if err != nil {
			return mapSFTPError(err)
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("close new remote file: %w", err)
		}
		s.record(ctx, ownerID, serverID, "file.create", clean, "success", "")
		return nil
	})
}

func (s *Service) Rename(ctx context.Context, ownerID, serverID, fromRaw, toRaw string, overwrite bool) error {
	return s.withClient(ctx, ownerID, serverID, func(client *sftp.Client) error {
		from, err := resolveRequiredPath(fromRaw)
		if err != nil {
			return err
		}
		to, err := resolveRequiredPath(toRaw)
		if err != nil {
			return err
		}
		if from == "/" || to == "/" {
			return ErrPathInvalid
		}
		if !overwrite {
			if _, err := client.Lstat(to); err == nil {
				return ErrConflict
			} else if !errors.Is(err, os.ErrNotExist) {
				return mapSFTPError(err)
			}
		}
		if overwrite {
			if err := replaceFile(client, from, to); err != nil {
				return err
			}
		} else if err := client.Rename(from, to); err != nil {
			return mapSFTPError(err)
		}
		s.record(ctx, ownerID, serverID, "file.rename", from+" -> "+to, "success", "")
		return nil
	})
}

func (s *Service) Delete(ctx context.Context, ownerID, serverID, rawPath string, recursive bool) error {
	return s.withClient(ctx, ownerID, serverID, func(client *sftp.Client) error {
		clean, err := resolveRequiredPath(rawPath)
		if err != nil || clean == "/" {
			return ErrPathInvalid
		}
		info, err := client.Lstat(clean)
		if err != nil {
			return mapSFTPError(err)
		}
		if info.IsDir() {
			if recursive {
				count := 0
				if err := s.deleteTree(client, clean, 0, &count); err != nil {
					return err
				}
			}
			if err := client.RemoveDirectory(clean); err != nil {
				return mapSFTPError(err)
			}
		} else {
			if err := client.Remove(clean); err != nil {
				return mapSFTPError(err)
			}
		}
		s.record(ctx, ownerID, serverID, "file.delete", clean, "success", "")
		return nil
	})
}

func (s *Service) OpenDownload(ctx context.Context, ownerID, serverID, rawPath string) (*Download, error) {
	sshClient, _, err := s.servers.Connect(ctx, ownerID, serverID)
	if err != nil {
		return nil, err
	}
	client, err := s.openSFTP(ctx, sshClient)
	if err != nil {
		_ = sshClient.Close()
		return nil, err
	}
	clean, err := resolveRequiredPath(rawPath)
	if err != nil {
		_ = client.Close()
		_ = sshClient.Close()
		return nil, err
	}
	info, err := client.Lstat(clean)
	if err != nil {
		_ = client.Close()
		_ = sshClient.Close()
		return nil, mapSFTPError(err)
	}
	if info.IsDir() {
		_ = client.Close()
		_ = sshClient.Close()
		return nil, ErrIsDirectory
	}
	if info.Mode()&os.ModeSymlink != 0 {
		_ = client.Close()
		_ = sshClient.Close()
		return nil, ErrSymlink
	}
	file, err := client.Open(clean)
	if err != nil {
		_ = client.Close()
		_ = sshClient.Close()
		return nil, mapSFTPError(err)
	}
	return &Download{
		Reader:     file,
		Name:       path.Base(clean),
		Size:       info.Size(),
		ModifiedAt: info.ModTime().UTC(),
		close: func() {
			_ = file.Close()
			_ = client.Close()
			_ = sshClient.Close()
		},
	}, nil
}

func (s *Service) Upload(ctx context.Context, ownerID, serverID, rawPath string, reader io.Reader, size int64, overwrite bool) error {
	if size > s.cfg.SFTPMaxUploadBytes {
		return ErrTooLarge
	}
	return s.withClient(ctx, ownerID, serverID, func(client *sftp.Client) error {
		clean, err := resolveRequiredPath(rawPath)
		if err != nil || clean == "/" {
			return ErrPathInvalid
		}
		mode := os.FileMode(0o644)
		if info, err := client.Lstat(clean); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return ErrSymlink
			}
			if !overwrite {
				return ErrConflict
			}
			mode = info.Mode().Perm()
		} else if !errors.Is(err, os.ErrNotExist) {
			return mapSFTPError(err)
		}
		temp, err := s.tempPath(clean)
		if err != nil {
			return err
		}
		file, err := client.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
		if err != nil {
			return mapSFTPError(err)
		}
		if err := client.Chmod(temp, 0o600); err != nil {
			_ = file.Close()
			_ = client.Remove(temp)
			return fmt.Errorf("secure staging file: %w", err)
		}
		written, copyErr := io.Copy(file, io.LimitReader(&contextReader{ctx: ctx, reader: reader}, s.cfg.SFTPMaxUploadBytes+1))
		closeErr := file.Close()
		if copyErr != nil {
			_ = client.Remove(temp)
			return fmt.Errorf("upload remote file: %w", copyErr)
		}
		if closeErr != nil {
			_ = client.Remove(temp)
			return fmt.Errorf("close uploaded file: %w", closeErr)
		}
		if written > s.cfg.SFTPMaxUploadBytes {
			_ = client.Remove(temp)
			return ErrTooLarge
		}
		if err := client.Chmod(temp, mode); err != nil {
			_ = client.Remove(temp)
			return fmt.Errorf("preserve file mode: %w", err)
		}
		if err := replaceFile(client, temp, clean); err != nil {
			_ = client.Remove(temp)
			return err
		}
		s.record(ctx, ownerID, serverID, "file.upload", clean, "success", "")
		return nil
	})
}

func (s *Service) deleteTree(client *sftp.Client, directory string, depth int, count *int) error {
	if depth > 64 {
		return fmt.Errorf("%w: directory nesting is too deep", ErrTooLarge)
	}
	entries, err := client.ReadDir(directory)
	if err != nil {
		return mapSFTPError(err)
	}
	for _, info := range entries {
		*count++
		if *count > s.cfg.SFTPMaxRecursiveEntries {
			return fmt.Errorf("%w: too many entries in recursive delete", ErrTooLarge)
		}
		child := path.Join(directory, info.Name())
		if info.IsDir() {
			if err := s.deleteTree(client, child, depth+1, count); err != nil {
				return err
			}
			if err := client.RemoveDirectory(child); err != nil {
				return mapSFTPError(err)
			}
			continue
		}
		if err := client.Remove(child); err != nil {
			return mapSFTPError(err)
		}
	}
	return nil
}

func (s *Service) withClient(ctx context.Context, ownerID, serverID string, fn func(*sftp.Client) error) error {
	sshClient, _, err := s.servers.Connect(ctx, ownerID, serverID)
	if err != nil {
		return err
	}
	defer func() { _ = sshClient.Close() }()

	client, err := s.openSFTP(ctx, sshClient)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = sshClient.Close()
		case <-done:
		}
	}()
	defer close(done)

	return fn(client)
}

func (s *Service) openSFTP(_ context.Context, sshClient *ssh.Client) (*sftp.Client, error) {
	client, err := sftp.NewClient(sshClient)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSFTPUnavailable, err)
	}
	return client, nil
}

func (s *Service) tempPath(target string) (string, error) {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate temp name: %w", err)
	}
	return path.Join(path.Dir(target), ".drovik-tmp-"+hex.EncodeToString(raw)), nil
}

func (s *Service) record(ctx context.Context, actorID, serverID, eventType, target, outcome, code string) {
	var safeCode *string
	if code != "" {
		safeCode = &code
	}
	s.audit.Record(ctx, storage.AuditEvent{
		ActorUserID:   &actorID,
		ServerID:      &serverID,
		EventType:     eventType,
		TargetSummary: target,
		Outcome:       outcome,
		SafeErrorCode: safeCode,
		CreatedAt:     time.Now().UTC(),
	})
}

func resolvePath(client *sftp.Client, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		wd, err := client.Getwd()
		if err != nil {
			return "", mapSFTPError(err)
		}
		return wd, nil
	}
	return resolveRequiredPath(raw)
}

func resolveRequiredPath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsAny(raw, "\x00\r\n") || !strings.HasPrefix(raw, "/") {
		return "", ErrPathInvalid
	}
	clean := path.Clean(raw)
	if len(clean) > 4096 {
		return "", ErrPathInvalid
	}
	return clean, nil
}

func toEntry(directory string, info os.FileInfo) Entry {
	entry := Entry{
		Name:       info.Name(),
		Path:       path.Join(directory, info.Name()),
		Size:       info.Size(),
		Mode:       info.Mode().String(),
		ModifiedAt: info.ModTime().UTC(),
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		entry.Type = "symlink"
	case info.IsDir():
		entry.Type = "directory"
	case info.Mode().IsRegular():
		entry.Type = "file"
	default:
		entry.Type = "other"
	}
	if stat, ok := info.Sys().(*sftp.FileStat); ok {
		entry.UID = stat.UID
		entry.GID = stat.GID
	}
	return entry
}

// replaceFile performs an atomic overwrite. OpenSSH's standard SFTP rename
// refuses to replace an existing target; the posix-rename extension is the
// correct primitive for editor saves and uploads. Fail closed when the server
// does not expose it rather than deleting the target first.
func replaceFile(client *sftp.Client, oldPath, newPath string) error {
	if err := client.PosixRename(oldPath, newPath); err != nil {
		return fmt.Errorf("atomic replace failed: %w", mapSFTPError(err))
	}
	return nil
}

func mapSFTPError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	}
	if errors.Is(err, os.ErrPermission) {
		return ErrPermission
	}
	return err
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(data []byte) (int, error) {
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
		return r.reader.Read(data)
	}
}
