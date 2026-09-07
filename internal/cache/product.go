// Package cache contains optional, non-authoritative acceleration layers.
// Cache failures are surfaced to callers so they can log and fall back to the
// PostgreSQL source of truth.
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
)

type ProductCache interface {
	Get(context.Context, uint) (model.Product, bool, error)
	Set(context.Context, model.Product) error
	Delete(context.Context, uint) error
	Close() error
}

type NoopProductCache struct{}

func (NoopProductCache) Get(context.Context, uint) (model.Product, bool, error) {
	return model.Product{}, false, nil
}
func (NoopProductCache) Set(context.Context, model.Product) error { return nil }
func (NoopProductCache) Delete(context.Context, uint) error       { return nil }
func (NoopProductCache) Close() error                             { return nil }

type RedisProductCache struct {
	client *redis.Client
	ttl    time.Duration
}

func NewProductCache(redisURL string, ttl time.Duration) (ProductCache, error) {
	if redisURL == "" {
		return NoopProductCache{}, nil
	}
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse REDIS_URL: %w", err)
	}
	// Cache outages must not turn product reads into multi-second stalls.
	options.DialTimeout = 300 * time.Millisecond
	options.ReadTimeout = 200 * time.Millisecond
	options.WriteTimeout = 200 * time.Millisecond
	return &RedisProductCache{client: redis.NewClient(options), ttl: ttl}, nil
}

func (c *RedisProductCache) Get(ctx context.Context, id uint) (model.Product, bool, error) {
	value, err := c.client.Get(ctx, productKey(id)).Bytes()
	if err == redis.Nil {
		return model.Product{}, false, nil
	}
	if err != nil {
		return model.Product{}, false, err
	}
	var product model.Product
	if err := json.Unmarshal(value, &product); err != nil {
		_ = c.client.Del(ctx, productKey(id)).Err()
		return model.Product{}, false, fmt.Errorf("decode cached product: %w", err)
	}
	return product, true, nil
}

func (c *RedisProductCache) Set(ctx context.Context, product model.Product) error {
	value, err := json.Marshal(product)
	if err != nil {
		return fmt.Errorf("encode product cache: %w", err)
	}
	return c.client.Set(ctx, productKey(product.ID), value, c.ttl).Err()
}

func (c *RedisProductCache) Delete(ctx context.Context, id uint) error {
	return c.client.Del(ctx, productKey(id)).Err()
}

func (c *RedisProductCache) Close() error { return c.client.Close() }

func productKey(id uint) string { return fmt.Sprintf("commerce:product:v2:%d", id) }
