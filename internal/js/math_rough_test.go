package js

import (
	"encoding/json"
	"math"
	"os"
	"strconv"
	"testing"
)

// The golden file testdata/math_rough.json comes from
// tools/reference/gen-rough-crmath.mjs.

type mathRoughGolden struct {
	Chromium []struct {
		Source   string      `json:"source"`
		CenterX1 string      `json:"centerX1"`
		CenterX2 string      `json:"centerX2"`
		Radius   string      `json:"radius"`
		Arc1     [][2]string `json:"arc1"`
		Arc2     [][2]string `json:"arc2"`
	} `json:"chromium"`
	BC map[string][][2]string `json:"bc"`
}

func loadMathRoughGolden(t *testing.T) *mathRoughGolden {
	t.Helper()
	data, err := os.ReadFile("testdata/math_rough.json")
	if err != nil {
		t.Fatal(err)
	}
	var g mathRoughGolden
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	return &g
}

func mathRoughFromBits(t *testing.T, s string) float64 {
	t.Helper()
	u, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		t.Fatal(err)
	}
	return math.Float64frombits(u)
}

// TestCRMathChromiumStadium compares SinCR and CosCR with the reference
// browser. mermaid computes the arc points of a stadium node with Math.cos
// and Math.sin (generateCirclePoints in shapes/util.ts), and the reference
// SVGs have the points with all their digits.
func TestCRMathChromiumStadium(t *testing.T) {
	g := loadMathRoughGolden(t)
	if len(g.Chromium) == 0 {
		t.Fatal("the golden file has no stadium nodes")
	}
	type impl struct {
		name     string
		sin, cos func(float64) float64
		differ   int
	}
	impls := []*impl{
		{name: "SinCR and CosCR", sin: SinCR, cos: CosCR},
		{name: "Sin and Cos", sin: Sin, cos: Cos},
	}
	coords := 0
	// generateCirclePoints(centerX, 0, radius, 50, startAngle, endAngle)
	arc := func(im *impl, centerX, radius, startAngle, endAngle float64, want [][2]string) {
		pi := math.Pi
		startAngleRad := (startAngle * pi) / 180
		endAngleRad := (endAngle * pi) / 180
		angleRange := endAngleRad - startAngleRad
		angleStep := angleRange / (50 - 1)
		for i := 0; i < 50; i++ {
			angle := startAngleRad + float64(float64(i)*angleStep)
			x := centerX + float64(radius*im.cos(angle))
			y := 0 + float64(radius*im.sin(angle))
			// The point is {x: -x, y: -y}.
			if Str(-x) != want[i][0] {
				im.differ++
			}
			if Str(-y) != want[i][1] {
				im.differ++
			}
		}
	}
	for _, s := range g.Chromium {
		radius := Number(s.Radius)
		for _, im := range impls {
			arc(im, Number(s.CenterX1), radius, 90, 270, s.Arc1)
			arc(im, Number(s.CenterX2), radius, 270, 450, s.Arc2)
		}
		coords += 200
	}
	for _, im := range impls {
		t.Logf("%s: %d of %d coordinates differ from Chromium", im.name, im.differ, coords)
	}
	if impls[0].differ != 0 {
		t.Errorf("SinCR and CosCR differ from Chromium for %d of %d coordinates", impls[0].differ, coords)
	}
}

