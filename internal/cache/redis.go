package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/rueidis"
)

// RedisCache implements the Cache interface using rueidis.
// rueidis is chosen over go-redis for lower per-command allocations,
// native RESP3 support, and pipeline-native architecture.
type RedisCache struct {
	client rueidis.Client
	ttl    time.Duration // default TTL; callers may override per key
}

// NewRedis creates a new RedisCache with the given client and default TTL.
func NewRedis(client rueidis.Client, defaultTTL time.Duration) *RedisCache {
	return &RedisCache{client: client, ttl: defaultTTL}
}

// Get retrieves a cached value by key. Returns ErrCacheMiss if the key
// does not exist in Redis.
func (c *RedisCache) Get(ctx context.Context, key string) ([]byte, error) {
	cmd := c.client.B().Get().Key(key).Build()
	val, err := c.client.Do(ctx, cmd).AsBytes()
	if rueidis.IsRedisNil(err) {
		return nil, ErrCacheMiss
	}
	if err != nil {
		return nil, fmt.Errorf("cache get %s: %w", key, err)
	}
	return val, nil
}

// Set stores a value with the given TTL. If ttl is 0, the default TTL
// configured on the RedisCache is used.
func (c *RedisCache) Set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	if ttl == 0 {
		ttl = c.ttl
	}
	cmd := c.client.B().Set().Key(key).Value(rueidis.BinaryString(val)).
		Ex(ttl).Build()
	if err := c.client.Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("cache set %s: %w", key, err)
	}
	return nil
}

// Del removes one or more keys from Redis.
func (c *RedisCache) Del(ctx context.Context, keys ...string) error {
	cmd := c.client.B().Del().Key(keys...).Build()
	if err := c.client.Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("cache del: %w", err)
	}
	return nil
}
