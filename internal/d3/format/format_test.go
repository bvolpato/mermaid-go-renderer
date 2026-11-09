package format

import (
	"fmt"
	"math"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/internal/d3test"
)

type specFields struct {
	Fill, Align, Sign, Symbol string
	Zero                      bool
	Width                     d3test.F
	Comma                     bool
	Precision                 d3test.F
	Trim                      bool
	Type                      string
}

func (f specFields) specifier() *Specifier {
	return &Specifier{Fill: f.Fill, Align: f.Align, Sign: f.Sign, Symbol: f.Symbol, Zero: f.Zero, Width: f.Width.V(),
		Comma: f.Comma, Precision: f.Precision.V(), Trim: f.Trim, Type: f.Type}
}

// bundledVersion is the version of d3-format inside the mermaid 12.2.0
// bundle. The package in node_modules is newer.
const bundledVersion = "3.1.0"

// formatCase is one specifier with a list of values. PE is the module
// variable prefixExponent before the case, or nil for undefined. N has the
// outputs of the d3-format in node_modules. It is set only when they are not
// equal to O, the outputs of the bundled version.
type formatCase struct {
	S  string
	PE *int
	V  d3test.FS
	O  []string
	N  []string
}

// state returns the State that a case starts with.
func state(pe *int) *State {
	st := NewState()
	if pe != nil {
		st.prefixExponent, st.defined = *pe, true
	}
	return st
}

type golden struct {
	// D3Format is the version that made the expected values. Newer is the
	// version of d3-format in node_modules.
	D3Format string
	Newer    string
	Math     d3test.Math
	Groups   struct {
		Formats []formatCase
		Invalid []struct {
			S   string
			OK  bool
			Str *string
		}
		Parses []struct {
			specFields
			S   string
			Str string
		}
		Objects []struct {
			S   specFields
			Str string
			PE  *int
			V   d3test.FS
			O   []string
			N   []string
		}
		PrefixCases []struct {
			S   string
			Ref d3test.F
			V   d3test.FS
			O   []string
			N   []string
		}
		Precisions []struct {
			A, R d3test.FS
			N    *d3test.FS
		}
		Locales []formatCase
	}
}

// versions counts the cases that tell the bundled d3-format from the newer
// one, and those where the port gives the result of the newer one.
type versions struct {
	cases, wrong int
}

// outputs records one case that has the outputs of the newer version.
func (v *versions) outputs(f Formatter, vals []float64, newer []string) {
	if newer == nil || f == nil {
		return
	}
	v.cases++
	if len(newer) != len(vals) {
		return
	}
	for i, val := range vals {
		if f(val) != newer[i] {
			return
		}
	}
	v.wrong++
}

// mathExceptions lists, for each build mode (see d3test.Mode), the number of
// golden cases that differ from Node.js only because a Math function returns
// another last bit. The test proves each one: the case passes when it runs
// again with the results that Node.js returned for the same calls. Measured
// with go1.27.3.
//
// There are none: d3-format calls only Math.pow(10, n) with an integer n.
var mathExceptions = map[string]map[string]int{
	"arm64":       {},
	"arm64/nofma": {},
	"amd64":       {},
}

func compareOutputs(f Formatter, vals []float64, want []string) string {
	for i, v := range vals {
		if got := f(v); got != want[i] {
			return fmt.Sprintf("value %v: got %q, want %q", v, got, want[i])
		}
	}
	return ""
}

