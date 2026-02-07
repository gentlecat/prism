package admin

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.roman.zone/prism/allowlist"
	"go.roman.zone/prism/ipinfo"
)

//go:embed templates/*
var templatesFS embed.FS

var tmpl = template.Must(template.New("").Funcs(template.FuncMap{
	"formatTime":     formatTime,
	"formatLocation": formatLocation,
	"formatExpiry":   formatExpiry,
}).ParseFS(templatesFS, "templates/*.html"))

func formatTime(t time.Time) string {
	diff := time.Since(t)
	if diff < time.Minute {
		return "Just now"
	}
	if diff < time.Hour {
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	}
	if diff < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(diff.Hours()/24))
}

func formatLocation(info *ipinfo.IPInfo) string {
	if info == nil {
		return ""
	}
	var parts []string
	if info.City != "" {
		parts = append(parts, info.City)
	}
	if info.Region != "" {
		parts = append(parts, info.Region)
	}
	if info.Country != "" {
		parts = append(parts, info.Country)
	}
	return strings.Join(parts, ", ")
}

func formatExpiry(t time.Time) string {
	diff := time.Until(t)
	if diff <= 0 {
		return "Expired"
	}
	if diff < time.Hour {
		return fmt.Sprintf("Expires in %dm", int(diff.Minutes()))
	}
	if diff < 24*time.Hour {
		return fmt.Sprintf("Expires in %dh", int(diff.Hours()))
	}
	return fmt.Sprintf("Expires in %dd", int(diff.Hours()/24))
}

//go:embed static/*
var staticFiles embed.FS

type Server struct {
	storage *allowlist.Allowlist
}

func New(store *allowlist.Allowlist) (*Server, error) {
	return &Server{storage: store}, nil
}

func (a *Server) HomeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "index.html", nil); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (a *Server) StaticHandler() http.Handler {
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("Failed to setup static files: %v", err)
	}
	return http.StripPrefix("/static/", http.FileServer(http.FS(staticFS)))
}

func (a *Server) PendingListHandler(w http.ResponseWriter, r *http.Request) {
	attempts := a.storage.GetRecentAttempts()
	var pending []allowlist.ConnectionAttempt
	for _, att := range attempts {
		if !att.Allowed {
			pending = append(pending, att)
		}
	}
	sort.Slice(pending, func(i, j int) bool {
		return pending[i].Timestamp.After(pending[j].Timestamp)
	})

	if err := tmpl.ExecuteTemplate(w, "pending_list.html", pending); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (a *Server) AllowedListHandler(w http.ResponseWriter, r *http.Request) {
	allowedIPs := a.storage.GetAllowedIPs()

	attempts := a.storage.GetRecentAttempts()
	infoMap := make(map[string]*ipinfo.IPInfo)
	for _, att := range attempts {
		infoMap[att.IP] = att.Info
	}

	type allowedWithInfo struct {
		IP        string
		ExpiresAt time.Time
		Info      *ipinfo.IPInfo
	}

	var allowed []allowedWithInfo
	for _, ip := range allowedIPs {
		allowed = append(allowed, allowedWithInfo{
			IP:        ip.IP,
			ExpiresAt: ip.ExpiresAt,
			Info:      infoMap[ip.IP],
		})
	}

	if err := tmpl.ExecuteTemplate(w, "allowed_list.html", allowed); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (a *Server) AllowIPHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ip := r.FormValue("ip")
	if ip == "" {
		http.Error(w, "IP address required", http.StatusBadRequest)
		return
	}

	if net.ParseIP(ip) == nil {
		http.Error(w, "Invalid IP", http.StatusBadRequest)
		return
	}

	durationStr := r.FormValue("duration")
	if durationStr == "" {
		http.Error(w, "Duration required", http.StatusBadRequest)
		return
	}

	durationHours, err := strconv.Atoi(durationStr)
	if err != nil || durationHours <= 0 {
		http.Error(w, "Invalid duration", http.StatusBadRequest)
		return
	}

	if err := a.storage.AllowIP(ip, time.Duration(durationHours)*time.Hour); err != nil {
		log.Printf("Allow error: %v", err)
		http.Error(w, "Failed", http.StatusInternalServerError)
		return
	}

	log.Printf("IP allowed: %s for %d hours", ip, durationHours)

	if r.Header.Get("HX-Request") != "" {
		w.Header().Set("HX-Trigger", "refreshList")
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *Server) DenyIPHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ip := r.FormValue("ip")
	if ip == "" {
		http.Error(w, "IP required", http.StatusBadRequest)
		return
	}

	if net.ParseIP(ip) == nil {
		http.Error(w, "Invalid IP", http.StatusBadRequest)
		return
	}

	if err := a.storage.DenyIP(ip); err != nil {
		log.Printf("Failed to deny IP: %v", err)
		http.Error(w, "Failed", http.StatusInternalServerError)
		return
	}

	log.Printf("IP denied: %s", ip)

	if r.Header.Get("HX-Request") != "" {
		w.Header().Set("HX-Trigger", "refreshList")
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
