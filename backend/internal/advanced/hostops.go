package advanced

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/crypto/ssh"
)

var (
	ErrFeatureDisabled = errors.New("destructive host operations are disabled")
	devicePattern      = regexp.MustCompile(`^/dev/[A-Za-z0-9]+$`)
	mountpointPattern  = regexp.MustCompile(`^/[A-Za-z0-9/_.-]*$`)
)

var destructiveFilesystems = map[string]string{
	"ext4":  "ext4",
	"xfs":   "xfs",
	"btrfs": "btrfs",
	"vfat":  "vfat",
}

var protectedMountpoints = map[string]bool{
	"/": true, "/boot": true, "/boot/efi": true, "/proc": true, "/sys": true,
	"/dev": true, "/run": true, "/etc": true, "/usr": true, "/var": true,
}

type HostOpResult struct {
	Command   string `json:"command"`
	Output    string `json:"output"`
	Success   bool   `json:"success"`
	Truncated bool   `json:"truncated"`
}

func (s *Service) requireDestructive() error {
	if !s.cfg.EnableDestructiveHostOps {
		return ErrFeatureDisabled
	}
	return nil
}

func validateDevice(device string) error {
	if !devicePattern.MatchString(device) {
		return fmt.Errorf("%w: invalid block device", ErrInvalidTarget)
	}
	return nil
}

func validateMountpoint(mountpoint string) error {
	if !mountpointPattern.MatchString(mountpoint) || strings.Contains(mountpoint, "..") {
		return fmt.Errorf("%w: invalid mountpoint", ErrInvalidTarget)
	}
	if protectedMountpoints[mountpoint] || mountpoint == "" {
		return fmt.Errorf("%w: the mountpoint is protected", ErrInvalidTarget)
	}
	return nil
}

func (s *Service) Mount(
	ctx context.Context, ownerID, serverID, device, mountpoint, filesystem string, confirm bool,
) (*HostOpResult, error) {
	if err := s.requireDestructive(); err != nil {
		return nil, err
	}
	if !confirm {
		return nil, fmt.Errorf("%w: mount confirmation is required", ErrInvalidTarget)
	}
	if err := validateDevice(device); err != nil {
		return nil, err
	}
	if err := validateMountpoint(mountpoint); err != nil {
		return nil, err
	}
	if _, ok := destructiveFilesystems[filesystem]; !ok {
		return nil, fmt.Errorf("%w: unsupported filesystem", ErrInvalidTarget)
	}
	command := "mount -t " + destructiveFilesystems[filesystem] + " " + device + " " + mountpoint
	return s.runDestructive(ctx, ownerID, serverID, "storage.mount", command, mountpoint)
}

func (s *Service) Unmount(
	ctx context.Context, ownerID, serverID, mountpoint string, confirm bool,
) (*HostOpResult, error) {
	if err := s.requireDestructive(); err != nil {
		return nil, err
	}
	if !confirm {
		return nil, fmt.Errorf("%w: unmount confirmation is required", ErrInvalidTarget)
	}
	if err := validateMountpoint(mountpoint); err != nil {
		return nil, err
	}
	return s.runDestructive(ctx, ownerID, serverID, "storage.unmount", "umount "+mountpoint, mountpoint)
}

func (s *Service) Format(
	ctx context.Context, ownerID, serverID, device, filesystem, confirmDevice string, confirm bool,
) (*HostOpResult, error) {
	if err := s.requireDestructive(); err != nil {
		return nil, err
	}
	if !confirm || confirmDevice != device {
		return nil, fmt.Errorf("%w: type the exact device to confirm formatting", ErrInvalidTarget)
	}
	if err := validateDevice(device); err != nil {
		return nil, err
	}
	if _, ok := destructiveFilesystems[filesystem]; !ok {
		return nil, fmt.Errorf("%w: unsupported filesystem", ErrInvalidTarget)
	}
	if err := s.ensureNotMounted(ctx, ownerID, serverID, device); err != nil {
		return nil, err
	}
	var command string
	switch filesystem {
	case "ext4":
		command = "mkfs.ext4 -F " + device
	case "xfs":
		command = "mkfs.xfs -f " + device
	case "btrfs":
		command = "mkfs.btrfs -f " + device
	case "vfat":
		command = "mkfs.vfat -I " + device
	}
	return s.runDestructive(ctx, ownerID, serverID, "storage.format", command, device)
}

func (s *Service) Partition(
	ctx context.Context, ownerID, serverID, device, filesystem, confirmDevice string, confirm bool,
) (*HostOpResult, error) {
	if err := s.requireDestructive(); err != nil {
		return nil, err
	}
	if !confirm || confirmDevice != device {
		return nil, fmt.Errorf("%w: type the exact device to confirm partitioning", ErrInvalidTarget)
	}
	if err := validateDevice(device); err != nil {
		return nil, err
	}
	if _, ok := destructiveFilesystems[filesystem]; !ok {
		return nil, fmt.Errorf("%w: unsupported filesystem", ErrInvalidTarget)
	}
	client, _, err := s.servers.Connect(ctx, ownerID, serverID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()
	kind, _, err := s.run(ctx, client, "lsblk -ndo TYPE "+device)
	if err != nil || strings.TrimSpace(kind) != "disk" {
		return nil, fmt.Errorf("%w: %s is not an unmounted whole disk", ErrInvalidTarget, device)
	}
	if err := s.ensureNotMountedWithClient(ctx, client, device); err != nil {
		return nil, err
	}
	command := "parted -s " + device + " mklabel gpt mkpart primary " + filesystem + " 1MiB 100%"
	return s.runDestructiveWithClient(ctx, client, ownerID, serverID, "storage.partition", command, device)
}

func (s *Service) ensureNotMounted(ctx context.Context, ownerID, serverID, device string) error {
	client, _, err := s.servers.Connect(ctx, ownerID, serverID)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	return s.ensureNotMountedWithClient(ctx, client, device)
}

func (s *Service) ensureNotMountedWithClient(ctx context.Context, client *ssh.Client, device string) error {
	output, stderr, err := s.run(ctx, client, "lsblk -no MOUNTPOINT "+device)
	if err != nil {
		return fmt.Errorf("%w: mount state could not be read: %s", ErrUnavailable, safeStderr(stderr))
	}
	if strings.TrimSpace(output) != "" {
		return fmt.Errorf("%w: %s is mounted at %s", ErrInvalidTarget, device, strings.TrimSpace(output))
	}
	return nil
}

func (s *Service) runDestructive(
	ctx context.Context, ownerID, serverID, eventType, command, target string,
) (*HostOpResult, error) {
	client, _, err := s.servers.Connect(ctx, ownerID, serverID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()
	return s.runDestructiveWithClient(ctx, client, ownerID, serverID, eventType, command, target)
}

func (s *Service) runDestructiveWithClient(
	ctx context.Context, client *ssh.Client, ownerID, serverID, eventType, command, target string,
) (*HostOpResult, error) {
	stdout, stderr, err := s.run(ctx, client, command)
	output := stdout
	if stderr != "" {
		output += stderr
	}
	result := &HostOpResult{
		Command: command, Output: output, Success: err == nil,
		Truncated: strings.Contains(output, "[output truncated]"),
	}
	if err != nil {
		s.record(ctx, ownerID, serverID, eventType, target, "failure", "HOST_OPERATION_FAILED")
		return result, nil
	}
	s.record(ctx, ownerID, serverID, eventType, target, "success", "")
	return result, nil
}
