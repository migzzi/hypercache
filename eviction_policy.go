package hypercache

type EvictionPolicy interface {
	// This method is called when a new entry is added to the cache.
	// It should return true if the entry should be added to the cache,
	// or false if it should be discarded.
	ShouldAddEntry(key string, value interface{}) bool
	// This method is called when an entry is accessed from the cache.
	// It should return true if the entry should be kept in the cache,
	// or false if it should be discarded.
	ShouldKeepEntry(key string, value interface{}) bool
	// This method is called when an entry is removed from the cache.
	// It should return true if the entry should be removed from the
	// cache, or false if it should be kept.
	ShouldRemoveEntry(key string, value interface{}) bool
}
