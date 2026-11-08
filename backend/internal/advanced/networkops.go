package advanced

import (
	"context"
	"fmt"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
)

var interfacePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,14}$`)

type NetworkOpResult = HostOpResult

func validateInterface(name string) error {
	if !interfacePattern.MatchString(name) {
		return fmt.Errorf("%w: invalid network interface", ErrInvalidTarget)
	}
	return nil
}

func validateCIDR(raw string) (string, error) {
	prefix, err := netip.ParsePrefix(strings.TrimSpace(raw))
	if err != nil || prefix.Addr().Zone() != "" {
		return "", fmt.Errorf("%w: invalid CIDR", ErrInvalidTarget)
	}
	return prefix.String(), nil
}

func (s *Service) LinkState(
	ctx context.Context, ownerID, serverID, iface, state string, confirm bool,
) (*NetworkOpResult, error) {
	if err := s.requireDestructive(); err != nil {
		return nil, err
	}
	if !confirm {
		return nil, fmt.Errorf("%w: link confirmation is required", ErrInvalidTarget)
	}
	if err := validateInterface(iface); err != nil {
		return nil, err
	}
	if state != "up" && state != "down" {
		return nil, fmt.Errorf("%w: link state must be up or down", ErrInvalidTarget)
	}
	return s.runDestructive(ctx, ownerID, serverID, "network.link", "ip link set "+iface+" "+state, iface+" "+state)
}

func (s *Service) AddressChange(
	ctx context.Context, ownerID, serverID, iface, cidr, action string, confirm bool,
) (*NetworkOpResult, error) {
	if err := s.requireDestructive(); err != nil {
		return nil, err
	}
	if !confirm {
		return nil, fmt.Errorf("%w: address confirmation is required", ErrInvalidTarget)
	}
	if err := validateInterface(iface); err != nil {
		return nil, err
	}
	normalized, err := validateCIDR(cidr)
	if err != nil {
		return nil, err
	}
	if action != "add" && action != "del" {
		return nil, fmt.Errorf("%w: address action must be add or del", ErrInvalidTarget)
	}
	command := "ip addr " + action + " " + normalized + " dev " + iface
	return s.runDestructive(ctx, ownerID, serverID, "network.address", command, iface+" "+normalized)
}

func (s *Service) RouteChange(
	ctx context.Context, ownerID, serverID, iface, destination, gateway, action string, confirm bool,
) (*NetworkOpResult, error) {
	if err := s.requireDestructive(); err != nil {
		return nil, err
	}
	if !confirm {
		return nil, fmt.Errorf("%w: route confirmation is required", ErrInvalidTarget)
	}
	if err := validateInterface(iface); err != nil {
		return nil, err
	}
	if action != "add" && action != "del" {
		return nil, fmt.Errorf("%w: route action must be add or del", ErrInvalidTarget)
	}
	dest := strings.TrimSpace(destination)
	if dest != "default" {
		normalized, err := validateCIDR(dest)
		if err != nil {
			return nil, err
		}
		dest = netip.MustParsePrefix(normalized).Masked().String()
	}
	gw, err := netip.ParseAddr(strings.TrimSpace(gateway))
	if err != nil || gw.Zone() != "" {
		return nil, fmt.Errorf("%w: invalid gateway", ErrInvalidTarget)
	}
	command := "ip route " + action + " " + dest + " via " + gw.String() + " dev " + iface
	return s.runDestructive(ctx, ownerID, serverID, "network.route", command, dest+" via "+gw.String())
}

func (s *Service) FirewallChange(
	ctx context.Context, ownerID, serverID string, port int, protocol, action string, confirm bool,
) (*NetworkOpResult, error) {
	if err := s.requireDestructive(); err != nil {
		return nil, err
	}
	if !confirm {
		return nil, fmt.Errorf("%w: firewall confirmation is required", ErrInvalidTarget)
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("%w: port must be 1-65535", ErrInvalidTarget)
	}
	if protocol != "tcp" && protocol != "udp" {
		return nil, fmt.Errorf("%w: protocol must be tcp or udp", ErrInvalidTarget)
	}
	if action != "allow" && action != "deny" {
		return nil, fmt.Errorf("%w: firewall action must be allow or deny", ErrInvalidTarget)
	}
	client, _, err := s.servers.Connect(ctx, ownerID, serverID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()

	manager, stderr, err := s.run(ctx, client, `if command -v ufw >/dev/null 2>&1; then printf 'ufw\n'; elif command -v firewall-cmd >/dev/null 2>&1; then printf 'firewalld\n'; else printf 'none\n'; fi`)
	if err != nil {
		return nil, fmt.Errorf("%w: firewall manager detection failed: %s", ErrUnavailable, safeStderr(stderr))
	}
	rule := strconv.Itoa(port) + "/" + protocol
	switch strings.TrimSpace(manager) {
	case "ufw":
		return s.runDestructiveWithClient(ctx, client, ownerID, serverID, "network.firewall", "ufw "+action+" "+rule, rule)
	case "firewalld":
		verb := "--add-port="
		if action == "deny" {
			verb = "--remove-port="
		}
		return s.runDestructiveWithClient(ctx, client, ownerID, serverID, "network.firewall", "firewall-cmd "+verb+rule, rule)
	default:
		return nil, fmt.Errorf("%w: no supported firewall manager detected", ErrUnavailable)
	}
}
