package scale

import (
	"fmt"
	"math"
	"testing"
	stdtime "time"
	_ "time/tzdata"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/color"
	"github.com/bvolpato/mermaid-go-renderer/internal/d3/internal/d3test"
	"github.com/bvolpato/mermaid-go-renderer/internal/d3/interpolate"
	d3time "github.com/bvolpato/mermaid-go-renderer/internal/d3/time"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

type golden struct {
	Math   d3test.Math
	Groups struct {
		Linear []struct {
			Domain, Range d3test.NS
			Clamp, Round  bool
			Nice          any
			Niced         d3test.NS
			Xs            d3test.NS
			Out           d3test.FS
			Ys            d3test.NS
			Inv           d3test.FS
			Count         *float64
			Ticks         d3test.NS
			Spec          *string
			Copy          d3test.FS
			Tv            d3test.NS
			Tf            *[]string
		}
		Colors []struct {
			Domain d3test.NS
			Range  []string
			Ip     string
			Xs     d3test.NS
			Out    []string
		}
		Bands []struct {
			Point    bool
			Domain   []string
			R        d3test.NS
			Ops      [][]any
			Out      d3test.FS
			Bw, Step d3test.F
			Copy     d3test.FS
			Dom      []string
		}
		Ordinals []struct {
			Domain, Range []string
			Unknown       *string
			Lookups       []string
			Out           []*string
			Dom, CopyDom  []string
		}
		NumberKeys []struct {
			Lookups d3test.NS
			Out     []string
			Dom     d3test.NS
		}
	}
}

type timeGolden struct {
	TZ       string
	Math     d3test.Math
	Defaults struct{ Time, UTC d3test.FS }
	Groups   struct {
		Times []struct {
			Fam          string
			Domain       d3test.FS
			Range        d3test.NS
			Round, Clamp bool
			NiceMode     string
			NiceCount    float64
			NiceIv       string
			NiceK        float64
			Niced        d3test.FS
			Ts, Out      d3test.FS
			Ys           d3test.NS
			Inv, Copy    d3test.FS
			Count        *float64
			Ticks        d3test.FS
			Tf, Tfx      []string
			Spec         string
			Tfs          []string
			Iv           string
			IvK          float64
			IvTicks      *d3test.FS
		}
	}
}

// mathExceptions lists, for each build mode (see d3test.Mode), the number of
// golden cases that differ from Node.js only because a Math function returns
// another last bit. The test proves each one: the case passes when it runs
// again with the results that Node.js returned for the same calls. Measured
// with go1.27.3.
//
// There are none.
var mathExceptions = map[string]map[string]int{
	"arm64":       {},
	"arm64/nofma": {},
	"amd64":       {},
}

func diff(what string, got, want []float64) string {
	if d3test.SameSlice(got, want) {
		return ""
	}
	return fmt.Sprintf("%s: got %v, want %v", what, got, want)
}

func diffS(what string, got, want []string) string {
	if fmt.Sprintf("%q", got) == fmt.Sprintf("%q", want) {
		return ""
	}
	return fmt.Sprintf("%s: got %q, want %q", what, got, want)
}

// jsonNumber reads a number that the generator wrote with nums.
func jsonNumber(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case string:
		switch x {
		case "NaN":
			return math.NaN()
		case "-0":
			return math.Copysign(0, -1)
		}
	}
	return math.NaN()
}

