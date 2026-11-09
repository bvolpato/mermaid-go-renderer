package theme

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/memo"
)

// shape prints a result with the identity of its objects and arrays
// relative to the overrides: "shared" marks a value that is a value of the
// overrides itself.
func shape(vars *js.Obj[any], overrides any) string {
	fromOverrides := map[any]bool{}
	var collect func(v any)
	collect = func(v any) {
		if id, ok := js.Identity(v); ok {
			fromOverrides[id] = true
		}
		switch x := v.(type) {
		case *js.Obj[any]:
			if x != nil {
				x.Each(func(_ string, e any) { collect(e) })
			}
		case []any:
			for _, e := range x {
				collect(e)
			}
		}
	}
	collect(overrides)
	var sb strings.Builder
	var write func(v any)
	write = func(v any) {
		if id, ok := js.Identity(v); ok && fromOverrides[id] {
			sb.WriteString("shared:")
		}
		switch x := v.(type) {
		case nil:
			sb.WriteString("undefined")
		case *js.Obj[any]:
			if x == nil {
				sb.WriteString("nil-object")
				return
			}
			sb.WriteByte('{')
			x.Each(func(k string, e any) {
				fmt.Fprintf(&sb, "%q:", k)
				write(e)
				sb.WriteByte(',')
			})
			sb.WriteByte('}')
		case []any:
			sb.WriteByte('[')
			for _, e := range x {
				write(e)
				sb.WriteByte(',')
			}
			sb.WriteByte(']')
		case float64:
			fmt.Fprintf(&sb, "num:%x", math.Float64bits(x))
		default:
			fmt.Fprintf(&sb, "%T:%v", v, v)
		}
	}
	write(vars)
	return sb.String()
}

// cacheOverrides are overrides of every kind: with a key for the cache and
// without one.
func cacheOverrides(t *testing.T) []func() any {
	t.Helper()
	texts := []string{
		`{}`,
		`{"primaryColor":"#123456"}`,
		`{"primaryColor":"#123457"}`,
		`{"primaryColor":"#123456","darkMode":true,"fontSize":"20px","THEME_COLOR_LIMIT":3}`,
		`{"darkMode":true,"primaryColor":"#123456"}`,
		`{"xyChart":{"titleColor":"red"},"radar":{"axisColor":"#111"}}`,
		`{"xyChart":{"titleColor":"red","deep":{"x":1}}}`,
		`{"xyChart":{"plotColorPalette":["#111","#222"]}}`,
		`{"borderColorArray":["#111"],"primaryColor":"#abcdef"}`,
		`{"cScale0":null,"nodeBorder":"#f00","useGradient":false}`,
		`{"__proto__":{"x":1},"primaryColor":"#010203"}`,
		`{"updateColors":1}`,
		`{"primaryColor":"not a color"}`,
		`{"0":"a","10":"b","2":"c","background":"#000"}`,
		`{"borderColorArray":["#111","#222"],"bkgColorArray":["#333"],"primaryColor":"#abcdef"}`,
		`{"noteFontWeight":{"a":1},"fontFamily":["x"],"xyChart":["not an object"],"radar":"text"}`,
		`{"cScale0":[],"extra":{"a":1},"other":{"a":1}}`,
	}
	var out []func() any
	out = append(out, func() any { return nil }, func() any { return (*js.Obj[any])(nil) },
		func() any { return "text" }, func() any { return 3.0 }, func() any { return js.Null },
		func() any { return []any{"#111"} })
	for _, text := range texts {
		out = append(out, func() any {
			v, err := js.ParseJSON([]byte(text))
			if err != nil {
				t.Fatal(err)
			}
			return v
		})
	}
	// The variables of a theme as overrides, as setConfig(getConfig()) gives
	// them to the theme.
	for _, name := range []string{"forest", "redux-color"} {
		out = append(out, func() any {
			vars, err := computeThemeVariables(themes[name], nil)
			if err != nil {
				t.Fatal(err)
			}
			return vars
		})
	}
	return out
}

