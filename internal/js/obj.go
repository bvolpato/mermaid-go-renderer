// This file ports no upstream file. It has the object, the map and the set
// with the property order and the iteration order of ECMAScript.

package js

import (
	"maps"
	"slices"
	"sort"
	"sync/atomic"
)

var nextObjSourceID atomic.Uint64

// Obj is a string-keyed map with the property order of a JavaScript object.
// Keys that are array indices come first in ascending numeric order. All other
// keys follow in insertion order. Code that ports `for (k in obj)`,
// Object.keys, Object.values or Object.entries must use Obj, because layout
// results depend on this order.
//
// The properties are in two slices, in insertion order: the keys and the
// values. A small object finds a key with a scan of the keys. A larger one
// has an index from key to position. A copy of an object shares the keys and
// the index with the original and has its own values, so a copy costs one
// allocation of values (see Copy).
//
// An object is not safe for concurrent use, with one exception: a frozen
// object does not change, so many goroutines can read it (see Freeze).
type Obj[V any] struct {
	// keys and vals hold the properties by slot. A slot is the position of a
	// property in insertion order. A deleted property keeps its slot until
	// the object is compacted (see objExt.dead). The arrays behind keys are
	// never written in place below len(keys), so a reader can keep a copy of
	// the slice header while the object changes.
	keys []string
	vals []V
	// index maps a key to its slot. It is nil while the object is small.
	index map[string]int32
	// ext has what few objects need: index keys and deleted slots.
	ext *objExt
	// gen changes when a property is deleted or the slots move. Each uses
	// it to see that the object changed under it.
	gen      uint32
	flags    uint8
	cowDepth uint8
	// srcID identifies the frozen template object this Obj was copied from
	// and stays non-zero until this Obj is mutated.
	srcID uint64
}

// SourceID returns a non-zero identifier when o is frozen or an unmodified
// shallow copy of a frozen object.
func (o *Obj[V]) SourceID() uint64 {
	if o == nil {
		return 0
	}
	return o.srcID
}

const (
	// objFrozen marks an object that cannot change.
	objFrozen uint8 = 1 << iota
	// objSharedKeys marks an object whose keys, index and ext are also the
	// keys of another object. The object copies them before it adds or
	// deletes a key.
	objSharedKeys
	// objSharedVals marks an object whose vals slice is shared read-only with
	// a frozen object. Any write or lazy child materialization calls ownVals.
	objSharedVals
	// objDeepFrozen marks a frozen object whose descendant objects and arrays
	// are also frozen (set by FreezeValue).
	objDeepFrozen
	// objCOWClone marks a lazy copy-on-write clone (js.Clone semantics: no
	// assignWithDepth array deduplication, deep clone of slices on access).
	objCOWClone
)

// objSmall is the largest number of slots that an object searches with a
// scan of its keys.
const objSmall = 12

// objExt is the part of an object that only some objects have.
type objExt struct {
	// idx lists the slots whose key is an array index. It is sorted by the
	// number when idxDirt is false.
	idx     []idxSlot
	idxDirt bool
	// dead is the bitmap of the deleted slots, or nil.
	dead  []uint64
	nDead int
}

type idxSlot struct {
	num  uint32
	slot int32
}

func (e *objExt) isDead(slot int) bool {
	w := slot >> 6
	return w < len(e.dead) && e.dead[w]&(1<<(uint(slot)&63)) != 0
}

func (e *objExt) clone() *objExt {
	c := &objExt{idxDirt: e.idxDirt, nDead: e.nDead}
	c.idx = slices.Clone(e.idx)
	c.dead = slices.Clone(e.dead)
	return c
}

// NewObj returns an empty object.
func NewObj[V any]() *Obj[V] {
	return &Obj[V]{}
}

// NewObjCap returns an empty object with backing capacity for n properties.
func NewObjCap[V any](n int) *Obj[V] {
	if n <= 0 {
		return &Obj[V]{}
	}
	return &Obj[V]{
		keys: make([]string, 0, n),
		vals: make([]V, 0, n),
	}
}

// Len is the number of own properties.
func (o *Obj[V]) Len() int {
	if o.ext == nil {
		return len(o.keys)
	}
	return len(o.keys) - o.ext.nDead
}

