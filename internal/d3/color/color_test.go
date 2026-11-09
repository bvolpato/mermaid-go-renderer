package color

import (
	"fmt"
	"math"
	"strconv"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/internal/d3test"
)

type golden struct {
	Math d3test.Math
	Misc struct {
		Gray, Lch, NullRgb, NullHsl, NullLab, NullHcl, NullCubehelix d3test.FS
	}
	Groups struct {
		Parse []struct {
			S        string
			K        *string
			C        d3test.FS
			O        []string
			RGB, HSL d3test.FS
		}
		Convert []struct {
			K  string
			C  d3test.FS
			O  []string
			To d3test.FS
		}
		Ops []struct {
			K            string
			C            d3test.FS
			Kk           d3test.F
			B0, D0, B, D d3test.FS
			Clamp        *d3test.FS
		}
	}
}

// mathExceptions lists, for each build mode (see d3test.Mode), the number of
// golden cases that differ from Node.js only because a Math function returns
// another last bit. The test proves each one: the case passes when it runs
// again with the results that Node.js returned for the same calls.
//
// The port calls the Math functions of internal/js, which return the bits of
// Chromium 156 on Linux. Node.js is not Chromium here: its Math.atan2,
// Math.cos and Math.sin differ by 1 ulp for some arguments (in 135, 80 and
// 60 calls of the conversions). The golden comes from Node.js on macOS,
// whose Math.pow is the pow of that system: it differs from the pow of
// Linux by 1 ulp in 8 calls of the conversions and in 2 calls of brighter
// and darker. In all these cases the port has the value of Chromium on
// Linux.
var mathExceptions = map[string]map[string]int{
	"arm64":       {"conversions": 135, "brighter/darker/clamp": 2},
	"arm64/nofma": {"conversions": 135, "brighter/darker/clamp": 2},
	"amd64":       {"conversions": 135, "brighter/darker/clamp": 2},
}

func build(kind string, c []float64) Color {
	switch kind {
	case "rgb":
		return RGB{c[0], c[1], c[2], c[3]}
	case "hsl":
		return HSL{c[0], c[1], c[2], c[3]}
	case "lab":
		return Lab{c[0], c[1], c[2], c[3]}
	case "hcl":
		return HCL{c[0], c[1], c[2], c[3]}
	}
	return Cubehelix{c[0], c[1], c[2], c[3]}
}

func channels(c Color) []float64 {
	switch o := c.(type) {
	case RGB:
		return []float64{o.R, o.G, o.B, o.Opacity}
	case HSL:
		return []float64{o.H, o.S, o.L, o.Opacity}
	case Lab:
		return []float64{o.L, o.A, o.B, o.Opacity}
	case HCL:
		return []float64{o.H, o.C, o.L, o.Opacity}
	case Cubehelix:
		return []float64{o.H, o.S, o.L, o.Opacity}
	}
	return nil
}

func formats(c Color) []string {
	return []string{c.FormatHex(), c.FormatHex8(), c.FormatRgb(), c.FormatHsl(), c.String(), strconv.FormatBool(c.Displayable())}
}

func diff(what string, got, want []float64) string {
	if d3test.SameSlice(got, want) {
		return ""
	}
	return fmt.Sprintf("%s: got %v, want %v", what, got, want)
}

func brighterOf(c Color, k ...float64) Color {
	switch o := c.(type) {
	case RGB:
		return o.Brighter(k...)
	case HSL:
		return o.Brighter(k...)
	case Lab:
		return o.Brighter(k...)
	case HCL:
		return o.Brighter(k...)
	}
	return c.(Cubehelix).Brighter(k...)
}

func darkerOf(c Color, k ...float64) Color {
	switch o := c.(type) {
	case RGB:
		return o.Darker(k...)
	case HSL:
		return o.Darker(k...)
	case Lab:
		return o.Darker(k...)
	case HCL:
		return o.Darker(k...)
	}
	return c.(Cubehelix).Darker(k...)
}

