package kafka

import (
	"context"
	"crypto/sha256"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
)

// TestGetOrCreateCacheHit verifies that cached clients are reused with same credentials.
func TestGetOrCreateCacheHit(t *testing.T) {
	cache := &ClientCache{}
	creds := []byte("secret123")
	var callCount int32

	newFn := func() (*kadm.Client, error) {
		atomic.AddInt32(&callCount, 1)
		return &kadm.Client{}, nil
	}

	client1, err := cache.GetOrCreate(creds, newFn)
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))

	client2, err := cache.GetOrCreate(creds, newFn)
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount)) // Should not call newFn again
	assert.Same(t, client1, client2)
}

// TestGetOrCreateErrorHandling verifies that errors from newFn are propagated.
func TestGetOrCreateErrorHandling(t *testing.T) {
	cache := &ClientCache{}
	creds := []byte("secret")
	testErr := errors.New("creation failed")

	newFn := func() (*kadm.Client, error) {
		return nil, testErr
	}

	_, err := cache.GetOrCreate(creds, newFn)
	require.Error(t, err)
	assert.Equal(t, testErr, err)

	// Cache should be empty after error
	assert.Empty(t, cache.clients)
}

// TestGetOrCreateConcurrentAccess verifies thread-safety with concurrent calls.
func TestGetOrCreateConcurrentAccess(t *testing.T) {
	cache := &ClientCache{}
	creds := []byte("secret")
	var creationCount int32

	newFn := func() (*kadm.Client, error) {
		atomic.AddInt32(&creationCount, 1)
		return &kadm.Client{}, nil
	}

	// Launch multiple goroutines requesting the same credentials
	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_, err := cache.GetOrCreate(creds, newFn)
			assert.NoError(t, err)
		}()
	}

	wg.Wait()

	// With the lock held during newFn(), only 1 client should be created
	assert.Equal(t, int32(1), atomic.LoadInt32(&creationCount))
	assert.Len(t, cache.clients, 1)
}

// TestGetOrCreateEmptyCredentials verifies behavior with empty credential bytes.
func TestGetOrCreateEmptyCredentials(t *testing.T) {
	cache := &ClientCache{}
	emptyCreds := []byte{}
	var callCount int32

	newFn := func() (*kadm.Client, error) {
		atomic.AddInt32(&callCount, 1)
		return &kadm.Client{}, nil
	}

	_, err := cache.GetOrCreate(emptyCreds, newFn)
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))

	_, err = cache.GetOrCreate(emptyCreds, newFn)
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount)) // Should reuse cached client
}

// TestGetOrCreateCredentialComparison verifies that credential comparison is byte-exact.
func TestGetOrCreateCredentialComparison(t *testing.T) {
	cache := &ClientCache{}
	creds1 := []byte("secret")
	creds2 := []byte("secret")
	var callCount int32

	newFn := func() (*kadm.Client, error) {
		atomic.AddInt32(&callCount, 1)
		return &kadm.Client{}, nil
	}

	// Same credentials (different objects, same content)
	_, err := cache.GetOrCreate(creds1, newFn)
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))

	// Should reuse client even though it's a different object
	_, err = cache.GetOrCreate(creds2, newFn)
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))
}

// TestGetOrCreateErrorPreservesCacheState verifies that cache remains unchanged if newFn fails.
func TestGetOrCreateErrorPreservesCacheState(t *testing.T) {
	cache := &ClientCache{}
	creds1 := []byte("secret1")
	creds2 := []byte("secret2")
	var callCount int32

	newFn := func() (*kadm.Client, error) {
		atomic.AddInt32(&callCount, 1)
		return &kadm.Client{}, nil
	}

	// Create initial client with creds1
	client1, err := cache.GetOrCreate(creds1, newFn)
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))

	// Try to rotate to creds2, but newFn fails
	failingFn := func() (*kadm.Client, error) {
		return nil, errors.New("connection failed")
	}

	_, err = cache.GetOrCreate(creds2, failingFn)
	require.Error(t, err)

	// Verify cache is unchanged - still has only the original client
	require.Len(t, cache.clients, 1)
	assert.Same(t, client1, cache.clients[sha256.Sum256(creds1)].client)
}

// TestGetOrCreateNilClientRejected verifies that a nil client is treated as an error.
func TestGetOrCreateNilClientRejected(t *testing.T) {
	cache := &ClientCache{}
	creds := []byte("secret")

	nilClientFn := func() (*kadm.Client, error) {
		return nil, nil
	}

	_, err := cache.GetOrCreate(creds, nilClientFn)
	require.Error(t, err)
	assert.Empty(t, cache.clients)
}

