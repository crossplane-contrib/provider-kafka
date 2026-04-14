package kafka

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
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
	assert.Nil(t, cache.cachedClient)
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

	for range goroutines {
		go func() {
			defer wg.Done()
			_, err := cache.GetOrCreate(creds, newFn)
			assert.NoError(t, err)
		}()
	}

	wg.Wait()

	// With the lock held during newFn(), only 1 client should be created
	assert.Equal(t, int32(1), atomic.LoadInt32(&creationCount))
	assert.NotNil(t, cache.cachedClient)
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
	originalDigest := cache.credsDigest

	// Try to rotate to creds2, but newFn fails
	failingFn := func() (*kadm.Client, error) {
		return nil, errors.New("connection failed")
	}

	_, err = cache.GetOrCreate(creds2, failingFn)
	require.Error(t, err)

	// Verify cache is unchanged - still has original client and digest
	assert.Same(t, client1, cache.cachedClient)
	assert.Equal(t, originalDigest, cache.credsDigest)
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
	assert.Nil(t, cache.cachedClient)
}
