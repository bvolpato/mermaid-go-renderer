// Ported from the hash collections of the GWT runtime in elk-worker.js of
// elkjs 0.9.3 (java.util.HashMap with InternalHashCodeMap and
// InternalStringMap, java.util.HashSet, java.util.LinkedHashMap)
// (Apache-2.0).
//
// GWT has the Apache License 2.0. elk-worker.js has no notice for this
// code. The Java sources of GWT for it are HashMap.java,
// InternalHashCodeMap.java, InternalStringMap.java, HashSet.java and
// LinkedHashMap.java in user/super/com/google/gwt/emul/java/util. Their
// copyright lines, read from the main branch of
// https://github.com/gwtproject/gwt on 2026-11-09:
//
//	Copyright 2008 Google Inc. (HashMap.java, LinkedHashMap.java)
//	Copyright 2014 Google Inc. (InternalHashCodeMap.java, InternalStringMap.java)
//	Copyright 2007 Google Inc. (HashSet.java)
//
// This file is a Go translation of that code. It is a changed version of
// it. LICENSE-GWT.txt in this directory has the license, and NOTICE in this
// directory has the license notice of the GWT files.

package gwt

// Map is a hash map with the iteration order of the GWT collections for keys
// whose hash codes are all different: java.util.HashMap and HashSet with keys
// that do not override hashCode (they hash by an identity counter), with
// Integer keys, with enum keys or with String keys, and LinkedHashMap with any
// key. All of them iterate in the order of the first insertion of each key. A
// key that is removed and put again goes to the end. Putting a key that the
// map has keeps its place.
//
// The JVM iterates a HashMap in the order of the hash buckets. GWT keeps the
// buckets in a JavaScript Map, which iterates in insertion order.
//
// For keys whose hash codes can collide without the keys being equal (Double,
// KVector, Pair, lists), use HashMap.
type Map[K comparable, V any] struct {
	index map[K]*mapEntry[K, V]
	// head and tail are the ends of the list of entries in iteration order.
	head, tail *mapEntry[K, V]
}

type mapEntry[K comparable, V any] struct {
	key        K
	value      V
	prev, next *mapEntry[K, V]
	removed    bool
}

// NewMap returns an empty map.
func NewMap[K comparable, V any]() *Map[K, V] {
	return &Map[K, V]{index: map[K]*mapEntry[K, V]{}}
}

// Len is size().
func (m *Map[K, V]) Len() int { return len(m.index) }

// IsEmpty is isEmpty().
func (m *Map[K, V]) IsEmpty() bool { return len(m.index) == 0 }

// Get is get(key). It returns the zero value for a missing key.
func (m *Map[K, V]) Get(key K) V {
	if e := m.index[key]; e != nil {
		return e.value
	}
	var zero V
	return zero
}

// Lookup returns the value of key and whether the map has the key.
func (m *Map[K, V]) Lookup(key K) (V, bool) {
	if e := m.index[key]; e != nil {
		return e.value, true
	}
	var zero V
	return zero, false
}

// ContainsKey is containsKey(key).
func (m *Map[K, V]) ContainsKey(key K) bool {
	_, ok := m.index[key]
	return ok
}

// Put is put(key, value). It returns the old value and whether the map had
// the key.
func (m *Map[K, V]) Put(key K, value V) (old V, had bool) {
	if e := m.index[key]; e != nil {
		old = e.value
		e.value = value
		return old, true
	}
	e := &mapEntry[K, V]{key: key, value: value, prev: m.tail}
	if m.tail != nil {
		m.tail.next = e
	} else {
		m.head = e
	}
	m.tail = e
	m.index[key] = e
	return old, false
}

// Remove is remove(key). It returns the old value and whether the map had
// the key.
func (m *Map[K, V]) Remove(key K) (old V, had bool) {
	e := m.index[key]
	if e == nil {
		return old, false
	}
	delete(m.index, key)
	if e.prev != nil {
		e.prev.next = e.next
	} else {
		m.head = e.next
	}
	if e.next != nil {
		e.next.prev = e.prev
	} else {
		m.tail = e.prev
	}
	// The entry keeps its next pointer, so that a loop that removes the
	// current entry continues.
	e.removed = true
	return e.value, true
}

// Clear is clear().
func (m *Map[K, V]) Clear() {
	m.index = map[K]*mapEntry[K, V]{}
	m.head, m.tail = nil, nil
}

// Each calls fn for each entry in iteration order until fn returns false. fn
// may remove the current entry. An entry that fn adds is visited.
func (m *Map[K, V]) Each(fn func(key K, value V) bool) {
	for e := m.head; e != nil; e = e.next {
		if e.removed {
			continue
		}
		if !fn(e.key, e.value) {
			return
		}
	}
}

// Keys is keySet() as a slice in iteration order.
func (m *Map[K, V]) Keys() []K {
	out := make([]K, 0, len(m.index))
	for e := m.head; e != nil; e = e.next {
		out = append(out, e.key)
	}
	return out
}

// Values is values() as a slice in iteration order.
func (m *Map[K, V]) Values() []V {
	out := make([]V, 0, len(m.index))
	for e := m.head; e != nil; e = e.next {
		out = append(out, e.value)
	}
	return out
}

// Set is HashSet or LinkedHashSet with the iteration order of GWT. See Map.
type Set[K comparable] struct {
	m Map[K, struct{}]
}

