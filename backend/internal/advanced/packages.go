package advanced

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type Package struct {
	Name    string `json:"name"`
	Version string `json:"version"`
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