// TestAgainstD3 compares with values that tools/reference/gen-d3-format.mjs
// recorded from d3-format.
func TestAgainstD3(t *testing.T) {
	var g golden
	d3test.Load(t, "testdata/format.json.gz", &g)
	if g.D3Format != bundledVersion {
		t.Fatalf("the golden file comes from d3-format %q, want %s, the version in the mermaid bundle", g.D3Format, bundledVersion)
	}
	c := d3test.NewChecker(t, g.Math)
	var ver versions

	for i, tc := range g.Groups.Formats {
		c.Check("format", fmt.Sprintf("#%d %q", i, tc.S), func() string {
			f, err := state(tc.PE).Format(tc.S)
			if err != nil {
				return err.Error()
			}
			return compareOutputs(f, tc.V, tc.O)
		})
		if tc.N != nil {
			f, _ := state(tc.PE).Format(tc.S)
			ver.outputs(f, tc.V, tc.N)
		}
	}
	for _, tc := range g.Groups.Invalid {
		c.Check("formatSpecifier errors", fmt.Sprintf("%q", tc.S), func() string {
			spec, err := ParseSpecifier(tc.S)
			if (err == nil) != tc.OK {
				return fmt.Sprintf("error %v, want ok=%v", err, tc.OK)
			}
			if err == nil && spec.String() != *tc.Str {
				return fmt.Sprintf("got %q, want %q", spec.String(), *tc.Str)
			}
			return ""
		})
	}
	for i, tc := range g.Groups.Parses {
		c.Check("formatSpecifier", fmt.Sprintf("#%d %q", i, tc.S), func() string {
			spec, err := ParseSpecifier(tc.S)
			if err != nil {
				return err.Error()
			}
			want := tc.specFields.specifier()
			if spec.Fill != want.Fill || spec.Align != want.Align || spec.Sign != want.Sign || spec.Symbol != want.Symbol ||
				spec.Zero != want.Zero || !d3test.Same(spec.Width, want.Width) || spec.Comma != want.Comma ||
				!d3test.Same(spec.Precision, want.Precision) || spec.Trim != want.Trim || spec.Type != want.Type {
				return fmt.Sprintf("got %+v, want %+v", *spec, *want)
			}
			if spec.String() != tc.Str {
				return fmt.Sprintf("String() = %q, want %q", spec.String(), tc.Str)
			}
			return ""
		})
	}
	for i, tc := range g.Groups.Objects {
		c.Check("format(specifier object)", fmt.Sprintf("#%d %q", i, tc.Str), func() string {
			spec := tc.S.specifier()
			if spec.String() != tc.Str {
				return fmt.Sprintf("String() = %q, want %q", spec.String(), tc.Str)
			}
			f, err := state(tc.PE).Format(spec.String())
			if err != nil {
				return err.Error()
			}
			return compareOutputs(f, tc.V, tc.O)
		})
		if tc.N != nil {
			f, _ := state(tc.PE).Format(tc.S.specifier().String())
			ver.outputs(f, tc.V, tc.N)
		}
	}
	for i, tc := range g.Groups.PrefixCases {
		c.Check("formatPrefix", fmt.Sprintf("#%d %q ref %v", i, tc.S, tc.Ref.V()), func() string {
			f, err := FormatPrefix(tc.S, tc.Ref.V())
			if err != nil {
				return err.Error()
			}
			return compareOutputs(f, tc.V, tc.O)
		})
		if tc.N != nil {
			f, _ := FormatPrefix(tc.S, tc.Ref.V())
			ver.outputs(f, tc.V, tc.N)
		}
	}
	for i, tc := range g.Groups.Precisions {
		c.Check("precisionFixed/Prefix/Round", fmt.Sprintf("#%d %v", i, tc.A), func() string {
			got := []float64{PrecisionFixed(tc.A[0]), PrecisionPrefix(tc.A[0], tc.A[1]), PrecisionRound(tc.A[0], tc.A[1])}
			if !d3test.SameSlice(got, tc.R) {
				return fmt.Sprintf("got %v, want %v", got, tc.R)
			}
			return ""
		})
		if tc.N != nil {
			ver.cases++
			got := []float64{PrecisionFixed(tc.A[0]), PrecisionPrefix(tc.A[0], tc.A[1]), PrecisionRound(tc.A[0], tc.A[1])}
			if d3test.SameSlice(got, *tc.N) {
				ver.wrong++
			}
		}
	}
	loc := NewLocale(LocaleDefinition{
		Decimal: ",", Thousands: ".", Grouping: []int{3, 2}, Currency: [2]string{"", " €"},
		Numerals: []string{"٠", "١", "٢", "٣", "٤", "٥", "٦", "٧", "٨", "٩"}, Percent: " %", Minus: "-", Nan: "n/a",
	})
	for i, tc := range g.Groups.Locales {
		c.Check("formatLocale", fmt.Sprintf("#%d %q", i, tc.S), func() string {
			f, err := loc.FormatState(state(tc.PE), tc.S)
			if err != nil {
				return err.Error()
			}
			return compareOutputs(f, tc.V, tc.O)
		})
		if tc.N != nil {
			f, _ := loc.FormatState(state(tc.PE), tc.S)
			ver.outputs(f, tc.V, tc.N)
		}
	}

	c.Report()
	c.CheckExceptions(mathExceptions)

	t.Logf("%d cases have another result in d3-format %s", ver.cases, g.Newer)
	if g.Newer != bundledVersion && ver.cases < 500 {
		t.Errorf("%d cases tell d3-format %s from d3-format %s, want at least 500", ver.cases, bundledVersion, g.Newer)
	}
	if ver.wrong > 0 {
		t.Errorf("%d cases have the result of d3-format %s: the port must follow d3-format %s", ver.wrong, g.Newer, bundledVersion)
	}
}

