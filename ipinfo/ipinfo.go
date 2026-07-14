package ipinfo

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

const (
	// Looked-up IPs are attacker-controlled, so lookups are bounded:
	// the cache has a size cap, failures are cached to avoid refetch
	// loops, and outbound requests go through a token bucket so a flood
	// of unique IPs cannot exhaust the ipinfo.io quota.
	maxCacheEntries  = 10000
	failureRetryWait = time.Hour
	rateBurst        = 10
	rateRefillEvery  = 6 * time.Second
)

type IPInfo struct {
	City    string `json:"city"`
	Region  string `json:"region"`
	Country string `json:"country"`
	Org     string `json:"org"`
}

type cacheEntry struct {
	info      *IPInfo // nil means the lookup failed
	fetchedAt time.Time
}

type IPInfoRetriever struct {
	mu      sync.Mutex
	cache   map[string]cacheEntry
	baseURL string

	tokens     float64
	lastRefill time.Time
}

func NewRetriever() *IPInfoRetriever {
	return &IPInfoRetriever{
		cache:      make(map[string]cacheEntry),
		baseURL:    "https://ipinfo.io/",
		tokens:     rateBurst,
		lastRefill: time.Now(),
	}
}

func (c *IPInfoRetriever) SetBaseURL(url string) {
	c.baseURL = url
}

func (c *IPInfoRetriever) Lookup(ip string) *IPInfo {
	c.mu.Lock()
	if entry, ok := c.cache[ip]; ok {
		if entry.info != nil || time.Since(entry.fetchedAt) < failureRetryWait {
			c.mu.Unlock()
			return entry.info
		}
	}
	if !c.takeToken() {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	return c.fetchAndCache(ip)
}

// takeToken implements a token bucket; the caller must hold c.mu.
func (c *IPInfoRetriever) takeToken() bool {
	now := time.Now()
	c.tokens += now.Sub(c.lastRefill).Seconds() / rateRefillEvery.Seconds()
	if c.tokens > rateBurst {
		c.tokens = rateBurst
	}
	c.lastRefill = now
	if c.tokens < 1 {
		return false
	}
	c.tokens--
	return true
}

func (c *IPInfoRetriever) fetchAndCache(ip string) *IPInfo {
	client := &http.Client{Timeout: 5 * time.Second}

	var info *IPInfo
	resp, err := client.Get(c.baseURL + ip + "/json")
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == 200 {
			var decoded IPInfo
			if err := json.NewDecoder(resp.Body).Decode(&decoded); err == nil {
				info = &decoded
			}
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.cache) >= maxCacheEntries {
		for k := range c.cache {
			delete(c.cache, k)
			break
		}
	}
	c.cache[ip] = cacheEntry{info: info, fetchedAt: time.Now()}

	return info
}
