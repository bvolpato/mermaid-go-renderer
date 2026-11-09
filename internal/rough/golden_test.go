package rough

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// The golden file comes from tools/reference/gen-rough.mjs, which runs the
// roughjs 4.6.6 bundle in Node.js. Each call has the hash of each operation
// list (every number as its 8 IEEE 754 bytes), the attribute lists of
// RoughSVG.draw, the hash of the toPaths result and of the opsToPath strings,
// and the number of Math.random draws.
//
// The generator replaces Math.random with the PRNG of the reference harness,
// and one stream runs through all calls. It replaces Math.sin, Math.cos,
// Math.tan, Math.atan and Math.asin with the fdlibm source code of V8 in
// JavaScript, because the native functions depend on the build of Node.js.
//
// TestGolden gives the port the same functions (fdlibm_test.go) and needs
// every call to be identical. TestGoldenJSMath runs the port as the renderer
// runs it, with the wrappers of package js. A call can differ there only
// when one of its calls to a wrapper returns another value than the function
// of the golden file, and the test counts those calls for each function.

type goldenCall struct {
	F   string            `json:"f"`
	A   []json.RawMessage `json:"a"`
	O   json.RawMessage   `json:"o"`
	R   int               `json:"r"`  // Math.random draws of this call
	RB  uint32            `json:"rb"` // Math.random draws before this call
	I   int               `json:"i"`
	E   []string          `json:"e"` // [error class, message]
	Sh  string            `json:"sh"`
	S   [][]any           `json:"s"` // [type, number of ops, hash]
	D   [][][2]string     `json:"d"`
	T   string            `json:"t"`
	P2  *string           `json:"p2"`
	Pt  *string           `json:"pt"`
	Out string            `json:"outcome"`
}

type goldenCase struct {
	N     string          `json:"n"`
	C     json.RawMessage `json:"c"`
	Calls []goldenCall    `json:"calls"`
}

type goldenFile struct {
	Meta struct {
		Roughjs    string `json:"roughjs"`
		Node       string `json:"node"`
		V8         string `json:"v8"`
		RandomSeed uint32 `json:"randomSeed"`
		Calls      int    `json:"calls"`
		Draws      int    `json:"draws"`
	} `json:"meta"`
	Cases []goldenCase `json:"cases"`
	Hangs []goldenCall `json:"hangs"`
}

func loadGolden(t testing.TB) *goldenFile {
	t.Helper()
	f, err := os.Open("testdata/rough.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var g goldenFile
	if err := json.NewDecoder(zr).Decode(&g); err != nil {
		t.Fatal(err)
	}
	return &g
}

// decodeValue turns the JSON encoding of the generator into a JavaScript
// value: {"$u":1} is undefined, {"$n":"NaN"} is a number that JSON cannot
// hold, and null is null.
func decodeValue(raw json.RawMessage) any {
	var v any
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		panic(err)
	}
	return convertValue(v)
}

func convertValue(v any) any {
	switch x := v.(type) {
	case nil:
		return js.Null
	case json.Number:
		f, err := strconv.ParseFloat(string(x), 64)
		if err != nil {
			panic(err)
		}
		return f
	case map[string]any:
		if _, ok := x["$u"]; ok {
			return nil
		}
		if n, ok := x["$n"].(string); ok {
			switch n {
			case "NaN":
				return math.NaN()
			case "Infinity":
				return math.Inf(1)
			case "-Infinity":
				return math.Inf(-1)
			case "-0":
				return math.Copysign(0, -1)
			}
		}
		panic(fmt.Sprintf("unknown value %v", x))
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = convertValue(e)
		}
		return out
	}
	return v
}

func decodeOptions(raw json.RawMessage) Options {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var pairs [][2]json.RawMessage
	if err := json.Unmarshal(raw, &pairs); err != nil {
		panic(err)
	}
	o := js.NewObj[any]()
	for _, p := range pairs {
		var key string
		if err := json.Unmarshal(p[0], &key); err != nil {
			panic(err)
		}
		o.Set(key, decodeValue(p[1]))
	}
	return o
}

