// Package d3test holds the helpers that the golden tests of the d3 port share.
// The golden files come from the real d3 in node (tools/reference/gen-d3-*.mjs).
package d3test

import (
	"compress/gzip"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/internal/jsm"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// F is a float64 that the golden files store as the 16 hex digits of its bits.
type F float64

func (f *F) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	u, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		return err
	}
	*f = F(math.Float64frombits(u))
	return nil
}

// V returns the value.
func (f F) V() float64 { return float64(f) }

// FS is a list of float64 values that the golden files store as base64 of
// the little-endian bytes.
type FS []float64

func (f *FS) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	vals, err := decodeFloats(s)
	if err != nil {
		return err
	}
	*f = vals
	return nil
}

// NS is a list of float64 values that the golden files store as JSON
// numbers. The values that JSON cannot hold are the strings "NaN",
// "Infinity", "-Infinity" and "-0".
type NS []float64

func (f *NS) UnmarshalJSON(b []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	vals := make([]float64, len(raw))
	for i, r := range raw {
		if len(r) > 0 && r[0] == '"' {
			var s string
			if err := json.Unmarshal(r, &s); err != nil {
				return err
			}
			switch s {
			case "NaN":
				vals[i] = math.NaN()
			case "Infinity":
				vals[i] = math.Inf(1)
			case "-Infinity":
				vals[i] = math.Inf(-1)
			case "-0":
				vals[i] = math.Copysign(0, -1)
			default:
				return fmt.Errorf("unknown number %q", s)
			}
			continue
		}
		v, err := strconv.ParseFloat(string(r), 64)
		if err != nil {
			return err
		}
		vals[i] = v
	}
	*f = vals
	return nil
}

func decodeFloats(s string) ([]float64, error) {
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	if len(raw)%8 != 0 {
		return nil, fmt.Errorf("float list has %d bytes", len(raw))
	}
	vals := make([]float64, len(raw)/8)
	for i := range vals {
		vals[i] = math.Float64frombits(binary.LittleEndian.Uint64(raw[i*8:]))
	}
	return vals, nil
}

// Load reads a gzip-compressed JSON golden file into v.
func Load(t testing.TB, path string, v any) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

// Same reports whether a and b have the same bits. Every NaN equals every NaN,
// because JavaScript has one NaN.
func Same(a, b float64) bool {
	if math.IsNaN(a) && math.IsNaN(b) {
		return true
	}
	return math.Float64bits(a) == math.Float64bits(b)
}

// SameSlice compares two lists with Same.
func SameSlice(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !Same(a[i], b[i]) {
			return false
		}
	}
	return true
}

// Ulps is the distance between a and b in units of the last place.
func Ulps(a, b float64) int64 {
	if Same(a, b) {
		return 0
	}
	if math.IsNaN(a) || math.IsNaN(b) {
		return math.MaxInt64
	}
	ord := func(x float64) int64 {
		u := int64(math.Float64bits(x))
		if u < 0 {
			return math.MinInt64 - u
		}
		return u
	}
	d := ord(a) - ord(b)
	if d < 0 {
		d = -d
	}
	return d
}

// Math is the table of Math calls that Node.js made while it produced a
// golden file: function name to base64 float lists (arguments, then the
// result).
type Math map[string]string

type mathKey struct {
	fn   string
	a, b uint64
}

// FuncStat counts the calls of one Math function in which the port and
// Node.js differ.
type FuncStat struct {
	Calls   int   // calls that the table knows
	Differ  int   // calls with a different result
	MaxUlps int64 // largest distance
}

// Oracle replaces the functions of jsm with the results that Node.js
// returned.
type Oracle struct {
	table map[mathKey]float64
	// Stats of the current run, by function name.
	stats  map[string]*FuncStat
	misses int
}

// NewOracle builds an oracle from the math table of a golden file.
func NewOracle(t testing.TB, m Math) *Oracle {
	t.Helper()
	o := &Oracle{table: map[mathKey]float64{}}
	for fn, enc := range m {
		vals, err := decodeFloats(enc)
		if err != nil {
			t.Fatalf("math table %s: %v", fn, err)
		}
		n := 2
		if fn == "atan2" || fn == "pow" {
			n = 3
		}
		for i := 0; i+n <= len(vals); i += n {
			k := mathKey{fn: fn, a: math.Float64bits(vals[i])}
			if n == 3 {
				k.b = math.Float64bits(vals[i+1])
			}
			o.table[k] = vals[i+n-1]
		}
	}
	return o
}

