package kafka

import (
	"crypto/sha256"
	"errors"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
)

// clientIdleTimeout is how long a cached client may stay unused before it is closed.
var clientIdleTimeout = 10 * time.Minute

// SetClientIdleGracePeriod sets clientIdleTimeout to
// pollInterval + maxReconcileDuration + gracePeriod and returns it.
func SetClientIdleGracePeriod(gracePeriod, pollInterval time.Duration) time.Duration {
	clientIdleTimeout = pollInterval + maxReconcileDuration + max(gracePeriod, 0)
	return clientIdleTimeout
}

// ClientCache caches one *kadm.Client per set of credentials and closes
// clients unused for longer than clientIdleTimeout.
type ClientCache struct {
	mu      sync.Mutex
	clients map[[sha256.Size]byte]*cachedClient // keyed by SHA-256 of credentials, avoids storing secret material
}

type cachedClient struct {
	client   *kadm.Client
	lastUsed time.Time
}

// GetOrCreate returns the client cached for creds, or creates one with newFn.
// It also closes idle clients.
func (c *ClientCache) GetOrCreate(creds []byte, newFn func() (*kadm.Client, error)) (*kadm.Client, error) {
	client, idle, err := c.getOrCreate(sha256.Sum256(creds), newFn)

	// Close outside the lock: Close waits for the client's goroutines to stop.
	for _, idleClient := range idle {
		idleClient.Close()
	}
	return client, err
}

// getOrCreate does GetOrCreate's work under the lock and returns the removed
// idle clients for the caller to close.
func (c *ClientCache) getOrCreate(digest [sha256.Size]byte, newFn func() (*kadm.Client, error)) (*kadm.Client, []*kadm.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	idle := c.removeIdle(now)

	if cached, ok := c.clients[digest]; ok {
		cached.lastUsed = now
		return cached.client, idle, nil
	}

	client, err := newFn()
	if err != nil {
		return nil, idle, err
	}

	if client == nil {
		return nil, idle, errors.New("newFn returned nil client")
	}

	if c.clients == nil {
		c.clients = make(map[[sha256.Size]byte]*cachedClient)
	}

	c.clients[digest] = &cachedClient{client: client, lastUsed: now}
	return client, idle, nil
}

// removeIdle removes and returns the clients unused for longer than clientIdleTimeout.
func (c *ClientCache) removeIdle(now time.Time) []*kadm.Client {
	var idle []*kadm.Client
	for digest, cached := range c.clients {
		if now.Sub(cached.lastUsed) > clientIdleTimeout {
			idle = append(idle, cached.client)
			delete(c.clients, digest)
		}
	}
	return idle
}