// TestAgainstD3 compares with values that tools/reference/gen-d3-scale.mjs
// recorded from d3-scale.
func TestAgainstD3(t *testing.T) {
	var g golden
	d3test.Load(t, "testdata/scale.json.gz", &g)
	c := d3test.NewChecker(t, g.Math)

	for i, tc := range g.Groups.Linear {
		name := fmt.Sprintf("#%d domain=%v range=%v clamp=%v round=%v nice=%v", i, tc.Domain, tc.Range, tc.Clamp, tc.Round, tc.Nice)
		build := func() *Linear[float64] {
			s := NewLinear().Domain(tc.Domain...)
			if tc.Round {
				s.RangeRound(tc.Range...)
			} else {
				s.Range(tc.Range...)
			}
			s.Clamp(tc.Clamp)
			switch n := tc.Nice.(type) {
			case string: // "default"
				s.Nice()
			case float64:
				s.Nice(n)
			}
			return s
		}
		c.Check("scaleLinear", name, func() string {
			s := build()
			if x := diff("domain after nice", s.DomainValues(), tc.Niced); x != "" {
				return x
			}
			out := make([]float64, len(tc.Xs))
			for j, x := range tc.Xs {
				out[j] = s.Scale(x)
			}
			if x := diff(fmt.Sprintf("scale(%v)", tc.Xs), out, tc.Out); x != "" {
				return x
			}
			inv := make([]float64, len(tc.Ys))
			for j, y := range tc.Ys {
				inv[j] = s.Invert(y)
			}
			if x := diff(fmt.Sprintf("invert(%v)", tc.Ys), inv, tc.Inv); x != "" {
				return x
			}
			cp := s.Copy()
			for j, x := range tc.Xs {
				out[j] = cp.Scale(x)
			}
			return diff("copy", out, tc.Copy)
		})
		c.Check("scaleLinear ticks", name, func() string {
			s := build()
			if tc.Count == nil {
				return diff("ticks()", s.Ticks(), tc.Ticks)
			}
			return diff(fmt.Sprintf("ticks(%v)", *tc.Count), s.Ticks(*tc.Count), tc.Ticks)
		})
		c.Check("scaleLinear tickFormat", name, func() string {
			s := build()
			count := 10.0
			if tc.Count != nil {
				count = *tc.Count
			}
			var f func(float64) string
			var err error
			if tc.Spec == nil {
				f, err = s.TickFormat(count)
			} else {
				f, err = s.TickFormat(count, *tc.Spec)
			}
			if tc.Tf == nil {
				if err == nil {
					return "no error, d3 throws"
				}
				return ""
			}
			if err != nil {
				return err.Error()
			}
			got := make([]string, len(tc.Tv))
			for j, v := range tc.Tv {
				got[j] = f(v)
			}
			return diffS(fmt.Sprintf("tickFormat(%v, %v) of %v", count, tc.Spec, tc.Tv), got, *tc.Tf)
		})
	}

	for i, tc := range g.Groups.Colors {
		c.Check("scaleLinear with colors", fmt.Sprintf("#%d %v %v %s", i, tc.Domain, tc.Range, tc.Ip), func() string {
			ip := interpolate.RGB
			if tc.Ip == "hcl" {
				ip = interpolate.HCL
			}
			// The range holds color strings; the interpolator parses them.
			s := NewLinearOf(func(a, b string) func(float64) string { return ip(color.Parse(a), color.Parse(b)) }).
				Domain(tc.Domain...).Range(tc.Range...)
			got := make([]string, len(tc.Xs))
			for j, x := range tc.Xs {
				got[j] = s.Scale(x)
			}
			return diffS("scale", got, tc.Out)
		})
	}

	for i, tc := range g.Groups.Bands {
		name := fmt.Sprintf("#%d point=%v domain=%q range=%v ops=%v", i, tc.Point, tc.Domain, tc.R, tc.Ops)
		probe := append(append([]string(nil), tc.Domain...), "not in the domain")
		if tc.Point {
			c.Check("scalePoint", name, func() string {
				s := NewPoint[string]().Domain(tc.Domain...)
				for _, op := range tc.Ops {
					switch op[0] {
					case "range":
						s.Range(tc.R[0], tc.R[1])
					case "rangeRound":
						s.RangeRound(tc.R[0], tc.R[1])
					case "padding":
						s.Padding(jsonNumber(op[1]))
					case "align":
						s.Align(jsonNumber(op[1]))
					case "round":
						s.Round(jsonNumber(op[1]) == 1)
					}
				}
				out, cp := make([]float64, len(probe)), make([]float64, len(probe))
				copied := s.Copy()
				for j, d := range probe {
					out[j], cp[j] = s.Scale(d), copied.Scale(d)
				}
				if x := diff("scale", out, tc.Out); x != "" {
					return x
				}
				if x := diff("copy", cp, tc.Copy); x != "" {
					return x
				}
				if x := diff("bandwidth, step", []float64{s.Bandwidth(), s.Step()}, []float64{tc.Bw.V(), tc.Step.V()}); x != "" {
					return x
				}
				return diffS("domain", s.DomainValues(), tc.Dom)
			})
			continue
		}
		c.Check("scaleBand", name, func() string {
			s := NewBand[string]().Domain(tc.Domain...)
			for _, op := range tc.Ops {
				switch op[0] {
				case "range":
					s.Range(tc.R[0], tc.R[1])
				case "rangeRound":
					s.RangeRound(tc.R[0], tc.R[1])
				case "padding":
					s.Padding(jsonNumber(op[1]))
				case "paddingInner":
					s.PaddingInner(jsonNumber(op[1]))
				case "paddingOuter":
					s.PaddingOuter(jsonNumber(op[1]))
				case "align":
					s.Align(jsonNumber(op[1]))
				case "round":
					s.Round(jsonNumber(op[1]) == 1)
				}
			}
			out, cp := make([]float64, len(probe)), make([]float64, len(probe))
			copied := s.Copy()
			for j, d := range probe {
				out[j], cp[j] = s.Scale(d), copied.Scale(d)
			}
			if x := diff("scale", out, tc.Out); x != "" {
				return x
			}
			if x := diff("copy", cp, tc.Copy); x != "" {
				return x
			}
			if x := diff("bandwidth, step", []float64{s.Bandwidth(), s.Step()}, []float64{tc.Bw.V(), tc.Step.V()}); x != "" {
				return x
			}
			return diffS("domain", s.DomainValues(), tc.Dom)
		})
	}

	for i, tc := range g.Groups.Ordinals {
		c.Check("scaleOrdinal", fmt.Sprintf("#%d domain=%q range=%q lookups=%q", i, tc.Domain, tc.Range, tc.Lookups), func() string {
			// The zero value "" stands for undefined in this test.
			s := NewOrdinal[string, string]().Domain(tc.Domain...).Range(tc.Range...)
			if tc.Unknown != nil {
				s.Unknown(*tc.Unknown)
			}
			got := make([]string, len(tc.Lookups))
			want := make([]string, len(tc.Lookups))
			for j, d := range tc.Lookups {
				got[j] = s.Scale(d)
				if tc.Out[j] != nil {
					want[j] = *tc.Out[j]
				}
			}
			if x := diffS("scale", got, want); x != "" {
				return x
			}
			if x := diffS("domain", s.DomainValues(), tc.Dom); x != "" {
				return x
			}
			return diffS("copy domain", s.Copy().DomainValues(), tc.CopyDom)
		})
	}
	for i, tc := range g.Groups.NumberKeys {
		c.Check("scaleOrdinal with number keys", fmt.Sprintf("#%d %v", i, tc.Lookups), func() string {
			s := NewOrdinal[float64, string]().Range("a", "b", "c")
			got := make([]string, len(tc.Lookups))
			for j, d := range tc.Lookups {
				got[j] = s.Scale(d)
			}
			if x := diffS("scale", got, tc.Out); x != "" {
				return x
			}
			return diff("domain", s.DomainValues(), tc.Dom)
		})
	}

	c.Report()
	c.CheckExceptions(mathExceptions)
}

