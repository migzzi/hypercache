package hypercache

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	chanName = "test-channel"
)

type testStruct struct {
	Name string
	Age  int
}

func createRedisClient() *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	},
	)
	return client
}

func TestCacheSyncMessageSerialize(t *testing.T) {
	um := cacheSyncMessage{
		keyHashSlot: 1,
		uuid:        uuid.New(),
	}
	buff := um.serialize()
	println(buff)
	if len(buff) != 18 {
		t.Errorf("Failed to serialize")
	}
	dum := cacheSyncMessage{}
	dum.deserialize(buff)
	if dum.keyHashSlot != 1 {
		t.Errorf("Failed to deserialize")
	}

	if dum.uuid != um.uuid {
		t.Errorf("Failed to deserialize")
	}
}
func TestCreateSyncCache(t *testing.T) {
	cache := NewSynchronizedCache(createRedisClient(), chanName, 10)
	if cache == nil {
		t.Errorf("Failed to create sync cache")
	}

	if cache.inMemCache.maxEntries.Load() != 10 {
		t.Errorf("Failed to create maxEntries")
	}

	if cache.inMemCache.numEntries.Load() != 0 {
		t.Errorf("Failed to create numEntries")
	}

	if cache.clients == nil {
		t.Errorf("Failed to create redis client")
	}

	if cache.updateChannelName != chanName {
		t.Errorf("Failed to create updateChannelName")
	}

	if cache.uuid == uuid.Nil {
		t.Errorf("Failed to create uuid")
	}

	if cache.clients.Ping(cache.ctx).Err() != nil {
		print(cache.clients.Ping(cache.ctx).Err().Error())
		t.Errorf("Failed to ping redis")
	}
}

func TestSyncCacheAdd(t *testing.T) {
	cache := NewSynchronizedCache(createRedisClient(), chanName, 10)
	if cache == nil {
		t.Errorf("Failed to create sync cache")
	}

	err := cache.Set("k1", "v1", 0)
	if err != nil {
		t.Errorf("Failed to add entry")
	}
	if cache.inMemCache.numEntries.Load() != 1 {
		t.Errorf("Failed to add entry")
	}
	if _, ok := cache.inMemCache.Get("k1"); !ok {
		t.Errorf("Should have found entry in in-memory cache")
	}
}

func TestSyncCacheGet(t *testing.T) {
	cache := NewSynchronizedCache(createRedisClient(), chanName, 10)
	if cache == nil {
		t.Errorf("Failed to create sync cache")
	}

	cache.Set("k1", "v1", 0)
	val := ""
	err := cache.Get("k1", &val)
	if err != nil {
		t.Errorf("Failed to get entry with error %v", err.Error())
	}

	if val != "v1" {
		t.Errorf("Failed to get entry")
	}

	if _, ok := cache.inMemCache.Get("k1"); !ok {
		t.Errorf("Should have found entry in in-memory cache")
	}
}

func TestSyncCacheGetNonExistent(t *testing.T) {
	cache := NewSynchronizedCache(createRedisClient(), chanName, 10)
	if cache == nil {
		t.Errorf("Failed to create sync cache")
	}

	var val *string
	err := cache.Get("k11", val)
	if err != ErrCacheMiss {
		t.Errorf("Should have failed to get entry with ErrCacheMiss")
	}
}

func TestSyncCacheWithTwoClients_SecondDoesnotHaveFirstValue(t *testing.T) {
	cache1 := NewSynchronizedCache(createRedisClient(), chanName, 10)
	if cache1 == nil {
		t.Errorf("Failed to create sync cache")
	}

	cache2 := NewSynchronizedCache(createRedisClient(), chanName, 10)
	if cache2 == nil {
		t.Errorf("Failed to create sync cache")
	}

	err := cache1.Set("k1", "v1", 0)
	if err != nil {
		t.Errorf("Failed to add entry")
	}

	time.Sleep(1 * time.Second)

	val := ""
	err = cache2.Get("k1", &val)
	if err != nil {
		t.Errorf("Failed to get entry")
	}

	if val != "v1" {
		t.Errorf("Failed to get entry")
	}
}

