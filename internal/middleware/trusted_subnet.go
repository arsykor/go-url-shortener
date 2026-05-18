package middleware

import (
	"net"
	"net/http"
	"strings"
)

// RequireTrustedSubnet protects handlers so only requests whose X-Real-IP falls
// inside trusted may proceed. Nil trusted denies every request (empty subnet config).
func RequireTrustedSubnet(trusted *net.IPNet) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if trusted == nil {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
			clientIP, ok := ParseXRealIP(r.Header.Get("X-Real-IP"))
			if !ok || !trusted.Contains(clientIP) {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ParseXRealIP parses the client IP from X-Real-IP (first entry if comma-separated).
func ParseXRealIP(s string) (net.IP, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, false
	}
	if i := strings.IndexByte(s, ','); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	if ip := net.ParseIP(s); ip != nil {
		return ip, true
	}
	host, _, err := net.SplitHostPort(s)
	if err == nil {
		if ip := net.ParseIP(host); ip != nil {
			return ip, true
		}
	}
	return nil, false
}