// find returns the slot of a key, or -1.
func (o *Obj[V]) find(key string) int {
	if o.index != nil {
		if i, ok := o.index[key]; ok {
			return int(i)
		}
		return -1
	}
	keys := o.keys
	for i := range keys {
		if keys[i] == key {
			if o.ext != nil && o.ext.isDead(i) {
				continue
			}
			return i
		}
	}
	return -1
}

// Has reports whether key is an own property.
func (o *Obj[V]) Has(key string) bool {
	return o.find(key) >= 0
}

// ownVals gives a copy-on-write object its own vals slice. When cowDepth > 0,
// any []any elements are cloned with assignWithDepth deduplication semantics
// (or deep Clone semantics when objCOWClone is set).
func (o *Obj[V]) ownVals() {
	if o.flags&objSharedVals == 0 {
		return
	}
	vals := make([]V, len(o.vals))
	copy(vals, o.vals)
	if o.cowDepth > 0 {
		if o.flags&objCOWClone != 0 {
			for i := range vals {
				if arr, ok := any(vals[i]).([]any); ok {
					vals[i] = any(Clone(arr)).(V)
				}
			}
		} else {
			for i := range vals {
				if arr, ok := any(vals[i]).([]any); ok {
					out := make([]any, 0, len(arr))
					for _, e := range arr {
						if !arrayIncludes(out, e) {
							out = append(out, e)
						}
					}
					vals[i] = any(out[:len(out):len(out)]).(V)
				}
			}
		}
	}
	o.vals = vals
	o.flags &^= objSharedVals
}

// readSlotCOW lazily materializes a copy-on-write child object or slice.
func (o *Obj[V]) readSlotCOW(i int) V {
	v := o.vals[i]
	switch x := any(v).(type) {
	case *Obj[any]:
		if x != nil && x.flags&objFrozen != 0 {
			if o.flags&objSharedVals != 0 {
				o.ownVals()
			}
			var child *Obj[any]
			if o.flags&objCOWClone != 0 {
				child = newCOWClone(x)
			} else {
				child = NewCOW(x, o.cowDepth-1)
			}
			o.vals[i] = any(child).(V)
			return o.vals[i]
		}
	case []any:
		if o.flags&objSharedVals != 0 {
			o.ownVals()
			return o.vals[i]
		}
	}
	return v
}

// materializeCOW resolves all lazy copy-on-write children on o.
func (o *Obj[V]) materializeCOW() {
	if o.cowDepth == 0 {
		return
	}
	for i := range o.vals {
		if o.ext != nil && o.ext.isDead(i) {
			continue
		}
		_ = o.readSlotCOW(i)
	}
	if o.flags&objSharedVals != 0 {
		o.ownVals()
	}
	o.cowDepth = 0
}

// Get returns the value for key, or the zero value.
func (o *Obj[V]) Get(key string) V {
	if i := o.find(key); i >= 0 {
		if o.cowDepth == 0 {
			return o.vals[i]
		}
		return o.readSlotCOW(i)
	}
	var zero V
	return zero
}

// Lookup returns the value for key and whether the key exists.
func (o *Obj[V]) Lookup(key string) (V, bool) {
	if i := o.find(key); i >= 0 {
		if o.cowDepth == 0 {
			return o.vals[i], true
		}
		return o.readSlotCOW(i), true
	}
	var zero V
	return zero, false
}

// Set assigns a property. A new key goes to the end of its group.
func (o *Obj[V]) Set(key string, v V) {
	if o.flags&objFrozen != 0 {
		panic("js: Set on a frozen object")
	}
	o.srcID = 0
	if o.flags&objSharedVals != 0 {
		o.ownVals()
	}
	if i := o.find(key); i >= 0 {
		o.vals[i] = v
		return
	}
	o.add(key, v)
}

// add appends a property whose key the object does not have.
func (o *Obj[V]) add(key string, v V) {
	if o.flags&objSharedKeys != 0 {
		o.ownKeys()
	}
	if o.flags&objSharedVals != 0 {
		o.ownVals()
	}
	slot := len(o.keys)
	if cap(o.keys) == 0 {
		// Most objects get more than one property.
		o.keys = make([]string, 0, 4)
	}
	if cap(o.vals) == 0 {
		o.vals = make([]V, 0, 4)
	}
	o.keys = append(o.keys, key)
	o.vals = append(o.vals, v)
	if num, isIdx := IsArrayIndex(key); isIdx {
		if o.ext == nil {
			o.ext = &objExt{}
		}
		e := o.ext
		if n := len(e.idx); n > 0 && e.idx[n-1].num > num {
			e.idxDirt = true
		}
		e.idx = append(e.idx, idxSlot{num, int32(slot)})
	}
	if o.index != nil {
		o.index[key] = int32(slot)
	} else if slot >= objSmall {
		o.buildIndex()
	}
}

