package hypercache

import (
	"context"
	"encoding/binary"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	HASH_SLOT_COUNT = 16384 // Redis cluster has 16384 hash slots.
)

var (
	setAndPublishScript = `
		if ARGV[2] == "0" then
			redis.call("SET", KEYS[1], ARGV[1])
		else
			redis.call("SET", KEYS[1], ARGV[1], "EX", ARGV[2])
		end
		redis.call("PUBLISH", ARGV[3], ARGV[4])
	`

	getCacheAndTTLRemainingScript = `
		local result={}
    result[1] = redis.call('GET', KEYS[1])
    result[2] = redis.call('TTL', KEYS[1])
    return result;
	`

	deleteAndPublishScript = `
		redis.call("DEL", KEYS[1])
		redis.call("PUBLISH", ARGV[1], ARGV[2])
	`

	DEBUG = false

	ErrCacheMiss = errors.New("cache: key is missing")
)

func logDebug(format string, a ...interface{}) {
	if DEBUG {
		log.Printf(format, a...)
	}
}

type redisCacheEntry struct {
	// This is the value we're storing in the cache.
	value interface{}
	// This is the time this entry was last update.
	lastUpdatedTimestamp int64
	// This is the key hash slot this entry belongs to.
	keyHashSlot uint16
}

type cacheSyncMessage struct {
	// This is the slot of the key of the entry that was updated.
	keyHashSlot uint16
	// GUID of the cache instance updated the entry.
	uuid uuid.UUID
}

func (um cacheSyncMessage) serialize() []byte {
	buff := make([]byte, 18)
	copy(buff[0:16], um.uuid[:])
	binary.BigEndian.PutUint16(buff[16:18], um.keyHashSlot)
	return buff
}

func (um *cacheSyncMessage) deserialize(buff []byte) {
	copy(um.uuid[:], buff[0:16])
	um.keyHashSlot = binary.BigEndian.Uint16(buff[16:18])
}

type synchronizedCache struct {
	clients redis.UniversalClient
	// This is the last time each hash slot was updated.
	hashSlotLastUpdated []int64
	// GUID of the cache.
	uuid uuid.UUID
	// In-memory cache.
	inMemCache *memoryCache
	//No of the redis channel to listen for updates.
	updateChannelName string

	mu sync.RWMutex

	ctx context.Context

	serde serde
}

func NewSynchronizedCache(clients redis.UniversalClient, updateChannelName string, maxEntries int64) *synchronizedCache {
	if clients == nil {
		panic("clients cannot be nil")
	}
	sc := &synchronizedCache{
		clients:             clients,
		hashSlotLastUpdated: make([]int64, 16384),
		uuid:                uuid.New(),
		inMemCache:          newMemoryCache(maxEntries),
		updateChannelName:   updateChannelName,
		ctx:                 context.Background(),
		serde:               &defaultSerde{},
	}
	// Start the update listener.
	go sc.updateListener()
	return sc
}

func (sc *synchronizedCache) updateListener() {
	log.Printf("Starting update listener for cache %s", sc.uuid.String())
	// Subscribe to the update channel.
	pubsub := sc.clients.Subscribe(sc.ctx, sc.updateChannelName)
	// Wait for confirmation that subscription is created before publishing anything.
	ch := pubsub.Channel()
	// Loop forever, listening for updates.
	for msg := range ch {
		// Wait for a message.
		// msg := <-ch
		// Deserialize the message.
		message := cacheSyncMessage{}
		message.deserialize([]byte(msg.Payload))

		logDebug("Received message from UUID %v", message.uuid.String())

		// Check if the message was sent by this cache instance.
		if message.uuid == sc.uuid {
			// The message was sent by this cache instance, so ignore it.
			continue
		}

		sc.mu.Lock()
		sc.hashSlotLastUpdated[message.keyHashSlot] = time.Now().UnixMicro()
		sc.mu.Unlock()
	}
}

func (sc *synchronizedCache) Get(key string, dest interface{}) error {
	timestamp := time.Now().UnixMicro()
	// Get the cache entry from the in-memory cache.
	entry, ok := sc.inMemCache.Get(key)
	var cacheEntry *redisCacheEntry
	slot := -1
	if ok {
		cacheEntry = entry.(*redisCacheEntry)
		slot = int(cacheEntry.keyHashSlot)
		// Check if the entry has expired.
		sc.mu.RLock()
		hasExpired := sc.hashSlotLastUpdated[cacheEntry.keyHashSlot] < cacheEntry.lastUpdatedTimestamp
		sc.mu.RUnlock()
		if hasExpired {
			// copy struct to dest
			// err := copyStruct(cacheEntry.value, dest)
			err := sc.serde.deserialize(cacheEntry.value.([]byte), dest)
			return err
		}
	}

	// Either the entry doesn't exist, or it has expired.
	// So get the entry from Redis.
	result, err := sc.clients.Eval(sc.ctx, getCacheAndTTLRemainingScript, []string{key}).Result()
	if err != redis.Nil && err != nil {
		return err
	}

	val, ttl := result.([]interface{})[0], result.([]interface{})[1]
	logDebug("Val %v -- TTL%v", result.([]interface{})[0], result.([]interface{})[1])
	if val == nil {
		// The entry doesn't exist in Redis, so it doesn't exist in the cache.
		return ErrCacheMiss
	}

	if slot == -1 {
		slot = int(crc16CCITT([]byte(key)) % HASH_SLOT_COUNT)
	}
	err = sc.serde.deserialize([]byte(val.(string)), dest)
	if err != nil {
		return err
	}

	// Create a new cache entry.
	entry = &redisCacheEntry{
		value:                dest,
		lastUpdatedTimestamp: timestamp,
		keyHashSlot:          uint16(slot),
	}
	// Set the entry in the in-memory cache.
	if err := sc.inMemCache.Set(key, entry, time.Duration(ttl.(int64))*time.Second); err != nil {
		return err
	}

	return nil
}

func (sc *synchronizedCache) Set(key string, value interface{}, ttl time.Duration) error {
	// Create a new cache entry.
	slot := crc16CCITT([]byte(key)) % HASH_SLOT_COUNT
	timestamp := time.Now().UnixMicro()
	ttlSeconds := int64(ttl / time.Second)
	// serialize value to byte array
	serializedVal, err := sc.serde.serialize(value)
	if err != nil {
		return err
	}
	logDebug("Setting %v", value)

	entry := &redisCacheEntry{
		value:                serializedVal,
		lastUpdatedTimestamp: timestamp,
		keyHashSlot:          slot,
	}

	// Set and publish the entry.
	_, err = sc.clients.Eval(sc.ctx, setAndPublishScript, []string{key}, serializedVal, ttlSeconds, sc.updateChannelName, cacheSyncMessage{
		keyHashSlot: slot,
		uuid:        sc.uuid,
	}.serialize()).Result()

	if err != redis.Nil && err != nil {
		return err
	}

	sc.inMemCache.Set(key, entry, ttl)
	return nil
}

func (sc *synchronizedCache) Delete(key string) {
	// Delete the entry from Redis.
	sc.clients.Eval(sc.ctx, deleteAndPublishScript, []string{key}, sc.updateChannelName, cacheSyncMessage{
		keyHashSlot: crc16CCITT([]byte(key)) % HASH_SLOT_COUNT,
		uuid:        sc.uuid,
	}.serialize())
	// Delete the entry from the in-memory cache.
	sc.inMemCache.Delete(key)
}
