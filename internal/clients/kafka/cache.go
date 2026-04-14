package kafka

import (
	"crypto/sha256"
	"sync"

	"github.com/twmb/franz-go/pkg/kadm"
)

// ClientCache caches a *kadm.Client keyed by a digest of credential bytes.
// If the provided secret changes/rotates, a new client is created.
type ClientCache struct {
	mu           sync.Mutex
	cachedClient *kadm.Client
	credsDigest  [sha256.Size]byte // SHA-256 hash of credentials, avoids storing secret material
}

// GetOrCreate returns the cached client if the credential digest is unchanged,
// otherwise closes the old client and calls newFn to create a new one.
func (c *ClientCache) GetOrCreate(creds []byte, newFn func() (*kadm.Client, error)) (*kadm.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	digest := sha256.Sum256(creds)
	if c.cachedClient != nil && digest == c.credsDigest {
		return c.cachedClient, nil
	}

	svc, err := newFn()
	if err != nil {
		return nil, err
	}

	// Only close the old client after successfully creating the new one, ensuring cache
	// consistency even if newFn() fails. Also avoids storing raw secret material.
	if c.cachedClient != nil {
		c.cachedClient.Close()
	}

	c.cachedClient = svc
	c.credsDigest = digest
	return svc, nil
}
