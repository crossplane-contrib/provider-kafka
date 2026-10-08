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

	client1, _, err := cache.GetOrCreate(creds, newFn)
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))

	client2, _, err := cache.GetOrCreate(creds, newFn)
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

	_, _, err := cache.GetOrCreate(creds, newFn)
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
			_, _, err := cache.GetOrCreate(creds, newFn)
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

	_, _, err := cache.GetOrCreate(emptyCreds, newFn)
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))

	_, _, err = cache.GetOrCreate(emptyCreds, newFn)
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
	_, _, err := cache.GetOrCreate(creds1, newFn)
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))

	// Should reuse client even though it's a different object
	_, _, err = cache.GetOrCreate(creds2, newFn)
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
	client1, _, err := cache.GetOrCreate(creds1, newFn)
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))

	// Try to rotate to creds2, but newFn fails
	failingFn := func() (*kadm.Client, error) {
		return nil, errors.New("connection failed")
	}

	_, _, err = cache.GetOrCreate(creds2, failingFn)
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

	_, _, err := cache.GetOrCreate(creds, nilClientFn)
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

	clientA, _, err := cache.GetOrCreate(credsA, newFn)
	require.NoError(t, err)
	clientB, _, err := cache.GetOrCreate(credsB, newFn)
	require.NoError(t, err)
	assert.NotSame(t, clientA, clientB)

	existingClientA, _, err := cache.GetOrCreate(credsA, newFn)
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

	clientA, _, err := cache.GetOrCreate([]byte("broker-a"), newFn)
	require.NoError(t, err)

	_, _, err = cache.GetOrCreate([]byte("broker-b"), newFn)
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
	// A zero timeout closes every unused client on the next acquire, so only
	// reference counting keeps the clients in use open.
	setClientIdleTimeout(t, 0)

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
				client, release, err := cache.GetOrCreate(creds[(w+r)%len(creds)], newFn)
				if !assert.NoError(t, err) {
					return
				}
				var dialErr *net.OpError
				if !errors.As(listTopics(client), &dialErr) {
					atomic.AddInt32(&failedOnClosedClient, 1)
				}
				release()
			}
		}()
	}

	wg.Wait()
	assert.Zero(t, atomic.LoadInt32(&failedOnClosedClient), "reconciles used clients closed by other reconciles")
}

// TestGetOrCreateNeverClosesClientInUse verifies that a client in use stays open, however long it is held.
func TestGetOrCreateNeverClosesClientInUse(t *testing.T) {
	setClientIdleTimeout(t, 10*time.Minute)

	synctest.Test(t, func(t *testing.T) {
		cache := &ClientCache{}
		newFn := func() (*kadm.Client, error) {
			return newUnconnectedClient(t)
		}

		client, _, err := cache.GetOrCreate([]byte("broker-a"), newFn)
		require.NoError(t, err)

		time.Sleep(2 * ClientIdleTimeout)
		_, _, err = cache.GetOrCreate([]byte("broker-b"), newFn)
		require.NoError(t, err)

		var dialErr *net.OpError
		assert.ErrorAs(t, listTopics(client), &dialErr, "a client in use must stay open")
	})
}

// TestGetOrCreateClosesIdleClient verifies that a released client is closed
// on the next acquire after the idle timeout, and replaced.
func TestGetOrCreateClosesIdleClient(t *testing.T) {
	setClientIdleTimeout(t, 10*time.Minute)

	synctest.Test(t, func(t *testing.T) {
		cache := &ClientCache{}
		var callCount int32
		newFn := func() (*kadm.Client, error) {
			atomic.AddInt32(&callCount, 1)
			return newUnconnectedClient(t)
		}

		idleClient, release, err := cache.GetOrCreate([]byte("broker-a"), newFn)
		require.NoError(t, err)
		release()

		// Past the timeout, the next call closes broker-a's client.
		time.Sleep(ClientIdleTimeout + time.Minute)
		_, _, err = cache.GetOrCreate([]byte("broker-b"), newFn)
		require.NoError(t, err)

		var dialErr *net.OpError
		assert.NotErrorAs(t, listTopics(idleClient), &dialErr, "the idle client should be closed")

		client, _, err := cache.GetOrCreate([]byte("broker-a"), newFn)
		require.NoError(t, err)
		assert.NotSame(t, idleClient, client, "broker-a should get a new client")
		assert.Equal(t, int32(3), atomic.LoadInt32(&callCount))
	})
}

