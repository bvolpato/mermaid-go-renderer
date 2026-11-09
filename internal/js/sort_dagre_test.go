package js

import (
	"encoding/json"
	"math"
	"os"
	"sort"
	"testing"
)

type sortElem struct {
	id  int
	key float64
	nan bool
}

func sortMix(x uint32) uint32 {
	x = (x ^ (x >> 16)) * 0x85ebca6b
	x = (x ^ (x >> 13)) * 0xc2b2ae35
	return x ^ (x >> 16)
}

// sortInput builds the same array as makeInput in tools/reference/gen-jssort.mjs.
func sortInput(n int, seed uint32, shape int) []sortElem {
	arr := make([]sortElem, 0, n)
	runLen := 5 + int(seed%60)
	mod := uint32(n)
	if mod == 0 {
		mod = 1
	}
	for i := 0; i < n; i++ {
		h := sortMix(seed*0x9e3779b1 + uint32(i))
		var key int
		switch shape {
		case 0:
			key = int(h % mod)
		case 1:
			key = int(h % 4)
		case 2:
			key = i % runLen
		case 3:
			if h%10 == 0 {
				key = int(h % mod)
			} else {
				key = n - i
			}
		case 4:
			key = i / 3
		default:
			if (i/runLen)%2 == 0 {
				key = i % runLen
			} else {
				key = runLen - (i % runLen)
			}
		}
		arr = append(arr, sortElem{id: i, key: float64(key), nan: sortMix(uint32(i)^(seed*7))%5 == 0})
	}
	return arr
}

// sortCompare builds the same comparator as makeCompare in gen-jssort.mjs.
func sortCompare(seed uint32, kind int) func(a, b sortElem) float64 {
	pair := func(a, b sortElem) uint32 {
		return sortMix(uint32(a.id)*0x1f123bb5 ^ uint32(b.id)*0x5f356495 ^ seed)
	}
	k := func(e sortElem) float64 {
		if e.nan {
			return math.NaN()
		}
		return e.key
	}
	compareAscending := func(value, other float64) float64 {
		if value != other {
			if value > other || value != value {
				return 1
			}
			if value < other || other != other {
				return -1
			}
		}
		return 0
	}
	switch kind {
	case 0:
		return func(a, b sortElem) float64 { return a.key - b.key }
	case 1:
		return func(a, b sortElem) float64 { return float64(pair(a, b)%3) - 1 }
	case 2:
		return func(a, b sortElem) float64 {
			if k(a) < k(b) {
				return -1
			} else if k(a) > k(b) {
				return 1
			}
			return float64(a.id - b.id)
		}
	case 3:
		return func(a, b sortElem) float64 {
			if k(a) < k(b) {
				return -1
			} else if k(a) > k(b) {
				return 1
			}
			return float64(b.id - a.id)
		}
	case 4:
		return func(a, b sortElem) float64 {
			if pair(a, b)%8 == 0 {
				return b.key - a.key
			}
			return a.key - b.key
		}
	case 5:
		return func(a, b sortElem) float64 {
			if c := compareAscending(k(a), k(b)); c != 0 {
				return c
			}
			return float64(a.id - b.id)
		}
	}
	return func(a, b sortElem) float64 {
		if pair(a, b)%7 == 0 {
			return math.NaN()
		}
		return a.key - b.key
	}
}

func sortHashIDs(arr []sortElem) uint32 {
	h := uint32(0x811c9dc5)
	for _, e := range arr {
		id := uint32(e.id)
		h = (h ^ (id & 0xff)) * 0x01000193
		h = (h ^ ((id >> 8) & 0xff)) * 0x01000193
		h = (h ^ ((id >> 16) & 0xff)) * 0x01000193
	}
	return h
}

type sortGolden struct {
	V8    string
	Cases [][5]float64
}

func loadSortGolden(t *testing.T, name string) *sortGolden {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var g sortGolden
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	return &g
}