// TestGetOrCreateKeepsClientPerCredentials verifies that each set of credentials keeps its own client.
func TestGetOrCreateKeepsClientPerCredentials(t *testing.T) {
	cache := &ClientCache{}
	credsA := []byte("broker-a")
	credsB := []byte("broker-b")
	var callCount int32

	newFn := func() (*kadm.Client, error) {
		atomic.AddInt32(&callCount, 1)
		return newUnconnectedClient(t)
	}

	clientA, err := cache.GetOrCreate(credsA, newFn)
	require.NoError(t, err)
	clientB, err := cache.GetOrCreate(credsB, newFn)
	require.NoError(t, err)
	assert.NotSame(t, clientA, clientB)

	existingClientA, err := cache.GetOrCreate(credsA, newFn)
	require.NoError(t, err)
	assert.Same(t, clientA, existingClientA, "credentials A should get its existing client back")
	assert.Equal(t, int32(2), atomic.LoadInt32(&callCount), "one client per credentials, not one per switch")
}

// TestGetOrCreateOtherCredentialsDoNotCloseClientInUse reproduces the
// "client closed" bug: getting a client for B must not close A's client.
func TestGetOrCreateOtherCredentialsDoNotCloseClientInUse(t *testing.T) {
	cache := &ClientCache{}
	newFn := func() (*kadm.Client, error) {
		return newUnconnectedClient(t)
	}

	clientA, err := cache.GetOrCreate([]byte("broker-a"), newFn)
	require.NoError(t, err)

	_, err = cache.GetOrCreate([]byte("broker-b"), newFn)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_, err = clientA.ListTopics(ctx)

	// An open client fails with a dial error; a closed one fails without dialing.
	var dialErr *net.OpError
	assert.ErrorAs(t, err, &dialErr, "client A must still reach the network after a reconcile for broker B")
}

// TestGetOrCreateConcurrentReconcilesKeepClientsUsable verifies that parallel
// reconciles for different credentials never get a closed client.
func TestGetOrCreateConcurrentReconcilesKeepClientsUsable(t *testing.T) {
	cache := &ClientCache{}
	creds := [][]byte{[]byte("broker-a"), []byte("broker-b")}
	newFn := func() (*kadm.Client, error) {
		return newUnconnectedClient(t)
	}

	const workers, reconcilesPerWorker = 8, 5
	var wg sync.WaitGroup
	var failedOnClosedClient int32
	wg.Add(workers)

	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for r := 0; r < reconcilesPerWorker; r++ {
				client, err := cache.GetOrCreate(creds[(w+r)%len(creds)], newFn)
				if !assert.NoError(t, err) {
					return
				}
				var dialErr *net.OpError
				if !errors.As(listTopics(client), &dialErr) {
					atomic.AddInt32(&failedOnClosedClient, 1)
				}
			}
		}()
	}

	wg.Wait()
	assert.Zero(t, atomic.LoadInt32(&failedOnClosedClient), "reconciles used clients closed by other reconciles")
}

// TestGetOrCreateClosesIdleClient verifies that an idle client is closed and replaced.
func TestGetOrCreateClosesIdleClient(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cache := &ClientCache{}
		var callCount int32
		newFn := func() (*kadm.Client, error) {
			atomic.AddInt32(&callCount, 1)
			return newUnconnectedClient(t)
		}

		idleClient, err := cache.GetOrCreate([]byte("broker-a"), newFn)
		require.NoError(t, err)

		// Past the timeout, the next call closes broker-a's client.
		time.Sleep(clientIdleTimeout + time.Minute)
		_, err = cache.GetOrCreate([]byte("broker-b"), newFn)
		require.NoError(t, err)

		var dialErr *net.OpError
		assert.NotErrorAs(t, listTopics(idleClient), &dialErr, "the idle client should be closed")

		client, err := cache.GetOrCreate([]byte("broker-a"), newFn)
		require.NoError(t, err)
		assert.NotSame(t, idleClient, client, "broker-a should get a new client")
		assert.Equal(t, int32(3), atomic.LoadInt32(&callCount))
	})
}

// TestGetOrCreateKeepsRecentlyUsedClient verifies that idle time counts from the last use, not from creation.
func TestGetOrCreateKeepsRecentlyUsedClient(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cache := &ClientCache{}
		var callCount int32
		newFn := func() (*kadm.Client, error) {
			atomic.AddInt32(&callCount, 1)
			return newUnconnectedClient(t)
		}

		client, err := cache.GetOrCreate([]byte("broker-a"), newFn)
		require.NoError(t, err)

		// Use broker-a again just before the timeout.
		time.Sleep(clientIdleTimeout - time.Minute)
		_, err = cache.GetOrCreate([]byte("broker-a"), newFn)
		require.NoError(t, err)

		// Past the timeout since creation, but not since last use.
		time.Sleep(2 * time.Minute)
		_, err = cache.GetOrCreate([]byte("broker-b"), newFn)
		require.NoError(t, err)

		var dialErr *net.OpError
		require.ErrorAs(t, listTopics(client), &dialErr, "a recently used client should stay open")

		got, err := cache.GetOrCreate([]byte("broker-a"), newFn)
		require.NoError(t, err)
		assert.Same(t, client, got)
		assert.Equal(t, int32(2), atomic.LoadInt32(&callCount))
	})
}

