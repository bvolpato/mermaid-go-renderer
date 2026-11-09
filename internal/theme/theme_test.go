package theme

import (
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

// The golden file holds the results of the real mermaid 12.2.0 themes in V8.
// It is written by tools/reference/gen-theme.mjs.
type golden struct {
	Names   []string
	Keysets [][]string
	Cases   []struct {
		Theme     string
		Label     string
		Overrides json.RawMessage
		Result    struct {
			KS *int `json:"ks"`
			V  []json.RawMessage
			E  *string
			T  bool
		}
	}
}

func loadGolden(t *testing.T) *golden {
	t.Helper()
	f, err := os.Open("testdata/themes.json.gz")
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

// decode reads a value in the encoding of the generators: {"$u":1} is
// undefined and {"$n":"NaN"} is a number that JSON cannot hold.
func decode(t *testing.T, raw json.RawMessage) any {
	t.Helper()
	v, err := js.ParseJSON(raw)
	if err != nil {
		t.Fatalf("%v in %s", err, raw)
	}
	return revive(v)
}

func revive(v any) any {
	switch x := v.(type) {
	case []any:
		for i := range x {
			x[i] = revive(x[i])
		}
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
		for _, k := range x.Keys() {
			x.Set(k, revive(x.Get(k)))
		}
	}
	return v
}

// diff returns the path of the first difference between two dynamic values,
// or "". Objects must have the same keys in the same order.
func diff(path string, got, want any) string {
	switch w := want.(type) {
	case *js.Obj[any]:
		g, ok := got.(*js.Obj[any])
		if !ok || g == nil {
			return fmt.Sprintf("%s: got %s, want an object", path, show(got))
		}
		gk, wk := g.Keys(), w.Keys()
		for i := 0; i < len(gk) || i < len(wk); i++ {
			switch {
			case i >= len(gk):
				return fmt.Sprintf("%s: key %d: missing %q", path, i, wk[i])
			case i >= len(wk):
				return fmt.Sprintf("%s: key %d: extra %q", path, i, gk[i])
			case gk[i] != wk[i]:
				return fmt.Sprintf("%s: key %d: got %q, want %q", path, i, gk[i], wk[i])
			}
			if d := diff(path+"."+gk[i], g.Get(gk[i]), w.Get(wk[i])); d != "" {
				return d
			}
		}
		return ""
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			return fmt.Sprintf("%s: got %s, want %s", path, show(got), show(want))
		}
		for i := range w {
			if d := diff(fmt.Sprintf("%s[%d]", path, i), g[i], w[i]); d != "" {
				return d
			}
		}
		return ""
	case float64:
		g, ok := got.(float64)
		if !ok || !(math.Float64bits(g) == math.Float64bits(w) || (math.IsNaN(g) && math.IsNaN(w))) {
			return fmt.Sprintf("%s: got %s, want %s", path, show(got), show(want))
		}
		return ""
	}
	if _, isObj := got.(*js.Obj[any]); isObj || js.IsArray(got) || !js.StrictEquals(got, want) {
		return fmt.Sprintf("%s: got %s, want %s", path, show(got), show(want))
	}
	return ""
}

func show(v any) string {
	if v == nil {
		return "undefined"
	}
	if f, ok := v.(float64); ok {
		return js.Str(f)
	}
	return fmt.Sprintf("%T %s", v, js.StringifyJSON(v))
}

func TestAgainstMermaid(t *testing.T) {
	g := loadGolden(t)
	if d := fmt.Sprint(g.Names); d != fmt.Sprint(Names()) {
		t.Fatalf("theme names: got %v, want %v", Names(), g.Names)
	}
	failed := 0
	for i, c := range g.Cases {
		name := fmt.Sprintf("case %d: theme[%q].getThemeVariables(%s) %s", i, c.Theme, c.Overrides, c.Label)
		if len(name) > 300 {
			name = name[:300] + "..."
		}
		got, err := GetThemeVariables(c.Theme, decode(t, c.Overrides))
		var problem string
		switch {
		case c.Result.E != nil:
			var te *Error
			switch {
			case err == nil:
				problem = fmt.Sprintf("no error, want %q", *c.Result.E)
			case !errors.As(err, &te):
				problem = fmt.Sprintf("error %v is not a *theme.Error", err)
			case te.TypeError != c.Result.T:
				problem = fmt.Sprintf("error %q: TypeError is %v, want %v (%q)", err, te.TypeError, c.Result.T, *c.Result.E)
			case !c.Result.T && err.Error() != *c.Result.E:
				// The text of a TypeError depends on the V8 call site.
				problem = fmt.Sprintf("error %q, want %q", err, *c.Result.E)
			}
		case err != nil:
			problem = fmt.Sprintf("unexpected error %v", err)
		default:
			want := js.NewObj[any]()
			for j, k := range g.Keysets[*c.Result.KS] {
				want.Set(k, decode(t, c.Result.V[j]))
			}
			problem = diff("vars", got, want)
		}
		if problem != "" {
			failed++
			if failed <= 20 {
				t.Errorf("%s:\n  %s", name, problem)
			}
		}
	}
	t.Logf("%d golden cases, %d failed", len(g.Cases), failed)
}

// TestOverridesAreNotMutated checks the one mutation that upstream allows:
// none. The theme reads the overrides and may share nested values with them.
func TestOverridesAreNotMutated(t *testing.T) {
	ov, err := js.ParseJSON([]byte(`{"primaryColor":"#123456","xyChart":{"titleColor":"red"},"borderColorArray":["#111"]}`))
	if err != nil {
		t.Fatal(err)
	}
	before := js.StringifyJSON(ov)
	for _, name := range Names() {
		if _, err := Variables(name, ov.(*js.Obj[any])); err != nil {
			t.Fatal(err)
		}
	}
	if after := js.StringifyJSON(ov); after != before {
		t.Errorf("overrides changed:\n  before %s\n  after  %s", before, after)
	}
}

func TestUnknownTheme(t *testing.T) {
	if Has("nope") || !Has("redux-dark-color") {
		t.Error("Has is wrong")
	}
	if _, err := Variables("nope", nil); err == nil {
		t.Error("no error for an unknown theme")
	}
}

func TestLoopBound(t *testing.T) {
	ov := js.NewObj[any]()
	ov.Set("THEME_COLOR_LIMIT", 1e12)
	if _, err := Variables("base", ov); err == nil {
		t.Error("no error for a huge THEME_COLOR_LIMIT")
	}
}
