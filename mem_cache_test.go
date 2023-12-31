package hypercache

import (
	"fmt"
	"testing"
	"time"
)

func TestCreateMemCache(t *testing.T) {
	cache := newMemoryCache(100)
	if cache == nil {
		t.Errorf("Failed to create memory cache")
	}

	if cache.maxEntries.Load() != 100 {
		t.Errorf("Failed to create maxEntries")
	}

	if cache.numEntries.Load() != 0 {
		t.Errorf("Failed to create numEntries")
	}
}

func TestMemCacheAdd(t *testing.T) {
	cache := newMemoryCache(100)
	if cache == nil {
		t.Errorf("Failed to create memory cache")
	}

	cache.Set("k1", "v1", 0)
	if cache.numEntries.Load() != 1 {
		t.Errorf("Failed to add entry")
	}
}

func TestMemCacheGet(t *testing.T) {
	cache := newMemoryCache(100)
	if cache == nil {
		t.Errorf("Failed to create memory cache")
	}

	cache.Set("k1", "v1", 0)
	val, ok := cache.Get("k1")
	if !ok {
		t.Errorf("Failed to get entry")
	}

	if val != "v1" {
		t.Errorf("Failed to get entry")
	}
}

func TestMemCacheGetNonExistent(t *testing.T) {
	cache := newMemoryCache(100)
	if cache == nil {
		t.Errorf("Failed to create memory cache")
	}

	_, ok := cache.Get("k1")
	if ok {
		t.Errorf("Failed to get entry")
	}
}

func TestMemCacheGetExpired(t *testing.T) {
	cache := newMemoryCache(100)

	cache.Set("k1", "v1", 100*time.Millisecond)
	time.Sleep(200 * time.Millisecond)
	_, ok := cache.Get("k1")
	if ok {
		t.Errorf("Failed to get expired entry")
	}
}

func TestMemCacheDelete(t *testing.T) {
	cache := newMemoryCache(100)
	if cache == nil {
		t.Errorf("Failed to create memory cache")
	}

	cache.Set("k1", "v1", 0)
	cache.Delete("k1")
	if cache.numEntries.Load() != 0 {
		t.Errorf("Failed to delete entry")
	}

	_, ok := cache.Get("k1")
	if ok {
		t.Errorf("Failed to delete entry")
	}
}

func TestMemCacheEvict(t *testing.T) {
	cache := newMemoryCache(10)
	if cache == nil {
		t.Errorf("Failed to create memory cache")
	}

	for i := 0; i < 15; i++ {
		cache.Set(fmt.Sprintf("%d", i), i, 0)
	}

	if cache.numEntries.Load() != 10 {
		t.Errorf("Failed to evict entries")
	}

	for i := 0; i < 5; i++ {
		_, ok := cache.Get(fmt.Sprintf("%d", i))
		if ok {
			t.Errorf("Failed to evict entries")
		}
	}

	for i := 5; i < 15; i++ {
		_, ok := cache.Get(fmt.Sprintf("%d", i))
		if !ok {
			t.Errorf("Failed to evict entries")
		}
	}
}

func TestMemCacheEvictWithExpiry(t *testing.T) {
	cache := newMemoryCache(10)
	if cache == nil {
		t.Errorf("Failed to create memory cache")
	}

	for i := 0; i < 10; i++ {
		cache.Set(fmt.Sprintf("%d", i), i, 100*time.Millisecond)
	}

	if cache.numEntries.Load() != 10 {
		t.Errorf("Failed to evict entries")
	}

	time.Sleep(200 * time.Millisecond)

	for i := 10; i < 15; i++ {
		cache.Set(fmt.Sprintf("%d", i), i, 100*time.Millisecond)
	}

	if cache.numEntries.Load() != 5 {
		t.Errorf("Failed to evict entries")
	}

	for i := 10; i < 15; i++ {
		_, ok := cache.Get(fmt.Sprintf("%d", i))
		if !ok {
			t.Errorf("Failed to evict entries")
		}
	}
}