func TestSyncCacheWithTwoClients_SecondHasFirstValue(t *testing.T) {
	cache1 := NewSynchronizedCache(createRedisClient(), chanName, 10)
	if cache1 == nil {
		t.Errorf("Failed to create sync cache")
	}

	cache2 := NewSynchronizedCache(createRedisClient(), chanName, 10)
	if cache2 == nil {
		t.Errorf("Failed to create sync cache")
	}

	if err := cache1.Set("k1", "v1", 0); err != nil {
		t.Errorf("Failed to add entry")
	}

	if err := cache2.Set("k1", "v2", 0); err != nil {
		t.Errorf("Failed to add entry")
	}

	time.Sleep(1 * time.Second)

	val := ""
	err := cache1.Get("k1", &val)
	if err != nil {
		t.Errorf("Failed to get entry")
	}

	if val != "v2" {
		t.Errorf("Failed to get entry")
	}
}

func TestSyncCacheSetWithComplexType(t *testing.T) {
	cache := NewSynchronizedCache(createRedisClient(), chanName, 10)
	if cache == nil {
		t.Errorf("Failed to create sync cache")
	}

	err := cache.Set("k1", &testStruct{Name: "v1", Age: 22}, 0)
	if err != nil {
		println(err.Error())
		t.Errorf("Failed to add entry")
	}
	if cache.inMemCache.numEntries.Load() != 1 {
		t.Errorf("Failed to add entry")
	}
	if _, ok := cache.inMemCache.Get("k1"); !ok {
		t.Errorf("Should have found entry in in-memory cache")
	}

	var val testStruct
	err = cache.Get("k1", &val)
	if err != nil {
		t.Errorf("Failed to get entry")
	}

	if val.Name != "v1" {
		t.Errorf("Failed to get entry")
	}

	if val.Age != 22 {
		t.Errorf("Failed to get entry")
	}

	//Remove it from in-memory cache to force it to get from redis
	cache.inMemCache.Delete("k1")

	var val2 testStruct
	err = cache.Get("k1", &val2)
	// defSerde.deserialize([]byte(val.(string)), &desVal)
	if err != nil {
		t.Errorf("Failed to get entry")
	}

	// println(val)

	if val2.Name != "v1" {
		t.Errorf("Failed to get entry")
	}

	if val2.Age != 22 {
		t.Errorf("Failed to get entry")
	}
}

func TestSyncCacheSetWithSliceValue(t *testing.T) {
	cache := NewSynchronizedCache(createRedisClient(), chanName, 10)
	if cache == nil {
		t.Errorf("Failed to create sync cache")
	}

	err := cache.Set("k1", []string{"v1", "v2"}, 0)
	if err != nil {
		println(err.Error())
		t.Errorf("Failed to add entry")
	}
	if cache.inMemCache.numEntries.Load() != 1 {
		t.Errorf("Failed to add entry")
	}
	if _, ok := cache.inMemCache.Get("k1"); !ok {
		t.Errorf("Should have found entry in in-memory cache")
	}

	val := []string{}
	err = cache.Get("k1", &val)
	if err != nil {
		t.Errorf("Failed to get entry")
	}

	if val[0] != "v1" {
		t.Errorf("Failed to get entry")
	}

	if val[1] != "v2" {
		t.Errorf("Failed to get entry")
	}

	//Remove it from in-memory cache to force it to get from redis
	cache.inMemCache.Delete("k1")

	val2 := []string{}
	err = cache.Get("k1", &val2)
	if err != nil {
		t.Errorf("Failed to get entry")
	}

	if val2[0] != "v1" {
		t.Errorf("Failed to get entry")
	}

	if val2[1] != "v2" {
		t.Errorf("Failed to get entry")
	}
}

func TestSyncCacheSetWithMapValue(t *testing.T) {
	cache := NewSynchronizedCache(createRedisClient(), chanName, 10)
	if cache == nil {
		t.Errorf("Failed to create sync cache")
	}

	err := cache.Set("k1", map[string]string{"v1": "v2"}, 0)
	if err != nil {
		println(err.Error())
		t.Errorf("Failed to add entry")
	}
	if cache.inMemCache.numEntries.Load() != 1 {
		t.Errorf("Failed to add entry")
	}
	if _, ok := cache.inMemCache.Get("k1"); !ok {
		t.Errorf("Should have found entry in in-memory cache")
	}

	val := map[string]string{}
	err = cache.Get("k1", &val)
	if err != nil {
		t.Errorf("Failed to get entry")
	}

	if val["v1"] != "v2" {
		t.Errorf("Failed to get entry")
	}

	//Remove it from in-memory cache to force it to get from redis
	cache.inMemCache.Delete("k1")

	val2 := map[string]string{}
	err = cache.Get("k1", &val2)
	if err != nil {
		t.Errorf("Failed to get entry")
	}

	if val2["v1"] != "v2" {
		t.Errorf("Failed to get entry")
	}
}

