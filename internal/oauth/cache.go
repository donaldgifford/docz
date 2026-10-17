package oauth

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache holds fetched documents between requests: an issuer's JWKS under
// jwks:<sha256(issuer)>, a client's metadata under cimd:<sha256(url)>. Get
// returns ErrCacheMiss for an absent key. *RedisCache and *MemoryCache
// satisfy it.
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	// Take returns the value and deletes the key in one step, so a pending
	// authorization or a code is redeemed exactly once across replicas.
	Take(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}

// ErrCacheMiss is returned by Cache.Get for a key that is absent or expired.
var ErrCacheMiss = errors.New("cache miss")

// RedisCache is the Cache the server uses, so every replica shares one
// fetch of a key set.
type RedisCache struct {
	client *redis.Client
}

var _ Cache = (*RedisCache)(nil)

// NewRedisCache opens a client on redisURL. The client is the cache's own,
// closed by Close, so its lifecycle is independent of the session store's
// and the queue's.
func NewRedisCache(redisURL string) (*RedisCache, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}

	return &RedisCache{client: redis.NewClient(opt)}, nil
}

// Get implements Cache.
func (c *RedisCache) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := c.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrCacheMiss
	}

	return val, err
}

// Set implements Cache.
func (c *RedisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

// Take implements Cache through GETDEL.
func (c *RedisCache) Take(ctx context.Context, key string) ([]byte, error) {
	val, err := c.client.GetDel(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrCacheMiss
	}

	return val, err
}

// Delete implements Cache.
func (c *RedisCache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

// Close releases the client.
func (c *RedisCache) Close() error {
	return c.client.Close()
}

// MemoryCache is an in-process Cache for tests and for a verifier with no
// Redis, such as a contract test's.
type MemoryCache struct {
	mu      sync.Mutex
	entries map[string]memoryEntry
	now     func() time.Time
}

type memoryEntry struct {
	value   []byte
	expires time.Time
}

var _ Cache = (*MemoryCache)(nil)

// NewMemoryCache returns an empty cache on the wall clock.
func NewMemoryCache() *MemoryCache {
	return &MemoryCache{entries: make(map[string]memoryEntry), now: time.Now}
}

// Get implements Cache.
func (c *MemoryCache) Get(_ context.Context, key string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.entries[key]
	if !ok || !e.expires.IsZero() && c.now().After(e.expires) {
		delete(c.entries, key)

		return nil, ErrCacheMiss
	}

	return e.value, nil
}

// Set implements Cache. A zero ttl never expires.
func (c *MemoryCache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var expires time.Time
	if ttl > 0 {
		expires = c.now().Add(ttl)
	}
	c.entries[key] = memoryEntry{value: value, expires: expires}

	return nil
}

// Take implements Cache.
func (c *MemoryCache) Take(ctx context.Context, key string) ([]byte, error) {
	val, err := c.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()

	return val, nil
}

// Delete implements Cache.
func (c *MemoryCache) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()

	return nil
}
