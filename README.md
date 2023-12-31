# hypercache
[![License](https://img.shields.io/github/license/migzzi/hypercache)]()
[![Release](https://img.shields.io/github/v/release/migzzi/hypercache)](https://goreportcard.com/report/github.com/migzzi/hypercache)
[![Build Status](https://img.shields.io/github/workflow/status/migzzi/hypercache/Test?label=tests)](https://github.com/migzzi/hypercache/actions)


An in-memory cache backed by redis for cache synchronization between instances.

## Install
```bash
    $ go get -u github.com/migzzi/hypercache
```

## Usage
```go
package main

import (
	"time"

	"github.com/migzzi/hypercache"
)

type testStruct struct {
    Name string
    Title  string
}

func main() {
	cache := NewSynchronizedCache(createRedisClient(), chanName, 10)

    // Add simple string values with 1 min TTL.
    cache.Set("key1", "val1", 60 * time.Seconds)

    // At this point the cache is populated to all instances of this app.
    val := ""
    if err := cache.Get("key1", &val); err != nil {
        panic("Shit!")
    }

    Assert(val == "val1")

    // Set ttl to 0 so the entry never expires.
    cache.Set("key2", "val2", 0)

    // Can Cache complex types as well (structs, slices, and maps)
    cache.Set("key3", testStruct{ Name: "val3", Title: "Title3" }, 0)
    val3 := testStruct{}
    if err := cache.Get("key3", &val3); err != nil {
        panic("Shit!")
    }

    Assert(val3.Name == "val3")

}

```