package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	port := getEnv("PORT", "3000")

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/api/status", statusHandler)
	http.HandleFunc("/api/echo", echoHandler)
	http.HandleFunc("/redirect", redirectHandler)

	log.Printf("Test backend listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Test Backend</title>
    <style>
        body { font-family: monospace; max-width: 800px; margin: 50px auto; padding: 20px; background: #f5f5f5; }
        .success { background: #d4edda; color: #155724; padding: 20px; border-radius: 8px; margin-bottom: 20px; }
        .info { background: #fff; padding: 15px; border-radius: 8px; margin-bottom: 10px; border-left: 4px solid #007bff; }
        code { background: #e9ecef; padding: 2px 6px; border-radius: 3px; }
    </style>
</head>
<body>
    <div class="success">
        <h1>✓ Success!</h1>
        <p>Your request reached the backend through Prism proxy.</p>
    </div>
    <div class="info">
        <strong>Client IP:</strong> <code>%s</code><br>
        <strong>Time:</strong> <code>%s</code><br>
        <strong>Path:</strong> <code>%s</code><br>
        <strong>Method:</strong> <code>%s</code>
    </div>
    	<div class="info">
    		<strong>Endpoints:</strong><br>
    		<code>GET /</code> - This page<br>
    		<code>GET /api/status</code> - JSON status<br>
    		<code>POST /api/echo</code> - Echo request body<br>
    		<code>GET /redirect</code> - Test redirect (302 to /)
    	</div>
    </body>
    </html>`

	clientIP := getClientIP(r)
	fmt.Fprintf(w, html, clientIP, time.Now().Format(time.RFC3339), r.URL.Path, r.Method)
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().Format(time.RFC3339),
		"client_ip": getClientIP(r),
		"backend":   "test-backend",
	})
}

func echoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"echo":      body,
		"timestamp": time.Now().Format(time.RFC3339),
		"client_ip": getClientIP(r),
	})
}

func redirectHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/", http.StatusFound)
}

func getClientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	return r.RemoteAddr
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