// TestGetOrCreateUsesConfiguredGracePeriod verifies that the cache uses the timeout from SetClientIdleGracePeriod.
func TestGetOrCreateUsesConfiguredGracePeriod(t *testing.T) {
	restoreClientIdleTimeout(t)
	require.Equal(t, 7*time.Minute+30*time.Second, SetClientIdleGracePeriod(5*time.Minute, time.Minute))

	synctest.Test(t, func(t *testing.T) {
		cache := &ClientCache{}
		newFn := func() (*kadm.Client, error) {
			return newUnconnectedClient(t)
		}

		idleClient, err := cache.GetOrCreate([]byte("broker-a"), newFn)
		require.NoError(t, err)

		// 8m is past 1m + 90s + 5m, but within the default.
		time.Sleep(8 * time.Minute)
		_, err = cache.GetOrCreate([]byte("broker-b"), newFn)
		require.NoError(t, err)

		var dialErr *net.OpError
		assert.NotErrorAs(t, listTopics(idleClient), &dialErr, "the idle client should be closed after the configured grace period")
	})
}

// TestGetOrCreateKeepsClientThroughPollAndReconcile verifies that with a zero
// grace period a client still lives for the poll interval plus 90s.
func TestGetOrCreateKeepsClientThroughPollAndReconcile(t *testing.T) {
	restoreClientIdleTimeout(t)
	SetClientIdleGracePeriod(0, time.Minute)

	synctest.Test(t, func(t *testing.T) {
		cache := &ClientCache{}
		newFn := func() (*kadm.Client, error) {
			return newUnconnectedClient(t)
		}

		client, err := cache.GetOrCreate([]byte("broker-a"), newFn)
		require.NoError(t, err)

		// 2m: within 1m + 90s.
		time.Sleep(2 * time.Minute)
		_, err = cache.GetOrCreate([]byte("broker-b"), newFn)
		require.NoError(t, err)

		var dialErr *net.OpError
		require.ErrorAs(t, listTopics(client), &dialErr, "the client should survive the poll interval and a reconcile")

		// 3m: past 1m + 90s.
		time.Sleep(time.Minute)
		_, err = cache.GetOrCreate([]byte("broker-b"), newFn)
		require.NoError(t, err)

		assert.NotErrorAs(t, listTopics(client), &dialErr, "the client should be closed once past both")
	})
}

// TestSetClientIdleGracePeriod verifies that the timeout is pollInterval + 90s + gracePeriod.
func TestSetClientIdleGracePeriod(t *testing.T) {
	restoreClientIdleTimeout(t)

	cases := map[string]struct {
		gracePeriod  time.Duration
		pollInterval time.Duration
		want         time.Duration
	}{
		"ShortPollInterval": {
			gracePeriod:  15 * time.Minute,
			pollInterval: time.Minute,
			want:         17*time.Minute + 30*time.Second,
		},
		"LongPollInterval": {
			gracePeriod:  15 * time.Minute,
			pollInterval: time.Hour,
			want:         time.Hour + 16*time.Minute + 30*time.Second,
		},
		"ZeroGracePeriod": {
			gracePeriod:  0,
			pollInterval: time.Minute,
			want:         150 * time.Second,
		},
		"NegativeGracePeriod": {
			gracePeriod:  -5 * time.Minute,
			pollInterval: time.Minute,
			want:         150 * time.Second,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, SetClientIdleGracePeriod(tc.gracePeriod, tc.pollInterval))
		})
	}
}

// listTopics makes the request Topic's Observe makes. An open client fails with
// a dial error (*net.OpError); a closed one fails without dialing.
func listTopics(client *kadm.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_, err := client.ListTopics(ctx)
	return err
}

// newUnconnectedClient returns a real admin client for a closed port. Unlike a
// zero-value kadm.Client, it can be closed.
func newUnconnectedClient(t *testing.T) (*kadm.Client, error) {
	kc, err := kgo.NewClient(kgo.SeedBrokers("127.0.0.1:1"))
	if err != nil {
		return nil, err
	}
	t.Cleanup(kc.Close) // closing twice is safe if the cache already closed it
	return kadm.NewClient(kc), nil
}

// restoreClientIdleTimeout restores clientIdleTimeout after the test.
func restoreClientIdleTimeout(t *testing.T) {
	t.Helper()
	previous := clientIdleTimeout
	t.Cleanup(func() { clientIdleTimeout = previous })
}
