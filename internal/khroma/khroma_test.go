package khroma

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// The golden file holds results of the real khroma 2.1.0 in V8. It is written
// by tools/reference/gen-khroma.mjs.
type golden struct {
	Colors   []string
	Unary    map[string][]json.RawMessage
	Amount   map[string][][]json.RawMessage
	Channels map[string][][]json.RawMessage
	Mix      [][]json.RawMessage
	Contrast [][]json.RawMessage
	RGBA     [][]json.RawMessage `json:"rgba"`
	HSLA     [][]json.RawMessage `json:"hsla"`
	Foreign  [][]json.RawMessage
}

func loadGolden(t *testing.T) *golden {
	t.Helper()
	f, err := os.Open("testdata/khroma.json.gz")
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
	var g golden
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	return &g
}

// encNum prints a number as the generator does: String(x), and "-0" for -0.
func encNum(x float64) string {
	if x == 0 && math.Signbit(x) {
		return "-0"
	}
	return js.Str(x)
}

// encode turns a Go result into the JSON form of the golden file.
func encode(v any, err error) string {
	if err != nil {
		var ke *Error
		if errors.As(err, &ke) && ke.TypeError {
			return `{"t":true}`
		}
		b, _ := json.Marshal(err.Error())
		return `{"e":` + string(b) + `,"t":false}`
	}
	switch x := v.(type) {
	case nil:
		return "null"
	case float64:
		return `"` + encNum(x) + `"`
	case bool:
		return fmt.Sprint(x)
	case string:
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(x)
		return string(bytes.TrimSpace(buf.Bytes()))
	}
	panic(fmt.Sprintf("unexpected result type %T", v))
}

