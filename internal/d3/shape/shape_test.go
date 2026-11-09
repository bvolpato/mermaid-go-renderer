package shape

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/internal/d3test"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

type golden struct {
	Math   d3test.Math
	Groups struct {
		Lines []struct {
			C      string
			P      *float64
			Pts    d3test.NS
			Mask   *string
			Digits *int
			D      *string
		}
		Defaults []struct {
			Pts       d3test.NS
			Cx        d3test.F
			D, Dx, Dy *string
		}
		Areas []struct {
			C             string
			P             *float64
			Rows          d3test.NS
			Mask          *string
			Mode          string
			Base          d3test.F
			D             *string
			Lx0, Ly1, Lx1 *string
			hasLines      bool
		}
		Arcs []struct {
			A             d3test.NS
			PadUndefined  bool
			PadRadiusNull bool
			D, Raw        string
			C             d3test.FS
		}
		Fixed []struct {
			A d3test.NS
			D string
			C d3test.FS
		}
		DatumArcs []struct {
			A     d3test.NS
			D     string
			C     d3test.FS
			Empty string
		}
		Pies []struct {
			V    d3test.NS
			Sort string
			A    d3test.NS
			R    d3test.NS
			D    *[]string
			C    *d3test.FS
		}
		PlainPies []struct{ V, R d3test.NS }
	}
}

// mathExceptions lists, for each build mode (see d3test.Mode), the number of
// golden cases that differ from Node.js only because a Math function returns
// another last bit. The test proves each one: the case passes when it runs
// again with the results that Node.js returned for the same calls.
//
// The port calls the Math functions of internal/js, which return the bits of
// Chromium 156. Node.js is not Chromium here: it is off by 1 ulp in some
// calls. For the arcs without rounding these are 238 calls of Math.atan2,
// 188 of Math.cos, 113 of Math.sin and 3 of Math.acos. With the default of
// 3 digits, no arc differs. The centroid has no rounding: 47 of 2800 differ,
// and 40 of the 675 pie charts have a label position that differs. In these
// cases the port has the value of Chromium.
var mathExceptions = map[string]map[string]int{
	"arm64": {
		"arc, default accessors": 11, "arc, no rounding": 214, "arc.centroid": 47, "pie, label centroids of the slices": 40,
	},
	"arm64/nofma": {
		"arc, default accessors": 11, "arc, no rounding": 214, "arc.centroid": 47, "pie, label centroids of the slices": 40,
	},
	"amd64": {
		"arc, default accessors": 11, "arc, no rounding": 214, "arc.centroid": 47, "pie, label centroids of the slices": 40,
	},
}

func curveFactory(name string, p *float64) CurveFactory {
	if p != nil {
		switch name {
		case "bundle":
			return CurveBundleBeta(*p)
		case "cardinal":
			return CurveCardinalTension(*p)
		case "cardinalClosed":
			return CurveCardinalClosedTension(*p)
		case "cardinalOpen":
			return CurveCardinalOpenTension(*p)
		case "catmullRom":
			return CurveCatmullRomAlpha(*p)
		case "catmullRomClosed":
			return CurveCatmullRomClosedAlpha(*p)
		case "catmullRomOpen":
			return CurveCatmullRomOpenAlpha(*p)
		}
	}
	return map[string]CurveFactory{
		"linear": CurveLinear, "linearClosed": CurveLinearClosed, "basis": CurveBasis, "basisClosed": CurveBasisClosed,
		"basisOpen": CurveBasisOpen, "bumpX": CurveBumpX, "bumpY": CurveBumpY, "bundle": CurveBundle,
		"cardinal": CurveCardinal, "cardinalClosed": CurveCardinalClosed, "cardinalOpen": CurveCardinalOpen,
		"catmullRom": CurveCatmullRom, "catmullRomClosed": CurveCatmullRomClosed, "catmullRomOpen": CurveCatmullRomOpen,
		"monotoneX": CurveMonotoneX, "monotoneY": CurveMonotoneY, "natural": CurveNatural,
		"step": CurveStep, "stepAfter": CurveStepAfter, "stepBefore": CurveStepBefore,
	}[name]
}