// NewSet returns a set with the given elements.
func NewSet[K comparable](values ...K) *Set[K] {
	s := &Set[K]{m: Map[K, struct{}]{index: map[K]*mapEntry[K, struct{}]{}}}
	for _, v := range values {
		s.Add(v)
	}
	return s
}

// Len is size().
func (s *Set[K]) Len() int { return s.m.Len() }

// IsEmpty is isEmpty().
func (s *Set[K]) IsEmpty() bool { return s.m.IsEmpty() }

// Contains is contains(key).
func (s *Set[K]) Contains(key K) bool { return s.m.ContainsKey(key) }

// Add is add(key). It returns true when the set did not have the key.
func (s *Set[K]) Add(key K) bool {
	_, had := s.m.Put(key, struct{}{})
	return !had
}

// AddAll adds the keys in order.
func (s *Set[K]) AddAll(keys ...K) {
	for _, k := range keys {
		s.Add(k)
	}
}

// Remove is remove(key). It returns true when the set had the key.
func (s *Set[K]) Remove(key K) bool {
	_, had := s.m.Remove(key)
	return had
}

// Clear is clear().
func (s *Set[K]) Clear() { s.m.Clear() }

// Values returns the elements in iteration order.
func (s *Set[K]) Values() []K { return s.m.Keys() }

// Each calls fn for each element in iteration order until fn returns false.
func (s *Set[K]) Each(fn func(key K) bool) {
	s.m.Each(func(k K, _ struct{}) bool { return fn(k) })
}

// HashMap is java.util.HashMap of GWT for keys with their own hashCode and
// equals. Keys with the same hash code share a chain. The map iterates the
// chains in the order of the first insertion of each hash code, and a chain
// in insertion order. A chain that becomes empty is dropped, so its hash code
// goes to the end when a key with it is put again.
//
// GWT keeps String keys in a second map that iterates first. Do not mix
// String keys with other keys in one HashMap.
type HashMap[K any, V any] struct {
	hash   func(K) int32
	equal  func(a, b K) bool
	chains *Map[int32, *[]hashEntry[K, V]]
	size   int
}

type hashEntry[K any, V any] struct {
	key   K
	value V
}

// NewHashMap returns an empty map with the hashCode and equals of the keys.
func NewHashMap[K any, V any](hash func(K) int32, equal func(a, b K) bool) *HashMap[K, V] {
	return &HashMap[K, V]{hash: hash, equal: equal, chains: NewMap[int32, *[]hashEntry[K, V]]()}
}

// Len is size().
func (m *HashMap[K, V]) Len() int { return m.size }

func (m *HashMap[K, V]) find(key K) (chain *[]hashEntry[K, V], i int) {
	chain = m.chains.Get(m.hash(key))
	if chain == nil {
		return nil, -1
	}
	for i := range *chain {
		if m.equal(key, (*chain)[i].key) {
			return chain, i
		}
	}
	return chain, -1
}

// Lookup returns the value of key and whether the map has the key.
func (m *HashMap[K, V]) Lookup(key K) (V, bool) {
	if chain, i := m.find(key); i >= 0 {
		return (*chain)[i].value, true
	}
	var zero V
	return zero, false
}

// Get is get(key).
func (m *HashMap[K, V]) Get(key K) V {
	v, _ := m.Lookup(key)
	return v
}

// ContainsKey is containsKey(key).
func (m *HashMap[K, V]) ContainsKey(key K) bool {
	_, i := m.find(key)
	return i >= 0
}

// Put is put(key, value).
func (m *HashMap[K, V]) Put(key K, value V) (old V, had bool) {
	chain, i := m.find(key)
	if i >= 0 {
		old = (*chain)[i].value
		(*chain)[i].value = value
		return old, true
	}
	if chain == nil {
		chain = &[]hashEntry[K, V]{}
		m.chains.Put(m.hash(key), chain)
	}
	*chain = append(*chain, hashEntry[K, V]{key, value})
	m.size++
	return old, false
}

// Remove is remove(key).
func (m *HashMap[K, V]) Remove(key K) (old V, had bool) {
	chain, i := m.find(key)
	if i < 0 {
		return old, false
	}
	old = (*chain)[i].value
	if len(*chain) == 1 {
		m.chains.Remove(m.hash(key))
	} else {
		*chain = append((*chain)[:i:i], (*chain)[i+1:]...)
	}
	m.size--
	return old, true
}

// Each calls fn for each entry in iteration order until fn returns false. fn
// must not change the map.
func (m *HashMap[K, V]) Each(fn func(key K, value V) bool) {
	stop := false
	m.chains.Each(func(_ int32, chain *[]hashEntry[K, V]) bool {
		for _, e := range *chain {
			if !fn(e.key, e.value) {
				stop = true
				return false
			}
		}
		return !stop
	})
}

// Keys returns the keys in iteration order.
func (m *HashMap[K, V]) Keys() []K {
	out := make([]K, 0, m.size)
	m.Each(func(k K, _ V) bool { out = append(out, k); return true })
	return out
}

// EachAny calls fn for each entry in iteration order, with untyped values.
func (m *Map[K, V]) EachAny(fn func(key, value any)) {
	m.Each(func(k K, v V) bool { fn(k, v); return true })
}

// EachAny calls fn for each element in iteration order, with untyped values.
func (s *Set[K]) EachAny(fn func(key any)) {
	s.Each(func(k K) bool { fn(k); return true })
}

// EachAny calls fn for each entry in iteration order, with untyped values.
func (m *HashMap[K, V]) EachAny(fn func(key, value any)) {
	m.Each(func(k K, v V) bool { fn(k, v); return true })
}
