package hypercache

import (
	"sync"
	"sync/atomic"
	"time"
)

type cacheEntry struct {
	// Entry key
	key string
	// This is the value we're storing in the cache.
	value interface{}
	// This is the time this entry was last update.
	ttl time.Duration
	// Time when the entry was first added.
	created time.Time
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
	// cacheEntry pointers.
	cache sync.Map
	// Linked list of cache entries sorted by last access time.
	// This is used to remove the oldest entry when the cache
	// is full.
	list *dlList[*cacheEntry]
	// This is the maximum number of entries we'll store in the cache.
	maxEntries *atomic.Int64
	// This is the number of entries currently in the cache.
	numEntries *atomic.Int64
}

func newMemoryCache(maxEntries int64) *memoryCache {
	mc := &memoryCache{
		maxEntries: &atomic.Int64{},
		numEntries: &atomic.Int64{},
		list:       newDLList[*cacheEntry](),
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
	item := entry.(*dlListNode[*cacheEntry])
	cacheEntry := item.value
	// Check if the entry has expired.
	if cacheEntry.isExpired() {
		// The entry has expired, so delete it from the cache.
		mc.Delete(key)
		return nil, false
	}
	cacheEntry.lastAccessed = time.Now()
	cacheEntry.accessCount.Add(1)
	mc.list.moveToFront(item)
	// Return the value and true to indicate success.
	return cacheEntry.value, true
}

func (mc *memoryCache) Set(key string, value interface{}, ttl time.Duration) error {
	now := time.Now()
	item, ok := mc.cache.Load(key)
	// Check if the entry already exists.
	if ok {
		listItem := item.(*dlListNode[*cacheEntry])
		entry := listItem.value
		// The entry already exists, so update it.
		entry.value = value
		entry.lastUpdated = now
		entry.lastAccessed = now
		entry.accessCount.Add(1)
		mc.list.moveToFront(listItem)
		return nil
	}
	// Create a new cache entry.
	entry := &cacheEntry{
		key:          key,
		value:        value,
		created:      now,
		lastUpdated:  now,
		lastAccessed: now,
		accessCount:  atomic.Uint64{},
	}
	// Set the TTL if one was provided.
	if ttl != 0 {
		entry.ttl = ttl
	}

	mc.evictIfNeeded()
	item = mc.list.pushFront(entry)
	// Add the entry to the cache.
	mc.cache.Store(key, item)
	mc.numEntries.Add(1)
	return nil
}

func (mc *memoryCache) Delete(key string) {
	// Get the entry from the cache.
	item, ok := mc.cache.Load(key)
	if !ok {
		return
	}
	// Remove the entry from the list.
	mc.list.remove(item.(*dlListNode[*cacheEntry]))
	// Delete the entry from the cache.
	mc.cache.Delete(key)
	mc.numEntries.Add(-1)
}

func (mc *memoryCache) evictIfNeeded() bool {
	// Check if we've exceeded the maximum number of entries.
	if mc.numEntries.Load() >= mc.maxEntries.Load() { // TODO: this is not thread safe
		// Check if we have expired entries.
		if !mc.checkAndRemoveExpired() {
			// We've exceeded the maximum number of entries, so we need to
			// Get the oldest entry from the list.
			entry := mc.list.popBack()
			// Remove the entry from the cache.
			mc.cache.Delete(entry.key)
			mc.numEntries.Add(-1)
		}
		return true
	}
	return false
}

/*
 * TODO: In case of large number of entries, this function can be called in a separate goroutine
 * Or a tree-like structure can be used to store the entries sorted by expiration time
 * for faster removal of expired entries.
 */
func (mc *memoryCache) checkAndRemoveExpired() bool {
	var expiredKeys []string
	mc.cache.Range(func(key, value interface{}) bool {
		entry := value.(*dlListNode[*cacheEntry]).value
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
