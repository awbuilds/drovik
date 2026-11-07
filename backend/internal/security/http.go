package security

import (
	"crypto/subtle"
	"net"
	"net/http"
	"strings"
)

func IsMutationMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func IsTrustedOrigin(trusted []string, origin string) bool {
	origin = strings.TrimRight(strings.TrimSpace(origin), "/")
	if origin == "" {
		return false
	}
	for _, allowed := range trusted {
		if subtle.ConstantTimeCompare([]byte(origin), []byte(strings.TrimRight(allowed, "/"))) == 1 {
			return true
		}
	}
	return false
}

func SecureEqual(a, b string) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// ClientIP resolves the caller for rate limiting. The backend is expected to be
// bound to localhost/private networking behind a reverse proxy; direct public
// exposure is documented as unsupported. Forwarded headers are therefore trusted
// for client identification but never for authorization.
func ClientIP(r *http.Request) string {
	if value := strings.TrimSpace(r.Header.Get("X-Real-IP")); value != "" {
		return value
	}
	if value := r.Header.Get("X-Forwarded-For"); value != "" {
		first, _, _ := strings.Cut(value, ",")
		if first = strings.TrimSpace(first); first != "" {
			return first
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