func decodeNumber(raw json.RawMessage) float64 {
	return decodeValue(raw).(float64)
}

func decodePoints(v any) []Point {
	list := v.([]any)
	pts := make([]Point, len(list))
	for i, p := range list {
		xy := p.([]any)
		pts[i] = Point{xy[0].(float64), xy[1].(float64)}
	}
	return pts
}

// oracleRandom is the Math.random of the reference harness (SEEDED_RANDOM in
// tools/reference/oracle.mjs).
type oracleRandom struct {
	state uint32
	calls int
}

func (r *oracleRandom) next() float64 {
	r.calls++
	r.state += 0x6d2b79f5
	t := r.state
	t = (t ^ (t >> 15)) * (t | 1)
	t ^= t + (t^(t>>7))*(t|61)
	return float64(t^(t>>14)) / 4294967296
}

// seek sets the generator to its state after n draws.
func (r *oracleRandom) seek(seed, n uint32) {
	r.state = seed + n*0x6d2b79f5
	r.calls = 0
}

// fdlibmMathFuncs are the transcendental functions that the generator script
// gives to roughjs (tools/reference/rough-fdlibm.mjs).
func fdlibmMathFuncs() mathFuncs {
	return mathFuncs{sin: fdlibmSin, cos: fdlibmCos, tan: fdlibmTan, atan: fdlibmAtan, asin: fdlibmAsin, pow: js.Pow}
}

func float64Bytes(buf []byte, v float64) {
	bits := math.Float64bits(v)
	if v != v {
		bits = 0x7ff8000000000000
	}
	binary.BigEndian.PutUint64(buf, bits)
}

func hashOps(ops []Op) string {
	h := sha256.New()
	for _, op := range ops {
		b := make([]byte, 2+8*len(op.Data))
		switch op.Op {
		case OpMove:
			b[0] = 0
		case OpBcurveTo:
			b[0] = 1
		case OpLineTo:
			b[0] = 2
		}
		b[1] = byte(len(op.Data))
		for i, v := range op.Data {
			float64Bytes(b[2+8*i:], v)
		}
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil))[:20]
}

func hashString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:20]
}

func hashPoints(lists [][]Point) string {
	h := sha256.New()
	for _, pts := range lists {
		b := make([]byte, 16*len(pts)+1)
		for i, p := range pts {
			float64Bytes(b[16*i:], p[0])
			float64Bytes(b[16*i+8:], p[1])
		}
		b[16*len(pts)] = 0xff
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil))[:20]
}

// tag is the type and the string form of a dynamic value.
func tag(v any) string {
	return js.TypeOf(v) + ":" + js.ToString(v)
}

// callResult is what one call of the port gives, in the form of the golden record.
type callResult struct {
	err    error
	shape  string
	sets   [][]any
	draw   [][][2]string
	t      string
	p2, pt *string
	draws  int
	detail func() string
}

