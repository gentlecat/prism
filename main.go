package main

import (
	"log"
	"net/http"
	"os"

	"go.roman.zone/prism/admin"
	"go.roman.zone/prism/allowlist"
	"go.roman.zone/prism/proxy"
)

func main() {
	targetURL := os.Getenv("TARGET_URL")
	if targetURL == "" {
		log.Fatal("TARGET_URL environment variable is required")
	}
	proxyPort := getEnv("PROXY_PORT", "8000")
	adminPort := getEnv("ADMIN_PORT", "8080")

	store := allowlist.New()

	// PROXY
	go func() {
		proxyMux := http.NewServeMux()
		proxyMux.HandleFunc("/", proxy.Handler(store, targetURL))
		log.Printf("Proxy listening on port %s, forwarding to %s", proxyPort, targetURL)
		if err := http.ListenAndServe(":"+proxyPort, proxyMux); err != nil {
			log.Fatalf("Proxy server creation failed: %v", err)
		}
	}()

	// ADMIN INTERFACE
	adminMux := http.NewServeMux()
	adminServer, err := admin.New(store)
	if err != nil {
		log.Fatalf("Failed to initialize admin server: %v", err)
	}
	adminMux.HandleFunc("/", adminServer.HomeHandler)
	adminMux.HandleFunc("/pending", adminServer.PendingListHandler)
	adminMux.HandleFunc("/allowed", adminServer.AllowedListHandler)
	adminMux.HandleFunc("/allow", adminServer.AllowIPHandler)
	adminMux.HandleFunc("/deny", adminServer.DenyIPHandler)
	adminMux.Handle("/static/", adminServer.StaticHandler())

	log.Printf("Admin interface listening on port %s", adminPort)
	if err := http.ListenAndServe(":"+adminPort, adminMux); err != nil {
		log.Fatalf("Admin interface creation failed: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
