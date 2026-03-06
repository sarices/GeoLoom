package geo

import (
	"sync"
)

// CountryCache 定义 IP 到国家码缓存。
type CountryCache interface {
	Get(ip string) (string, bool)
	Set(ip, countryCode string)
}

// InMemoryCountryCache 是并发安全的内存缓存实现。
type InMemoryCountryCache struct {
	mu    sync.RWMutex
	items map[string]string
}

func NewInMemoryCountryCache() *InMemoryCountryCache {
	return &InMemoryCountryCache{
		items: make(map[string]string),
	}
}

func (c *InMemoryCountryCache) Get(ip string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, ok := c.items[ip]
	return value, ok
}

func (c *InMemoryCountryCache) Set(ip, countryCode string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[ip] = countryCode
}
