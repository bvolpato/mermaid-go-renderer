package js

import (
	"fmt"
	"math/rand"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
)

// refObj is the first implementation of Obj: a Go map plus the key order.
// It is simple enough to read as the definition of the behaviour, and
// TestObjOrder and TestAgainstV8 tie that behaviour to JavaScript. The tests
// below run the same operations on a refObj and on an Obj and compare
// everything that a caller can see.
type refObj[V any] struct {
	m       map[string]V
	strKeys []string
	idxKeys []uint32
}

func newRefObj[V any]() *refObj[V] { return &refObj[V]{m: map[string]V{}} }

func (o *refObj[V]) Len() int { return len(o.m) }

func (o *refObj[V]) Has(key string) bool {
	_, ok := o.m[key]
	return ok
}

func (o *refObj[V]) Lookup(key string) (V, bool) {
	v, ok := o.m[key]
	return v, ok
}

func (o *refObj[V]) Set(key string, v V) {
	if _, ok := o.m[key]; !ok {
		if idx, isIdx := IsArrayIndex(key); isIdx {
			o.idxKeys = append(o.idxKeys, idx)
		} else {
			o.strKeys = append(o.strKeys, key)
		}
	}
	o.m[key] = v
}

func (o *refObj[V]) Delete(key string) {
	if _, ok := o.m[key]; !ok {
		return
	}
	delete(o.m, key)
	if idx, isIdx := IsArrayIndex(key); isIdx {
		if i := slices.Index(o.idxKeys, idx); i >= 0 {
			o.idxKeys = slices.Delete(o.idxKeys, i, i+1)
		}
		return
	}
	if i := slices.Index(o.strKeys, key); i >= 0 {
		o.strKeys = slices.Delete(o.strKeys, i, i+1)
	}
}

func (o *refObj[V]) Keys() []string {
	sort.Slice(o.idxKeys, func(i, j int) bool { return o.idxKeys[i] < o.idxKeys[j] })
	keys := make([]string, 0, len(o.m))
	for _, idx := range o.idxKeys {
		keys = append(keys, uitoa(idx))
	}
	return append(keys, o.strKeys...)
}

func (o *refObj[V]) Values() []V {
	keys := o.Keys()
	vals := make([]V, len(keys))
	for i, k := range keys {
		vals[i] = o.m[k]
	}
	return vals
}

func (o *refObj[V]) Each(fn func(key string, v V)) {
	for _, k := range o.Keys() {
		if v, ok := o.m[k]; ok {
			fn(k, v)
		}
	}
}

func (o *refObj[V]) Copy() *refObj[V] {
	c := newRefObj[V]()
	o.Each(func(k string, v V) { c.Set(k, v) })
	return c
}

// objKeyPool makes keys of the kinds that the order rules tell apart: array
// indices, numbers that are not array indices, and names.
func objKeyPool(rng *rand.Rand, n int) []string {
	keys := make([]string, n)
	for i := range keys {
		switch rng.Intn(6) {
		case 0, 1:
			keys[i] = fmt.Sprint(rng.Intn(3 * n))
		case 2:
			keys[i] = []string{"01", "-1", "1.5", "4294967295", "4294967294", "", "1e3", "+2", " 3"}[rng.Intn(9)]
		default:
			keys[i] = fmt.Sprintf("k%d", rng.Intn(3*n))
		}
	}
	return keys
}

// objPair is an Obj and the reference object that had the same operations.
type objPair struct {
	got  *Obj[int]
	want *refObj[int]
}

