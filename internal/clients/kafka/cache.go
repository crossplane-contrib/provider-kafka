package kafka

import (
	"crypto/sha256"
	"errors"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
)

// closer is a client the cache can close once it has been idle too long.
type closer interface {
	comparable
	Close()
}

// ClientIdleTimeout is how long a cached client may stay unused before it is closed. Set from --client-idle-timeout.
var ClientIdleTimeout time.Duration

// Cache caches one client per set of credentials. Callers release a client
// when done; unused clients are closed once idle past ClientIdleTimeout.
type Cache[T closer] struct {
	mu      sync.Mutex
	clients map[[sha256.Size]byte]*cachedClient[T] // keyed by SHA-256 of credentials, avoids storing secret material
}

// ClientCache caches Kafka admin clients.
type ClientCache = Cache[*kadm.Client]

type cachedClient[T closer] struct {
	client       T
	refs         int       // callers currently using the client
	lastReleased time.Time // when refs last dropped to 0
}

// GetOrCreate returns the client cached for creds, or creates one with newFn,
// and a release func to call when done with it. It also closes idle clients.
func (c *Cache[T]) GetOrCreate(creds []byte, newFn func() (T, error)) (T, func(), error) {
	cached, idle, err := c.acquire(sha256.Sum256(creds), newFn)

	// Close outside the lock: Close waits for the client's goroutines to stop.
	for _, idleClient := range idle {
		idleClient.Close()
	}
	if err != nil {
		var zero T
		return zero, nil, err
	}
	return cached.client, sync.OnceFunc(func() { c.release(cached) }), nil
}

// acquire does GetOrCreate's work under the lock and returns the removed idle
// clients for the caller to close.
func (c *Cache[T]) acquire(digest [sha256.Size]byte, newFn func() (T, error)) (*cachedClient[T], []T, error) {
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

	var zero T
	if client == zero {
		return nil, idle, errors.New("newFn returned nil client")
	}

	if c.clients == nil {
		c.clients = make(map[[sha256.Size]byte]*cachedClient[T])
	}

	cached := &cachedClient[T]{client: client, refs: 1}
	c.clients[digest] = cached
	return cached, idle, nil
}

// release drops one reference to cached.
func (c *Cache[T]) release(cached *cachedClient[T]) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cached.refs--
	if cached.refs == 0 {
		cached.lastReleased = time.Now()
	}
}

// removeIdle removes and returns the unused clients idle for longer than ClientIdleTimeout.
func (c *Cache[T]) removeIdle(now time.Time) []T {
	var idle []T
	for digest, cached := range c.clients {
		if cached.refs == 0 && now.Sub(cached.lastReleased) > ClientIdleTimeout {
			idle = append(idle, cached.client)
			delete(c.clients, digest)
		}
	}
	return idle
}
