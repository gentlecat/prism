package proxy

import (
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"go.roman.zone/prism/allowlist"
)

func Handler(store *allowlist.Allowlist, targetURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clientIP := extractClientIP(r)
		if clientIP == "" {
			log.Printf("Connection denied: unable to determine client IP for request %s %s", r.Method, r.URL.Path)
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		allowed := store.IsAllowed(clientIP)
		if !allowed {
			store.RecordAttempt(clientIP, allowed)
			log.Printf("Connection denied for %s requesting %s %s", clientIP, r.Method, r.URL.Path)
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		proxyRequest(w, r, targetURL)
	}
}

func proxyRequest(w http.ResponseWriter, r *http.Request, targetURL string) {
	target := targetURL + r.URL.Path
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}

	proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, target, r.Body)
	if err != nil {
		log.Printf("Error creating proxy request: %v", err)
		http.Error(w, "Error creating proxy request", http.StatusInternalServerError)
		return
	}

	for key, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(proxyReq)
	if err != nil {
		log.Printf("Error forwarding request: %v", err)
		http.Error(w, "Error forwarding request", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func extractClientIP(r *http.Request) string {
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
