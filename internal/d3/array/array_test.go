package array

import (
	"fmt"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/internal/d3test"
)

type golden struct {
	Math   d3test.Math
	Groups struct {
		Ticks []struct {
			A    d3test.FS
			T    *d3test.FS
			Inc  d3test.F
			Step d3test.F
			Nice *d3test.FS
		}
		Stats []struct {
			V                     d3test.FS
			Min, Max              d3test.F
			OK                    bool
			E                     d3test.FS
			Sum, Min2, Max2, Sum2 d3test.F
		}
		Ranges []struct {
			A d3test.FS
			R *d3test.FS
		}
		Bisects []struct {
			A      d3test.FS
			X      d3test.F
			Lo, Hi int
			R      []int
		}
		Orders []struct{ A, B, Asc, Desc d3test.F }
	}
}

// mathExceptions lists, for each build mode (see d3test.Mode), the number of
// golden cases that differ from Node.js only because a Math function returns
// another last bit. The test proves each one: the case passes when it runs
// again with the results that Node.js returned for the same calls. Measured
// with go1.27.3.
//
// d3-array calls Math.log10 and Math.pow(10, n). Package jsm has the exact
// values for the powers of ten, so that no case differs on arm64.
var mathExceptions = map[string]map[string]int{
	"arm64":       {},
	"arm64/nofma": {},
	// On amd64, math.Log10 of a subnormal number is wrong by about 13
	// decimal orders (for example log10(1e-321) gives about -308). The
	// port does not call it: Math.log10 comes from internal/js/crmath, which
	// returns the bits of Chromium on every CPU. So no case differs.
	"amd64": {},
}

// TestAgainstD3 compares with values that tools/reference/gen-d3-array.mjs
// recorded from d3-array.
func TestAgainstD3(t *testing.T) {
	var g golden
	d3test.Load(t, "testdata/array.json.gz", &g)
	c := d3test.NewChecker(t, g.Math)

	for i, tc := range g.Groups.Ticks {
		start, stop, count := tc.A[0], tc.A[1], tc.A[2]
		name := fmt.Sprintf("#%d (%v, %v, %v)", i, start, stop, count)
		c.Check("ticks", name, func() string {
			got := Ticks(start, stop, count)
			if tc.T == nil {
				if got != nil {
					return fmt.Sprintf("got %v, want nil (JavaScript throws)", got)
				}
				return ""
			}
			if got == nil || !d3test.SameSlice(got, *tc.T) {
				return fmt.Sprintf("got %v, want %v", got, *tc.T)
			}
			return ""
		})
		c.Check("tickIncrement", name, func() string {
			if got := TickIncrement(start, stop, count); !d3test.Same(got, tc.Inc.V()) {
				return fmt.Sprintf("got %v, want %v", got, tc.Inc.V())
			}
			return ""
		})
		c.Check("tickStep", name, func() string {
			if got := TickStep(start, stop, count); !d3test.Same(got, tc.Step.V()) {
				return fmt.Sprintf("got %v, want %v", got, tc.Step.V())
			}
			return ""
		})
		if tc.Nice != nil {
			c.Check("nice", name, func() string {
				a, b := Nice(start, stop, count)
				if !d3test.SameSlice([]float64{a, b}, *tc.Nice) {
					return fmt.Sprintf("got %v %v, want %v", a, b, *tc.Nice)
				}
				return ""
			})
		}
	}

	twice := func(d float64, _ int, _ []float64) float64 { return float64(d * 2) }
	for i, tc := range g.Groups.Stats {
		name := fmt.Sprintf("#%d %v", i, tc.V)
		c.Check("min/max/extent/sum", name, func() string {
			min, okMin := Min(tc.V)
			max, okMax := Max(tc.V)
			lo, hi, okExt := Extent(tc.V)
			min2, _ := MinFunc(tc.V, twice)
			max2, _ := MaxFunc(tc.V, twice)
			lo2, hi2, _ := ExtentFunc(tc.V, twice)
			got := []float64{min, max, lo, hi, Sum(tc.V), min2, max2, SumFunc(tc.V, twice), lo2, hi2}
			want := []float64{tc.Min.V(), tc.Max.V(), tc.E[0], tc.E[1], tc.Sum.V(), tc.Min2.V(), tc.Max2.V(), tc.Sum2.V(), tc.Min2.V(), tc.Max2.V()}
			if !d3test.SameSlice(got, want) || okMin != tc.OK || okMax != tc.OK || okExt != tc.OK {
				return fmt.Sprintf("got %v %v, want %v %v", got, okMin, want, tc.OK)
			}
			return ""
		})
	}

	for i, tc := range g.Groups.Ranges {
		name := fmt.Sprintf("#%d %v", i, tc.A)
		c.Check("range", name, func() string {
			got := Range(tc.A[0], tc.A[1], tc.A[2])
			if tc.R == nil {
				if got != nil {
					return fmt.Sprintf("got %v, want nil (JavaScript throws)", got)
				}
				return ""
			}
			if got == nil || !d3test.SameSlice(got, *tc.R) {
				return fmt.Sprintf("got %v, want %v", got, *tc.R)
			}
			return ""
		})
	}

	type obj struct{ v float64 }
	acc := NewBisector(func(d obj) float64 { return d.v })
	cmp := NewBisectorFunc(func(d obj, x float64) float64 { return d.v - x })
	for i, tc := range g.Groups.Bisects {
		name := fmt.Sprintf("#%d %v x=%v lo=%d hi=%d", i, tc.A, tc.X, tc.Lo, tc.Hi)
		c.Check("bisect", name, func() string {
			objs := make([]obj, len(tc.A))
			for j, v := range tc.A {
				objs[j] = obj{v}
			}
			x := tc.X.V()
			got := []int{
				BisectLeft(tc.A, x, tc.Lo, tc.Hi), BisectRight(tc.A, x, tc.Lo, tc.Hi), BisectCenter(tc.A, x, tc.Lo, tc.Hi),
				acc.Left(objs, x, tc.Lo, tc.Hi), acc.Right(objs, x, tc.Lo, tc.Hi), acc.Center(objs, x, tc.Lo, tc.Hi),
				cmp.Left(objs, x, tc.Lo, tc.Hi), cmp.Right(objs, x, tc.Lo, tc.Hi), cmp.Center(objs, x, tc.Lo, tc.Hi),
			}
			if fmt.Sprint(got) != fmt.Sprint(tc.R) {
				return fmt.Sprintf("got %v, want %v", got, tc.R)
			}
			return ""
		})
	}

	for i, tc := range g.Groups.Orders {
		c.Check("ascending/descending", fmt.Sprintf("#%d %v %v", i, tc.A, tc.B), func() string {
			asc, desc := Ascending(tc.A.V(), tc.B.V()), Descending(tc.A.V(), tc.B.V())
			if !d3test.Same(asc, tc.Asc.V()) || !d3test.Same(desc, tc.Desc.V()) {
				return fmt.Sprintf("got %v %v, want %v %v", asc, desc, tc.Asc, tc.Desc)
			}
			return ""
		})
	}

	c.Report()
	c.CheckExceptions(mathExceptions)
}
