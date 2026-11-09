package styles

import (
	"math/rand"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// TestMergedProperty compares mergedProperty with the property of the
// whole merge, for random values of every kind that the merge tells apart.
func TestMergedProperty(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	keys := []string{"a", "b", "0", "1", "__proto__", "length", "titleColor"}
	var value func(depth int) any
	value = func(depth int) any {
		switch n := rng.Intn(12); {
		case n < 3 && depth > 0:
			o := js.NewObj[any]()
			for i, count := 0, rng.Intn(4); i < count; i++ {
				o.Set(keys[rng.Intn(len(keys))], value(depth-1))
			}
			return o
		case n < 5 && depth > 0:
			arr := []any{}
			for i, count := 0, rng.Intn(3); i < count; i++ {
				arr = append(arr, value(depth-1))
			}
			return arr
		case n < 6:
			return nil
		case n < 8:
			return js.Null
		case n < 9:
			return true
		case n < 10:
			return float64(rng.Intn(3))
		}
		return []string{"x", "", "red"}[rng.Intn(3)]
	}
	for i := 0; i < 20000; i++ {
		defaults, data := value(3), value(3)
		before := js.StringifyJSON((defaults)) + js.StringifyJSON((data))
		for _, key := range keys {
			want := js.StringifyJSON((cleanAndMerge(defaults, data).Get(key)))
			got := js.StringifyJSON((mergedProperty(defaults, data, key)))
			if got != want {
				t.Fatalf("mergedProperty(%s, %s, %q) = %s, want %s", js.StringifyJSON((defaults)), js.StringifyJSON((data)), key, got, want)
			}
		}
		if after := js.StringifyJSON((defaults)) + js.StringifyJSON((data)); after != before {
			t.Fatalf("mergedProperty changed its arguments")
		}
	}
}