// runCall makes the call that the golden record describes.
func runCall(g *Generator, c *goldenCall, rnd *oracleRandom) callResult {
	var d *Drawable
	var err error
	var pointLists [][]Point
	hasPoints := c.F == "linearPath" || c.F == "curve" || c.F == "curves" || c.F == "polygon"
	options := decodeOptions(c.O)
	num := func(i int) float64 { return decodeNumber(c.A[i]) }
	switch c.F {
	case "line":
		d = g.Line(num(0), num(1), num(2), num(3), options)
	case "rectangle":
		d, err = g.Rectangle(num(0), num(1), num(2), num(3), options)
	case "ellipse":
		d, err = g.Ellipse(num(0), num(1), num(2), num(3), options)
	case "circle":
		d, err = g.Circle(num(0), num(1), num(2), options)
	case "linearPath":
		pts := decodePoints(decodeValue(c.A[0]))
		pointLists = [][]Point{pts}
		d = g.LinearPath(pts, options)
	case "arc":
		closed := decodeValue(c.A[6]).(bool)
		d, err = g.Arc(num(0), num(1), num(2), num(3), num(4), num(5), closed, options)
	case "curve":
		pts := decodePoints(decodeValue(c.A[0]))
		pointLists = [][]Point{pts}
		d, err = g.Curve(pts, options)
	case "curves":
		for _, l := range decodeValue(c.A[0]).([]any) {
			pointLists = append(pointLists, decodePoints(l))
		}
		d, err = g.Curves(pointLists, options)
	case "polygon":
		pts := decodePoints(decodeValue(c.A[0]))
		pointLists = [][]Point{pts}
		d, err = g.Polygon(pts, options)
	case "path":
		d, err = g.Path(decodeValue(c.A[0]).(string), options)
	default:
		panic("unknown shape " + c.F)
	}
	res := callResult{err: err, draws: rnd.calls}
	if err != nil {
		return res
	}
	res.shape = d.Shape
	for _, s := range d.Sets {
		res.sets = append(res.sets, []any{string(s.Type), float64(len(s.Ops)), hashOps(s.Ops)})
	}
	for _, p := range g.Draw(d) {
		var attrs [][2]string
		for _, a := range p.Attrs {
			v := a.Value
			if a.Name == "d" {
				v = fmt.Sprintf("%d:%s", len(v), hashString(v))
			}
			attrs = append(attrs, [2]string{a.Name, v})
		}
		res.draw = append(res.draw, attrs)
	}
	var infos []string
	for _, p := range g.ToPaths(d) {
		infos = append(infos, strings.Join([]string{p.D, tag(p.Stroke), tag(p.StrokeWidth), tag(p.Fill)}, "\x00"))
	}
	res.t = hashString(strings.Join(infos, "\n"))
	if c.P2 != nil {
		var p2 []string
		for i := range d.Sets {
			p2 = append(p2, g.OpsToPath(&d.Sets[i], 2.0))
		}
		h := hashString(strings.Join(p2, "\n"))
		res.p2 = &h
	}
	if hasPoints {
		h := hashPoints(pointLists)
		res.pt = &h
	}
	res.draws = rnd.calls
	res.detail = func() string {
		var sb strings.Builder
		for _, s := range d.Sets {
			fmt.Fprintf(&sb, "set %s:\n", s.Type)
			for _, op := range s.Ops {
				sb.WriteString("  " + string(op.Op))
				for _, v := range op.Data {
					fmt.Fprintf(&sb, " %016x", math.Float64bits(v))
				}
				sb.WriteString("\n")
			}
		}
		for _, p := range g.Draw(d) {
			fmt.Fprintf(&sb, "path %v\n", p.Attrs)
		}
		return sb.String()
	}
	return res
}

// compare returns the differences between the result of the port and the
// golden record.
func compare(c *goldenCall, res callResult) []string {
	var diffs []string
	if len(c.E) > 0 {
		var e *Error
		if !errors.As(res.err, &e) {
			return []string{fmt.Sprintf("no error, want %s: %s", c.E[0], c.E[1])}
		}
		if e.Name != c.E[0] || e.Message != c.E[1] {
			diffs = append(diffs, fmt.Sprintf("error %s: %s, want %s: %s", e.Name, e.Message, c.E[0], c.E[1]))
		}
		if res.draws != c.R {
			diffs = append(diffs, fmt.Sprintf("%d Math.random draws before the error, want %d", res.draws, c.R))
		}
		return diffs
	}
	if res.err != nil {
		return []string{"error: " + res.err.Error()}
	}
	if res.draws != c.R {
		diffs = append(diffs, fmt.Sprintf("%d Math.random draws, want %d", res.draws, c.R))
	}
	if res.shape != c.Sh {
		diffs = append(diffs, fmt.Sprintf("shape %q, want %q", res.shape, c.Sh))
	}
	if len(res.sets) != len(c.S) {
		diffs = append(diffs, fmt.Sprintf("%d sets, want %d", len(res.sets), len(c.S)))
	} else {
		for i := range c.S {
			if res.sets[i][0] != c.S[i][0] || res.sets[i][1] != c.S[i][1] || res.sets[i][2] != c.S[i][2] {
				diffs = append(diffs, fmt.Sprintf("set %d is %v, want %v", i, res.sets[i], c.S[i]))
			}
		}
	}
	if len(res.draw) != len(c.D) {
		diffs = append(diffs, fmt.Sprintf("%d path elements, want %d", len(res.draw), len(c.D)))
	} else {
		for i := range c.D {
			if fmt.Sprint(res.draw[i]) != fmt.Sprint(c.D[i]) {
				diffs = append(diffs, fmt.Sprintf("path element %d has attributes %v, want %v", i, res.draw[i], c.D[i]))
			}
		}
	}
	if res.t != c.T {
		diffs = append(diffs, "the toPaths result differs")
	}
	if c.P2 != nil && (res.p2 == nil || *res.p2 != *c.P2) {
		diffs = append(diffs, "the opsToPath strings with 2 decimal places differ")
	}
	if (res.pt == nil) != (c.Pt == nil) || (res.pt != nil && *res.pt != *c.Pt) {
		diffs = append(diffs, "the points of the caller after the call differ")
	}
	return diffs
}