// normalize rewrites an expected value so that it compares with encode. The
// message of a TypeError depends on the V8 call site and is not compared.
func normalize(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var probe struct {
		E *string
		T bool
	}
	if len(raw) > 0 && raw[0] == '{' {
		if err := json.Unmarshal(raw, &probe); err != nil {
			t.Fatal(err)
		}
		if probe.T {
			return `{"t":true}`
		}
		b, _ := json.Marshal(*probe.E)
		return `{"e":` + string(b) + `,"t":false}`
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	if s, ok := v.(string); ok {
		return encode(s, nil)
	}
	return string(raw)
}

type checker struct {
	t      *testing.T
	failed int
	total  int
}

func (c *checker) check(what string, want json.RawMessage, got any, err error) {
	c.t.Helper()
	c.total++
	w, g := normalize(c.t, want), encode(got, err)
	if w != g {
		c.failed++
		if c.failed <= 25 {
			c.t.Errorf("%s:\n  got  %s\n  want %s", what, g, w)
		}
	}
}

func num(t *testing.T, raw json.RawMessage) float64 {
	t.Helper()
	if string(raw) == "null" {
		return math.NaN() // an omitted argument is undefined
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func index(t *testing.T, raw json.RawMessage) int {
	t.Helper()
	var i int
	if err := json.Unmarshal(raw, &i); err != nil {
		t.Fatal(err)
	}
	return i
}

func unaryFunc(name string) func(color any) (any, error) {
	str := func(f func(any) (string, error)) func(any) (any, error) {
		return func(c any) (any, error) { return f(c) }
	}
	flt := func(f func(any) (float64, error)) func(any) (any, error) {
		return func(c any) (any, error) { return f(c) }
	}
	boolean := func(f func(any) (bool, error)) func(any) (any, error) {
		return func(c any) (any, error) { return f(c) }
	}
	switch name {
	case "toHex":
		return str(ToHex)
	case "toRgba":
		return str(ToRgba)
	case "toHsla":
		return str(ToHsla)
	case "toKeyword":
		return func(c any) (any, error) {
			name, ok, err := ToKeyword(c)
			if err != nil || !ok {
				return nil, err
			}
			return name, nil
		}
	case "red":
		return flt(Red)
	case "green":
		return flt(Green)
	case "blue":
		return flt(Blue)
	case "hue":
		return flt(Hue)
	case "saturation":
		return flt(Saturation)
	case "lightness":
		return flt(Lightness)
	case "alpha":
		return flt(Alpha)
	case "luminance":
		return flt(Luminance)
	case "isDark":
		return boolean(IsDark)
	case "isLight":
		return boolean(IsLight)
	case "isTransparent":
		return boolean(IsTransparent)
	case "isValid":
		return func(c any) (any, error) { return IsValid(c), nil }
	case "invert":
		return func(c any) (any, error) { return Invert(c) }
	case "complement":
		return str(Complement)
	case "grayscale":
		return str(Grayscale)
	}
	return nil
}

func amountFunc(name string) func(color any, amount float64) (string, error) {
	switch name {
	case "darken":
		return Darken
	case "lighten":
		return Lighten
	case "saturate":
		return Saturate
	case "desaturate":
		return Desaturate
	case "opacify":
		return Opacify
	case "transparentize":
		return Transparentize
	case "rgba":
		return WithAlpha
	case "invert":
		return func(color any, amount float64) (string, error) {
			if math.IsNaN(amount) {
				return Invert(color) // the default weight
			}
			return Invert(color, amount)
		}
	}
	return nil
}

func TestAgainstKhroma(t *testing.T) {
	g := loadGolden(t)
	c := &checker{t: t}

	for name, results := range g.Unary {
		fn := unaryFunc(name)
		if fn == nil {
			t.Fatalf("no Go function for %s", name)
		}
		for i, want := range results {
			got, err := fn(g.Colors[i])
			c.check(fmt.Sprintf("%s(%q)", name, g.Colors[i]), want, got, err)
		}
	}

	for name, cases := range g.Amount {
		fn := amountFunc(name)
		if fn == nil {
			t.Fatalf("no Go function for %s", name)
		}
		for _, cs := range cases {
			color := g.Colors[index(t, cs[0])]
			amount := num(t, cs[1])
			got, err := fn(color, amount)
			c.check(fmt.Sprintf("%s(%q, %v)", name, color, amount), cs[2], got, err)
		}
	}

	for name, cases := range g.Channels {
		fn := map[string]func(any, ...ChannelValue) (string, error){"adjust": Adjust, "change": Change, "scale": Scale}[name]
		if fn == nil {
			t.Fatalf("no Go function for %s", name)
		}
		for _, cs := range cases {
			color := g.Colors[index(t, cs[0])]
			var pairs [][]json.RawMessage
			if err := json.Unmarshal(cs[1], &pairs); err != nil {
				t.Fatal(err)
			}
			var values []ChannelValue
			for _, p := range pairs {
				var ch string
				if err := json.Unmarshal(p[0], &ch); err != nil {
					t.Fatal(err)
				}
				values = append(values, ChannelValue{ch[0], num(t, p[1])})
			}
			got, err := fn(color, values...)
			c.check(fmt.Sprintf("%s(%q, %s)", name, color, cs[1]), cs[2], got, err)
		}
	}

	for _, cs := range g.Mix {
		a, b := g.Colors[index(t, cs[0])], g.Colors[index(t, cs[1])]
		var got string
		var err error
		if string(cs[2]) == "null" {
			got, err = Mix(a, b)
		} else {
			got, err = Mix(a, b, num(t, cs[2]))
		}
		c.check(fmt.Sprintf("mix(%q, %q, %s)", a, b, cs[2]), cs[3], got, err)
	}

	for _, cs := range g.Contrast {
		a, b := g.Colors[index(t, cs[0])], g.Colors[index(t, cs[1])]
		got, err := Contrast(a, b)
		c.check(fmt.Sprintf("contrast(%q, %q)", a, b), cs[2], got, err)
	}

	for _, cs := range g.RGBA {
		var args []float64
		if err := json.Unmarshal(cs[0], &args); err != nil {
			t.Fatal(err)
		}
		c.check(fmt.Sprintf("rgba(%v)", args), cs[1], RGBA(args[0], args[1], args[2], args[3]), nil)
	}
	for _, cs := range g.HSLA {
		var args []float64
		if err := json.Unmarshal(cs[0], &args); err != nil {
			t.Fatal(err)
		}
		c.check(fmt.Sprintf("hsla(%v)", args), cs[1], HSLA(args[0], args[1], args[2], args[3]), nil)
	}

	t.Logf("%d golden cases, %d failed", c.total, c.failed)
}

// decodeForeign reads a value that is not a color string from the golden file.
func decodeForeign(t *testing.T, raw json.RawMessage) any {
	t.Helper()
	switch string(raw) {
	case `{"u":1}`:
		return nil
	case `{"nan":1}`:
		return math.NaN()
	}
	v, err := js.ParseJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// TestForeignValues covers colors that are not strings. khroma passes them
// through Color.parse unchanged.
func TestForeignValues(t *testing.T) {
	g := loadGolden(t)
	c := &checker{t: t}
	for _, cs := range g.Foreign {
		var name string
		if err := json.Unmarshal(cs[0], &name); err != nil {
			t.Fatal(err)
		}
		v := decodeForeign(t, cs[1])
		var got any
		var err error
		switch name {
		case "darken":
			got, err = Darken(v, 10)
		case "lighten":
			got, err = Lighten(v, 10)
		case "invert":
			got, err = Invert(v, 10)
		case "complement":
			got, err = Complement(v)
		case "grayscale":
			got, err = Grayscale(v)
		case "adjust":
			got, err = Adjust(v, H(10))
		case "mix":
			got, err = Mix(v, "#102030")
		default:
			fn := unaryFunc(name)
			if fn == nil {
				t.Fatalf("no Go function for %s", name)
			}
			got, err = fn(v)
		}
		c.check(fmt.Sprintf("%s(%s)", name, cs[1]), cs[2], got, err)
	}
	t.Logf("%d golden cases, %d failed", c.total, c.failed)
}

func TestErrorMessages(t *testing.T) {
	if _, err := Darken("nope", 10); err == nil || err.Error() != `Unsupported color format: "nope"` {
		t.Errorf("unexpected error %v", err)
	}
	if _, err := Adjust("#fff", H(10), R(5)); err == nil || err.Error() != "Cannot change both RGB and HSL channels at the same time" {
		t.Errorf("unexpected error %v", err)
	}
}
