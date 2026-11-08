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

	// MetricsFixture, when set, replaces the default monitoring output. Set it
	// before the first connection; the handler reads it on each command.
	MetricsFixture string
	// MetricsUnavailable omits the /proc/stat section to simulate a host
	// without usable /proc metrics.
	MetricsUnavailable bool
	// CommandHook runs before the built-in responses. Return handled=true to
	// send stdout/stderr and exit with the given status.
	CommandHook func(command string) (stdout, stderr string, exitCode int, handled bool)

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

	result := &Server{}
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
			respond(session, command, result)
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

	result.Addr = listener.Addr().String()
	result.HostKey = hostSigner.PublicKey()
	result.Password = "test-password"
	result.ClientSigner = clientSigner
	result.ClientPrivateKey = clientPrivate
	result.listener = listener
	result.server = server
	return result, nil
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

func respond(session gssh.Session, command string, server *Server) {
	if server.CommandHook != nil {
		if stdout, stderr, exitCode, handled := server.CommandHook(command); handled {
			_, _ = io.WriteString(session, stdout)
			if stderr != "" {
				_, _ = io.WriteString(session.Stderr(), stderr)
			}
			if exitCode != 0 {
				_ = session.Exit(exitCode)
			}
			return
		}
	}
	switch {
	case strings.Contains(command, "@@stat@@"):
		if server.MetricsUnavailable {
			_, _ = io.WriteString(session, "@@os-release@@\nPRETTY_NAME=\"Minimal Linux\"\n@@stat@@\n@@end@@\n")
			return
		}
		if server.MetricsFixture != "" {
			_, _ = io.WriteString(session, server.MetricsFixture)
			return
		}
		_, _ = io.WriteString(session, DefaultMetricsFixture)
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

// DefaultMetricsFixture is a deterministic two-core Linux host with one data
// disk and two network interfaces. Timestamps are not part of the output.
const DefaultMetricsFixture = `@@os-release@@
PRETTY_NAME="Drovik Test Linux"
NAME="Drovik Test Linux"
ID=drovik
VERSION_ID="1"
@@uname@@
Linux 6.8.0-drovik-test x86_64
@@hostname@@
drovik-test
@@uptime@@
123456.78 987654.32
@@loadavg@@
0.42 0.31 0.25 2/128 4242
@@stat@@
cpu  1000 20 300 8000 100 10 5 0 0 0
cpu0 500 10 150 4000 50 5 2 0 0 0
cpu1 500 10 150 4000 50 5 3 0 0 0
intr 123456 0 0 0
ctxt 654321
btime 1700000000
processes 1234
procs_running 2
procs_blocked 0
@@meminfo@@
MemTotal:        2048000 kB
MemFree:          512000 kB
MemAvailable:    1536000 kB
Buffers:           32000 kB
Cached:           640000 kB
SwapTotal:        524288 kB
SwapFree:         262144 kB
@@netdev@@
Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo:    2048      32    0    0    0     0          0         0     2048      32    0    0    0     0       0          0
  eth0: 104857600  512000    4    2    0     0          0         0  52428800  256000    1    0    0     0       0          0
@@cpuinfo@@
processor	: 0
model name	: Drovik Test CPU @ 2.40GHz
architecture	: x86_64
processor	: 1
model name	: Drovik Test CPU @ 2.40GHz
architecture	: x86_64
@@mounts@@
/dev/sda1 / ext4 rw,relatime 0 0
/dev/sdb1 /srv/data xfs rw,noatime 0 0
tmpfs /run tmpfs rw,nosuid,nodev 0 0
proc /proc proc rw,nosuid,nodev,noexec,relatime 0 0
@@df@@
Filesystem     1024-blocks      Used Available Capacity Mounted on
/dev/sda1         10240000   4096000   6144000      40% /
/dev/sdb1         20480000   2048000  18432000      10% /srv/data
tmpfs               204800      1024    203776       1% /run
@@end@@
`