func intervalByName(s *d3time.Intervals, name string) *d3time.Interval {
	switch name {
	case "millisecond":
		return s.Millisecond
	case "second":
		return s.Second
	case "minute":
		return s.Minute
	case "hour":
		return s.Hour
	case "day":
		return s.Day
	case "monday":
		return s.Monday
	case "sunday":
		return s.Sunday
	case "month":
		return s.Month
	}
	return s.Year
}

// TestTimeAgainstD3 compares the time scales with values that
// tools/reference/gen-d3-scale.mjs recorded from d3-scale, in UTC and in
// America/New_York.
func TestTimeAgainstD3(t *testing.T) {
	for _, file := range []string{"testdata/scale_time_utc.json.gz", "testdata/scale_time_new_york.json.gz"} {
		var g timeGolden
		d3test.Load(t, file, &g)
		loc, err := stdtime.LoadLocation(g.TZ)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(g.TZ, func(t *testing.T) {
			c := d3test.NewChecker(t, g.Math)
			c.Check("default domain", g.TZ, func() string {
				if x := diff("scaleTime", NewTime(loc).DomainValues(), g.Defaults.Time); x != "" {
					return x
				}
				return diff("scaleUtc", NewUTC().DomainValues(), g.Defaults.UTC)
			})
			for i, tc := range g.Groups.Times {
				name := fmt.Sprintf("#%d %s domain=%v range=%v round=%v clamp=%v nice=%s", i, tc.Fam, tc.Domain, tc.Range, tc.Round, tc.Clamp, tc.NiceMode)
				build := func() *Time {
					var s *Time
					if tc.Fam == "time" {
						s = NewTime(loc)
					} else {
						s = NewUTC()
					}
					s.Domain(tc.Domain...)
					if tc.Round {
						s.RangeRound(tc.Range...)
					} else {
						s.Range(tc.Range...)
					}
					s.Clamp(tc.Clamp)
					switch tc.NiceMode {
					case "default":
						s.Nice()
					case "count":
						s.Nice(tc.NiceCount)
					case "interval":
						s.NiceInterval(intervalByName(s.Intervals(), tc.NiceIv).Every(tc.NiceK))
					}
					return s
				}
				c.Check("scale"+map[string]string{"time": "Time", "utc": "Utc"}[tc.Fam], name, func() string {
					s := build()
					if x := diff("domain after nice", s.DomainValues(), tc.Niced); x != "" {
						return x
					}
					out, cp := make([]float64, len(tc.Ts)), make([]float64, len(tc.Ts))
					copied := s.Copy()
					for j, x := range tc.Ts {
						out[j], cp[j] = s.Scale(x), copied.Scale(x)
					}
					if x := diff("scale", out, tc.Out); x != "" {
						return x
					}
					if x := diff("copy", cp, tc.Copy); x != "" {
						return x
					}
					inv := make([]float64, len(tc.Ys))
					for j, y := range tc.Ys {
						inv[j] = s.Invert(y)
					}
					return diff("invert", inv, tc.Inv)
				})
				c.Check("time scale ticks", name, func() string {
					s := build()
					var ticks []float64
					if tc.Count == nil {
						ticks = s.Ticks()
					} else {
						ticks = s.Ticks(*tc.Count)
					}
					if x := diff("ticks", ticks, tc.Ticks); x != "" {
						return x
					}
					if tc.IvTicks != nil {
						got := s.TicksInterval(intervalByName(s.Intervals(), tc.Iv).Every(tc.IvK))
						if x := diff(fmt.Sprintf("ticks(%s.every(%v))", tc.Iv, tc.IvK), got, *tc.IvTicks); x != "" {
							return x
						}
					}
					return ""
				})
				c.Check("time scale tickFormat", name, func() string {
					s := build()
					f := s.TickFormat()
					got := make([]string, len(tc.Ticks))
					for j, x := range tc.Ticks {
						got[j] = f(x)
					}
					if x := diffS("tickFormat() of the ticks", got, tc.Tf); x != "" {
						return x
					}
					// The generator passes Date objects, which clip the time value.
					got = make([]string, len(tc.Ts))
					for j, x := range tc.Ts {
						got[j] = f(js.TimeClip(x))
					}
					if x := diffS("tickFormat() of other dates", got, tc.Tfx); x != "" {
						return x
					}
					fs := s.TickFormat(tc.Spec)
					got = make([]string, len(tc.Tfs))
					for j := range tc.Tfs {
						got[j] = fs(tc.Ticks[j])
					}
					return diffS("tickFormat(specifier)", got, tc.Tfs)
				})
			}
			c.Report()
			c.CheckExceptions(mathExceptions)
		})
	}
}
