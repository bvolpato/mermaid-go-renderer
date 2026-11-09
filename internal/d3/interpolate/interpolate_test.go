package interpolate

import (
	"fmt"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/color"
	"github.com/bvolpato/mermaid-go-renderer/internal/d3/internal/d3test"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

type colorSpec struct {
	S *string
	K string
	C d3test.FS
}

func (s colorSpec) color() color.Color {
	if s.S != nil {
		return color.Parse(*s.S)
	}
	c := s.C
	switch s.K {
	case "rgb":
		return color.RGB{R: c[0], G: c[1], B: c[2], Opacity: c[3]}
	case "hsl":
		return color.HSL{H: c[0], S: c[1], L: c[2], Opacity: c[3]}
	case "lab":
		return color.Lab{L: c[0], A: c[1], B: c[2], Opacity: c[3]}
	case "hcl":
		return color.HCL{H: c[0], C: c[1], L: c[2], Opacity: c[3]}
	}
	return color.Cubehelix{H: c[0], S: c[1], L: c[2], Opacity: c[3]}
}

type golden struct {
	Math   d3test.Math
	Groups struct {
		Numbers []struct {
			A, B          d3test.F
			T, N, R, H, D d3test.FS
		}
		Colors []struct {
			F    string
			A, B colorSpec
			T    d3test.FS
			O    []string
		}
		Strings []struct {
			A, B string
			T    d3test.FS
			O    []string
		}
		Values []struct {
			A, B string
			T    d3test.FS
			O    []string
		}
		Splines []struct {
			V             d3test.FS
			Cols          []string
			T, Bs, Bc     d3test.FS
			Rb, Rc        []string
			Dis           d3test.FS
			Q             int
			Quant, Pt, Pw d3test.FS
			Pc            []string
		}
	}
}

// mathExceptions lists, for each build mode (see d3test.Mode), the number of
// golden cases that differ from Node.js only because a Math function returns
// another last bit. The test proves each one: the case passes when it runs
// again with the results that Node.js returned for the same calls. Measured
// with go1.27.3.
//
// In each of these cases a color channel is on a rounding boundary, and
// Math.sin or Math.pow differs by 1 ulp. For example one case gives
// rgb(128, 25, 0) where d3 in Node.js gives rgb(128, 26, 0).
var mathExceptions = map[string]map[string]int{
	"arm64":       {"color cubehelixGamma": 4, "color cubehelixLong": 1, "color cubehelixLongGamma": 4},
	"arm64/nofma": {"color cubehelixGamma": 4, "color cubehelixLong": 1, "color cubehelixLongGamma": 4},
	"amd64":       {"color cubehelixGamma": 4, "color cubehelixLong": 1, "color cubehelixLongGamma": 4},
}

var colorInterpolators = map[string]func(a, b color.Color) func(float64) string{
	"rgb": RGB, "rgbGamma": RGBGamma(2.2), "hsl": HSL, "hslLong": HSLLong, "lab": Lab, "hcl": HCL, "hclLong": HCLLong,
	"cubehelix": Cubehelix, "cubehelixLong": CubehelixLong, "cubehelixGamma": CubehelixGamma(3), "cubehelixLongGamma": CubehelixLongGamma(0.5),
}

func apply(f func(float64) float64, ts []float64) []float64 {
	out := make([]float64, len(ts))
	for i, t := range ts {
		out[i] = f(t)
	}
	return out
}

func applyS(f func(float64) string, ts []float64) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = f(t)
	}
	return out
}

func diffF(what string, got, want []float64) string {
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

// TestAgainstD3 compares with values that
// tools/reference/gen-d3-interpolate.mjs recorded from d3-interpolate.
func TestAgainstD3(t *testing.T) {
	var g golden
	d3test.Load(t, "testdata/interpolate.json.gz", &g)
	c := d3test.NewChecker(t, g.Math)

	for i, tc := range g.Groups.Numbers {
		a, b := tc.A.V(), tc.B.V()
		c.Check("number/round/hue/date", fmt.Sprintf("#%d %v %v t=%v", i, a, b, tc.T), func() string {
			for _, p := range []struct {
				name string
				f    func(float64) float64
				want []float64
			}{{"number", Number(a, b), tc.N}, {"round", Round(a, b), tc.R}, {"hue", Hue(a, b), tc.H}, {"date", Date(a, b), tc.D}} {
				if s := diffF(p.name, apply(p.f, tc.T), p.want); s != "" {
					return s
				}
			}
			return ""
		})
	}

	for i, tc := range g.Groups.Colors {
		c.Check("color "+tc.F, fmt.Sprintf("#%d t=%v", i, tc.T), func() string {
			f := colorInterpolators[tc.F](tc.A.color(), tc.B.color())
			return diffS(tc.F, applyS(f, tc.T), tc.O)
		})
	}

	for i, tc := range g.Groups.Strings {
		c.Check("string", fmt.Sprintf("#%d %q %q", i, tc.A, tc.B), func() string {
			return diffS("string", applyS(String(tc.A, tc.B), tc.T), tc.O)
		})
	}

	for i, tc := range g.Groups.Values {
		c.Check("interpolate(a, b)", fmt.Sprintf("#%d %s %s", i, tc.A, tc.B), func() string {
			a, err := js.ParseJSON([]byte(tc.A))
			if err != nil {
				return err.Error()
			}
			b, err := js.ParseJSON([]byte(tc.B))
			if err != nil {
				return err.Error()
			}
			f := Value(a, b)
			got := make([]string, len(tc.T))
			for j, x := range tc.T {
				got[j] = js.StringifyJSON(f(x))
			}
			return diffS("value", got, tc.O)
		})
	}

	for i, tc := range g.Groups.Splines {
		c.Check("basis/discrete/quantize/piecewise", fmt.Sprintf("#%d %v t=%v", i, tc.V, tc.T), func() string {
			cols := make([]color.Color, len(tc.Cols))
			for j, s := range tc.Cols {
				cols[j] = color.Parse(s)
			}
			if s := diffF("basis", apply(Basis(tc.V), tc.T), tc.Bs); s != "" {
				return s
			}
			if s := diffF("basisClosed", apply(BasisClosed(tc.V), tc.T), tc.Bc); s != "" {
				return s
			}
			if s := diffS("rgbBasis", applyS(RGBBasis(cols), tc.T), tc.Rb); s != "" {
				return s
			}
			if s := diffS("rgbBasisClosed", applyS(RGBBasisClosed(cols), tc.T), tc.Rc); s != "" {
				return s
			}
			// d3 returns undefined for a NaN argument; the port returns zero.
			var dt, dw []float64
			for j, x := range tc.T {
				if x == x {
					dt, dw = append(dt, x), append(dw, tc.Dis[j])
				}
			}
			if s := diffF("discrete", apply(Discrete(tc.V), dt), dw); s != "" {
				return s
			}
			if s := diffF("quantize", Quantize(Number(tc.V[0], tc.V[len(tc.V)-1]), tc.Q), tc.Quant); s != "" {
				return s
			}
			if len(tc.V) >= 2 {
				if s := diffF("piecewise", apply(Piecewise(Number, tc.V), tc.Pt), tc.Pw); s != "" {
					return s
				}
				if s := diffS("piecewise rgb", applyS(Piecewise(RGB, cols), tc.Pt), tc.Pc); s != "" {
					return s
				}
			}
			return ""
		})
	}

	c.Report()
	c.CheckExceptions(mathExceptions)
}
