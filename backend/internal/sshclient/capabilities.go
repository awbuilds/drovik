package sshclient

import (
	"context"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type Capabilities struct {
	OS           string    `json:"os"`
	Kernel       string    `json:"kernel"`
	Architecture string    `json:"architecture"`
	Distribution string    `json:"distribution"`
	Systemd      bool      `json:"systemd"`
	Journalctl   bool      `json:"journalctl"`
	Proc         bool      `json:"proc"`
	ProcessTools bool      `json:"process_tools"`
	DiskTools    bool      `json:"disk_tools"`
	SFTP         bool      `json:"sftp"`
	DetectedAt   time.Time `json:"detected_at"`
}

func DetectCapabilities(ctx context.Context, client *ssh.Client) Capabilities {
	capabilities := Capabilities{DetectedAt: time.Now().UTC()}

	capabilities.OS = commandOutput(ctx, client, "uname -s")
	capabilities.Kernel = commandOutput(ctx, client, "uname -r")
	capabilities.Architecture = commandOutput(ctx, client, "uname -m")
	capabilities.Distribution = parseDistribution(commandOutput(ctx, client, "cat /etc/os-release 2>/dev/null"))
	capabilities.Systemd = commandYes(ctx, client, "command -v systemctl >/dev/null 2>&1 && echo yes || echo no")
	capabilities.Journalctl = commandYes(ctx, client, "command -v journalctl >/dev/null 2>&1 && echo yes || echo no")
	capabilities.Proc = commandYes(ctx, client, "test -r /proc/stat && test -r /proc/meminfo && echo yes || echo no")
	capabilities.ProcessTools = commandYes(ctx, client, "command -v ps >/dev/null 2>&1 && echo yes || echo no")
	capabilities.DiskTools = commandYes(ctx, client, "command -v df >/dev/null 2>&1 && echo yes || echo no")

	sftpClient, err := sftp.NewClient(client)
	if err == nil {
		capabilities.SFTP = true
		_ = sftpClient.Close()
	}

	return capabilities
}

func commandOutput(ctx context.Context, client *ssh.Client, command string) string {
	stdout, _, err := RunCommand(ctx, client, command, 16*1024)
	if err != nil {
		return ""
	}
	return stdout
}

func commandYes(ctx context.Context, client *ssh.Client, command string) bool {
	return strings.EqualFold(commandOutput(ctx, client, command), "yes")
}

func parseDistribution(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if value, ok := strings.CutPrefix(line, "PRETTY_NAME="); ok {
			return strings.Trim(strings.TrimSpace(value), `"'`)
		}
	}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if value, ok := strings.CutPrefix(line, "ID="); ok {
			return strings.Trim(strings.TrimSpace(value), `"'`)
		}
	}
	return ""
}
