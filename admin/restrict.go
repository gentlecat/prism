package admin

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// ParseNets parses a comma-separated list of CIDRs, such as
// "10.0.0.0/16,100.64.0.0/10". An empty string yields no networks.
func ParseNets(s string) ([]netip.Prefix, error) {
	var nets []netip.Prefix
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(part)
		if err != nil {
			return nil, fmt.Errorf("invalid network %q: %w", part, err)
		}
		nets = append(nets, prefix.Masked())
	}
	return nets, nil
}

// RestrictTo only lets requests through from the given networks, judged by
// the connection's address. Forwarded-IP headers are ignored on purpose: the
// admin interface is reached directly, and anyone could set them. With no
// networks, every request is allowed.
func RestrictTo(nets []netip.Prefix, next http.Handler) http.Handler {
	if len(nets) == 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowed(nets, r.RemoteAddr) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func allowed(nets []netip.Prefix, remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return false
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	for _, prefix := range nets {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}
