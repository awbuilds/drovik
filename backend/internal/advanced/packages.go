package advanced

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type Package struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

var packageNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9+._:@=-]{0,127}$`)

const (
	PackageInstall = "install"
	PackageUpgrade = "upgrade"
	PackageRemove  = "remove"
)

type PackageActionResult struct {
	Manager   string `json:"manager"`
	Package   string `json:"package"`
	Action    string `json:"action"`
	Output    string `json:"output"`
	Success   bool   `json:"success"`
	Truncated bool   `json:"truncated"`
}

type PackageOverview struct {
	Available   bool      `json:"available"`
	Manager     string    `json:"manager"`
	Packages    []Package `json:"packages"`
	Truncated   bool      `json:"truncated"`
	UpdateCheck bool      `json:"update_check_available"`
	CollectedAt time.Time `json:"collected_at"`
}

const packageCommand = `if command -v dpkg-query >/dev/null 2>&1; then
  printf 'dpkg\n'
  dpkg-query -W -f='${Package}\t${Version}\n' 2>/dev/null
elif command -v rpm >/dev/null 2>&1; then
  printf 'rpm\n'
  rpm -qa --qf '%{NAME}\t%{VERSION}-%{RELEASE}\n' 2>/dev/null
elif command -v apk >/dev/null 2>&1; then
  printf 'apk\n'
  apk info -v 2>/dev/null
elif command -v pacman >/dev/null 2>&1; then
  printf 'pacman\n'
  pacman -Q 2>/dev/null
else
  printf 'none\n'
fi`

const detectManagerCommand = `if command -v apt-get >/dev/null 2>&1; then printf 'apt\n'; elif command -v dnf >/dev/null 2>&1; then printf 'dnf\n'; elif command -v apk >/dev/null 2>&1; then printf 'apk\n'; elif command -v pacman >/dev/null 2>&1; then printf 'pacman\n'; else printf 'none\n'; fi`

func packageActionCommand(manager, action, name string) (string, error) {
	switch manager {
	case "apt":
		switch action {
		case PackageInstall:
			return "apt-get install -y --no-install-recommends " + name, nil
		case PackageUpgrade:
			return "apt-get install -y --only-upgrade " + name, nil
		case PackageRemove:
			return "apt-get remove -y " + name, nil
		}
	case "dnf":
		switch action {
		case PackageInstall:
			return "dnf install -y " + name, nil
		case PackageUpgrade:
			return "dnf upgrade -y " + name, nil
		case PackageRemove:
			return "dnf remove -y " + name, nil
		}
	case "apk":
		switch action {
		case PackageInstall:
			return "apk add " + name, nil
		case PackageUpgrade:
			return "apk upgrade " + name, nil
		case PackageRemove:
			return "apk del " + name, nil
		}
	case "pacman":
		switch action {
		case PackageInstall, PackageUpgrade:
			return "pacman -S --noconfirm " + name, nil
		case PackageRemove:
			return "pacman -R --noconfirm " + name, nil
		}
	}
	return "", fmt.Errorf("%w: unsupported package manager or action", ErrInvalidTarget)
}

// PackageAction runs one enumerated package-manager action as the connected SSH
// user. It never elevates privileges; a non-root account receives the remote
// permission error as normal output.
func (s *Service) PackageAction(
	ctx context.Context,
	ownerID, serverID, name, action string,
	confirm bool,
) (*PackageActionResult, error) {
	if !confirm {
		return nil, fmt.Errorf("%w: package confirmation is required", ErrInvalidTarget)
	}
	switch action {
	case PackageInstall, PackageUpgrade, PackageRemove:
	default:
		return nil, fmt.Errorf("%w: unsupported package action", ErrInvalidTarget)
	}
	if !packageNamePattern.MatchString(name) {
		return nil, fmt.Errorf("%w: invalid package name", ErrInvalidTarget)
	}

	client, _, err := s.servers.Connect(ctx, ownerID, serverID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()

	managerOutput, stderr, err := s.run(ctx, client, detectManagerCommand)
	manager := strings.TrimSpace(managerOutput)
	if err != nil || manager == "" || manager == "none" {
		return nil, fmt.Errorf("%w: no supported package manager detected (%s)", ErrUnavailable, safeStderr(stderr))
	}
	command, err := packageActionCommand(manager, action, name)
	if err != nil {
		return nil, err
	}
	stdout, actionStderr, actionErr := s.run(ctx, client, command)
	output := stdout
	if actionStderr != "" {
		output += actionStderr
	}
	result := &PackageActionResult{
		Manager: manager, Package: name, Action: action,
		Output: output, Success: actionErr == nil,
		Truncated: strings.Contains(output, "[output truncated]"),
	}
	if actionErr != nil {
		if errors.Is(actionErr, context.Canceled) || errors.Is(actionErr, context.DeadlineExceeded) {
			s.record(ctx, ownerID, serverID, "package.action", action+" "+name, "unknown", "OUTCOME_UNKNOWN")
			return result, nil
		}
		s.record(ctx, ownerID, serverID, "package.action", action+" "+name, "failure", "PACKAGE_ACTION_FAILED")
		return result, nil
	}
	s.record(ctx, ownerID, serverID, "package.action", action+" "+name, "success", "")
	return result, nil
}

func (s *Service) Packages(ctx context.Context, ownerID, serverID string) (*PackageOverview, error) {
	client, _, err := s.servers.Connect(ctx, ownerID, serverID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()
	output, stderr, err := s.run(ctx, client, packageCommand)
	if err != nil && output == "" {
		return nil, classify(stderr, err)
	}
	lines := strings.Split(output, "\n")
	manager := ""
	if len(lines) > 0 {
		manager = strings.TrimSpace(lines[0])
	}
	if manager == "" || manager == "none" {
		return nil, fmt.Errorf("%w: no supported package manager detected", ErrUnavailable)
	}
	result := &PackageOverview{
		Available: true, Manager: manager, Packages: make([]Package, 0, 256),
		UpdateCheck: false, CollectedAt: time.Now().UTC(),
	}
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len(result.Packages) >= s.cfg.PackageMaxEntries {
			result.Truncated = true
			break
		}
		pkg := Package{}
		switch manager {
		case "dpkg", "rpm":
			name, version, ok := strings.Cut(line, "\t")
			if !ok {
				continue
			}
			pkg = Package{Name: name, Version: version}
		case "pacman":
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			pkg = Package{Name: fields[0], Version: fields[1]}
		case "apk":
			index := strings.LastIndex(line, "-")
			if index <= 0 {
				continue
			}
			pkg = Package{Name: line[:index], Version: line[index+1:]}
		}
		if pkg.Name != "" {
			result.Packages = append(result.Packages, pkg)
		}
	}
	return result, nil
}
