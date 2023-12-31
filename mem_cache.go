package hypercache

import (
	"sync"
	"sync/atomic"
	"time"
)

type cacheEntry struct {
	// This is the value we're storing in the cache.
	value interface{}
	// This is the time this entry was last update.
	ttl time.Duration
	// Time when the entry was last updated.
	lastUpdated time.Time
	// Time when the entry was last accessed.
	lastAccessed time.Time
	// Number of times the entry has been accessed.
	accessCount atomic.Uint64
}

func (ce *cacheEntry) isExpired() bool {
	return ce.ttl > 0 && time.Now().After(ce.lastUpdated.Add(ce.ttl))
}

type memoryCache struct {
	// This is the cache itself. It's a map of strings to
	// *cacheEntry objects.
	cache sync.Map
	// This is the maximum number of entries we'll store in the cache.
	maxEntries *atomic.Int64
	// This is the number of entries currently in the cache.
	numEntries *atomic.Int64
}

func newMemoryCache(maxEntries int64) *memoryCache {
	mc := &memoryCache{
		maxEntries: &atomic.Int64{},
		numEntries: &atomic.Int64{},
	}
	mc.maxEntries.Store(maxEntries)
	mc.numEntries.Store(0)
	return mc
}

func (mc *memoryCache) Get(key string) (interface{}, bool) {
	// Get the cache entry from the map.
	entry, ok := mc.cache.Load(key)
	if !ok {
		return nil, false
	}
	// Cast the entry to a *cacheEntry.
	cacheEntry := entry.(*cacheEntry)
	// Check if the entry has expired.
	if cacheEntry.isExpired() {
		// The entry has expired, so delete it from the cache.
		mc.Delete(key)
		return nil, false
	}
	cacheEntry.lastAccessed = time.Now()
	cacheEntry.accessCount.Add(1)
	// Return the value and true to indicate success.
	return cacheEntry.value, true
}

func (mc *memoryCache) Set(key string, value interface{}, ttl time.Duration) error {
	now := time.Now()
	// Create a new cache entry.
	entry := &cacheEntry{
		value:        value,
		lastUpdated:  now,
		lastAccessed: now,
		accessCount:  atomic.Uint64{},
	}
	// Set the TTL if one was provided.
	if ttl != 0 {
		entry.ttl = ttl
	}

	// Check if we've exceeded the maximum number of entries.
	if mc.numEntries.Load() >= mc.maxEntries.Load() { // TODO: this is not thread safe
		// Check if we have expired entries.
		if !mc.checkAndRemoveExpired() {
			// We've exceeded the maximum number of entries, so we need to
			// remove the oldest entry.
			var oldestKey string
			var oldestTime time.Time
			mc.cache.Range(func(key, value interface{}) bool {
				// Get the cache entry from the map.
				entry := value.(*cacheEntry)
				// Check if this is the oldest entry we've seen so far.
				if oldestTime.IsZero() || entry.lastAccessed.Before(oldestTime) {
					// This is the oldest entry we've seen so far, so store
					// it's key and last accessed time.
					oldestKey = key.(string)
					oldestTime = entry.lastAccessed
				}
				return true
			})
			// Delete the oldest entry from the cache.
			mc.Delete(oldestKey)
		}
		// We've removed some expired entries, so we don't need to
		// remove any more.
	}

	// Add the entry to the cache.
	mc.cache.Store(key, entry)
	mc.numEntries.Add(1)
	return nil
}

func (mc *memoryCache) Delete(key string) {
	// Delete the entry from the cache.
	mc.cache.Delete(key)
	mc.numEntries.Add(-1)
}

/*
 * TODO: In case of large number of entries, this function can be called in a separate goroutine
 * Or a tree-like structure can be used to store the entries sorted by expiration time
 * for faster removal of expired entries.
 */
func (mc *memoryCache) checkAndRemoveExpired() bool {
	var expiredKeys []string
	mc.cache.Range(func(key, value interface{}) bool {
		entry := value.(*cacheEntry)
		// Check if the entry has expired.
		if entry.isExpired() {
			// The entry has expired, so delete it from the cache.
			expiredKeys = append(expiredKeys, key.(string))
		}
		return true
	})
	// Delete the expired entries from the cache.
	for _, key := range expiredKeys {
		mc.Delete(key)
	}
	return len(expiredKeys) > 0
}