func describe(cs *goldenCase, c *goldenCall) string {
	a, _ := json.Marshal(c.A)
	if len(a) > 160 {
		a = append(a[:160], "..."...)
	}
	return fmt.Sprintf("call %d (%s) %s%s options %s config %s", c.I, cs.N, c.F, a, c.O, cs.C)
}

// TestGolden compares every call with roughjs. The port gets the same
// transcendental functions as the generator script, so every number must be
// identical.
func TestGolden(t *testing.T) {
	golden := loadGolden(t)
	dump := -1
	if s := os.Getenv("ROUGH_DUMP"); s != "" {
		dump, _ = strconv.Atoi(s)
	}
	rnd := &oracleRandom{}
	calls, failed, errs := 0, 0, 0
	shapes := map[string]int{}
	for ci := range golden.Cases {
		cs := &golden.Cases[ci]
		g := NewGenerator(Config{Options: decodeOptions(cs.C), Random: rnd.next})
		g.m = fdlibmMathFuncs()
		for i := range cs.Calls {
			c := &cs.Calls[i]
			rnd.seek(golden.Meta.RandomSeed, c.RB)
			res := runCall(g, c, rnd)
			calls++
			shapes[c.F]++
			if len(c.E) > 0 {
				errs++
			}
			if c.I == dump && res.detail != nil {
				t.Logf("%s\n%s", describe(cs, c), res.detail())
			}
			if diffs := compare(c, res); len(diffs) > 0 {
				failed++
				if failed <= 15 {
					t.Errorf("%s:\n  %s", describe(cs, c), strings.Join(diffs, "\n  "))
				}
			}
		}
	}
	if calls != golden.Meta.Calls {
		t.Errorf("the golden file has %d calls, want %d", calls, golden.Meta.Calls)
	}
	if calls < 3000 {
		t.Errorf("the golden file has %d calls, want at least 3000", calls)
	}
	t.Logf("%d calls (%d that throw in roughjs), %d differ; calls for each shape function: %v", calls, errs, failed, shapes)
	if failed > 0 {
		t.Errorf("%d of %d calls differ from roughjs", failed, calls)
	}
}

// mathTally counts the calls of one wrapper of package js, and the calls
// for which the wrapper gives another result than the function of the golden
// file.
type mathTally struct {
	calls, differ int
	maxULP        uint64
}

func ulpDistance(a, b float64) uint64 {
	if a != a || b != b {
		if (a != a) == (b != b) {
			return 0
		}
		return math.MaxUint64
	}
	ia, ib := int64(math.Float64bits(a)), int64(math.Float64bits(b))
	if ia < 0 {
		ia = math.MinInt64 - ia
	}
	if ib < 0 {
		ib = math.MinInt64 - ib
	}
	if ia > ib {
		return uint64(ia - ib)
	}
	return uint64(ib - ia)
}

