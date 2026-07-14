package proxy

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"go.roman.zone/prism/allowlist"
)

type clientIPKey struct{}

// Handler builds the proxy handler. The client IP is taken from the
// CF-Connecting-IP, X-Forwarded-For, or X-Real-IP header, falling back
// to the peer address. The headers are trusted unconditionally, so the
// proxy port must only be reachable through a layer that sets them
// (e.g. a Cloudflare Tunnel); anyone with direct access could
// impersonate an allowlisted IP.
func Handler(store *allowlist.Allowlist, targetURL string) (http.HandlerFunc, error) {
	target, err := url.Parse(targetURL)
	if err != nil || target.Scheme == "" || target.Host == "" {
		return nil, fmt.Errorf("invalid TARGET_URL %q", targetURL)
	}

	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.Header.Del("CF-Connecting-IP")
			pr.Out.Header.Del("X-Real-IP")
			if ip, ok := pr.In.Context().Value(clientIPKey{}).(string); ok {
				pr.Out.Header.Set("X-Forwarded-For", ip)
				pr.Out.Header.Set("X-Real-IP", ip)
			}
		},
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			IdleConnTimeout:       90 * time.Second,
			ForceAttemptHTTP2:     true,
		},
		FlushInterval: 100 * time.Millisecond,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("Forwarding error: %v", err)
			http.Error(w, "Error", http.StatusBadGateway)
		},
	}

	return func(w http.ResponseWriter, r *http.Request) {
		clientIP := extractClientIP(r)
		if clientIP == "" {
			log.Printf("Denied: unidentifiable client %s %s %s", r.RemoteAddr, r.Method, r.URL.Path)
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		if !store.IsAllowed(clientIP) {
			store.RecordAttempt(clientIP, false, r.URL.Path)
			log.Printf("Denied: %s %s %s", clientIP, r.Method, r.URL.Path)
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		ctx := context.WithValue(r.Context(), clientIPKey{}, clientIP)
		rp.ServeHTTP(w, r.WithContext(ctx))
	}, nil
}

func extractClientIP(r *http.Request) string {
	if ip, ok := allowlist.CanonicalIP(r.Header.Get("CF-Connecting-IP")); ok {
		return ip
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// The rightmost entry is the one appended by the proxy in
		// front of us; anything left of it is client-supplied.
		parts := strings.Split(xff, ",")
		if ip, ok := allowlist.CanonicalIP(parts[len(parts)-1]); ok {
			return ip
		}
	}
	if ip, ok := allowlist.CanonicalIP(r.Header.Get("X-Real-IP")); ok {
		return ip
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if peer, err := netip.ParseAddr(host); err == nil {
		return peer.Unmap().WithZone("").String()
	}
	return ""
}
