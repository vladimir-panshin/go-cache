package cache

import (
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSetGet(t *testing.T) {
	c := New[string](time.Minute)
	defer c.Close()

	c.Set("key", "value", 0)

	value, ok := c.Get("key")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if value != "value" {
		t.Fatalf("expected value, got %q", value)
	}
}

func TestGetMissingReturnsZeroValue(t *testing.T) {
	c := New[int](time.Minute)
	defer c.Close()

	value, ok := c.Get("absent")
	if ok {
		t.Fatal("expected missing key to report ok=false")
	}
	if value != 0 {
		t.Fatalf("expected zero value 0 for missing int key, got %d", value)
	}
}

func TestSetOverwrite(t *testing.T) {
	c := New[int](time.Minute)
	defer c.Close()

	c.Set("k", 1, 0)
	c.Set("k", 2, 0)

	value, ok := c.Get("k")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if value != 2 {
		t.Fatalf("expected overwritten value 2, got %d", value)
	}
}

func TestDelete(t *testing.T) {
	c := New[int](time.Minute)
	defer c.Close()

	c.Set("key", 123, 0)
	c.Delete("key")

	if _, ok := c.Get("key"); ok {
		t.Fatal("expected key to be deleted")
	}
}

func TestDeleteAbsentKeyIsNoop(t *testing.T) {
	c := New[int](time.Minute)
	defer c.Close()

	c.Delete("never-existed")

	if c.Len() != 0 {
		t.Fatalf("expected empty cache, got len %d", c.Len())
	}
}

func TestLen(t *testing.T) {
	c := New[int](time.Minute)
	defer c.Close()

	c.Set("1", 1, 0)
	c.Set("2", 2, 0)
	c.Set("3", 3, 0)

	if c.Len() != 3 {
		t.Fatalf("expected 3 keys, got %d", c.Len())
	}
}

func TestClear(t *testing.T) {
	c := New[int](time.Minute)
	defer c.Close()

	c.Set("a", 1, 0)
	c.Set("b", 2, 0)
	c.Clear()

	if c.Len() != 0 {
		t.Fatalf("cache should be empty, got len %d", c.Len())
	}
}

func TestTTLExpiration(t *testing.T) {
	c := New[string](time.Hour)
	defer c.Close()

	c.Set("key", "value", 100*time.Millisecond)
	time.Sleep(150 * time.Millisecond)

	if _, ok := c.Get("key"); ok {
		t.Fatal("expected key to expire")
	}
}

func TestPersistentKey(t *testing.T) {
	c := New[string](time.Hour)
	defer c.Close()

	c.Set("key", "value", 0)
	time.Sleep(200 * time.Millisecond)

	if _, ok := c.Get("key"); !ok {
		t.Fatal("persistent key disappeared")
	}
}

func TestTTLRemaining(t *testing.T) {
	c := New[string](time.Minute)
	defer c.Close()

	c.Set("key", "value", time.Second)

	ttl, ok := c.TTL("key")
	if !ok {
		t.Fatal("expected ttl")
	}
	if ttl <= 0 || ttl > time.Second {
		t.Fatalf("unexpected ttl: %v", ttl)
	}
}

func TestTTLPersistentKey(t *testing.T) {
	c := New[string](time.Minute)
	defer c.Close()

	c.Set("key", "value", 0)

	ttl, ok := c.TTL("key")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if ttl != -1 {
		t.Fatalf("expected -1 for persistent key, got %v", ttl)
	}
}

func TestTTLMissingKey(t *testing.T) {
	c := New[string](time.Minute)
	defer c.Close()

	if _, ok := c.TTL("absent"); ok {
		t.Fatal("expected ok=false for missing key")
	}
}

func TestSetNX(t *testing.T) {
	c := New[int](time.Minute)
	defer c.Close()

	if !c.SetNX("key", 1, 0) {
		t.Fatal("first SetNX should succeed")
	}
	if c.SetNX("key", 2, 0) {
		t.Fatal("second SetNX should fail")
	}

	value, _ := c.Get("key")
	if value != 1 {
		t.Fatalf("expected 1, got %d", value)
	}
}

func TestSetNXOverExpiredKey(t *testing.T) {
	c := New[int](time.Hour)
	defer c.Close()

	if !c.SetNX("key", 1, 50*time.Millisecond) {
		t.Fatal("first SetNX should succeed")
	}

	time.Sleep(100 * time.Millisecond)

	if !c.SetNX("key", 2, 0) {
		t.Fatal("SetNX over an expired key should succeed")
	}

	value, ok := c.Get("key")
	if !ok {
		t.Fatal("expected key to exist after second SetNX")
	}
	if value != 2 {
		t.Fatalf("expected refreshed value 2, got %d", value)
	}
}

func TestBackgroundGC(t *testing.T) {
	c := New[string](50 * time.Millisecond)
	defer c.Close()

	c.Set("key", "value", 100*time.Millisecond)
	time.Sleep(250 * time.Millisecond)

	if c.Len() != 0 {
		t.Fatalf("background GC did not remove expired key, len %d", c.Len())
	}
}

func TestNoGCWhenIntervalNonPositive(t *testing.T) {
	c := New[string](0)
	defer c.Close()

	c.Set("key", "value", 50*time.Millisecond)
	time.Sleep(100 * time.Millisecond)

	if c.Len() != 1 {
		t.Fatalf("expected key to remain in map without background GC, got len %d", c.Len())
	}

	if _, ok := c.Get("key"); ok {
		t.Fatal("expected expired key to be lazily removed on Get")
	}
	if c.Len() != 0 {
		t.Fatalf("expected len 0 after lazy removal, got %d", c.Len())
	}
}

func TestNegativeIntervalDoesNotPanic(t *testing.T) {
	c := New[int](-time.Second)
	defer c.Close()

	c.Set("k", 1, 0)
	if _, ok := c.Get("k"); !ok {
		t.Fatal("expected key to exist")
	}
}

func TestDoubleClose(t *testing.T) {
	c := New[int](time.Minute)

	c.Close()
	c.Close()
}

func TestConcurrentClose(t *testing.T) {
	c := New[int](time.Minute)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Close()
		}()
	}
	wg.Wait()
}

