package memo

import (
	"fmt"
	"sync"
	"testing"
)

// TestCacheIsBounded checks that the cache never has more than twice its
// size, that it keeps the entries that are in use, and that it drops the
// ones that are not.
func TestCacheIsBounded(t *testing.T) {
	const size = 8
	c := New[int](size)
	c.Put("kept", -1)
	for i := 0; i < 1000; i++ {
		c.Put(fmt.Sprint("k", i), i)
		// A read keeps the entry in the current generation.
		if v, ok := c.Get("kept"); !ok || v != -1 {
			t.Fatalf("after %d entries the entry in use is gone", i)
		}
		if n := c.Len(); n > 2*size {
			t.Fatalf("the cache has %d entries, the limit is %d", n, 2*size)
		}
	}
	if _, ok := c.Get("k0"); ok {
		t.Error("an entry that was not read is still in the cache")
	}
	if v, ok := c.Get("k999"); !ok || v != 999 {
		t.Error("the last entry is not in the cache")
	}
	if v, ok := c.GetBytes([]byte("k999")); !ok || v != 999 {
		t.Error("GetBytes does not find the last entry")
	}
}

// TestSetEnabled checks the switch that the tests of the renderer use: a
// cache that is off finds nothing and stores nothing.
func TestSetEnabled(t *testing.T) {
	c := New[string](4)
	c.Put("a", "1")
	SetEnabled(false)
	defer SetEnabled(true)
	if Enabled() {
		t.Fatal("Enabled() is true after SetEnabled(false)")
	}
	if _, ok := c.Get("a"); ok {
		t.Error("Get found an entry while the caches are off")
	}
	c.Put("b", "2")
	SetEnabled(true)
	if _, ok := c.Get("b"); ok {
		t.Error("Put stored an entry while the caches were off")
	}
	if v, ok := c.Get("a"); !ok || v != "1" {
		t.Error("the entry from before is gone")
	}
	Purge()
	if c.Len() != 0 {
		t.Errorf("after Purge the cache has %d entries", c.Len())
	}
}

// TestCacheConcurrent uses one cache from many goroutines. The race
// detector checks the locking.
func TestCacheConcurrent(t *testing.T) {
	c := New[int](16)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 2000; i++ {
				key := fmt.Sprint("k", (i*7+g)%40)
				if v, ok := c.Get(key); ok && v != len(key) {
					t.Errorf("Get(%q) = %d", key, v)
					return
				}
				c.Put(key, len(key))
			}
		}(g)
	}
	wg.Wait()
	if n := c.Len(); n > 32 {
		t.Errorf("the cache has %d entries, the limit is 32", n)
	}
}