func (p objPair) check(t *testing.T, what string) {
	t.Helper()
	if p.got.Len() != p.want.Len() {
		t.Fatalf("%s: Len() = %d, want %d", what, p.got.Len(), p.want.Len())
	}
	wantKeys := p.want.Keys()
	if gotKeys := p.got.Keys(); !slices.Equal(gotKeys, wantKeys) {
		t.Fatalf("%s: Keys() = %q, want %q", what, gotKeys, wantKeys)
	}
	if got, want := p.got.Values(), p.want.Values(); !slices.Equal(got, want) {
		t.Fatalf("%s: Values() = %v, want %v", what, got, want)
	}
	var gotEach, wantEach []string
	p.got.Each(func(k string, v int) { gotEach = append(gotEach, fmt.Sprint(k, "=", v)) })
	p.want.Each(func(k string, v int) { wantEach = append(wantEach, fmt.Sprint(k, "=", v)) })
	if !slices.Equal(gotEach, wantEach) {
		t.Fatalf("%s: Each gave %q, want %q", what, gotEach, wantEach)
	}
	for _, k := range wantKeys {
		gv, gok := p.got.Lookup(k)
		wv, wok := p.want.Lookup(k)
		if gv != wv || gok != wok || p.got.Get(k) != wv || !p.got.Has(k) {
			t.Fatalf("%s: Lookup(%q) = %v, %v, want %v, %v", what, k, gv, gok, wv, wok)
		}
	}
}

// TestObjAgainstModel runs random operations, including changes inside the
// callback of Each and copies, on Obj and on the reference object.
func TestObjAgainstModel(t *testing.T) {
	for seed := int64(1); seed <= 300; seed++ {
		rng := rand.New(rand.NewSource(seed))
		// Small pools make small objects, large pools reach the index and
		// the removal of deleted slots.
		pool := objKeyPool(rng, []int{3, 6, 12, 40, 200}[seed%5])
		pairs := []objPair{{NewObj[int](), newRefObj[int]()}}
		steps := 400
		for step := 0; step < steps; step++ {
			p := pairs[rng.Intn(len(pairs))]
			key := pool[rng.Intn(len(pool))]
			what := fmt.Sprintf("seed %d step %d", seed, step)
			switch op := rng.Intn(20); {
			case op < 9:
				p.got.Set(key, step)
				p.want.Set(key, step)
			case op < 14:
				p.got.Delete(key)
				p.want.Delete(key)
			case op < 15:
				gv, gok := p.got.Lookup(key)
				wv, wok := p.want.Lookup(key)
				if gv != wv || gok != wok || p.got.Has(key) != wok || p.got.Get(key) != wv {
					t.Fatalf("%s: Lookup(%q) = %v, %v, want %v, %v", what, key, gv, gok, wv, wok)
				}
			case op < 16:
				p.check(t, what)
			case op < 17:
				if len(pairs) < 6 {
					pairs = append(pairs, objPair{p.got.Copy(), p.want.Copy()})
				}
			case op < 18:
				// The callback changes the object: it sets and deletes keys
				// that the walk has passed and keys that it has not reached.
				plan := make([]int, 64)
				for i := range plan {
					plan[i] = rng.Intn(1 << 20)
				}
				run := func(set func(string, int), del func(string), each func(func(string, int))) []string {
					var seen []string
					n := 0
					each(func(k string, v int) {
						seen = append(seen, fmt.Sprint(k, "=", v))
						r := plan[n%len(plan)]
						n++
						k2 := pool[r%len(pool)]
						switch (r >> 8) % 4 {
						case 0:
							set(k2, -n)
						case 1:
							del(k2)
						case 2:
							del(k2)
							set(k2, -n)
						}
					})
					return seen
				}
				got := run(p.got.Set, p.got.Delete, p.got.Each)
				want := run(p.want.Set, p.want.Delete, p.want.Each)
				if !slices.Equal(got, want) {
					t.Fatalf("%s: Each with changes gave %q, want %q", what, got, want)
				}
			default:
				// Delete many keys, to reach the removal of deleted slots.
				for _, k := range pool[:rng.Intn(len(pool)+1)] {
					p.got.Delete(k)
					p.want.Delete(k)
				}
			}
		}
		for i, p := range pairs {
			p.check(t, fmt.Sprintf("seed %d, object %d at the end", seed, i))
		}
	}
}