// TestGoldenJSMath runs the port with the wrappers of package js, as the
// renderer does. A wrapper can give a result that differs in the last bits
// from the function that the generator script gave to roughjs. The test
// separates those differences from a difference in the port: a call can
// differ from roughjs only when at least one of its calls to a wrapper
// returns another value than the function of the golden file.
//
// The test does not fail for a difference of the first kind. It reports the
// number of calls for each function, and the largest difference.
func TestGoldenJSMath(t *testing.T) {
	goldenFile := loadGolden(t)
	rnd := &oracleRandom{}
	tallies := map[string]*mathTally{}
	differInCall := 0
	wrap1 := func(name string, jsFn, goldenFn func(float64) float64) func(float64) float64 {
		tally := &mathTally{}
		tallies[name] = tally
		return func(x float64) float64 {
			got, want := jsFn(x), goldenFn(x)
			tally.calls++
			if math.Float64bits(got) != math.Float64bits(want) && !(got != got && want != want) {
				tally.differ++
				differInCall++
				if d := ulpDistance(got, want); d > tally.maxULP {
					tally.maxULP = d
				}
			}
			return got
		}
	}
	prod := jsMathFuncs()
	golden := fdlibmMathFuncs()
	funcs := mathFuncs{
		sin:  wrap1("js.Sin", prod.sin, golden.sin),
		cos:  wrap1("js.Cos", prod.cos, golden.cos),
		tan:  wrap1("js.Tan", prod.tan, golden.tan),
		atan: wrap1("js.Atan", prod.atan, golden.atan),
		asin: wrap1("js.Asin", prod.asin, golden.asin),
		pow:  prod.pow,
	}
	calls, exact, mathAffected, portFailures := 0, 0, 0, 0
	for ci := range goldenFile.Cases {
		cs := &goldenFile.Cases[ci]
		g := NewGenerator(Config{Options: decodeOptions(cs.C), Random: rnd.next})
		g.m = funcs
		for i := range cs.Calls {
			c := &cs.Calls[i]
			rnd.seek(goldenFile.Meta.RandomSeed, c.RB)
			differInCall = 0
			res := runCall(g, c, rnd)
			calls++
			diffs := compare(c, res)
			switch {
			case len(diffs) == 0:
				exact++
			case differInCall > 0:
				mathAffected++
			default:
				portFailures++
				if portFailures <= 10 {
					t.Errorf("%s: every transcendental call agrees with the golden file, but the result differs:\n  %s", describe(cs, c), strings.Join(diffs, "\n  "))
				}
			}
		}
	}
	names := make([]string, 0, len(tallies))
	for name := range tallies {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		tally := tallies[name]
		t.Logf("%s: %d calls, %d differ from the fdlibm function of the golden file, largest difference %d ulp", name, tally.calls, tally.differ, tally.maxULP)
	}
	t.Logf("%d calls: %d identical to roughjs, %d differ and have a transcendental call that differs from the golden file, %d differ for another reason", calls, exact, mathAffected, portFailures)
	if portFailures > 0 {
		t.Errorf("%d calls differ from roughjs for a reason other than a transcendental function", portFailures)
	}
}

// TestHangs checks the inputs for which roughjs does not terminate. The
// generator script runs each one in a child process and records that the
// process reaches its time limit or its memory limit. The port must return
// an error of the class Hang.
func TestHangs(t *testing.T) {
	golden := loadGolden(t)
	if len(golden.Hangs) == 0 {
		t.Fatal("the golden file has no hang cases")
	}
	for i := range golden.Hangs {
		c := &golden.Hangs[i]
		if c.Out != "hang" {
			t.Errorf("hang case %d has the outcome %q in the golden file", i, c.Out)
			continue
		}
		rnd := &oracleRandom{}
		rnd.seek(golden.Meta.RandomSeed, 0)
		g := NewGenerator(Config{Random: rnd.next})
		g.m = fdlibmMathFuncs()
		res := runCall(g, c, rnd)
		var e *Error
		if !errors.As(res.err, &e) || e.Name != "Hang" {
			a, _ := json.Marshal(c.A)
			t.Errorf("%s%s options %s: got error %v, want an error of the class Hang", c.F, a, c.O, res.err)
		}
	}
}
