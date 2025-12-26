package allowlist

import (
	"sync"
	"time"

	"go.roman.zone/prism/ipinfo"
)

type AllowedIP struct {
	IP        string
	ExpiresAt time.Time
}

type ConnectionAttempt struct {
	IP        string         `json:"ip"`
	Timestamp time.Time      `json:"timestamp"`
	Allowed   bool           `json:"allowed"`
	Info      *ipinfo.IPInfo `json:"info,omitempty"`
}

type Allowlist struct {
	mu                sync.RWMutex
	allowedIPs        map[string]AllowedIP
	attemptsByIP      map[string]ConnectionAttempt
	recentAttempts    []ConnectionAttempt
	maxRecentAttempts int
	ipInfoCache       *ipinfo.IPInfoRetriever
}

func New() *Allowlist {
	return &Allowlist{
		allowedIPs:        make(map[string]AllowedIP),
		attemptsByIP:      make(map[string]ConnectionAttempt),
		recentAttempts:    make([]ConnectionAttempt, 0),
		maxRecentAttempts: 30,
		ipInfoCache:       ipinfo.NewRetriever(),
	}
}

func (s *Allowlist) IsAllowed(ip string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	allowed, exists := s.allowedIPs[ip]
	if !exists {
		return false
	}
	return time.Now().Before(allowed.ExpiresAt)
}

func (s *Allowlist) AllowIP(ip string, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.allowedIPs[ip] = AllowedIP{
		IP:        ip,
		ExpiresAt: time.Now().Add(duration),
	}

	// Update the attempt status to allowed
	if attempt, ok := s.attemptsByIP[ip]; ok {
		attempt.Allowed = true
		s.attemptsByIP[ip] = attempt
	}
}

func (s *Allowlist) DenyIP(ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.allowedIPs, ip)
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

func (s *Allowlist) RecordAttempt(ip string, allowed bool) {
	info := s.ipInfoCache.Lookup(ip)

	s.mu.Lock()
	defer s.mu.Unlock()

	attempt := ConnectionAttempt{
		IP:        ip,
		Timestamp: time.Now(),
		Allowed:   allowed,
		Info:      info,
	}

	s.attemptsByIP[ip] = attempt
	s.recentAttempts = append(s.recentAttempts, attempt)

	if len(s.recentAttempts) > s.maxRecentAttempts {
		s.recentAttempts = s.recentAttempts[1:]
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
