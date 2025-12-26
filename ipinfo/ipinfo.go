package ipinfo

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type IPInfo struct {
	City    string `json:"city"`
	Region  string `json:"region"`
	Country string `json:"country"`
	Org     string `json:"org"`
}

type IPInfoRetriever struct {
	mu      sync.RWMutex
	cache   map[string]*IPInfo
	baseURL string
}

func NewRetriever() *IPInfoRetriever {
	return &IPInfoRetriever{
		cache:   make(map[string]*IPInfo),
		baseURL: "https://ipinfo.io/",
	}
}

func (c *IPInfoRetriever) SetBaseURL(url string) {
	c.baseURL = url
}

func (c *IPInfoRetriever) Lookup(ip string) *IPInfo {
	c.mu.RLock()
	if info, ok := c.cache[ip]; ok {
		c.mu.RUnlock()
		return info
	}
	c.mu.RUnlock()

	return c.fetchAndCache(ip)
}

func (c *IPInfoRetriever) fetchAndCache(ip string) *IPInfo {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(c.baseURL + ip + "/json")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil
	}

	var info IPInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil
	}

	c.mu.Lock()
	c.cache[ip] = &info
	c.mu.Unlock()

	return &info
}
