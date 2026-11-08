package sshclient

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"golang.org/x/crypto/ssh"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/awbuilds/drovik/backend/internal/config"
	"github.com/awbuilds/drovik/backend/internal/security"
	"github.com/awbuilds/drovik/backend/internal/storage"
)

var (
	ErrInvalidCredentials = errors.New("invalid SSH credentials")
	ErrInvalidPrivateKey  = errors.New("private key could not be parsed")
)

type Credentials struct {
	AuthMethod string
	Password   string
	PrivateKey []byte
	Passphrase string
}

type HostKeyInfo struct {
	KeyType     string `json:"key_type"`
	Fingerprint string `json:"fingerprint"`
	PublicKey   string `json:"public_key"`
}

type HostKeyUnknownError struct {
	Info HostKeyInfo
}

func (e *HostKeyUnknownError) Error() string {
	return "SSH host key is not trusted yet"
}

type HostKeyChangedError struct {
	Previous  HostKeyInfo
	Presented HostKeyInfo
}

func (e *HostKeyChangedError) Error() string {
	return "SSH host key has changed"
}

type Dialer struct {
	cfg    *config.Config
	egress *security.EgressPolicy
	known  *storage.KnownHostRepository
	logger *slog.Logger
}

func NewDialer(cfg *config.Config, known *storage.KnownHostRepository, logger *slog.Logger) *Dialer {
	return &Dialer{
		cfg:    cfg,
		egress: security.NewEgressPolicy(cfg.EgressAllowPrivate, cfg.EgressAllowedPrefixes, cfg.EgressDeniedPrefixes),
		known:  known,
		logger: logger,
	}
}

// Dial resolves, validates, connects and authenticates to a managed host. The
// returned HostKeyInfo describes the key actually presented by the server.
func (d *Dialer) Dial(ctx context.Context, server *storage.Server, credentials Credentials) (*ssh.Client, HostKeyInfo, error) {
	if server.Port < 1 || server.Port > 65535 {
		return nil, HostKeyInfo{}, fmt.Errorf("%w: invalid port", ErrInvalidCredentials)
	}
	auth, err := authMethods(credentials)
	if err != nil {
		return nil, HostKeyInfo{}, err
	}

	address, err := d.egress.Resolve(ctx, server.Host)
	if err != nil {
		return nil, HostKeyInfo{}, err
	}
	target := net.JoinHostPort(address.String(), strconv.Itoa(server.Port))

	dialer := &net.Dialer{Timeout: d.cfg.SSHDialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		return nil, HostKeyInfo{}, fmt.Errorf("dial SSH target: %w", err)
	}

	var (
		presented   HostKeyInfo
		presentedMu sync.Mutex
	)
	clientConfig := &ssh.ClientConfig{
		User: server.Username,
		Auth: auth,
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			info := hostKeyInfo(key)
			presentedMu.Lock()
			presented = info
			presentedMu.Unlock()

			record, err := d.known.GetActive(ctx, server.ID)
			if err != nil {
				if errors.Is(err, storage.ErrNotFound) {
					return &HostKeyUnknownError{Info: info}
				}
				return fmt.Errorf("load known host: %w", err)
			}
			if record.Fingerprint != info.Fingerprint || record.KeyType != info.KeyType {
				return &HostKeyChangedError{
					Previous: HostKeyInfo{
						KeyType:     record.KeyType,
						Fingerprint: record.Fingerprint,
						PublicKey:   record.PublicKey,
					},
					Presented: info,
				}
			}
			return nil
		},
		Timeout: d.cfg.SSHHandshakeTimeout,
	}

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(d.cfg.SSHHandshakeTimeout))
	}
	sshConn, channels, requests, err := ssh.NewClientConn(conn, net.JoinHostPort(server.Host, strconv.Itoa(server.Port)), clientConfig)
	_ = conn.SetDeadline(time.Time{})
	if err != nil {
		_ = conn.Close()
		presentedMu.Lock()
		info := presented
		presentedMu.Unlock()
		var unknown *HostKeyUnknownError
		if errors.As(err, &unknown) {
			return nil, info, unknown
		}
		var changed *HostKeyChangedError
		if errors.As(err, &changed) {
			return nil, info, changed
		}
		return nil, info, fmt.Errorf("SSH handshake: %w", err)
	}

	return ssh.NewClient(sshConn, channels, requests), presented, nil
}

func authMethods(credentials Credentials) ([]ssh.AuthMethod, error) {
	switch credentials.AuthMethod {
	case "password":
		if credentials.Password == "" {
			return nil, fmt.Errorf("%w: password is empty", ErrInvalidCredentials)
		}
		return []ssh.AuthMethod{ssh.Password(credentials.Password)}, nil
	case "private_key":
		var (
			signer ssh.Signer
			err    error
		)
		if credentials.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(credentials.PrivateKey, []byte(credentials.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(credentials.PrivateKey)
		}
		if err != nil {
			return nil, ErrInvalidPrivateKey
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
	default:
		return nil, fmt.Errorf("%w: unsupported authentication method", ErrInvalidCredentials)
	}
}

func hostKeyInfo(key ssh.PublicKey) HostKeyInfo {
	return HostKeyInfo{
		KeyType:     key.Type(),
		Fingerprint: ssh.FingerprintSHA256(key),
		PublicKey:   base64.StdEncoding.EncodeToString(key.Marshal()),
	}
}

// ParseHostKey reconstructs a public key from the stored base64 representation.
func ParseHostKey(encoded string) (ssh.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode host key: %w", err)
	}
	key, err := ssh.ParsePublicKey(raw)
	if err != nil {
		return nil, fmt.Errorf("parse host key: %w", err)
	}
	return key, nil
}
