package utils

// The golden files of testdata come from tools/reference/oracle-utils.mjs,
// which runs the functions of mermaid 12.2.0 in headless Chromium. Rerun it
// with `node oracle-utils.mjs` in tools/reference.

import (
	"compress/gzip"
	"io"
	"math"
	"os"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/browser"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// loadGolden reads a gzip JSON file of testdata as a dynamic value. The
// objects keep the property order of JavaScript.
func loadGolden(t *testing.T, name string) *js.Obj[any] {
	t.Helper()
	f, err := os.Open("testdata/" + name)
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
	v, err := js.ParseJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	return v.(*js.Obj[any])
}

// untag decodes the tagged form of the golden files: {"$u":1} is undefined,
// and {"$n":"NaN"} is a number that JSON cannot hold.
func untag(v any) any {
	switch x := v.(type) {
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = untag(e)
		}
		return out
	case *js.Obj[any]:
		if x.Len() == 1 {
			if x.Has("$u") {
				return nil
			}
			if s, ok := x.Get("$n").(string); ok {
				switch s {
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
		}
		out := js.NewObj[any]()
		x.Each(func(k string, e any) { out.Set(k, untag(e)) })
		return out
	}
	return v
}

// same compares two dynamic values. Numbers must have the same bits, except
// that every NaN is the same. Objects must have the same keys in the same
// order.
func same(a, b any) bool {
	switch x := a.(type) {
	case float64:
		y, ok := b.(float64)
		if !ok {
			return false
		}
		if math.IsNaN(x) && math.IsNaN(y) {
			return true
		}
		return math.Float64bits(x) == math.Float64bits(y)
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !same(x[i], y[i]) {
				return false
			}
		}
		return true
	case *js.Obj[any]:
		y, ok := b.(*js.Obj[any])
		if !ok || x.Len() != y.Len() {
			return false
		}
		xk, yk := x.Keys(), y.Keys()
		for i := range xk {
			if xk[i] != yk[i] || !same(x.Get(xk[i]), y.Get(yk[i])) {
				return false
			}
		}
		return true
	}
	return a == b
}

// show prints a dynamic value for a failure message.
func show(v any) string {
	switch x := v.(type) {
	case nil:
		return "undefined"
	case float64:
		if x == 0 && math.Signbit(x) {
			return "-0"
		}
		return js.Str(x)
	case []any:
		s := "["
		for i, e := range x {
			if i > 0 {
				s += ","
			}
			s += show(e)
		}
		return s + "]"
	case *js.Obj[any]:
		if x == nil {
			return "<nil>"
		}
		s := "{"
		for i, k := range x.Keys() {
			if i > 0 {
				s += ","
			}
			s += k + ":" + show(x.Get(k))
		}
		return s + "}"
	case string:
		return js.QuoteJSON(x)
	}
	return js.ToString(v)
}

// cases returns the list of cases of one function.
func cases(t *testing.T, g *js.Obj[any], name string, atLeast int) []*js.Obj[any] {
	t.Helper()
	list, _ := g.Get(name).([]any)
	if len(list) < atLeast {
		t.Fatalf("%s: %d cases, want at least %d", name, len(list), atLeast)
	}
	out := make([]*js.Obj[any], len(list))
	for i, c := range list {
		out[i] = c.(*js.Obj[any])
	}
	return out
}

// failures counts the failures of a test and prints the first ones.
type failures struct {
	t     *testing.T
	total int
	count int
}

func (f *failures) check(ok bool, format string, args ...any) {
	f.total++
	if ok {
		return
	}
	f.count++
	// UTILS_ALL_FAILURES=1 prints every failure, for an analysis.
	if f.count <= 12 || os.Getenv("UTILS_ALL_FAILURES") != "" {
		f.t.Errorf(format, args...)
	}
}

func (f *failures) done(name string) {
	if f.count > 0 {
		f.t.Errorf("%s: %d of %d cases fail", name, f.count, f.total)
	} else {
		f.t.Logf("%s: %d cases", name, f.total)
	}
}

// newTestContext makes the context of a render in a document that is built
// like the page of the reference: body > div#container.
func newTestContext() *render.Context {
	doc := browser.NewDocument()
	container := doc.Body.Append("div").Attr("id", "container")
	return &render.Context{
		ID:        render.DefaultID,
		Doc:       doc,
		Engine:    doc.Engine.(*browser.Engine),
		Body:      doc.Body,
		Container: container,
		Now:       render.DefaultNow,
	}
}

// toPoints reads a list of [x, y] pairs.
func toPoints(v any) []Point {
	list, _ := v.([]any)
	pts := make([]Point, len(list))
	for i, p := range list {
		xy := p.([]any)
		pts[i] = Point{X: untag(xy[0]).(float64), Y: untag(xy[1]).(float64)}
	}
	return pts
}

func samePoints(a, b []Point) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !same(a[i].X, b[i].X) || !same(a[i].Y, b[i].Y) {
			return false
		}
	}
	return true
}

// pointValue is a Point as the object { x, y }.
func pointValue(p Point) *js.Obj[any] {
	o := js.NewObj[any]()
	o.Set("x", p.X)
	o.Set("y", p.Y)
	return o
}

// str reads a string property. A missing or undefined property is "".
func str(o *js.Obj[any], key string) string {
	s, _ := untag(o.Get(key)).(string)
	return s
}

// number reads a number property.
func number(o *js.Obj[any], key string) float64 {
	n, _ := untag(o.Get(key)).(float64)
	return n
}