// TestGetOrCreateIdleTimeCountsFromLastRelease verifies that idle time starts at the last release, not at acquire.
func TestGetOrCreateIdleTimeCountsFromLastRelease(t *testing.T) {
	setClientIdleTimeout(t, 10*time.Minute)

	synctest.Test(t, func(t *testing.T) {
		cache := &ClientCache{}
		newFn := func() (*kadm.Client, error) {
			return newUnconnectedClient(t)
		}

		client, release, err := cache.GetOrCreate([]byte("broker-a"), newFn)
		require.NoError(t, err)

		// Held past the timeout, then released.
		time.Sleep(ClientIdleTimeout + time.Minute)
		release()

		// Past the timeout since acquire, but not since release.
		time.Sleep(2 * time.Minute)
		_, _, err = cache.GetOrCreate([]byte("broker-b"), newFn)
		require.NoError(t, err)

		var dialErr *net.OpError
		require.ErrorAs(t, listTopics(client), &dialErr, "a recently released client should stay open")

		got, _, err := cache.GetOrCreate([]byte("broker-a"), newFn)
		require.NoError(t, err)
		assert.Same(t, client, got)
	})
}

// TestGetOrCreateDoubleReleaseKeepsOtherHolder verifies that releasing twice drops only one reference.
func TestGetOrCreateDoubleReleaseKeepsOtherHolder(t *testing.T) {
	setClientIdleTimeout(t, 10*time.Minute)

	synctest.Test(t, func(t *testing.T) {
		cache := &ClientCache{}
		newFn := func() (*kadm.Client, error) {
			return newUnconnectedClient(t)
		}

		client, release, err := cache.GetOrCreate([]byte("broker-a"), newFn)
		require.NoError(t, err)
		_, _, err = cache.GetOrCreate([]byte("broker-a"), newFn) // a second reconcile holds it too
		require.NoError(t, err)

		release()
		release()

		time.Sleep(ClientIdleTimeout + time.Minute)
		_, _, err = cache.GetOrCreate([]byte("broker-b"), newFn)
		require.NoError(t, err)

		var dialErr *net.OpError
		assert.ErrorAs(t, listTopics(client), &dialErr, "the second holder's client must stay open")
	})
}

// TestGetOrCreateUsesConfiguredIdleTimeout verifies that the cache uses ClientIdleTimeout.
func TestGetOrCreateUsesConfiguredIdleTimeout(t *testing.T) {
	setClientIdleTimeout(t, 5*time.Minute)

	synctest.Test(t, func(t *testing.T) {
		cache := &ClientCache{}
		newFn := func() (*kadm.Client, error) {
			return newUnconnectedClient(t)
		}

		idleClient, release, err := cache.GetOrCreate([]byte("broker-a"), newFn)
		require.NoError(t, err)
		release()

		// 6m is past the configured 5m.
		time.Sleep(6 * time.Minute)
		_, _, err = cache.GetOrCreate([]byte("broker-b"), newFn)
		require.NoError(t, err)

		var dialErr *net.OpError
		assert.NotErrorAs(t, listTopics(idleClient), &dialErr, "the idle client should be closed after the configured timeout")
	})
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

// setClientIdleTimeout sets ClientIdleTimeout for the test and restores it afterwards.
func setClientIdleTimeout(t *testing.T, timeout time.Duration) {
	t.Helper()
	previous := ClientIdleTimeout
	ClientIdleTimeout = timeout
	t.Cleanup(func() { ClientIdleTimeout = previous })
}