// TestSortV8AgainstV8 compares SortV8 with Array.prototype.sort results
// recorded by tools/reference/gen-jssort.mjs. Most comparators in the golden
// file are not consistent, so a pass means that the port makes the same
// comparator calls in the same order as V8.
func TestSortV8AgainstV8(t *testing.T) {
	for _, tc := range []struct {
		file string
		mode V8Sort
	}{
		{"v8_sort_timsort.json", V8TimSort},
		{"v8_sort_powersort.json", V8PowerSort},
	} {
		g := loadSortGolden(t, tc.file)
		if g == nil {
			// A golden file exists only for an engine that was available
			// when the goldens were generated.
			t.Logf("%s: no golden file", tc.file)
			continue
		}
		failed := 0
		for _, c := range g.Cases {
			n, seed, kind, shape, want := int(c[0]), uint32(c[1]), int(c[2]), int(c[3]), uint32(c[4])
			arr := sortInput(n, seed, shape)
			SortV8(tc.mode, arr, sortCompare(seed, kind))
			if got := sortHashIDs(arr); got != want {
				failed++
				if failed <= 10 {
					t.Errorf("%s: n=%d seed=%d kind=%d shape=%d: hash %08x, want %08x", tc.file, n, seed, kind, shape, got, want)
				}
			}
		}
		t.Logf("%s (V8 %s): %d cases, %d failed", tc.file, g.V8, len(g.Cases), failed)
	}
}

// TestSortV8Stable checks both modes against a reference stable sort for a
// consistent comparator. This is the only check of the PowerSort merge
// policy when no V8 14.9+ golden file exists.
func TestSortV8Stable(t *testing.T) {
	for _, mode := range []V8Sort{V8PowerSort, V8TimSort} {
		seed := uint32(12345)
		for n := 0; n < 3000; n += 1 + n/7 {
			for shape := 0; shape < 6; shape++ {
				seed += 0x632be5ab
				in := sortInput(n, seed, shape)
				want := append([]sortElem(nil), in...)
				sort.SliceStable(want, func(i, j int) bool { return want[i].key < want[j].key })
				SortV8(mode, in, func(a, b sortElem) float64 { return a.key - b.key })
				for i := range in {
					if in[i] != want[i] {
						t.Fatalf("mode %d n=%d shape=%d: element %d is %v, want %v", mode, n, shape, i, in[i], want[i])
					}
				}
			}
		}
	}
}

// TestSortV8Permutation checks that an inconsistent comparator never loses
// or duplicates an element.
func TestSortV8Permutation(t *testing.T) {
	for _, mode := range []V8Sort{V8PowerSort, V8TimSort} {
		seed := uint32(777)
		for n := 0; n < 2500; n += 1 + n/5 {
			for kind := 0; kind < 7; kind++ {
				seed += 0x632be5ab
				arr := sortInput(n, seed, kind%6)
				SortV8(mode, arr, sortCompare(seed, kind))
				seen := make([]bool, n)
				for _, e := range arr {
					if seen[e.id] {
						t.Fatalf("mode %d n=%d kind=%d: id %d is duplicated", mode, n, kind, e.id)
					}
					seen[e.id] = true
				}
			}
		}
	}
}

func TestV8NodePower(t *testing.T) {
	// nodePower32 is the bit-by-bit definition (NodePower32 in the Torque source).
	nodePower32 := func(s1, n1, n2, n int) int {
		result := 0
		a := 2*s1 + n1
		b := a + n1 + n2
		for {
			result++
			if a >= n {
				a -= n
				b -= n
			} else if b >= n {
				break
			}
			a <<= 1
			b <<= 1
		}
		return result
	}
	seed := uint32(99)
	for i := 0; i < 20000; i++ {
		seed += 0x632be5ab
		n := 2 + int(sortMix(seed)%100000)
		s1 := int(sortMix(seed+1) % uint32(n-1))
		n1 := 1 + int(sortMix(seed+2)%uint32(n-s1-1))
		if s1+n1 >= n {
			continue
		}
		n2 := 1 + int(sortMix(seed+3)%uint32(n-s1-n1))
		if got, want := v8NodePower(s1, n1, n2, n), nodePower32(s1, n1, n2, n); got != want {
			t.Fatalf("v8NodePower(%d, %d, %d, %d) = %d, want %d", s1, n1, n2, n, got, want)
		}
	}
}
