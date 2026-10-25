package cache

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
)

// TEST_REDIS_URL must point to a dedicated test instance. These tests use
// isolated keys and never flush the database or touch another caller's keys.
func isolatedRedisCache(t *testing.T) (*RedisProductCache, uint) {
	t.Helper()
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		t.Skip("TEST_REDIS_URL is not configured for an isolated Redis instance")
	}
	cache, err := NewProductCache(url, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	redisCache, ok := cache.(*RedisProductCache)
	if !ok {
		t.Fatal("TEST_REDIS_URL did not enable Redis")
	}
	id := uint(time.Now().UnixNano())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := redisCache.Delete(ctx, id); err != nil {
			t.Errorf("cleanup Redis key: %v", err)
		}
		if err := redisCache.Close(); err != nil {
			t.Errorf("close Redis cache: %v", err)
		}
	})
	return redisCache, id
}

func TestRedisProductCacheNeverPersistsInventory(t *testing.T) {
	cache, id := isolatedRedisCache(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	product := model.Product{ID: id, Name: "catalogue", SKUs: []model.ProductSKU{{ID: 1, Inventory: &model.Inventory{SKUID: 1, Available: 7, Reserved: 2}}}}
	if err := cache.Set(ctx, product); err != nil {
		t.Fatal(err)
	}
	got, found, err := cache.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if !found || got.ID != id || got.Name != product.Name || len(got.SKUs) != 1 || got.SKUs[0].Inventory != nil {
		t.Errorf("Redis round trip = %+v, found=%t, want catalogue without inventory", got, found)
	}
	if product.SKUs[0].Inventory == nil || product.SKUs[0].Inventory.Available != 7 {
		t.Error("Redis Set changed caller's inventory")
	}
	ttl, err := cache.client.PTTL(ctx, productKey(id)).Result()
	if err != nil {
		t.Fatal(err)
	}
	if ttl <= 0 || ttl > 72*time.Second {
		t.Errorf("Redis product TTL=%s, want bounded positive TTL up to 72s", ttl)
	}
	if err := cache.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, found, err := cache.Get(ctx, id); found || err != nil {
		t.Errorf("Redis Get(deleted) found=%t err=%v, want false,nil", found, err)
	}
}

func TestRedisProductCacheRejectsAndEvictsCorruptEntry(t *testing.T) {
	cache, id := isolatedRedisCache(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := cache.client.Set(ctx, productKey(id), "invalid-json", time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	if _, found, err := cache.Get(ctx, id); found || err == nil {
		t.Errorf("Redis Get(corrupt) found=%t err=%v, want false,decode error", found, err)
	}
	if _, found, err := cache.Get(ctx, id); found || err != nil {
		t.Errorf("Redis Get(after corrupt eviction) found=%t err=%v, want false,nil", found, err)
	}
}

func TestRedisProductCacheHonorsCancellation(t *testing.T) {
	cache, id := isolatedRedisCache(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, getErr := cache.Get(ctx, id)
	setErr := cache.Set(ctx, model.Product{ID: id})
	deleteErr := cache.Delete(ctx, id)
	for name, err := range map[string]error{"Get": getErr, "Set": setErr, "Delete": deleteErr} {
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Redis %s(cancelled) err=%v, want context.Canceled", name, err)
		}
	}
}
