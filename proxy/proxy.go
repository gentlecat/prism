package proxy

import (
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"

	"go.roman.zone/prism/allowlist"
)

func Handler(store *allowlist.Allowlist, targetURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clientIP := extractClientIP(r)
		if clientIP == "" || net.ParseIP(clientIP) == nil {
			log.Printf("Denied: invalid IP %s %s %s", clientIP, r.Method, r.URL.Path)
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		if !store.IsAllowed(clientIP) {
			store.RecordAttempt(clientIP, false)
			log.Printf("Denied: %s %s %s", clientIP, r.Method, r.URL.Path)
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		proxyRequest(w, r, targetURL)
	}
}

func proxyRequest(w http.ResponseWriter, r *http.Request, targetURL string) {
	u, err := url.Parse(targetURL)
	if err != nil {
		log.Printf("Invalid target URL: %v", targetURL)
		http.Error(w, "Error", http.StatusInternalServerError)
		return
	}
	u.Path = r.URL.Path
	u.RawQuery = r.URL.RawQuery

	proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, u.String(), r.Body)
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
		log.Printf("Forwarding error: %v", err)
		http.Error(w, "Error", http.StatusBadGateway)
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