// buildIndex makes the index of an object that is no longer small.
func (o *Obj[V]) buildIndex() {
	index := make(map[string]int32, 2*len(o.keys))
	for i, k := range o.keys {
		if o.ext != nil && o.ext.isDead(i) {
			continue
		}
		index[k] = int32(i)
	}
	o.index = index
}

// ownKeys gives the object its own keys, index and ext, so that it can add
// and delete keys.
func (o *Obj[V]) ownKeys() {
	n := len(o.keys)
	keys := make([]string, n, n+max(n/4, 4))
	copy(keys, o.keys)
	o.keys = keys
	if o.index != nil {
		o.index = maps.Clone(o.index)
	}
	if o.ext != nil {
		o.ext = o.ext.clone()
	}
	o.flags &^= objSharedKeys
}

// Delete removes a property. A key that is set again later goes to the end.
func (o *Obj[V]) Delete(key string) {
	slot := o.find(key)
	if slot < 0 {
		return
	}
	if o.flags&objFrozen != 0 {
		panic("js: Delete on a frozen object")
	}
	o.srcID = 0
	if o.flags&objSharedKeys != 0 {
		o.ownKeys()
	}
	if o.flags&objSharedVals != 0 {
		o.ownVals()
	}
	o.gen++
	var zero V
	o.vals[slot] = zero
	if o.index != nil {
		delete(o.index, key)
	}
	if o.ext == nil {
		o.ext = &objExt{}
	}
	e := o.ext
	if _, isIdx := IsArrayIndex(key); isIdx {
		for i := range e.idx {
			if int(e.idx[i].slot) == slot {
				e.idx = slices.Delete(e.idx, i, i+1)
				break
			}
		}
	}
	e.nDead++
	if e.nDead == len(o.keys) {
		// The object is empty. New arrays: a reader can hold the old ones.
		o.keys, o.vals, o.index = nil, nil, nil
		e.idx, e.idxDirt, e.dead, e.nDead = nil, false, nil, 0
		return
	}
	for w := slot >> 6; w >= len(e.dead); {
		e.dead = append(e.dead, 0)
	}
	e.dead[slot>>6] |= 1 << (uint(slot) & 63)
	if e.nDead > 8 && e.nDead*2 > len(o.keys) {
		o.compact()
	}
}

// compact removes the deleted slots. It makes new arrays: a reader can hold
// the old ones.
func (o *Obj[V]) compact() {
	e := o.ext
	live := len(o.keys) - e.nDead
	keys := make([]string, 0, live+max(live/4, 4))
	vals := make([]V, 0, cap(keys))
	var idx []idxSlot
	for i, k := range o.keys {
		if e.isDead(i) {
			continue
		}
		if num, isIdx := IsArrayIndex(k); isIdx {
			idx = append(idx, idxSlot{num, int32(len(keys))})
		}
		keys = append(keys, k)
		vals = append(vals, o.vals[i])
	}
	o.keys, o.vals = keys, vals
	o.flags &^= objSharedVals
	// idx is now in the order of the slots. A sorted idx stays sorted.
	e.idx = idx
	if !e.idxDirt {
		sortIdx(e.idx)
	}
	e.dead, e.nDead = nil, 0
	o.index = nil
	if len(keys) > objSmall {
		o.buildIndex()
	}
	o.gen++
}

func sortIdx(idx []idxSlot) {
	sort.Slice(idx, func(i, j int) bool { return idx[i].num < idx[j].num })
}

// plain reports whether the slots are the properties in enumeration order:
// no index key and no deleted slot.
func (o *Obj[V]) plain() bool {
	return o.ext == nil || (len(o.ext.idx) == 0 && o.ext.nDead == 0)
}