// TestObjKeysIsACopy checks that the result of Keys and Values belongs to
// the caller.
func TestObjKeysIsACopy(t *testing.T) {
	for _, n := range []int{0, 3, 20} {
		o := NewObj[int]()
		for i := 0; i < n; i++ {
			o.Set(fmt.Sprintf("k%d", i), i)
		}
		keys, vals := o.Keys(), o.Values()
		if keys == nil || vals == nil {
			t.Fatalf("%d keys: Keys or Values is nil", n)
		}
		for i := range keys {
			keys[i], vals[i] = "changed", -1
		}
		for i, k := range o.Keys() {
			if want := fmt.Sprintf("k%d", i); k != want || o.Get(k) != i {
				t.Fatalf("%d keys: key %d is %q = %d after the caller changed the result", n, i, k, o.Get(k))
			}
		}
	}
}

// TestObjFreeze checks that a frozen object rejects changes, that a copy of
// it is a separate object that can change, and that neither changes the
// other.
func TestObjFreeze(t *testing.T) {
	for _, n := range []int{0, 2, 9, 50} {
		o := NewObj[int]()
		for i := 0; i < n; i++ {
			o.Set(fmt.Sprintf("k%d", i), i)
			o.Set(fmt.Sprint(n-i), i)
		}
		o.Delete("k0")
		o.Freeze()
		if !o.Frozen() {
			t.Fatal("Frozen() is false after Freeze")
		}
		want := o.Keys()
		mustPanic(t, "Set of a new key", func() { o.Set("new", 1) })
		if n > 1 {
			mustPanic(t, "Set of a key that exists", func() { o.Set("k1", 1) })
			mustPanic(t, "Delete", func() { o.Delete("k1") })
		}
		// Deleting a key that the object does not have changes nothing.
		o.Delete("absent")

		c := o.Copy()
		if c.Frozen() {
			t.Fatal("the copy of a frozen object is frozen")
		}
		c.Set("new", 1)
		c.Set("7", 7)
		c.Delete("k1")
		if n > 2 {
			c.Set("k2", -2)
			if o.Get("k2") != 2 {
				t.Fatalf("a change of the copy reached the frozen object: k2 = %d", o.Get("k2"))
			}
		}
		if got := o.Keys(); !slices.Equal(got, want) {
			t.Fatalf("the frozen object changed: %q, want %q", got, want)
		}
		if !c.Has("new") || c.Has("k1") {
			t.Fatalf("the copy did not change: %q", c.Keys())
		}
	}
}

func mustPanic(t *testing.T, what string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("%s on a frozen object did not panic", what)
		}
	}()
	fn()
}

// TestObjCopyIsIndependent checks both directions for an object that is not
// frozen: a change of the original does not reach the copy, and a change of
// the copy does not reach the original.
func TestObjCopyIsIndependent(t *testing.T) {
	for _, n := range []int{1, 5, 30} {
		o := NewObj[int]()
		for i := 0; i < n; i++ {
			o.Set(fmt.Sprintf("k%d", i), i)
		}
		c := o.Copy()
		o.Set("o-only", 1)
		o.Set("k0", 100)
		c.Set("c-only", 2)
		c.Delete("k0")
		if c.Has("o-only") || o.Has("c-only") || !o.Has("k0") || c.Has("k0") || o.Get("k0") != 100 {
			t.Fatalf("%d keys: the original has %q, the copy has %q", n, o.Keys(), c.Keys())
		}
		if o.Len() != n+1 || c.Len() != n {
			t.Fatalf("%d keys: Len is %d and %d", n, o.Len(), c.Len())
		}
	}
}

