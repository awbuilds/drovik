package security

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"time"
)

var ErrEgressDenied = errors.New("destination is not allowed by the egress policy")

var defaultDeniedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("ff00::/8"),
	netip.MustParsePrefix("::/128"),
}

type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

type EgressPolicy struct {
	allowPrivate bool
	allowed      []netip.Prefix
	denied       []netip.Prefix
	resolver     Resolver
}

func NewEgressPolicy(allowPrivate bool, allowed, denied []netip.Prefix) *EgressPolicy {
	return &EgressPolicy{
		allowPrivate: allowPrivate,
		allowed:      append([]netip.Prefix(nil), allowed...),
		denied:       append([]netip.Prefix(nil), denied...),
		resolver:     net.DefaultResolver,
	}
}

// Resolve validates a hostname or IP literal and returns the address the
// connection must use. It resolves DNS once so the validated address is the one
// dialed, reducing DNS rebinding and TOCTOU risk.
func (p *EgressPolicy) Resolve(ctx context.Context, host string) (netip.Addr, error) {
	if host == "" {
		return netip.Addr{}, fmt.Errorf("%w: empty host", ErrEgressDenied)
	}
	// IPv6 zone identifiers change how the address is scoped and are not part
	// of an SSH host identity. Reject them instead of allowing a zone to evade
	// prefix and address-class checks.
	if strings.ContainsRune(host, '%') {
		return netip.Addr{}, fmt.Errorf("%w: zone identifiers are not allowed", ErrEgressDenied)
	}

	if addr, err := netip.ParseAddr(host); err == nil {
		if err := p.check(addr.Unmap()); err != nil {
			return netip.Addr{}, err
		}
		return addr.Unmap(), nil
	}

	lookupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	addresses, err := p.resolver.LookupNetIP(lookupCtx, "ip", host)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("resolve %s: %w", host, err)
	}
	if len(addresses) == 0 {
		return netip.Addr{}, fmt.Errorf("resolve %s: no addresses", host)
	}

	var lastErr error
	for _, address := range addresses {
		address = address.Unmap()
		if err := p.check(address); err != nil {
			lastErr = err
			continue
		}
		return address, nil
	}
	if lastErr != nil {
		return netip.Addr{}, lastErr
	}
	return netip.Addr{}, ErrEgressDenied
}

func (p *EgressPolicy) check(addr netip.Addr) error {
	if !addr.IsValid() {
		return fmt.Errorf("%w: invalid address", ErrEgressDenied)
	}

	for _, prefix := range p.denied {
		if prefix.Contains(addr) {
			return fmt.Errorf("%w: %s is in an explicitly denied range", ErrEgressDenied, addr)
		}
	}

	explicitlyAllowed := false
	for _, prefix := range p.allowed {
		if prefix.Contains(addr) {
			explicitlyAllowed = true
			break
		}
	}

	if explicitlyAllowed {
		return nil
	}

	if addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsMulticast() || addr.IsUnspecified() {
		return fmt.Errorf("%w: %s is a blocked address class", ErrEgressDenied, addr)
	}

	for _, prefix := range defaultDeniedPrefixes {
		if prefix.Contains(addr) {
			return fmt.Errorf("%w: %s is a blocked address class", ErrEgressDenied, addr)
		}
	}

	if addr.IsPrivate() && !p.allowPrivate {
		return fmt.Errorf("%w: private addresses are disabled", ErrEgressDenied)
	}

	if len(p.allowed) > 0 {
		return fmt.Errorf("%w: %s is not in the allowed ranges", ErrEgressDenied, addr)
	}

	return nil
}
