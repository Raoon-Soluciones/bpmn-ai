package ai

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
)

type CacheEntry struct {
	Response  Response
	CreatedAt time.Time
	TTL       time.Duration
}

func (e *CacheEntry) IsExpired() bool {
	if e.TTL <= 0 {
		return false
	}
	return time.Since(e.CreatedAt) > e.TTL
}

type Cache interface {
	Get(ctx context.Context, key string) (*CacheEntry, error)
	Set(ctx context.Context, key string, entry *CacheEntry) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
}

type NoopCache struct{}

func NewNoopCache() *NoopCache {
	return &NoopCache{}
}

func (c *NoopCache) Get(_ context.Context, _ string) (*CacheEntry, error) {
	return nil, nil
}

func (c *NoopCache) Set(_ context.Context, _ string, _ *CacheEntry) error {
	return nil
}

func (c *NoopCache) Delete(_ context.Context, _ string) error {
	return nil
}

func (c *NoopCache) Clear(_ context.Context) error {
	return nil
}

type MemoryCache struct {
	mu    map[string]*CacheEntry
	ttl   time.Duration
}

func NewMemoryCache(ttl time.Duration) *MemoryCache {
	return &MemoryCache{
		mu:  make(map[string]*CacheEntry),
		ttl: ttl,
	}
}

func (c *MemoryCache) Get(_ context.Context, key string) (*CacheEntry, error) {
	entry, ok := c.mu[key]
	if !ok {
		return nil, nil
	}
	if entry.IsExpired() {
		delete(c.mu, key)
		return nil, nil
	}
	return entry, nil
}

func (c *MemoryCache) Set(_ context.Context, key string, entry *CacheEntry) error {
	if entry.TTL <= 0 {
		entry.TTL = c.ttl
	}
	entry.CreatedAt = time.Now()
	c.mu[key] = entry
	return nil
}

func (c *MemoryCache) Delete(_ context.Context, key string) error {
	delete(c.mu, key)
	return nil
}

func (c *MemoryCache) Clear(_ context.Context) error {
	c.mu = make(map[string]*CacheEntry)
	return nil
}

func CacheKey(model, system string, messages []Message) string {
	h := sha256.New()
	h.Write([]byte(model))
	h.Write([]byte("\x00"))
	h.Write([]byte(system))
	h.Write([]byte("\x00"))
	for _, m := range messages {
		h.Write([]byte(string(m.Role)))
		h.Write([]byte(m.Content))
		h.Write([]byte("\x00"))
	}
	return fmt.Sprintf("ai:%x", h.Sum(nil))
}

type CachedGateway struct {
	inner Gateway
	cache Cache
	ttl   time.Duration
}

func NewCachedGateway(inner Gateway, cache Cache, ttl time.Duration) *CachedGateway {
	return &CachedGateway{
		inner: inner,
		cache: cache,
		ttl:   ttl,
	}
}

func (g *CachedGateway) Generate(ctx context.Context, req Request) (Response, error) {
	if g.cache == nil {
		return g.inner.Generate(ctx, req)
	}

	key := CacheKey(req.Model, req.System, req.Messages)

	entry, err := g.cache.Get(ctx, key)
	if err == nil && entry != nil {
		resp := entry.Response
		return resp, nil
	}

	resp, err := g.inner.Generate(ctx, req)
	if err != nil {
		return resp, err
	}

	cacheErr := g.cache.Set(ctx, key, &CacheEntry{
		Response:  resp,
		CreatedAt: time.Now(),
		TTL:       g.ttl,
	})
	if cacheErr != nil {
		_ = cacheErr
	}

	return resp, nil
}

type CacheConfig struct {
	Enabled bool
	TTL     time.Duration
	Type    string // "memory" or "redis"
	RedisURL string
}

type redisCache struct {
	client interface {
		Get(ctx context.Context, key string) (string, error)
		Set(ctx context.Context, key string, value string, ttl time.Duration) error
		Del(ctx context.Context, key string) error
	}
	ttl time.Duration
}

func (c *redisCache) Get(ctx context.Context, key string) (*CacheEntry, error) {
	data, err := c.client.Get(ctx, key)
	if err != nil {
		return nil, nil
	}
	var entry CacheEntry
	if err := json.Unmarshal([]byte(data), &entry); err != nil {
		return nil, nil
	}
	return &entry, nil
}

func (c *redisCache) Set(ctx context.Context, key string, entry *CacheEntry) error {
	if entry.TTL <= 0 {
		entry.TTL = c.ttl
	}
	entry.CreatedAt = time.Now()
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, string(data), entry.TTL)
}

func (c *redisCache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key)
}

func (c *redisCache) Clear(ctx context.Context) error {
	return nil
}
