// Package terminal manages authenticated SSH PTY sessions streamed over WebSockets.
package terminal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/websocket"
	"golang.org/x/crypto/ssh"

	"github.com/awbuilds/drovik/backend/internal/config"
	"github.com/awbuilds/drovik/backend/internal/servers"
	"github.com/awbuilds/drovik/backend/internal/sshclient"
)

var (
	ErrLimitReached = errors.New("terminal session limit reached")
	ErrClosed       = errors.New("terminal manager is closed")
)

const (
	clientTypeInput  = "input"
	clientTypeResize = "resize"
	clientTypeClose  = "close"

	serverTypeOutput = "output"
	serverTypeStatus = "status"
	serverTypeError  = "error"

	maxMessageSize = 64 * 1024
	maxOutputChunk = 16 * 1024
	maxInputRate   = 200
)

type clientMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

type serverMessage struct {
	Type    string `json:"type"`
	Data    string `json:"data,omitempty"`
	State   string `json:"state,omitempty"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type Manager struct {
	cfg     *config.Config
	servers *servers.Service
	logger  *slog.Logger

	global chan struct{}

	mu       sync.Mutex
	perUser  map[string]int
	sessions map[*session]struct{}
	closed   bool
}

func NewManager(cfg *config.Config, serverService *servers.Service, logger *slog.Logger) *Manager {
	return &Manager{
		cfg:      cfg,
		servers:  serverService,
		logger:   logger,
		global:   make(chan struct{}, cfg.SSHMaxSessionsGlobal),
		perUser:  make(map[string]int),
		sessions: make(map[*session]struct{}),
	}
}

func (m *Manager) Acquire(userID string) (func(), error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, ErrClosed
	}
	if m.perUser[userID] >= m.cfg.SSHMaxSessionsPerUser {
		return nil, ErrLimitReached
	}
	select {
	case m.global <- struct{}{}:
	default:
		return nil, ErrLimitReached
	}
	m.perUser[userID]++
	var once sync.Once
	release := func() {
		once.Do(func() {
			<-m.global
			m.mu.Lock()
			defer m.mu.Unlock()
			m.perUser[userID]--
			if m.perUser[userID] <= 0 {
				delete(m.perUser, userID)
			}
		})
	}
	return release, nil
}

func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	sessions := make([]*session, 0, len(m.sessions))
	for item := range m.sessions {
		sessions = append(sessions, item)
	}
	m.mu.Unlock()
	for _, item := range sessions {
		item.Close()
	}
}

func (m *Manager) Serve(ctx context.Context, conn *websocket.Conn, serverID, userID string) {
	conn.SetReadLimit(maxMessageSize)

	release, err := m.Acquire(userID)
	if err != nil {
		m.closeWithError(conn, "RESOURCE_LIMIT_REACHED", "Too many active terminal sessions. Close one and try again.")
		return
	}
	defer release()

	client, server, err := m.servers.Connect(ctx, userID, serverID)
	if err != nil {
		code := "SSH_CONNECTION_FAILED"
		message := "The SSH connection could not be established."
		var unknown *sshclient.HostKeyUnknownError
		var changed *sshclient.HostKeyChangedError
		switch {
		case errors.As(err, &unknown):
			code = "SSH_HOST_KEY_UNKNOWN"
			message = "Verify this server's host key on the Servers page before opening a terminal."
		case errors.As(err, &changed):
			code = "SSH_HOST_KEY_CHANGED"
			message = "The server's host key changed. Re-verify it on the Servers page before opening a terminal."
		case errors.Is(err, sshclient.ErrInvalidCredentials), errors.Is(err, sshclient.ErrInvalidPrivateKey):
			code = "SSH_AUTH_FAILED"
			message = "SSH authentication failed."
		}
		m.logger.Warn("terminal connect failed", "error", err, "server_id", serverID, "user_id", userID)
		m.closeWithError(conn, code, message)
		return
	}
	defer func() { _ = client.Close() }()

	sshSession, err := client.NewSession()
	if err != nil {
		m.logger.Warn("terminal session creation failed", "error", err, "server_id", serverID)
		m.closeWithError(conn, "SSH_CHANNEL_FAILURE", "The SSH session could not be created.")
		return
	}
	defer func() { _ = sshSession.Close() }()

	stdin, err := sshSession.StdinPipe()
	if err != nil {
		m.closeWithError(conn, "SSH_CHANNEL_FAILURE", "The SSH stdin channel could not be opened.")
		return
	}
	stdout, err := sshSession.StdoutPipe()
	if err != nil {
		m.closeWithError(conn, "SSH_CHANNEL_FAILURE", "The SSH stdout channel could not be opened.")
		return
	}
	stderr, err := sshSession.StderrPipe()
	if err != nil {
		m.closeWithError(conn, "SSH_CHANNEL_FAILURE", "The SSH stderr channel could not be opened.")
		return
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := sshSession.RequestPty("xterm-256color", 24, 80, modes); err != nil {
		m.closeWithError(conn, "SSH_CHANNEL_FAILURE", "The remote host refused a PTY.")
		return
	}
	if err := sshSession.Shell(); err != nil {
		m.closeWithError(conn, "SSH_CHANNEL_FAILURE", "The remote shell could not be started.")
		return
	}

	item := &session{
		conn:     conn,
		client:   client,
		ssh:      sshSession,
		stdin:    stdin,
		logger:   m.logger,
		serverID: serverID,
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		item.Close()
		m.closeWithError(conn, "SERVICE_UNAVAILABLE", "Drovik is shutting down.")
		return
	}
	m.sessions[item] = struct{}{}
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		delete(m.sessions, item)
		m.mu.Unlock()
	}()

	_ = item.send(ctx, serverMessage{
		Type:  serverTypeStatus,
		State: "connected",
		Data:  server.Username + "@" + server.Host,
	})

	var output sync.WaitGroup
	output.Add(2)
	go func() {
		defer output.Done()
		copyOutput(ctx, item, stdout)
	}()
	go func() {
		defer output.Done()
		copyOutput(ctx, item, stderr)
	}()

	readErr := m.readLoop(ctx, item)
	_ = stdin.Close()
	_ = sshSession.Close()
	_ = client.Close()

	waitDone := make(chan struct{})
	go func() {
		output.Wait()
		close(waitDone)
	}()
	select {
	case <-waitDone:
	case <-time.After(2 * time.Second):
	}

	if readErr != nil && ctx.Err() == nil {
		m.logger.Debug("terminal read loop ended", "error", readErr, "server_id", serverID)
	}
	_ = item.send(context.Background(), serverMessage{Type: serverTypeStatus, State: "closed"})
	_ = conn.Close(websocket.StatusNormalClosure, "terminal closed")
}

func (m *Manager) readLoop(ctx context.Context, item *session) error {
	windowStart := time.Now()
	inputCount := 0
	for {
		typ, data, err := item.conn.Read(ctx)
		if err != nil {
			return err
		}
		if typ != websocket.MessageText {
			continue
		}
		var message clientMessage
		if err := json.Unmarshal(data, &message); err != nil {
			_ = item.send(ctx, serverMessage{Type: serverTypeError, Code: "INVALID_MESSAGE", Message: "Malformed terminal message."})
			continue
		}
		switch message.Type {
		case clientTypeInput:
			if time.Since(windowStart) > time.Second {
				windowStart = time.Now()
				inputCount = 0
			}
			inputCount++
			if inputCount > maxInputRate {
				_ = item.send(ctx, serverMessage{Type: serverTypeError, Code: "RATE_LIMITED", Message: "Terminal input rate exceeded."})
				return nil
			}
			if len(message.Data) > maxMessageSize {
				return fmt.Errorf("terminal input exceeds message limit")
			}
			if _, err := item.stdin.Write([]byte(message.Data)); err != nil {
				return err
			}
		case clientTypeResize:
			cols, rows := message.Cols, message.Rows
			if cols < 1 || cols > 1000 || rows < 1 || rows > 1000 {
				_ = item.send(ctx, serverMessage{Type: serverTypeError, Code: "INVALID_RESIZE", Message: "Terminal size is out of range."})
				continue
			}
			if err := item.ssh.WindowChange(rows, cols); err != nil {
				return err
			}
		case clientTypeClose:
			return nil
		}
	}
}

func copyOutput(ctx context.Context, item *session, reader io.Reader) {
	buffer := make([]byte, maxOutputChunk)
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			if sendErr := item.send(ctx, serverMessage{Type: serverTypeOutput, Data: string(buffer[:n])}); sendErr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}

type session struct {
	conn     *websocket.Conn
	client   *ssh.Client
	ssh      *ssh.Session
	stdin    io.WriteCloser
	logger   *slog.Logger
	serverID string

	writeMu   sync.Mutex
	closeOnce sync.Once
}

func (s *session) send(ctx context.Context, message serverMessage) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	writeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return s.conn.Write(writeCtx, websocket.MessageText, payload)
}

func (s *session) Close() {
	s.closeOnce.Do(func() {
		_ = s.conn.CloseNow()
		_ = s.stdin.Close()
		_ = s.ssh.Close()
		_ = s.client.Close()
	})
}

func (m *Manager) closeWithError(conn *websocket.Conn, code, message string) {
	payload, _ := json.Marshal(serverMessage{Type: serverTypeError, Code: code, Message: message})
	_ = conn.Write(context.Background(), websocket.MessageText, payload)
	_ = conn.Close(websocket.StatusNormalClosure, "terminal closed")
}