func TestConcurrentDistinctKeys(t *testing.T) {
	c := New[int](time.Minute)
	defer c.Close()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			key := "key-" + strconv.Itoa(id)
			c.Set(key, id, time.Minute)
			if v, ok := c.Get(key); ok {
				_ = v
			}
			c.Delete(key)
		}(i)
	}
	wg.Wait()
}

func TestConcurrentSharedKeys(t *testing.T) {
	c := New[int](10 * time.Millisecond)
	defer c.Close()

	const keys = 10
	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			key := "key-" + strconv.Itoa(i%keys)
			switch i % 5 {
			case 0:
				c.Set(key, i, 5*time.Millisecond)
			case 1:
				c.Get(key)
			case 2:
				c.Delete(key)
			case 3:
				c.TTL(key)
			case 4:
				c.SetNX(key, i, time.Minute)
			}
		}(i)
	}
	wg.Wait()
}

func TestConcurrentSetNX(t *testing.T) {
	c := New[string](time.Minute)
	defer c.Close()

	var wg sync.WaitGroup
	var success atomic.Int32

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if c.SetNX("lock", "owner", time.Minute) {
				success.Add(1)
			}
		}()
	}
	wg.Wait()

	if success.Load() != 1 {
		t.Fatalf("expected exactly one successful SetNX, got %d", success.Load())
	}
}

func TestGenericWithStruct(t *testing.T) {
	type user struct {
		Name string
		Age  int
	}

	c := New[user](time.Minute)
	defer c.Close()

	c.Set("u1", user{Name: "Vladimir", Age: 25}, 0)

	got, ok := c.Get("u1")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if got.Name != "Vladimir" || got.Age != 25 {
		t.Fatalf("unexpected struct value: %+v", got)
	}
}

func TestGenericWithSlice(t *testing.T) {
	c := New[[]int](time.Minute)
	defer c.Close()

	c.Set("nums", []int{1, 2, 3}, 0)

	got, ok := c.Get("nums")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if len(got) != 3 || got[0] != 1 || got[2] != 3 {
		t.Fatalf("unexpected slice value: %v", got)
	}
}

func BenchmarkSet(b *testing.B) {
	c := New[int](time.Minute)
	defer c.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set("key-"+strconv.Itoa(i%1000), i, time.Minute)
	}
}

func BenchmarkGet(b *testing.B) {
	c := New[int](time.Minute)
	defer c.Close()

	for i := 0; i < 1000; i++ {
		c.Set("key-"+strconv.Itoa(i), i, time.Minute)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get("key-" + strconv.Itoa(i%1000))
	}
}

func BenchmarkParallelMixed(b *testing.B) {
	c := New[int](time.Minute)
	defer c.Close()

	for i := 0; i < 1000; i++ {
		c.Set("key-"+strconv.Itoa(i), i, time.Minute)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := "key-" + strconv.Itoa(i%1000)
			if i%10 == 0 {
				c.Set(key, i, time.Minute)
			} else {
				c.Get(key)
			}
			i++
		}
	})
}
