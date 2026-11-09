package gwt

import (
	"math"
	"testing"
)

// The expected values come from java.util.Random in elk-worker.js of elkjs
// 0.9.3 in node:
//
//	r = new Random_0(fromInt(1)); [nextInt(10) x5, nextDouble, nextFloat,
//	nextBoolean, nextLong hi/lo, nextInt(16)]
//
// See tools/reference/gen-elk-gwt.mjs.
func TestRandomAgainstGWT(t *testing.T) {
	for _, c := range randomGolden {
		r := NewRandom(c.seed)
		for i, want := range c.ints {
			if got := r.NextInt(10); got != want {
				t.Fatalf("seed %d: NextInt(10) #%d = %d, want %d", c.seed, i, got, want)
			}
		}
		if got := r.NextDouble(); math.Float64bits(got) != math.Float64bits(c.double) {
			t.Fatalf("seed %d: NextDouble = %v, want %v", c.seed, got, c.double)
		}
		if got := r.NextFloat(); math.Float64bits(got) != math.Float64bits(c.float) {
			t.Fatalf("seed %d: NextFloat = %v, want %v", c.seed, got, c.float)
		}
		if got := r.NextBoolean(); got != c.boolean {
			t.Fatalf("seed %d: NextBoolean = %v, want %v", c.seed, got, c.boolean)
		}
		long := r.NextLong()
		r2 := NewRandom(long)
		if r2.SeedHi != c.longSeedHi || r2.SeedLo != c.longSeedLo {
			t.Fatalf("seed %d: Random(NextLong()) has state %v %v, want %v %v", c.seed, r2.SeedHi, r2.SeedLo, c.longSeedHi, c.longSeedLo)
		}
		if got := r.NextInt(16); got != c.pow2 {
			t.Fatalf("seed %d: NextInt(16) = %d, want %d", c.seed, got, c.pow2)
		}
		if r.SeedHi != c.endHi || r.SeedLo != c.endLo {
			t.Fatalf("seed %d: end state %v %v, want %v %v", c.seed, r.SeedHi, r.SeedLo, c.endHi, c.endLo)
		}
	}
	for _, c := range randomFromDoubleGolden {
		r := NewRandomFromDouble(c.seed)
		if r.SeedHi != c.hi || r.SeedLo != c.lo {
			t.Fatalf("NewRandomFromDouble(%v) has state %v %v, want %v %v", c.seed, r.SeedHi, r.SeedLo, c.hi, c.lo)
		}
	}
}

func TestSortIsStableMergeSort(t *testing.T) {
	type item struct{ key, seq int }
	var a []item
	state := uint32(7)
	for i := 0; i < 500; i++ {
		state = state*1664525 + 1013904223
		a = append(a, item{int(state>>24) % 13, i})
	}
	SortFunc(a, func(x, y item) int { return IntCompare(x.key, y.key) })
	for i := 1; i < len(a); i++ {
		if a[i-1].key > a[i].key || a[i-1].key == a[i].key && a[i-1].seq > a[i].seq {
			t.Fatalf("not a stable sort at %d: %v %v", i, a[i-1], a[i])
		}
	}
}

// A comparator that is not transitive: the result is the result of the merge
// sort of GWT (from node, see tools/reference/gen-elk-gwt.mjs).
func TestSortWithInconsistentComparator(t *testing.T) {
	a := append([]int(nil), sortGoldenIn...)
	SortFunc(a, func(x, y int) int {
		d := x - y
		if d > -3 && d < 3 {
			return 0
		}
		return d
	})
	for i := range a {
		if a[i] != sortGoldenOut[i] {
			t.Fatalf("sorted = %v, want %v", a, sortGoldenOut)
		}
	}
}

func TestMapOrder(t *testing.T) {
	m := NewMap[int, string]()
	m.Put(5, "a")
	m.Put(1, "b")
	m.Put(9, "c")
	m.Put(1, "d")
	m.Remove(5)
	m.Put(5, "e")
	got := m.Keys()
	want := []int{1, 9, 5}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("keys %v, want %v", got, want)
		}
	}
	// Removal of the current entry during iteration.
	var seen []int
	m.Each(func(k int, _ string) bool {
		seen = append(seen, k)
		m.Remove(k)
		return true
	})
	if len(seen) != 3 || m.Len() != 0 {
		t.Fatalf("seen %v, len %d", seen, m.Len())
	}
}

func TestHashMapChains(t *testing.T) {
	// Double keys: 1.5 and 1.2 have the hash code 1, 7.0 has 7.
	m := NewHashMap[float64, int](DoubleHash, func(a, b float64) bool { return a == b })
	m.Put(1.5, 1)
	m.Put(7, 2)
	m.Put(1.2, 3)
	got := m.Keys()
	want := []float64{1.5, 1.2, 7}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("keys %v, want %v", got, want)
		}
	}
	m.Remove(1.5)
	m.Remove(1.2)
	m.Put(1.5, 4)
	got = m.Keys()
	want = []float64{7, 1.5}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("keys %v, want %v", got, want)
		}
	}
}

func TestNumbers(t *testing.T) {
	if RoundInt(math.NaN()) != 0 || RoundInt(3e10) != 2147483647 || RoundInt(-3e10) != -2147483648 || RoundInt(-2.9) != -2 {
		t.Fatal("RoundInt")
	}
	if DoubleCompare(math.Copysign(0, -1), 0) != -1 || DoubleCompare(math.NaN(), 1) != 1 || DoubleCompare(1, math.NaN()) != -1 {
		t.Fatal("DoubleCompare")
	}
	if StringHash("org.eclipse.elk.padding") != -79508074 || StringHash("a😀") != 1866116 {
		t.Fatalf("StringHash %d %d", StringHash("org.eclipse.elk.padding"), StringHash("a😀"))
	}
	if IntegerReverse(1) != -2147483648 || IntegerReverse(0x12345678) != 0x1e6a2c48 {
		t.Fatal("IntegerReverse")
	}
}