func TestSyncCacheSetWithNilValue(t *testing.T) {
	cache := NewSynchronizedCache(createRedisClient(), chanName, 10)
	if cache == nil {
		t.Errorf("Failed to create sync cache")
	}

	err := cache.Set("k1", nil, 0)
	if err != nil {
		println(err.Error())
		t.Errorf("Failed to add entry")
	}
	if cache.inMemCache.numEntries.Load() != 1 {
		t.Errorf("Failed to add entry")
	}
	if _, ok := cache.inMemCache.Get("k1"); !ok {
		t.Errorf("Should have found entry in in-memory cache")
	}

	val := ""
	err = cache.Get("k1", &val)
	if err != nil {
		t.Errorf("Failed to get entry")
	}

	if val != "" {
		t.Errorf("Failed to get entry")
	}

	//Remove it from in-memory cache to force it to get from redis
	cache.inMemCache.Delete("k1")

	val2 := ""
	err = cache.Get("k1", &val2)
	if err != nil {
		t.Errorf("Failed to get entry")
	}

	if val2 != "" {
		t.Errorf("Failed to get entry")
	}
}

func TestSyncCacheSetWithEmptyStringValue(t *testing.T) {
	cache := NewSynchronizedCache(createRedisClient(), chanName, 10)
	if cache == nil {
		t.Errorf("Failed to create sync cache")
	}

	err := cache.Set("k1", "", 0)
	if err != nil {
		println(err.Error())
		t.Errorf("Failed to add entry")
	}
	if cache.inMemCache.numEntries.Load() != 1 {
		t.Errorf("Failed to add entry")
	}
	if _, ok := cache.inMemCache.Get("k1"); !ok {
		t.Errorf("Should have found entry in in-memory cache")
	}

	val := ""
	err = cache.Get("k1", &val)
	if err != nil {
		t.Errorf("Failed to get entry")
	}

	if val != "" {
		t.Errorf("Failed to get entry")
	}

	//Remove it from in-memory cache to force it to get from redis
	cache.inMemCache.Delete("k1")

	val2 := ""
	err = cache.Get("k1", &val2)
	if err != nil {
		t.Errorf("Failed to get entry")
	}

	if val2 != "" {
		t.Errorf("Failed to get entry")
	}
}

func TestSyncCacheSetWithEmptyByteSliceValue(t *testing.T) {
	cache := NewSynchronizedCache(createRedisClient(), chanName, 10)
	if cache == nil {
		t.Errorf("Failed to create sync cache")
	}
	var val []byte
	err := cache.Set("k1", val, 0)
	if err != nil {
		println(err.Error())
		t.Errorf("Failed to add entry")
	}
	if cache.inMemCache.numEntries.Load() != 1 {
		t.Errorf("Failed to add entry")
	}
	if _, ok := cache.inMemCache.Get("k1"); !ok {
		t.Errorf("Should have found entry in in-memory cache")
	}

	val2 := []byte{}
	err = cache.Get("k1", &val2)
	if err != nil {
		t.Errorf("Failed to get entry")
	}

	if len(val2) != 0 {
		t.Errorf("Failed to get entry")
	}

	//Remove it from in-memory cache to force it to get from redis
	cache.inMemCache.Delete("k1")

	val3 := []byte{}
	err = cache.Get("k1", &val3)
	if err != nil {
		t.Errorf("Failed to get entry")
	}

	if len(val3) != 0 {
		t.Errorf("Failed to get entry")
	}
}

func TestSyncCacheSetWithSliceOfStructsValue(t *testing.T) {
	cache := NewSynchronizedCache(createRedisClient(), chanName, 10)
	if cache == nil {
		t.Errorf("Failed to create sync cache")
	}
	val := []testStruct{{Name: "v1", Age: 22}, {Name: "v2", Age: 23}}
	err := cache.Set("k1", val, 0)
	if err != nil {
		println(err.Error())
		t.Errorf("Failed to add entry")
	}
	if cache.inMemCache.numEntries.Load() != 1 {
		t.Errorf("Failed to add entry")
	}
	if _, ok := cache.inMemCache.Get("k1"); !ok {
		t.Errorf("Should have found entry in in-memory cache")
	}

	val2 := []testStruct{}
	err = cache.Get("k1", &val2)
	if err != nil {
		t.Errorf("Failed to get entry")
	}

	if len(val2) != 2 {
		t.Errorf("Failed to get entry")
	}

	if val2[0].Name != "v1" && val2[0].Age != 22 {
		t.Errorf("Failed to get entry")
	}

	if val2[1].Name != "v2" && val2[1].Age != 23 {
		t.Errorf("Failed to get entry")
	}
}

