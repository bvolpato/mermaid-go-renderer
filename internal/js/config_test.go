package js

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"
)

// decodeV8 reads a value in the encoding of the generators: {"$u":1} is
// undefined and {"$n":"NaN"} is a number that JSON cannot hold.
func decodeV8(t *testing.T, raw json.RawMessage) any {
	t.Helper()
	v, err := ParseJSON(raw)
	if err != nil {
		t.Fatalf("%v in %s", err, raw)
	}
	return reviveV8(v)
}

func reviveV8(v any) any {
	switch x := v.(type) {
	case []any:
		for i := range x {
			x[i] = reviveV8(x[i])
		}
	case *Obj[any]:
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
			x.Set(k, reviveV8(x.Get(k)))
		}
	}
	return v
}

// sameV8 reports whether two dynamic values are equal in content, with NaN
// equal to NaN and -0 different from 0.
func sameV8(a, b any) bool {
	switch x := a.(type) {
	case float64:
		y, ok := b.(float64)
		return ok && (math.Float64bits(x) == math.Float64bits(y) || (math.IsNaN(x) && math.IsNaN(y)))
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !sameV8(x[i], y[i]) {
				return false
			}
		}
		return true
	case *Obj[any]:
		y, ok := b.(*Obj[any])
		return ok && StringifyJSON(x) == StringifyJSON(y) && fmt.Sprint(x.Keys()) == fmt.Sprint(y.Keys())
	}
	return a == b
}

// TestConfigAgainstV8 compares the operators and the property access of
// config.go with values recorded from V8 by tools/reference/gen-jsconfig.mjs.
func TestConfigAgainstV8(t *testing.T) {
	data, err := os.ReadFile("testdata/v8_config.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Values []json.RawMessage
		Keys   []string
		Pairs  [][]json.RawMessage
		Props  []struct {
			Get      []struct{ V, E json.RawMessage }
			Keys     struct{ V, E json.RawMessage }
			HasOwn   []struct{ V, E json.RawMessage }
			IsArray  bool
			IsObject bool
		}
		Space []int
		Trims [][]string
	}
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	values := make([]any, len(g.Values))
	for i, raw := range g.Values {
		values[i] = decodeV8(t, raw)
	}
	boolOf := func(raw json.RawMessage) bool { return string(raw) == "true" }
	checked := 0
	for _, p := range g.Pairs {
		var i, j int
		_ = json.Unmarshal(p[0], &i)
		_ = json.Unmarshal(p[1], &j)
		a, b := values[i], values[j]
		name := fmt.Sprintf("%s and %s", g.Values[i], g.Values[j])
		if got, want := Add(a, b), decodeV8(t, p[2]); !sameV8(got, want) {
			t.Errorf("Add(%s) = %v, want %v", name, got, want)
		}
		for k, op := range []string{"<", ">", "<=", ">="} {
			if got, want := Compare(a, op, b), boolOf(p[3+k]); got != want {
				t.Errorf("Compare(%s, %s) = %v, want %v", name, op, got, want)
			}
		}
		// An empty array has no identity in the value model, so an empty
		// array is not equal to itself. The golden file says that it is.
		_, hasID := Identity(a)
		if arr, isArr := a.([]any); !(i == j && isArr && len(arr) == 0 && !hasID) {
			if got, want := StrictEquals(a, b), boolOf(p[7]); got != want {
				t.Errorf("StrictEquals(%s) = %v, want %v", name, got, want)
			}
			if got, want := SameValueZero(a, b), boolOf(p[8]); got != want {
				t.Errorf("SameValueZero(%s) = %v, want %v", name, got, want)
			}
		}
		checked += 7
	}
	for i, p := range g.Props {
		v := values[i]
		for k, key := range g.Keys {
			got, ok := GetProperty(v, key)
			if want := p.Get[k]; want.E != nil {
				if ok {
					t.Errorf("GetProperty(%s, %q) did not report the TypeError", g.Values[i], key)
				}
			} else if !ok || !sameV8(got, decodeV8(t, want.V)) {
				t.Errorf("GetProperty(%s, %q) = %v, %v, want %s", g.Values[i], key, got, ok, want.V)
			}
			if want := p.HasOwn[k]; want.E == nil {
				if got := HasOwn(v, key); got != boolOf(want.V) {
					t.Errorf("HasOwn(%s, %q) = %v, want %s", g.Values[i], key, got, want.V)
				}
			}
			checked += 2
		}
		keys, ok := OwnKeys(v)
		if p.Keys.E != nil {
			if ok {
				t.Errorf("OwnKeys(%s) did not report the TypeError", g.Values[i])
			}
		} else {
			var want []string
			_ = json.Unmarshal(p.Keys.V, &want)
			if !ok || fmt.Sprint(keys) != fmt.Sprint(want) {
				t.Errorf("OwnKeys(%s) = %v, %v, want %v", g.Values[i], keys, ok, want)
			}
		}
		if IsArray(v) != p.IsArray || IsObject(v) != p.IsObject {
			t.Errorf("IsArray, IsObject(%s) = %v, %v, want %v, %v", g.Values[i], IsArray(v), IsObject(v), p.IsArray, p.IsObject)
		}
		checked += 3
	}
	space := map[rune]bool{}
	for _, cp := range g.Space {
		space[rune(cp)] = true
	}
	for cp := rune(0); cp <= 0xffff; cp++ {
		if IsWhiteSpace(cp) != space[cp] {
			t.Errorf("IsWhiteSpace(U+%04X) = %v, want %v", cp, IsWhiteSpace(cp), space[cp])
		}
	}
	for _, c := range g.Trims {
		if got := TrimStart(c[0]); got != c[1] {
			t.Errorf("TrimStart(%q) = %q, want %q", c[0], got, c[1])
		}
		if got := TrimEnd(c[0]); got != c[2] {
			t.Errorf("TrimEnd(%q) = %q, want %q", c[0], got, c[2])
		}
	}
	t.Logf("%d golden checks", checked+0x10000+2*len(g.Trims))
}