// Keys returns the keys in JavaScript enumeration order. The result is a copy.
func (o *Obj[V]) Keys() []string {
	if o.plain() {
		keys := make([]string, len(o.keys))
		copy(keys, o.keys)
		return keys
	}
	e := o.ext
	if e.idxDirt {
		sortIdx(e.idx)
		e.idxDirt = false
	}
	keys := make([]string, 0, o.Len())
	for _, s := range e.idx {
		keys = append(keys, o.keys[s.slot])
	}
	if len(keys) == o.Len() {
		return keys
	}
	hasIdx := len(e.idx) > 0
	for i, k := range o.keys {
		if e.nDead > 0 && e.isDead(i) {
			continue
		}
		if hasIdx && k != "" && k[0] >= '0' && k[0] <= '9' {
			if _, isIdx := IsArrayIndex(k); isIdx {
				continue
			}
		}
		keys = append(keys, k)
	}
	return keys
}

// Values returns the values in JavaScript enumeration order.
func (o *Obj[V]) Values() []V {
	if o.cowDepth > 0 {
		o.materializeCOW()
	}
	if o.plain() {
		vals := make([]V, len(o.vals))
		copy(vals, o.vals)
		return vals
	}
	keys := o.Keys()
	vals := make([]V, len(keys))
	for i, k := range keys {
		vals[i] = o.vals[o.find(k)]
	}
	return vals
}

// Each calls fn for every property in enumeration order. The key set is
// fixed when Each starts, as it is for Object.keys(o).forEach.
func (o *Obj[V]) Each(fn func(key string, v V)) {
	if !o.plain() {
		for _, k := range o.Keys() {
			if i := o.find(k); i >= 0 {
				if o.cowDepth == 0 {
					fn(k, o.vals[i])
				} else {
					fn(k, o.readSlotCOW(i))
				}
			}
		}
		return
	}
	// The slots are the properties in enumeration order. The array of keys
	// does not change in place, so the slice is the key set of this moment.
	keys := o.keys
	gen := o.gen
	if o.cowDepth == 0 {
		for i, k := range keys {
			if o.gen == gen {
				fn(k, o.vals[i])
			} else if j := o.find(k); j >= 0 {
				fn(k, o.vals[j])
			}
		}
		return
	}
	for i, k := range keys {
		if o.gen == gen {
			fn(k, o.readSlotCOW(i))
		} else if j := o.find(k); j >= 0 {
			fn(k, o.readSlotCOW(j))
		}
	}
}

// Copy returns a new object with the properties of o in the same order: a
// shallow copy, as { ...o } makes. The copy shares the keys with o until one
// of the two adds or deletes a key.
//
// Copy marks an object that is not frozen as shared. So an object that many
// goroutines read must be frozen before they copy it.
func (o *Obj[V]) Copy() *Obj[V] {
	if o.cowDepth > 0 {
		o.materializeCOW()
	}
	c := &Obj[V]{}
	o.copyInto(c, make([]V, len(o.vals)))
	return c
}

// copyOwned is Copy for an object that other goroutines may read and that
// is not frozen: it does not write to o, and the copy has its own keys
// unless o already shares immutable keys.
func (o *Obj[V]) copyOwned() *Obj[V] {
	if o.flags&objFrozen != 0 {
		return o.Copy()
	}
	if o.cowDepth > 0 {
		o.materializeCOW()
	}
	if o.flags&objSharedKeys != 0 && (o.ext == nil || !o.ext.idxDirt) {
		return &Obj[V]{
			keys:  o.keys,
			vals:  slices.Clone(o.vals),
			index: o.index,
			ext:   o.ext,
			flags: objSharedKeys,
		}
	}
	c := &Obj[V]{keys: slices.Clone(o.keys), vals: slices.Clone(o.vals)}
	if o.index != nil {
		c.index = maps.Clone(o.index)
	}
	if o.ext != nil {
		c.ext = o.ext.clone()
	}
	return c
}

// copyInto makes c a copy of o. vals becomes the values of c: its length is
// the number of slots of o.
func (o *Obj[V]) copyInto(c *Obj[V], vals []V) {
	if o.flags&objFrozen == 0 {
		if o.cowDepth > 0 {
			o.materializeCOW()
		}
		// Readers of a shared ext must not sort it.
		if e := o.ext; e != nil && e.idxDirt {
			sortIdx(e.idx)
			e.idxDirt = false
		}
		o.flags |= objSharedKeys
	}
	copy(vals, o.vals)
	c.keys, c.index, c.ext = o.keys, o.index, o.ext
	c.vals = vals[:len(o.vals):len(o.vals)]
	c.gen = 0
	c.flags = objSharedKeys
	c.cowDepth = 0
	c.srcID = o.srcID
}