func TestSyncCacheSetWithMapOfStructsValue(t *testing.T) {
	cache := NewSynchronizedCache(createRedisClient(), chanName, 10)
	if cache == nil {
		t.Errorf("Failed to create sync cache")
	}
	val := map[string]testStruct{"v1": {Name: "v1", Age: 22}, "v2": {Name: "v2", Age: 23}}
	err := cache.Set("k1", val, 0)
	if err != nil {
		println(err.Error())
		t.Errorf("Failed to add entry")
	}
	if cache.inMemCache.numEntries.Load() != 1 {
		t.Errorf("Failed to add entry")
	}
	if _, ok := cache.inMemCache.Get("k1"); !ok {
		t.Errorf("Should have found entry in in-memory cache")
	}

	val2 := map[string]testStruct{}
	err = cache.Get("k1", &val2)
	if err != nil {
		t.Errorf("Failed to get entry")
	}

	if len(val2) != 2 {
		t.Errorf("Failed to get entry")
	}

	if val2["v1"].Name != "v1" && val2["v1"].Age != 22 {
		t.Errorf("Failed to get entry")
	}

	if val2["v2"].Name != "v2" && val2["v2"].Age != 23 {
		t.Errorf("Failed to get entry")
	}
}

func TestSyncCacheSetWithSliceOfStructsPointersValue(t *testing.T) {
	cache := NewSynchronizedCache(createRedisClient(), chanName, 10)
	if cache == nil {
		t.Errorf("Failed to create sync cache")
	}
	val := []*testStruct{{Name: "v1", Age: 22}, {Name: "v2", Age: 23}}
	err := cache.Set("k1", val, 0)
	if err != nil {
		println(err.Error())
		t.Errorf("Failed to add entry")
	}
	if cache.inMemCache.numEntries.Load() != 1 {
		t.Errorf("Failed to add entry")
	}
	if _, ok := cache.inMemCache.Get("k1"); !ok {
		t.Errorf("Should have found entry in in-memory cache")
	}

	val2 := []*testStruct{}
	err = cache.Get("k1", &val2)
	if err != nil {
		t.Errorf("Failed to get entry")
	}

	if len(val2) != 2 {
		t.Errorf("Failed to get entry")
	}

	if val2[0].Name != "v1" && val2[0].Age != 22 {
		t.Errorf("Failed to get entry")
	}

	if val2[1].Name != "v2" && val2[1].Age != 23 {
		t.Errorf("Failed to get entry")
	}
}

func TestSyncCacheSetWithMapOfStructsPointersValue(t *testing.T) {
	cache := NewSynchronizedCache(createRedisClient(), chanName, 10)
	if cache == nil {
		t.Errorf("Failed to create sync cache")
	}
	val := map[string]*testStruct{"v1": {Name: "v1", Age: 22}, "v2": {Name: "v2", Age: 23}}
	err := cache.Set("k1", val, 0)
	if err != nil {
		println(err.Error())
		t.Errorf("Failed to add entry")
	}
	if cache.inMemCache.numEntries.Load() != 1 {
		t.Errorf("Failed to add entry")
	}
	if _, ok := cache.inMemCache.Get("k1"); !ok {
		t.Errorf("Should have found entry in in-memory cache")
	}

	val2 := map[string]*testStruct{}
	err = cache.Get("k1", &val2)
	if err != nil {
		t.Errorf("Failed to get entry")
	}

	if len(val2) != 2 {
		t.Errorf("Failed to get entry")
	}

	if val2["v1"].Name != "v1" && val2["v1"].Age != 22 {
		t.Errorf("Failed to get entry")
	}

	if val2["v2"].Name != "v2" && val2["v2"].Age != 23 {
		t.Errorf("Failed to get entry")
	}
}

func TestSyncCacheDelete(t *testing.T) {
	cache := NewSynchronizedCache(createRedisClient(), chanName, 10)
	if cache == nil {
		t.Errorf("Failed to create sync cache")
	}
	cache.Set("k1", "v1", 0)
	if cache.inMemCache.numEntries.Load() != 1 {
		t.Errorf("Failed to add entry")
	}

	cache.Delete("k1")
	if cache.inMemCache.numEntries.Load() != 0 {
		t.Errorf("Failed to delete entry")
	}

	val := ""
	err := cache.Get("k1", &val)
	if err != ErrCacheMiss {
		t.Errorf("Failed to get entry")
	}
}
