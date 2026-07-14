package allowlist

import (
	"errors"
	"fmt"
	"log"
	"net/netip"
	"os"
	"strings"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"

	"go.roman.zone/prism/ipinfo"
)

const (
	// Attempts come from attacker-controlled IPs (spoofable via identity
	// headers), so the map is capped: once full, the oldest entry is
	// evicted to make room.
	maxTrackedIPs = 1000

	expirySweepInterval = 10 * time.Minute
)

type AllowedIP struct {
	IP        string
	ExpiresAt time.Time
}

type ConnectionAttempt struct {
	IP            string         `json:"ip"`
	Timestamp     time.Time      `json:"timestamp"`
	Allowed       bool           `json:"allowed"`
	Info          *ipinfo.IPInfo `json:"info,omitempty"`
	AttemptNumber int            `json:"attempt_number"`
}

type Allowlist struct {
	mu                sync.RWMutex
	allowedIPs        map[string]AllowedIP
	attemptsByIP      map[string]ConnectionAttempt
	recentAttempts    []ConnectionAttempt
	maxRecentAttempts int
	ipInfoCache       *ipinfo.IPInfoRetriever
	logger            *log.Logger
}

// CanonicalIP normalizes an IP address so that equivalent notations
// (e.g. "::ffff:1.2.3.4" and "1.2.3.4") map to the same allowlist key.
func CanonicalIP(raw string) (string, bool) {
	addr, err := netip.ParseAddr(strings.TrimSpace(raw))
	if err != nil {
		return "", false
	}
	return addr.Unmap().WithZone("").String(), true
}

func New() *Allowlist {
	var logWriter = log.Writer()
	if logPath := os.Getenv("ACCESS_LOG"); logPath != "" {
		rotating := &lumberjack.Logger{
			Filename:   logPath,
			MaxSize:    100,
			MaxBackups: 3,
			MaxAge:     7,
		}
		// Verify the path is writable now: log.Logger discards write
		// errors, so a bad path would otherwise lose the log silently.
		line := time.Now().Format("2006/01/02 15:04:05") + " ACCESS_LOG opened\n"
		if _, err := rotating.Write([]byte(line)); err != nil {
			log.Printf("Cannot write access log %s: %v; logging to stderr instead", logPath, err)
		} else {
			logWriter = rotating
		}
	}

	logger := log.New(logWriter, "", log.LstdFlags)

	s := &Allowlist{
		allowedIPs:        make(map[string]AllowedIP),
		attemptsByIP:      make(map[string]ConnectionAttempt),
		recentAttempts:    make([]ConnectionAttempt, 0),
		maxRecentAttempts: 30,
		ipInfoCache:       ipinfo.NewRetriever(),
		logger:            logger,
	}

	go s.sweepExpired()

	return s
}

func (s *Allowlist) sweepExpired() {
	for range time.Tick(expirySweepInterval) {
		now := time.Now()
		s.mu.Lock()
		for ip, allowed := range s.allowedIPs {
			if !now.Before(allowed.ExpiresAt) {
				delete(s.allowedIPs, ip)
			}
		}
		s.mu.Unlock()
	}
}

func (s *Allowlist) IsAllowed(ip string) bool {
	ip, ok := CanonicalIP(ip)
	if !ok {
		return false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	allowed, exists := s.allowedIPs[ip]
	if !exists {
		return false
	}
	return time.Now().Before(allowed.ExpiresAt)
}

func (s *Allowlist) AllowIP(ip string, duration time.Duration) error {
	ip, ok := CanonicalIP(ip)
	if !ok {
		return errors.New("invalid IP address")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	expiresAt := time.Now().Add(duration)
	s.allowedIPs[ip] = AllowedIP{
		IP:        ip,
		ExpiresAt: expiresAt,
	}

	if s.logger != nil {
		s.logger.Printf("ALLOWLIST_CHANGED: IP %s added to allowlist, expires at %s", ip, expiresAt.Format(time.RFC3339))
	}

	if attempt, ok := s.attemptsByIP[ip]; ok {
		attempt.Allowed = true
		attempt.Timestamp = time.Now()
		s.attemptsByIP[ip] = attempt
	}

	return nil
}

func (s *Allowlist) DenyIP(ip string) error {
	ip, ok := CanonicalIP(ip)
	if !ok {
		return errors.New("invalid IP address")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.allowedIPs, ip)

	if s.logger != nil {
		s.logger.Printf("ALLOWLIST_CHANGED: IP %s removed from allowlist", ip)
	}

	return nil
}

func (s *Allowlist) GetAllowedIPs() []AllowedIP {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	ips := make([]AllowedIP, 0, len(s.allowedIPs))
	for _, allowed := range s.allowedIPs {
		if now.Before(allowed.ExpiresAt) {
			ips = append(ips, allowed)
		}
	}
	return ips
}

func (s *Allowlist) RecordAttempt(ip string, allowed bool, path string) {
	ip, ok := CanonicalIP(ip)
	if !ok {
		return
	}

	info := s.ipInfoCache.Lookup(ip)

	s.mu.Lock()
	defer s.mu.Unlock()

	attemptNumber := 1
	if existing, ok := s.attemptsByIP[ip]; ok {
		attemptNumber = existing.AttemptNumber + 1
	} else if len(s.attemptsByIP) >= maxTrackedIPs {
		s.evictOldestAttempt()
	}

	attempt := ConnectionAttempt{
		IP:            ip,
		Timestamp:     time.Now(),
		Allowed:       allowed,
		Info:          info,
		AttemptNumber: attemptNumber,
	}

	s.attemptsByIP[ip] = attempt
	s.recentAttempts = append(s.recentAttempts, attempt)

	if len(s.recentAttempts) > s.maxRecentAttempts {
		s.recentAttempts = s.recentAttempts[1:]
	}

	if !allowed && s.logger != nil {
		location := ""
		if info != nil {
			location = fmt.Sprintf(" (City: %s, Region: %s, Country: %s, Org: %s)", info.City, info.Region, info.Country, info.Org)
		}
		s.logger.Printf("UNSUCCESSFUL_ACCESS: IP %s attempt #%d path=%s%s", ip, attemptNumber, path, location)
	}
}

// evictOldestAttempt removes the attempt with the oldest timestamp.
// The caller must hold s.mu.
func (s *Allowlist) evictOldestAttempt() {
	var oldestIP string
	var oldestTime time.Time
	for ip, attempt := range s.attemptsByIP {
		if oldestIP == "" || attempt.Timestamp.Before(oldestTime) {
			oldestIP = ip
			oldestTime = attempt.Timestamp
		}
	}
	if oldestIP != "" {
		delete(s.attemptsByIP, oldestIP)
	}
}

func (s *Allowlist) GetRecentAttempts() []ConnectionAttempt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	attempts := make([]ConnectionAttempt, 0, len(s.attemptsByIP))
	for _, attempt := range s.attemptsByIP {
		attempts = append(attempts, attempt)
	}
	return attempts
}
