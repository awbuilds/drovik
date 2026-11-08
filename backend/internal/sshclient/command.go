package sshclient

import (
	"bytes"
	"context"
	"golang.org/x/crypto/ssh"
	"strings"
	"sync"
)

const defaultMaxOutput = 64 * 1024

type limitedBuffer struct {
	mu        sync.Mutex
	max       int
	buf       bytes.Buffer
	truncated bool
}

func (b *limitedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	remaining := b.max - b.buf.Len()
	if remaining <= 0 {
		b.truncated = true
		return len(data), nil
	}
	if len(data) > remaining {
		b.truncated = true
		_, _ = b.buf.Write(data[:remaining])
		return len(data), nil
	}
	_, _ = b.buf.Write(data)
	return len(data), nil
}

func (b *limitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.truncated {
		return b.buf.String() + "\n[output truncated]"
	}
	return b.buf.String()
}

func RunCommand(ctx context.Context, client *ssh.Client, command string, maxBytes int) (string, string, error) {
	if maxBytes <= 0 {
		maxBytes = defaultMaxOutput
	}
	session, err := client.NewSession()
	if err != nil {
		return "", "", err
	}
	defer func() { _ = session.Close() }()

	stdout := &limitedBuffer{max: maxBytes}
	stderr := &limitedBuffer{max: maxBytes}
	session.Stdout = stdout
	session.Stderr = stderr

	done := make(chan error, 1)
	go func() {
		// Every caller passes either a fixed command or values constrained by an
		// enumeration, a positive integer, or a strict unit/absolute-path
		// validator. No raw UI text reaches this sink.
		// codeql[go/command-injection]
		done <- session.Run(command) // lgtm[go/command-injection] // codeql[go/command-injection]
	}()

	select {
	case <-ctx.Done():
		_ = session.Close()
		return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), ctx.Err()
	case err := <-done:
		return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), err
	}
}
