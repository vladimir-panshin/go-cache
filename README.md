# go-cache

A generic, thread-safe in-memory cache for Go, with per-key TTL and automatic background cleanup.

![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)
![License](https://img.shields.io/badge/license-MIT-green)

## Install

```bash
go get github.com/vladimir-panshin/go-cache
```

## Quick start

```go
package main

import (
	"fmt"
	"time"

	"github.com/vladimir-panshin/go-cache"
)

func main() {
	// Background cleanup runs every minute. Pass 0 to disable it.
	c := cache.New[string](time.Minute)
	defer c.Close()

	c.Set("greeting", "hello", 5*time.Minute) // expires in 5 minutes
	c.Set("permanent", "stays", 0)            // never expires

	if v, ok := c.Get("greeting"); ok {
		fmt.Println(v) // hello
	}
}
```

The value type is chosen at construction, so the compiler enforces it — `Get` returns that type directly:

```go
type Session struct {
	UserID string
	Roles  []string
}

c := cache.New[Session](time.Minute)
defer c.Close()

c.Set("sess_1", Session{UserID: "u1", Roles: []string{"admin"}}, time.Hour)

s, ok := c.Get("sess_1") // s is a Session, not interface{}
```

## API

| Method | Description |
|---|---|
| `New[V](cleanupInterval)` | Create a cache. `cleanupInterval <= 0` disables the background sweeper. |
| `Set(key, value, ttl)` | Store a value. `ttl <= 0` means no expiration. |
| `SetNX(key, value, ttl)` | Store only if the key is absent or expired. Returns `false` if a live key exists. |
| `Get(key)` | Return `(value, true)` if present and not expired, otherwise `(zero, false)`. |
| `TTL(key)` | Remaining lifetime. `-1` for a key with no expiration; `ok == false` if absent. |
| `Delete(key)` | Remove a key. No-op if absent. |
| `Len()` | Number of stored keys (may include not-yet-swept expired entries). |
| `Clear()` | Remove all keys. |
| `Close()` | Stop the background sweeper. Safe to call multiple times and concurrently. |

## Expiration

A key expires when its TTL runs out. Expired entries are removed in two ways:

- **On access** — `Get` and `TTL` drop an expired entry when they hit it, so you never read a stale value, even with no background cleanup.
- **In the background** — when `New` gets a positive interval, a sweeper clears expired entries periodically so old keys don't pile up in memory.

If you pass `New[V](0)`, the sweeper is off but expiration still works — keys just get removed when you access them.

## SetNX as a lock

`SetNX` writes only if the key isn't already set (and not expired). That makes it a basic lock: many goroutines call it, one wins.

```go
if c.SetNX("job:42", "running", 30*time.Second) {
	// won the slot — do the work
}
```

## Concurrency & testing

All operations are safe for concurrent use. Reads share an `RWMutex`; writes and expiry take the write lock. Concurrency is covered by the test suite under the race detector — concurrent distinct keys, contended shared keys, concurrent `SetNX`, and concurrent `Close`.

```bash
go vet ./...
go test -race ./...
go test -bench=. -benchmem ./...
```