func themeResults(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, name := range Names() {
		for _, make := range cacheOverrides(t) {
			overrides := make()
			// The text of the overrides, with their own objects marked.
			text := func() string {
				if o, ok := overrides.(*js.Obj[any]); ok {
					if o == nil {
						return "nil-object"
					}
					return shape(o, nil)
				}
				return fmt.Sprint(overrides)
			}
			before := text()
			vars, err := GetThemeVariables(name, overrides)
			if after := text(); after != before {
				t.Fatalf("theme %s changed the overrides %s", name, before)
			}
			if err != nil {
				out = append(out, "error: "+err.Error())
				continue
			}
			out = append(out, shape(vars, overrides))
			// The result belongs to the caller: a change must not show in a
			// later result.
			vars.Set("primaryColor", "changed")
			vars.Set("added", true)
			vars.Each(func(_ string, v any) {
				if o, ok := v.(*js.Obj[any]); ok && o != nil && !o.Frozen() {
					o.Set("added", true)
				}
			})
		}
	}
	return out
}

// TestVariablesCache compares the results with the cache, on the first call
// and on later calls, with the results of the theme code for every call:
// the content, and which values the result shares with the overrides.
func TestVariablesCache(t *testing.T) {
	memo.SetEnabled(false)
	want := themeResults(t)
	memo.SetEnabled(true)
	memo.Purge()
	for round := 0; round < 3; round++ {
		got := themeResults(t)
		for i := range want {
			if got[i] != want[i] {
				n := len(cacheOverrides(t))
				t.Fatalf("round %d, theme %s, overrides %d:\n got  %.300s\n want %.300s", round, Names()[i/n], i%n, got[i], want[i])
			}
		}
	}
	if variables.Len() == 0 {
		t.Error("no result is in the cache")
	}
}

// TestOverridesKey checks which overrides have a key, and that the key has
// the types of the values.
func TestOverridesKey(t *testing.T) {
	obj := func(kv ...any) *js.Obj[any] {
		o := js.NewObj[any]()
		for i := 0; i+1 < len(kv); i += 2 {
			o.Set(kv[i].(string), kv[i+1])
		}
		return o
	}
	distinct := []any{
		nil, obj(), obj("a", nil), obj("a", js.Null), obj("a", false), obj("a", 0.0), obj("a", 0),
		obj("a", math.Copysign(0, -1)), obj("a", ""), obj("a", "0"), obj("a", obj()), obj("a", obj("b", 1.0)),
		obj("a", (*js.Obj[any])(nil)), obj("a", "b", "c", "d"), obj("c", "d", "a", "b"), obj("ab", "c"), obj("a", "bc"),
		obj("a", obj("b", "c")), obj("a", obj(), "b", "c"), obj("a", []any{}), obj("a", []any{"b"}),
		obj("a", []any{"b", "c"}), obj("a", []any{"bc"}), obj("a", []any{nil}), obj("a", []any{js.Null}),
	}
	seen := map[string]int{}
	for i, v := range distinct {
		key, ok := appendOverridesKey(nil, v)
		if !ok {
			t.Errorf("overrides %d have no key", i)
			continue
		}
		if j, dup := seen[string(key)]; dup {
			t.Errorf("overrides %d and %d have the same key", j, i)
		}
		seen[string(key)] = i
	}
	twice := obj("x", 1.0)
	list := []any{"#111"}
	for i, v := range []any{
		"text", 3.0, js.Null, []any{}, obj("a", obj("b", obj())), obj("a", obj("b", []any{})),
		obj("a", []any{obj()}), obj("a", []any{[]any{}}), obj("a", map[string]any{}),
		obj("a", twice, "b", twice), obj("a", list, "b", list),
	} {
		if _, ok := appendOverridesKey(nil, v); ok {
			t.Errorf("value %d must not have a key: %#v", i, v)
		}
	}
}

// TestVariablesCacheConcurrent asks for the same variables from many
// goroutines, and each goroutine changes its result.
func TestVariablesCacheConcurrent(t *testing.T) {
	memo.Purge()
	memo.SetEnabled(false)
	want := themeResults(t)
	memo.SetEnabled(true)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for round := 0; round < 2; round++ {
				got := themeResults(t)
				for i := range want {
					if got[i] != want[i] {
						t.Errorf("result %d differs", i)
						return
					}
				}
			}
		}()
	}
	wg.Wait()
}
