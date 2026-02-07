package allowlist

import (
	"errors"
	"net"
	"sync"
	"time"

	"go.roman.zone/prism/ipinfo"
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
	if net.ParseIP(ip) == nil {
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
	if net.ParseIP(ip) == nil {
		return errors.New("invalid IP address")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.allowedIPs[ip] = AllowedIP{
		IP:        ip,
		ExpiresAt: time.Now().Add(duration),
	}

	if attempt, ok := s.attemptsByIP[ip]; ok {
		attempt.Allowed = true
		attempt.Timestamp = time.Now()
		s.attemptsByIP[ip] = attempt
	}

	return nil
}

func (s *Allowlist) DenyIP(ip string) error {
	if net.ParseIP(ip) == nil {
		return errors.New("invalid IP address")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.allowedIPs, ip)
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

func (s *Allowlist) RecordAttempt(ip string, allowed bool) {
	if net.ParseIP(ip) == nil {
		return
	}

	info := s.ipInfoCache.Lookup(ip)

	s.mu.Lock()
	defer s.mu.Unlock()

	attemptNumber := 1
	if existing, ok := s.attemptsByIP[ip]; ok {
		attemptNumber = existing.AttemptNumber + 1
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