// TestCRMathBC compares the correctly rounded functions with results of the
// calculator bc with 80 decimal places, rounded to the nearest float64.
func TestCRMathBC(t *testing.T) {
	g := loadMathRoughGolden(t)
	funcs := []struct {
		name    string
		cr      func(float64) float64
		wrapper func(float64) float64
	}{
		{"sin", SinCR, Sin},
		{"cos", CosCR, Cos},
		{"tan", TanCR, Tan},
		{"atan", AtanCR, Atan},
		{"asin", AsinCR, Asin},
	}
	for _, f := range funcs {
		cases := g.BC[f.name]
		if len(cases) < 200 {
			t.Errorf("%s: only %d cases", f.name, len(cases))
		}
		wrapperDiffers := 0
		for _, c := range cases {
			x, want := mathRoughFromBits(t, c[0]), mathRoughFromBits(t, c[1])
			if got := f.cr(x); math.Float64bits(got) != math.Float64bits(want) {
				t.Errorf("%s(%v) = %v (%016x), want %v (%016x)", f.name, x, got, math.Float64bits(got), want, math.Float64bits(want))
			}
			if math.Float64bits(f.wrapper(x)) != math.Float64bits(want) {
				wrapperDiffers++
			}
		}
		t.Logf("%s: %d arguments; the wrapper of math.go is not correctly rounded for %d", f.name, len(cases), wrapperDiffers)
	}
}

// TestCRMathSpecial checks the special values that ECMAScript defines.
func TestCRMathSpecial(t *testing.T) {
	negZero := math.Copysign(0, -1)
	inf := math.Inf(1)
	nan := math.NaN()
	halfPi := 1.5707963267948966
	cases := []struct {
		name string
		got  float64
		want float64
	}{
		{"sin(0)", SinCR(0), 0},
		{"sin(-0)", SinCR(negZero), negZero},
		{"sin(Infinity)", SinCR(inf), nan},
		{"sin(NaN)", SinCR(nan), nan},
		{"cos(0)", CosCR(0), 1},
		{"cos(-0)", CosCR(negZero), 1},
		{"cos(-Infinity)", CosCR(-inf), nan},
		{"tan(-0)", TanCR(negZero), negZero},
		{"tan(Infinity)", TanCR(inf), nan},
		{"atan(-0)", AtanCR(negZero), negZero},
		{"atan(Infinity)", AtanCR(inf), halfPi},
		{"atan(-Infinity)", AtanCR(-inf), -halfPi},
		{"atan(1)", AtanCR(1), 0.7853981633974483},
		{"asin(-0)", AsinCR(negZero), negZero},
		{"asin(1)", AsinCR(1), halfPi},
		{"asin(-1)", AsinCR(-1), -halfPi},
		{"asin(1.0000000000000002)", AsinCR(1.0000000000000002), nan},
		{"asin(NaN)", AsinCR(nan), nan},
		{"sin(5e-324)", SinCR(5e-324), 5e-324},
		{"tan(5e-324)", TanCR(5e-324), 5e-324},
		{"atan(5e-324)", AtanCR(5e-324), 5e-324},
		{"asin(5e-324)", AsinCR(5e-324), 5e-324},
		{"sin(1e-200)", SinCR(1e-200), 1e-200},
		{"cos(1e-200)", CosCR(1e-200), 1},
		// The float64 that is nearest to a multiple of pi/2.
		{"cos(6381956970095103 * 2^797)", CosCR(math.Ldexp(6381956970095103, 797)), -4.68716592425462761112e-19},
		{"sin(1.7976931348623157e308)", SinCR(math.MaxFloat64), 0.004961954789184062},
		{"cos(1.7976931348623157e308)", CosCR(math.MaxFloat64), -0.9999876894265599},
	}
	for _, c := range cases {
		if math.IsNaN(c.want) {
			if !math.IsNaN(c.got) {
				t.Errorf("%s = %v, want NaN", c.name, c.got)
			}
			continue
		}
		if math.Float64bits(c.got) != math.Float64bits(c.want) {
			t.Errorf("%s = %v (%016x), want %v (%016x)", c.name, c.got, math.Float64bits(c.got), c.want, math.Float64bits(c.want))
		}
	}
}

func BenchmarkSinCR(b *testing.B) {
	x := 0.3
	for i := 0; i < b.N; i++ {
		SinCR(x)
		x += 0.37
		if x > 100 {
			x -= 100
		}
	}
}
