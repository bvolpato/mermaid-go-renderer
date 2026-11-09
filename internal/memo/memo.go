// Package memo has the cache that the render pipeline uses for results that
// depend only on a small input: the config of a site and of a diagram, the
// variables of a theme, the compiled style sheet.
//
// A render must give the same bytes with and without a cache. So a cache
// holds only values that are pure functions of their key, the key is the
// exact input and not a hash of it, and a cached value is immutable: the
// code that takes it out makes its own copy before it changes anything.
package memo

import (
	"sync"
	"sync/atomic"
)

// disabled turns every cache off. See SetEnabled.
var disabled atomic.Bool

// caches lists every cache, for Purge.
var (
	cachesMu sync.Mutex
	caches   []interface{ purge() }
)

// SetEnabled turns all caches on or off. While they are off, Get finds
// nothing and Put stores nothing, so every render computes all of its
// values. It is for tests that compare a render that uses the caches with a
// render that does not.
func SetEnabled(on bool) { disabled.Store(!on) }

// Enabled reports whether the caches are on.
func Enabled() bool { return !disabled.Load() }

// Purge empties all caches.
func Purge() {
	cachesMu.Lock()
	defer cachesMu.Unlock()
	for _, c := range caches {
		c.purge()
	}
}

// Map is a bounded cache from a key to a value. It is safe for concurrent
// use.
//
// It has two generations. A new entry goes to the current generation. When
// that is full, it becomes the old generation and the entries of the former
// old generation are dropped. An entry that is read in the old generation
// moves to the current one. So the cache keeps the entries that are in use
// and never has more than twice its size.
type Map[K comparable, V any] struct {
	mu       sync.RWMutex
	size     int
	cur, old map[K]V
}

// NewMap returns a cache that keeps at least size entries that are in use,
// and at most twice as many entries.
func NewMap[K comparable, V any](size int) *Map[K, V] {
	m := &Map[K, V]{size: max(size, 1), cur: map[K]V{}}
	cachesMu.Lock()
	caches = append(caches, m)
	cachesMu.Unlock()
	return m
}

func (m *Map[K, V]) purge() {
	m.mu.Lock()
	m.cur, m.old = map[K]V{}, nil
	m.mu.Unlock()
}

// Get returns the value of a key.
func (m *Map[K, V]) Get(key K) (V, bool) {
	if disabled.Load() {
		var zero V
		return zero, false
	}
	m.mu.RLock()
	v, ok := m.cur[key]
	if !ok && m.old != nil {
		v, ok = m.old[key]
		if ok {
			// Move the entry to the current generation.
			m.mu.RUnlock()
			m.put(key, v)
			return v, true
		}
	}
	m.mu.RUnlock()
	return v, ok
}

// Put stores the value of a key. When two goroutines compute the value of
// one key at the same time, both values are equal, and one of them stays.
func (m *Map[K, V]) Put(key K, v V) {
	if disabled.Load() {
		return
	}
	m.put(key, v)
}

func (m *Map[K, V]) put(key K, v V) {
	m.mu.Lock()
	if _, ok := m.cur[key]; !ok && len(m.cur) >= m.size {
		m.old, m.cur = m.cur, make(map[K]V, m.size)
	}
	m.cur[key] = v
	m.mu.Unlock()
}

// Len returns the number of entries.
func (m *Map[K, V]) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n := len(m.cur)
	for k := range m.old {
		if _, ok := m.cur[k]; !ok {
			n++
		}
	}
	return n
}

// Cache is a Map with string keys.
type Cache[V any] struct {
	Map[string, V]
}

// New returns a cache with string keys that keeps at least size entries
// that are in use, and at most twice as many entries.
func New[V any](size int) *Cache[V] {
	c := &Cache[V]{Map: Map[string, V]{size: max(size, 1), cur: map[string]V{}}}
	cachesMu.Lock()
	caches = append(caches, c)
	cachesMu.Unlock()
	return c
}

// GetBytes is Get for a key that the caller has as bytes. It does not
// allocate a string for the lookup.
func (c *Cache[V]) GetBytes(key []byte) (V, bool) {
	if disabled.Load() {
		var zero V
		return zero, false
	}
	c.mu.RLock()
	v, ok := c.cur[string(key)]
	if !ok && c.old != nil {
		v, ok = c.old[string(key)]
		if ok {
			c.mu.RUnlock()
			c.put(string(key), v)
			return v, true
		}
	}
	c.mu.RUnlock()
	return v, ok
}
