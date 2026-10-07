package kafka

import (
	"crypto/sha256"
	"errors"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
)

// ClientIdleTimeout is how long a cached client may stay unused before it is closed. Set from --client-idle-timeout.
var ClientIdleTimeout time.Duration

// ClientCache caches one *kadm.Client per set of credentials. Callers release
// a client when done; unused clients are closed once idle past ClientIdleTimeout.
type ClientCache struct {
	mu      sync.Mutex
	clients map[[sha256.Size]byte]*cachedClient // keyed by SHA-256 of credentials, avoids storing secret material
}

type cachedClient struct {
	client       *kadm.Client
	refs         int       // callers currently using the client
	lastReleased time.Time // when refs last dropped to 0
}

// GetOrCreate returns the client cached for creds, or creates one with newFn,
// and a release func to call when done with it. It also closes idle clients.
func (c *ClientCache) GetOrCreate(creds []byte, newFn func() (*kadm.Client, error)) (*kadm.Client, func(), error) {
	cached, idle, err := c.acquire(sha256.Sum256(creds), newFn)

	// Close outside the lock: Close waits for the client's goroutines to stop.
	for _, idleClient := range idle {
		idleClient.Close()
	}
	if err != nil {
		return nil, nil, err
	}
	return cached.client, sync.OnceFunc(func() { c.release(cached) }), nil
}

// acquire does GetOrCreate's work under the lock and returns the removed idle
// clients for the caller to close.
func (c *ClientCache) acquire(digest [sha256.Size]byte, newFn func() (*kadm.Client, error)) (*cachedClient, []*kadm.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	idle := c.removeIdle(time.Now())

	if cached, ok := c.clients[digest]; ok {
		cached.refs++
		return cached, idle, nil
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

	cached := &cachedClient{client: client, refs: 1}
	c.clients[digest] = cached
	return cached, idle, nil
}

// release drops one reference to cached.
func (c *ClientCache) release(cached *cachedClient) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cached.refs--
	if cached.refs == 0 {
		cached.lastReleased = time.Now()
	}
}

// removeIdle removes and returns the unused clients idle for longer than ClientIdleTimeout.
func (c *ClientCache) removeIdle(now time.Time) []*kadm.Client {
	var idle []*kadm.Client
	for digest, cached := range c.clients {
		if cached.refs == 0 && now.Sub(cached.lastReleased) > ClientIdleTimeout {
			idle = append(idle, cached.client)
			delete(c.clients, digest)
		}
	}
	return idle
}
