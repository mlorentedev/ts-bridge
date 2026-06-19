package discover

import (
	"context"
	"sync"
	"time"
)

// Discoverer abstracts the source of tailnet devices.
// Implemented by TailscaleDiscoverer and HeadscaleDiscoverer.
type Discoverer interface {
	// List returns all devices known to the tailnet.
	List(ctx context.Context) ([]Device, error)
}

// Cache wraps a Discoverer and caches results for a configurable TTL.
type Cache struct {
	discoverer Discoverer
	ttl        time.Duration
	mu         sync.RWMutex
	cached     []Device
	expiredAt  time.Time
	fetched    bool
}

// NewCache creates a cache that wraps the given discoverer.
func NewCache(d Discoverer, ttl time.Duration) *Cache {
	return &Cache{
		discoverer: d,
		ttl:        ttl,
	}
}

// List returns cached devices if fresh, otherwise fetches from the discoverer.
func (c *Cache) List(ctx context.Context) ([]Device, error) {
	c.mu.RLock()
	if c.isFresh() {
		result := make([]Device, len(c.cached))
		copy(result, c.cached)
		c.mu.RUnlock()
		return result, nil
	}
	c.mu.RUnlock()

	// Lock for write — another goroutine may also be fetching, but that's
	// acceptable for a discovery command (not a hot path).
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check: another goroutine may have refreshed while we waited.
	if c.isFresh() {
		result := make([]Device, len(c.cached))
		copy(result, c.cached)
		return result, nil
	}

	devices, err := c.discoverer.List(ctx)
	if err != nil {
		return nil, err
	}

	c.cached = devices
	c.fetched = true
	c.expiredAt = time.Now().Add(c.ttl)
	return devices, nil
}

func (c *Cache) isFresh() bool {
	return c.fetched && time.Now().Before(c.expiredAt)
}