func (o *Oracle) lookup(fn string, a, b float64, goResult float64) float64 {
	want, ok := o.table[mathKey{fn, math.Float64bits(a), math.Float64bits(b)}]
	if !ok {
		o.misses++
		return goResult
	}
	st := o.stats[fn]
	if st == nil {
		st = &FuncStat{}
		o.stats[fn] = st
	}
	st.Calls++
	if !Same(want, goResult) {
		st.Differ++
		if u := Ulps(want, goResult); u > st.MaxUlps {
			st.MaxUlps = u
		}
	}
	return want
}

func (o *Oracle) wrap1(fn string, f func(float64) float64) func(float64) float64 {
	return func(x float64) float64 { return o.lookup(fn, x, 0, f(x)) }
}

func (o *Oracle) wrap2(fn string, f func(float64, float64) float64) func(float64, float64) float64 {
	return func(x, y float64) float64 { return o.lookup(fn, x, y, f(x, y)) }
}

// Run calls fn with the functions of jsm replaced by the recorded results of
// Node.js. It returns the calls in which the port differs from Node.js, and
// the number of calls that the table does not know.
func (o *Oracle) Run(fn func()) (map[string]*FuncStat, int) {
	o.stats = map[string]*FuncStat{}
	o.misses = 0
	saved := []any{jsm.Sin, jsm.Cos, jsm.Tan, jsm.Asin, jsm.Acos, jsm.Atan, jsm.Atan2, jsm.Exp, jsm.Log, jsm.Log10, jsm.Log2, jsm.Cbrt, jsm.Pow}
	plainLog10 := saved[9].(func(float64) float64)
	jsm.Sin = o.wrap1("sin", saved[0].(func(float64) float64))
	jsm.Cos = o.wrap1("cos", saved[1].(func(float64) float64))
	jsm.Tan = o.wrap1("tan", saved[2].(func(float64) float64))
	jsm.Asin = o.wrap1("asin", saved[3].(func(float64) float64))
	jsm.Acos = o.wrap1("acos", saved[4].(func(float64) float64))
	jsm.Atan = o.wrap1("atan", saved[5].(func(float64) float64))
	jsm.Atan2 = o.wrap2("atan2", saved[6].(func(float64, float64) float64))
	jsm.Exp = o.wrap1("exp", js.Exp)
	jsm.Log = o.wrap1("log", js.Log)
	jsm.Log10 = o.wrap1("log10", plainLog10)
	jsm.Log2 = o.wrap1("log2", js.Log2)
	jsm.Cbrt = o.wrap1("cbrt", js.Cbrt)
	jsm.Pow = o.wrap2("pow", saved[12].(func(float64, float64) float64))
	defer func() {
		jsm.Sin = saved[0].(func(float64) float64)
		jsm.Cos = saved[1].(func(float64) float64)
		jsm.Tan = saved[2].(func(float64) float64)
		jsm.Asin = saved[3].(func(float64) float64)
		jsm.Acos = saved[4].(func(float64) float64)
		jsm.Atan = saved[5].(func(float64) float64)
		jsm.Atan2 = saved[6].(func(float64, float64) float64)
		jsm.Exp = saved[7].(func(float64) float64)
		jsm.Log = saved[8].(func(float64) float64)
		jsm.Log10 = saved[9].(func(float64) float64)
		jsm.Log2 = saved[10].(func(float64) float64)
		jsm.Cbrt = saved[11].(func(float64) float64)
		jsm.Pow = saved[12].(func(float64, float64) float64)
	}()
	fn()
	return o.stats, o.misses
}

// Checker runs golden cases. A case that fails with the Math functions of
// the port (package jsm) is run again with the results that Node.js returned
// for the same calls. If it passes then, the mismatch comes only from the
// last bits of a Math function, and the checker records it as an exception.
// Any other mismatch is a test failure.
//
// An exception is not always an error of the port. For Math.sin, Math.cos
// and the other trigonometric functions the port has the correctly rounded
// value, as Chromium has, and Node.js does not (see package jsm).
type Checker struct {
	t      testing.TB
	oracle *Oracle
	groups map[string]*groupStat
	order  []string
}

type groupStat struct {
	cases      int
	exceptions int
	funcs      map[string]*FuncStat
	examples   []string
}

// NewChecker returns a checker that uses the math table of a golden file.
func NewChecker(t testing.TB, m Math) *Checker {
	return &Checker{t: t, oracle: NewOracle(t, m), groups: map[string]*groupStat{}}
}