func curveName(name string, p *float64) string {
	if p == nil {
		return name
	}
	return fmt.Sprintf("%s(%v)", name, *p)
}

// same compares a path with the golden value, where null is the empty string.
func same(what, got string, want *string) string {
	w := ""
	if want != nil {
		w = *want
	}
	if got != w {
		return fmt.Sprintf("%s: got %q, want %q", what, got, w)
	}
	return ""
}

type xy struct{ x, y float64 }

func toPoints(flat []float64) []xy {
	pts := make([]xy, len(flat)/2)
	for i := range pts {
		pts[i] = xy{flat[2*i], flat[2*i+1]}
	}
	return pts
}

func ascending(a, b float64) float64 {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	case a >= b:
		return 0
	}
	return math.NaN()
}

// TestAgainstD3 compares with values that tools/reference/gen-d3-shape.mjs
// recorded from d3-shape.
func TestAgainstD3(t *testing.T) {
	var g golden
	d3test.Load(t, "testdata/shape.json.gz", &g)
	c := d3test.NewChecker(t, g.Math)

	for i, tc := range g.Groups.Lines {
		pts := toPoints(tc.Pts)
		digits := -1
		group := "line, no rounding"
		if tc.Digits != nil {
			digits = *tc.Digits
			group = "line, 0 to 5 digits"
		}
		// The curves that call a Math function are in groups of their own.
		if strings.HasPrefix(tc.C, "catmullRom") && (tc.P == nil || *tc.P != 0) {
			group += ", catmullRom"
		}
		c.Check(group, fmt.Sprintf("#%d %s digits=%d %v", i, curveName(tc.C, tc.P), digits, tc.Pts), func() string {
			line := NewLine[xy]().
				X(func(d xy, _ int, _ []xy) float64 { return d.x }).
				Y(func(d xy, _ int, _ []xy) float64 { return d.y }).
				Curve(curveFactory(tc.C, tc.P)).
				Digits(digits)
			if tc.Mask != nil {
				mask := *tc.Mask
				line.Defined(func(_ xy, j int, _ []xy) bool { return mask[j] == '1' })
			}
			return same("line", line.Path(pts), tc.D)
		})
	}

	for i, tc := range g.Groups.Defaults {
		c.Check("line, default accessors", fmt.Sprintf("#%d %v", i, tc.Pts), func() string {
			pairs := make([][2]float64, len(tc.Pts)/2)
			slices := make([][]float64, len(pairs))
			for j := range pairs {
				pairs[j] = [2]float64{tc.Pts[2*j], tc.Pts[2*j+1]}
				slices[j] = []float64{tc.Pts[2*j], tc.Pts[2*j+1]}
			}
			if s := same("line()", NewLine[[2]float64]().Path(pairs), tc.D); s != "" {
				return s
			}
			if s := same("line() on slices", NewLine[[]float64]().Path(slices), tc.D); s != "" {
				return s
			}
			if s := same("line().x(c)", NewLine[[2]float64]().XConst(tc.Cx.V()).Path(pairs), tc.Dx); s != "" {
				return s
			}
			return same("line().y(c)", NewLine[[2]float64]().YConst(tc.Cx.V()).Path(pairs), tc.Dy)
		})
	}

	for i, tc := range g.Groups.Areas {
		type row struct{ x, y0, y1, x1 float64 }
		rows := make([]row, len(tc.Rows)/4)
		for j := range rows {
			rows[j] = row{tc.Rows[4*j], tc.Rows[4*j+1], tc.Rows[4*j+2], tc.Rows[4*j+3]}
		}
		group := "area"
		if strings.HasPrefix(tc.C, "catmullRom") && (tc.P == nil || *tc.P != 0) {
			group += ", catmullRom"
		}
		c.Check(group, fmt.Sprintf("#%d %s %s %v", i, curveName(tc.C, tc.P), tc.Mode, tc.Rows), func() string {
			x := func(d row, _ int, _ []row) float64 { return d.x }
			y0 := func(d row, _ int, _ []row) float64 { return d.y0 }
			y1 := func(d row, _ int, _ []row) float64 { return d.y1 }
			x1 := func(d row, _ int, _ []row) float64 { return d.x1 }
			area := NewArea[row]().Curve(curveFactory(tc.C, tc.P))
			switch tc.Mode {
			case "y0y1":
				area.X(x).Y0(y0).Y1(y1)
			case "const":
				area.X(x).Y0Const(tc.Base.V()).Y1(y1)
			default:
				area.X0(x).X1(x1).Y0(y0).Y1(y1)
			}
			if tc.Mask != nil {
				mask := *tc.Mask
				area.Defined(func(_ row, j int, _ []row) bool { return mask[j] == '1' })
			}
			if s := same("area", area.Path(rows), tc.D); s != "" {
				return s
			}
			if i%3 != 0 && tc.Lx0 == nil && tc.Ly1 == nil && tc.Lx1 == nil {
				return ""
			}
			if s := same("lineX0", area.LineX0().Path(rows), tc.Lx0); s != "" {
				return s
			}
			if s := same("lineY1", area.LineY1().Path(rows), tc.Ly1); s != "" {
				return s
			}
			return same("lineX1", area.LineX1().Path(rows), tc.Lx1)
		})
	}

	centroid := func(x, y float64, want []float64) string {
		if !d3test.SameSlice([]float64{x, y}, want) {
			return fmt.Sprintf("centroid: got [%v %v], want %v", x, y, want)
		}
		return ""
	}
	for i, tc := range g.Groups.Arcs {
		a := tc.A
		build := func() *Arc[struct{}] {
			arc := NewArc[struct{}]().InnerRadius(a[0]).OuterRadius(a[1]).StartAngle(a[2]).EndAngle(a[3]).CornerRadius(a[5])
			if !tc.PadUndefined {
				arc.PadAngle(a[4])
			}
			if !tc.PadRadiusNull {
				arc.PadRadius(a[6])
			}
			return arc
		}
		name := fmt.Sprintf("#%d %v", i, a)
		c.Check("arc, 3 digits", name, func() string {
			if got := build().Path(struct{}{}); got != tc.D {
				return fmt.Sprintf("got %q, want %q", got, tc.D)
			}
			return ""
		})
		c.Check("arc, no rounding", name, func() string {
			if got := build().Digits(-1).Path(struct{}{}); got != tc.Raw {
				return fmt.Sprintf("got %q, want %q", got, tc.Raw)
			}
			return ""
		})
		c.Check("arc.centroid", name, func() string {
			x, y := build().Centroid(struct{}{})
			return centroid(x, y, tc.C)
		})
	}
	for i, tc := range g.Groups.Fixed {
		a := tc.A
		c.Check("arc of mermaid", fmt.Sprintf("#%d %v", i, a), func() string {
			arc := NewArc[struct{}]().StartAngle(a[2]).EndAngle(a[3]).InnerRadius(a[0]).OuterRadius(a[1])
			if got := arc.Path(struct{}{}); got != tc.D {
				return fmt.Sprintf("got %q, want %q", got, tc.D)
			}
			x, y := arc.Centroid(struct{}{})
			return centroid(x, y, tc.C)
		})
	}
	for i, tc := range g.Groups.DatumArcs {
		a := tc.A
		c.Check("arc, default accessors", fmt.Sprintf("#%d %v", i, a), func() string {
			datum := ArcDatum{InnerRadius: a[0], OuterRadius: a[1], StartAngle: a[2], EndAngle: a[3], PadAngle: a[4]}
			arc := NewArc[ArcDatum]()
			if got := arc.Path(datum); got != tc.D {
				return fmt.Sprintf("got %q, want %q", got, tc.D)
			}
			if got := NewArc[struct{}]().Path(struct{}{}); got != tc.Empty {
				return fmt.Sprintf("arc()({}): got %q, want %q", got, tc.Empty)
			}
			x, y := arc.Centroid(datum)
			return centroid(x, y, tc.C)
		})
	}

	type slice struct {
		v   float64
		key int
	}
	flatten := func(arcs []PieArcDatum[slice]) []float64 {
		var out []float64
		for _, a := range arcs {
			out = append(out, float64(a.Index), a.Value, a.StartAngle, a.EndAngle, a.PadAngle)
		}
		return out
	}
	for i, tc := range g.Groups.Pies {
		data := make([]slice, len(tc.V))
		for j, v := range tc.V {
			data[j] = slice{v, (j * 7) % 5}
		}
		// Node.js 26 has the TimSort of V8; current Chromium has PowerSort.
		pie := NewPie[slice]().Value(func(d slice, _ int, _ []slice) float64 { return d.v }).
			StartAngle(tc.A[0]).EndAngle(tc.A[1]).PadAngle(tc.A[2]).SortAlgorithm(js.V8TimSort)
		switch tc.Sort {
		case "none":
			pie.Sort(nil)
		case "ascending":
			pie.SortValues(ascending)
		case "data":
			pie.Sort(func(a, b slice) float64 { return float64(a.key - b.key) })
		}
		name := fmt.Sprintf("#%d %v sort=%s angles=%v", i, tc.V, tc.Sort, tc.A)
		c.Check("pie", name, func() string {
			arcs := pie.Arcs(data)
			if got := flatten(arcs); !d3test.SameSlice(got, tc.R) {
				return fmt.Sprintf("got %v, want %v", got, tc.R)
			}
			for j, a := range arcs {
				if a.Data.key != data[j].key || !d3test.Same(a.Data.v, data[j].v) {
					return fmt.Sprintf("arc %d has the data %v, want %v", j, a.Data, data[j])
				}
			}
			return ""
		})
		if tc.D == nil {
			continue
		}
		// The pie chart of mermaid: an arc path and a label position for each slice.
		c.Check("pie, arc paths of the slices", name, func() string {
			arc := NewArc[PieArcDatum[slice]]().InnerRadius(0).OuterRadius(185)
			for j, a := range pie.Arcs(data) {
				if got := arc.Path(a); got != (*tc.D)[j] {
					return fmt.Sprintf("slice %d: got %q, want %q", j, got, (*tc.D)[j])
				}
			}
			return ""
		})
		c.Check("pie, label centroids of the slices", name, func() string {
			label := NewArc[PieArcDatum[slice]]().InnerRadius(float64(185 * 0.75)).OuterRadius(float64(185 * 0.75))
			var got []float64
			for _, a := range pie.Arcs(data) {
				x, y := label.Centroid(a)
				got = append(got, x, y)
			}
			if !d3test.SameSlice(got, *tc.C) {
				return fmt.Sprintf("got %v, want %v", got, *tc.C)
			}
			return ""
		})
	}
	for i, tc := range g.Groups.PlainPies {
		c.Check("pie, default accessor", fmt.Sprintf("#%d %v", i, tc.V), func() string {
			var got []float64
			for _, a := range NewPie[float64]().SortAlgorithm(js.V8TimSort).Arcs(tc.V) {
				got = append(got, float64(a.Index), a.Value, a.StartAngle, a.EndAngle, a.PadAngle)
			}
			if !d3test.SameSlice(got, tc.R) {
				return fmt.Sprintf("got %v, want %v", got, tc.R)
			}
			return ""
		})
	}

	c.Report()
	c.CheckExceptions(mathExceptions)
}