// TestAgainstD3 compares with values that tools/reference/gen-d3-color.mjs
// recorded from d3-color.
func TestAgainstD3(t *testing.T) {
	var g golden
	d3test.Load(t, "testdata/color.json.gz", &g)
	c := d3test.NewChecker(t, g.Math)

	for i, tc := range g.Groups.Parse {
		c.Check("color(string)", fmt.Sprintf("#%d %q", i, tc.S), func() string {
			got := Parse(tc.S)
			if tc.K == nil {
				if got != nil {
					return fmt.Sprintf("got %v, want null", got)
				}
				return ""
			}
			if got == nil {
				return "got null"
			}
			kind := "hsl"
			if _, ok := got.(RGB); ok {
				kind = "rgb"
			}
			if kind != *tc.K {
				return fmt.Sprintf("got a %s color, want %s", kind, *tc.K)
			}
			if s := diff("channels", channels(got), tc.C); s != "" {
				return s
			}
			if f := formats(got); fmt.Sprint(f) != fmt.Sprint(tc.O) {
				return fmt.Sprintf("formats: got %q, want %q", f, tc.O)
			}
			if s := diff("rgb(string)", channels(ParseRGB(tc.S)), tc.RGB); s != "" {
				return s
			}
			return diff("hsl(string)", channels(ParseHSL(tc.S)), tc.HSL)
		})
	}

	for i, tc := range g.Groups.Convert {
		c.Check("conversions", fmt.Sprintf("#%d %s%v", i, tc.K, tc.C), func() string {
			col := build(tc.K, tc.C)
			if f := formats(col); fmt.Sprint(f) != fmt.Sprint(tc.O) {
				return fmt.Sprintf("formats: got %q, want %q", f, tc.O)
			}
			var got []float64
			got = append(got, channels(col.RGB())...)
			got = append(got, channels(HSLFrom(col))...)
			got = append(got, channels(LabFrom(col))...)
			got = append(got, channels(HCLFrom(col))...)
			got = append(got, channels(CubehelixFrom(col))...)
			for j, name := range []string{"rgb", "hsl", "lab", "hcl", "cubehelix"} {
				if s := diff(name, got[j*4:j*4+4], tc.To[j*4:j*4+4]); s != "" {
					return s
				}
			}
			return ""
		})
	}

	for i, tc := range g.Groups.Ops {
		c.Check("brighter/darker/clamp", fmt.Sprintf("#%d %s%v k=%v", i, tc.K, tc.C, tc.Kk.V()), func() string {
			col := build(tc.K, tc.C)
			if s := diff("brighter()", channels(brighterOf(col)), tc.B0); s != "" {
				return s
			}
			if s := diff("darker()", channels(darkerOf(col)), tc.D0); s != "" {
				return s
			}
			if s := diff("brighter(k)", channels(brighterOf(col, tc.Kk.V())), tc.B); s != "" {
				return s
			}
			if s := diff("darker(k)", channels(darkerOf(col, tc.Kk.V())), tc.D); s != "" {
				return s
			}
			if tc.Clamp != nil {
				var clamped Color
				if o, ok := col.(RGB); ok {
					clamped = o.Clamp()
				} else {
					clamped = col.(HSL).Clamp()
				}
				return diff("clamp()", channels(clamped), *tc.Clamp)
			}
			return ""
		})
	}

	c.Check("misc", "gray, lch, null", func() string {
		for _, p := range []struct {
			name string
			got  Color
			want []float64
		}{
			{"gray", Gray(40, 0.5), g.Misc.Gray},
			{"lch", LCH(50, 30, 120, 1), g.Misc.Lch},
			{"rgb(null)", ParseRGB("nope"), g.Misc.NullRgb},
			{"hsl(null)", ParseHSL("nope"), g.Misc.NullHsl},
			{"lab(null)", ParseLab("nope"), g.Misc.NullLab},
			{"hcl(null)", ParseHCL("nope"), g.Misc.NullHcl},
			{"cubehelix(null)", ParseCubehelix("nope"), g.Misc.NullCubehelix},
		} {
			if s := diff(p.name, channels(p.got), p.want); s != "" {
				return s
			}
		}
		if !math.IsNaN(RGBFrom(nil).R) {
			return "RGBFrom(nil) must have NaN channels"
		}
		return ""
	})

	c.Report()
	c.CheckExceptions(mathExceptions)
}