// Freeze makes the object immutable: Set and Delete panic from now on. A
// frozen object can be read by many goroutines at the same time, and Copy
// gives a goroutine an object of its own. Freeze is not deep: see
// FreezeValue for a tree of dynamic values.
func (o *Obj[V]) Freeze() {
	if o.flags&objFrozen != 0 {
		return
	}
	if o.cowDepth > 0 {
		o.materializeCOW()
	}
	if o.srcID == 0 {
		o.srcID = nextObjSourceID.Add(1)
	}
	if e := o.ext; e != nil {
		if o.flags&objSharedKeys != 0 {
			o.ownKeys()
			e = o.ext
		}
		if e.nDead > 0 {
			o.compact()
		}
		if e.idxDirt {
			sortIdx(e.idx)
			e.idxDirt = false
		}
	}
	o.flags |= objFrozen | objSharedKeys
}

// Frozen reports whether the object is immutable.
func (o *Obj[V]) Frozen() bool { return o.flags&objFrozen != 0 }

func uitoa(v uint32) string {
	if v == 0 {
		return "0"
	}
	var buf [10]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

// Map is an insertion-ordered map. It has the iteration order of an
// ECMAScript Map: every key in insertion order, with no special case for
// integer keys. A deleted key that is set again goes to the end.
type Map[K comparable, V any] struct {
	m    map[K]V
	keys []K
	dead int
}

// NewMap returns an empty map.
func NewMap[K comparable, V any]() *Map[K, V] {
	return &Map[K, V]{m: map[K]V{}}
}

func (m *Map[K, V]) Len() int { return len(m.m) }

func (m *Map[K, V]) Has(key K) bool {
	_, ok := m.m[key]
	return ok
}

func (m *Map[K, V]) Get(key K) V { return m.m[key] }

func (m *Map[K, V]) Lookup(key K) (V, bool) {
	v, ok := m.m[key]
	return v, ok
}

func (m *Map[K, V]) Set(key K, v V) {
	if _, ok := m.m[key]; !ok {
		m.keys = append(m.keys, key)
	}
	m.m[key] = v
}

func (m *Map[K, V]) Delete(key K) {
	if _, ok := m.m[key]; !ok {
		return
	}
	delete(m.m, key)
	if i := slices.Index(m.keys, key); i >= 0 {
		m.keys = slices.Delete(m.keys, i, i+1)
	}
}

// Clear removes every entry.
func (m *Map[K, V]) Clear() {
	m.m = map[K]V{}
	m.keys = nil
}

// Keys returns the keys in insertion order. The result is a copy.
func (m *Map[K, V]) Keys() []K { return slices.Clone(m.keys) }

// Values returns the values in insertion order.
func (m *Map[K, V]) Values() []V {
	vals := make([]V, len(m.keys))
	for i, k := range m.keys {
		vals[i] = m.m[k]
	}
	return vals
}

// Each calls fn for every entry in insertion order.
func (m *Map[K, V]) Each(fn func(key K, v V)) {
	for _, k := range m.Keys() {
		if v, ok := m.m[k]; ok {
			fn(k, v)
		}
	}
}

// Set is an insertion-ordered set with the iteration order of an ECMAScript Set.
type Set[K comparable] struct {
	m Map[K, struct{}]
}

// NewSet returns a set that holds the given values.
func NewSet[K comparable](values ...K) *Set[K] {
	s := &Set[K]{m: Map[K, struct{}]{m: map[K]struct{}{}}}
	for _, v := range values {
		s.Add(v)
	}
	return s
}

func (s *Set[K]) Len() int       { return s.m.Len() }
func (s *Set[K]) Has(key K) bool { return s.m.Has(key) }
func (s *Set[K]) Add(key K)      { s.m.Set(key, struct{}{}) }
func (s *Set[K]) Delete(key K)   { s.m.Delete(key) }
func (s *Set[K]) Values() []K    { return s.m.Keys() }