// The port follows the d3-format of the mermaid bundle (3.1.0) and not the
// newer d3-format of node_modules (3.1.2). Each case gives another result in
// 3.1.2, which is in the comment. The expected values are those of the bundled
// d3-format, as the golden file has them.
func TestBundledVersion(t *testing.T) {
	inf := math.Inf(1)
	must := func(f Formatter, err error) Formatter {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
	check := func(what, got, want string) {
		t.Helper()
		if got != want {
			t.Errorf("%s: got %q, want %q", what, got, want)
		}
	}
	// Zero keeps the digits of the precision. 3.1.2 gives "0" and "0%".
	check(`format(".3r")(0)`, must(Format(".3r"))(0), "0.00")
	check(`format(".2p")(0)`, must(Format(".2p"))(0), "0.0%")

	// The type "s" in a new page: the variable prefixExponent is undefined.
	// 3.1.2 gives "Infinity" and "NaN".
	st := NewState()
	s := must(st.Format("s"))
	check(`format("s")(Infinity) in a new page`, s(inf), "Infinityundefined")
	check(`format("s")(NaN) in a new page`, s(math.NaN()), "NaNundefined")
	// A finite number writes the variable, and the next value that is not
	// finite takes its SI prefix. 3.1.2 gives "Infinity" and "NaN".
	check(`format("s")(1234)`, s(1234), "1.23400k")
	check(`format("s")(Infinity) after 1234`, s(inf), "Infinityk")
	check(`format("s")(NaN) after 1234`, must(st.Format(".3s"))(math.NaN()), "NaNk")
	// A formatter of the package function has a State of its own.
	check(`Format("s")(Infinity)`, must(Format("s"))(inf), "Infinityundefined")

	// formatPrefix adds the SI prefix after the padding and the parenthesis.
	// 3.1.2 gives "     1.23k" and "(1.2k)".
	check(`formatPrefix("10.2", 1e3)(1234)`, must(FormatPrefix("10.2", 1e3))(1234), "      1.23k")
	check(`formatPrefix("(.1", 1e3)(-1234)`, must(FormatPrefix("(.1", 1e3))(-1234), "(1.2)k")
	// A reference value of zero is 1. 3.1.2 gives "NaN".
	check(`formatPrefix(".2", 0)(1234)`, must(FormatPrefix(".2", 0))(1234), "1234.00")
	// 3.1.2 gives "NaN".
	check(`formatPrefix(".2", NaN)(1234)`, must(FormatPrefix(".2", math.NaN()))(1234), "NaNundefined")

	// The exponent of zero is 0. 3.1.2 gives NaN for all three.
	if got := PrecisionFixed(0); got != 0 {
		t.Errorf("precisionFixed(0): got %v, want 0", got)
	}
	if got := PrecisionPrefix(0, 0); got != 0 {
		t.Errorf("precisionPrefix(0, 0): got %v, want 0", got)
	}
	if got := PrecisionRound(0, 0); got != 1 {
		t.Errorf("precisionRound(0, 0): got %v, want 1", got)
	}
}

func TestWidthLimit(t *testing.T) {
	if _, err := Format("999999999999d"); err == nil {
		t.Error("a width above the string length limit of V8 must be an error")
	}
}
