package kafka

import (
	"bytes"
	"sync"

	"github.com/twmb/franz-go/pkg/kadm"
)

// ClientCache caches a *kadm.Client keyed by credential bytes.
// If the provided secret changes/rotates, a new client is created.
type ClientCache struct {
	mu           sync.Mutex
	cachedClient *kadm.Client
	cachedCreds  []byte
}

// GetOrCreate returns the cached client if credentials are unchanged,
// otherwise closes the old client and calls newFn to create a new one.
func (c *ClientCache) GetOrCreate(creds []byte, newFn func() (*kadm.Client, error)) (*kadm.Client, error) {
	c.mu.Lock()
	if c.cachedClient != nil && bytes.Equal(creds, c.cachedCreds) {
		svc := c.cachedClient
		c.mu.Unlock()
		return svc, nil
	}
	if c.cachedClient != nil {
		c.cachedClient.Close()
		c.cachedClient = nil
	}
	c.mu.Unlock()

	svc, err := newFn()
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.cachedClient = svc
	c.cachedCreds = creds
	c.mu.Unlock()

	return svc, nil
}