// TestFrozenObjConcurrentReads reads and copies a frozen tree from many
// goroutines. The race detector finds a read that writes.
func TestFrozenObjConcurrentReads(t *testing.T) {
	root := NewObj[any]()
	for i := 0; i < 40; i++ {
		child := NewObj[any]()
		for j := 0; j < 12; j++ {
			child.Set(fmt.Sprint(11-j), float64(j))
			child.Set(fmt.Sprintf("p%d", j), []any{float64(j), "x"})
		}
		root.Set(fmt.Sprintf("c%d", i), child)
	}
	FreezeValue(root)
	want := StringifyJSON(root)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				if got := StringifyJSON(root); got != want {
					t.Errorf("a reader saw another tree")
					return
				}
				c := Clone(root).(*Obj[any])
				child := c.Get("c1").(*Obj[any])
				child.Set("mine", float64(g))
				child.Delete("3")
				c.Set(fmt.Sprintf("g%d", g), true)
				if strings.Contains(StringifyJSON(root), "mine") {
					t.Errorf("a change of a clone reached the frozen tree")
					return
				}
			}
		}(g)
	}
	wg.Wait()
	if got := StringifyJSON(root); got != want {
		t.Fatal("the frozen tree changed")
	}
}

// TestFreezeValueSharedPart freezes new trees that have a part of a frozen
// tree in them, while other goroutines read the frozen tree. A config has
// such a part: its deep values are the values of the config that it comes
// from. The race detector finds a freeze that writes to the frozen part.
func TestFreezeValueSharedPart(t *testing.T) {
	shared := NewObj[any]()
	shared.Set("list", []any{"a", float64(1), []any{"b"}})
	inner := NewObj[any]()
	inner.Set("k", []any{"c"})
	shared.Set("object", inner)
	array := []any{"d", shared, []any{"e", "f"}}
	FreezeValue(shared)
	array = FreezeValue(array).([]any)
	want := StringifyJSON(array)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				if g%2 == 0 {
					if got := StringifyJSON(array); got != want {
						t.Errorf("a reader saw another tree")
						return
					}
					continue
				}
				root := NewObj[any]()
				root.Set("shared", shared)
				root.Set("array", array)
				root.Set("own", append(make([]any, 0, 4), "x", shared.Get("list")))
				FreezeValue(root)
				if own := root.Get("own").([]any); cap(own) != len(own) {
					t.Errorf("the array of the new tree has spare capacity")
					return
				}
			}
		}(g)
	}
	wg.Wait()
	if got := StringifyJSON(array); got != want {
		t.Fatal("the frozen tree changed")
	}
}

// TestCloneMatchesModel checks that Clone gives the properties of the
// original in the same order, for objects with index keys and with deleted
// keys, and that the clone is deep.
func TestCloneMatchesModel(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	var build func(depth int) any
	build = func(depth int) any {
		o := NewObj[any]()
		pool := objKeyPool(rng, 10)
		for i := 0; i < 14; i++ {
			k := pool[rng.Intn(len(pool))]
			switch {
			case depth > 0 && rng.Intn(4) == 0:
				o.Set(k, build(depth-1))
			case depth > 0 && rng.Intn(5) == 0:
				o.Set(k, []any{float64(i), build(depth - 1)})
			default:
				o.Set(k, float64(i))
			}
			if rng.Intn(4) == 0 {
				o.Delete(pool[rng.Intn(len(pool))])
			}
		}
		return o
	}
	for i := 0; i < 50; i++ {
		v := build(3)
		want := StringifyJSON(v)
		c := Clone(v)
		if got := StringifyJSON(c); got != want {
			t.Fatalf("Clone gave\n %s\nwant\n %s", got, want)
		}
		// Every object of the clone is new.
		var change func(v any)
		change = func(v any) {
			switch x := v.(type) {
			case *Obj[any]:
				for _, k := range x.Keys() {
					change(x.Get(k))
				}
				x.Set("changed", true)
			case []any:
				for i, e := range x {
					change(e)
					x[i] = nil
				}
			}
		}
		change(c)
		if got := StringifyJSON(v); got != want {
			t.Fatalf("a change of the clone reached the original:\n %s\nwant\n %s", got, want)
		}
	}
}
