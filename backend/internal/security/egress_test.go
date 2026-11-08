package security

import (
	"context"
	"errors"
	"net/netip"
	"testing"
)

type fakeResolver struct {
	addresses []netip.Addr
	err       error
}

func (r fakeResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return r.addresses, r.err
}

func TestEgressPublicAndPrivate(t *testing.T) {
	policy := NewEgressPolicy(true, nil, nil)
	for _, host := range []string{"203.0.113.10", "10.0.0.5", "192.168.1.20", "2001:db8::1"} {
		if _, err := policy.Resolve(context.Background(), host); err != nil {
			t.Fatalf("expected %s to be allowed: %v", host, err)
		}
	}
}

func TestEgressBlocksLoopbackAndMetadata(t *testing.T) {
	policy := NewEgressPolicy(true, nil, nil)
	for _, host := range []string{"127.0.0.1", "::1", "169.254.169.254", "0.0.0.0", "224.0.0.1"} {
		if _, err := policy.Resolve(context.Background(), host); !errors.Is(err, ErrEgressDenied) {
			t.Fatalf("expected %s to be blocked, got %v", host, err)
		}
	}
}

func TestEgressExplicitAllowOverridesDefaultBlock(t *testing.T) {
	policy := NewEgressPolicy(false, []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}, nil)
	if _, err := policy.Resolve(context.Background(), "127.0.0.1"); err != nil {
		t.Fatalf("expected explicit loopback allowance: %v", err)
	}
}

func TestEgressDenyWins(t *testing.T) {
	policy := NewEgressPolicy(true, nil, []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")})
	if _, err := policy.Resolve(context.Background(), "10.1.2.3"); !errors.Is(err, ErrEgressDenied) {
		t.Fatalf("expected denied range, got %v", err)
	}
}

func TestEgressAllowlist(t *testing.T) {
	policy := NewEgressPolicy(true, []netip.Prefix{netip.MustParsePrefix("203.0.113.0/24")}, nil)
	if _, err := policy.Resolve(context.Background(), "203.0.113.5"); err != nil {
		t.Fatalf("expected allowlisted address: %v", err)
	}
	if _, err := policy.Resolve(context.Background(), "198.51.100.5"); !errors.Is(err, ErrEgressDenied) {
		t.Fatalf("expected non-allowlisted address to be blocked, got %v", err)
	}
}

func TestEgressPrivateDisabled(t *testing.T) {
	policy := NewEgressPolicy(false, nil, nil)
	if _, err := policy.Resolve(context.Background(), "10.0.0.5"); !errors.Is(err, ErrEgressDenied) {
		t.Fatalf("expected private address to be blocked, got %v", err)
	}
}

func TestEgressDNS(t *testing.T) {
	policy := NewEgressPolicy(true, nil, nil)
	policy.resolver = fakeResolver{addresses: []netip.Addr{netip.MustParseAddr("203.0.113.7")}}
	addr, err := policy.Resolve(context.Background(), "example.test")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if addr.String() != "203.0.113.7" {
		t.Fatalf("unexpected address: %s", addr)
	}
}

func TestEgressRejectsZonedIPv6Literals(t *testing.T) {
	policy := NewEgressPolicy(true, nil, nil)
	for _, host := range []string{"::1%lo", "fe80::1%lo", "::%lo", "ff02::1%lo"} {
		if _, err := policy.Resolve(context.Background(), host); !errors.Is(err, ErrEgressDenied) {
			t.Fatalf("expected zoned literal %q to be denied, got %v", host, err)
		}
	}
}