// Check runs one case of a group. The function run returns an empty string
// when the result equals the golden value, or a description of the difference.
func (c *Checker) Check(group, name string, run func() string) {
	c.t.Helper()
	g := c.groups[group]
	if g == nil {
		g = &groupStat{funcs: map[string]*FuncStat{}}
		c.groups[group] = g
		c.order = append(c.order, group)
	}
	g.cases++
	plain := run()
	if plain == "" {
		return
	}
	var withNode string
	stats, _ := c.oracle.Run(func() { withNode = run() })
	differ := 0
	for _, st := range stats {
		differ += st.Differ
	}
	if withNode != "" || differ == 0 {
		c.t.Errorf("%s %s: %s", group, name, plain)
		return
	}
	g.exceptions++
	var parts []string
	for fn, st := range stats {
		if st.Differ == 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s:%d ulp", fn, st.MaxUlps))
		agg := g.funcs[fn]
		if agg == nil {
			agg = &FuncStat{}
			g.funcs[fn] = agg
		}
		agg.Calls += st.Calls
		agg.Differ += st.Differ
		if st.MaxUlps > agg.MaxUlps {
			agg.MaxUlps = st.MaxUlps
		}
	}
	sort.Strings(parts)
	if len(g.examples) < 3 {
		g.examples = append(g.examples, fmt.Sprintf("%s (%s): %s", name, strings.Join(parts, ", "), plain))
	}
}

// Exceptions returns, for each group, the number of cases that differ from
// Node.js only by the last bits of a Math function.
func (c *Checker) Exceptions() map[string]int {
	out := map[string]int{}
	for name, g := range c.groups {
		if g.exceptions > 0 {
			out[name] = g.exceptions
		}
	}
	return out
}

// Report logs the number of cases and of Math exceptions for each group.
func (c *Checker) Report() {
	c.t.Helper()
	for _, name := range c.order {
		g := c.groups[name]
		if g.exceptions == 0 {
			c.t.Logf("%-28s %6d cases, all exact", name, g.cases)
			continue
		}
		var fns []string
		for fn, st := range g.funcs {
			fns = append(fns, fmt.Sprintf("%s differs in %d calls, max %d ulp", fn, st.Differ, st.MaxUlps))
		}
		sort.Strings(fns)
		c.t.Logf("%-28s %6d cases, %d differ from Node.js only by Math functions (%s)", name, g.cases, g.exceptions, strings.Join(fns, "; "))
		for _, ex := range g.examples {
			c.t.Logf("    example: %s", ex)
		}
	}
	c.t.Logf("Math exceptions in mode %s: %s", Mode(), literal(c.Exceptions()))
}

// literal prints the exceptions as a Go map literal, for the recorded lists.
func literal(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%q: %d", k, m[k])
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// Mode names the floating-point behaviour of this test build: the
// architecture, followed by "/nofma" when the compiler flag -d=fmahash=n
// disables fused multiply-add. The d3 port itself has no expression that the
// compiler can fuse (tools/fmacheck), so its results do not depend on the
// mode. Go's math package has such expressions, so the last bit of math.Pow,
// math.Sin and the other functions depends on the mode, and with it the
// number of cases that differ from Node.js.
func Mode() string {
	mode := runtime.GOARCH
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			if s.Key == "-gcflags" && strings.Contains(s.Value, "fmahash=n") {
				mode += "/nofma"
			}
		}
	}
	return mode
}

// CheckExceptions compares the measured exceptions with the recorded list for
// the mode of this build (see Mode). The list is the place where the known
// differences of the Math functions are written down. A difference from the
// list is a failure, so that nobody loosens or tightens the result without
// noticing.
func (c *Checker) CheckExceptions(recorded map[string]map[string]int) {
	c.t.Helper()
	mode := Mode()
	want, ok := recorded[mode]
	if !ok {
		c.t.Logf("no recorded Math exceptions for %s; measured: %s", mode, literal(c.Exceptions()))
		return
	}
	got := c.Exceptions()
	for name, n := range got {
		if want[name] != n {
			c.t.Errorf("%s: %d cases differ only by Math functions, the recorded list for %s has %d", name, n, mode, want[name])
		}
	}
	for name, n := range want {
		if _, ok := got[name]; !ok && n != 0 {
			c.t.Errorf("%s: no case differs by Math functions, the recorded list for %s has %d", name, mode, n)
		}
	}
}
