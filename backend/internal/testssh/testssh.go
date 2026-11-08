// Package testssh provides a disposable in-process SSH/SFTP server for tests.
package testssh

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"strings"

	gssh "github.com/gliderlabs/ssh"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type Server struct {
	Addr             string
	HostKey          ssh.PublicKey
	Password         string
	ClientSigner     ssh.Signer
	ClientPrivateKey ed25519.PrivateKey

	listener net.Listener
	server   *gssh.Server
}

func Start(workDir string) (*Server, error) {
	hostSigner, _, err := newSigner()
	if err != nil {
		return nil, err
	}
	clientSigner, clientPrivate, err := newSigner()
	if err != nil {
		return nil, err
	}
	clientPublic := clientSigner.PublicKey()

	server := &gssh.Server{
		PasswordHandler: func(_ gssh.Context, password string) bool {
			return password == "test-password"
		},
		PublicKeyHandler: func(_ gssh.Context, key gssh.PublicKey) bool {
			return bytes.Equal(key.Marshal(), clientPublic.Marshal())
		},
		Handler: func(session gssh.Session) {
			command := strings.Join(session.Command(), " ")
			if command == "" {
				interactive(session)
				return
			}
			respond(session, command)
		},
		SubsystemHandlers: map[string]gssh.SubsystemHandler{
			"sftp": func(session gssh.Session) {
				server, err := sftp.NewServer(session, sftp.WithServerWorkingDirectory(workDir))
				if err != nil {
					return
				}
				defer func() { _ = server.Close() }()
				_ = server.Serve()
			},
		},
	}
	server.AddHostKey(hostSigner)

	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen test SSH: %w", err)
	}
	go func() { _ = server.Serve(listener) }()

	return &Server{
		Addr:             listener.Addr().String(),
		HostKey:          hostSigner.PublicKey(),
		Password:         "test-password",
		ClientSigner:     clientSigner,
		ClientPrivateKey: clientPrivate,
		listener:         listener,
		server:           server,
	}, nil
}

func (s *Server) Close() {
	if s.server != nil {
		_ = s.server.Close()
	}
}

func newSigner() (ssh.Signer, ed25519.PrivateKey, error) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		return nil, nil, err
	}
	return signer, privateKey, nil
}

func respond(session gssh.Session, command string) {
	switch {
	case command == "uname -s":
		_, _ = io.WriteString(session, "Linux\n")
	case command == "uname -r":
		_, _ = io.WriteString(session, "6.8.0-drovik-test\n")
	case command == "uname -m":
		_, _ = io.WriteString(session, "x86_64\n")
	case strings.HasPrefix(command, "cat /etc/os-release"):
		_, _ = io.WriteString(session, "PRETTY_NAME=\"Drovik Test Linux\"\nID=drovik\n")
	case strings.Contains(command, "command -v systemctl"):
		_, _ = io.WriteString(session, "yes\n")
	case strings.Contains(command, "command -v journalctl"):
		_, _ = io.WriteString(session, "yes\n")
	case strings.Contains(command, "test -r /proc/stat"):
		_, _ = io.WriteString(session, "yes\n")
	case strings.Contains(command, "command -v ps"):
		_, _ = io.WriteString(session, "yes\n")
	case strings.Contains(command, "command -v df"):
		_, _ = io.WriteString(session, "yes\n")
	default:
		_, _ = io.WriteString(session, "ok\n")
	}
}

func interactive(session gssh.Session) {
	_, _ = io.WriteString(session, "drovik-test$ ")
	buffer := make([]byte, 1024)
	for {
		n, err := session.Read(buffer)
		if n > 0 {
			_, _ = session.Write(buffer[:n])
		}
		if err != nil {
			return
		}
	}
}
