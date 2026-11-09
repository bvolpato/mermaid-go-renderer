package js

import (
	"encoding/json"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
)

// The golden file testdata/member_treeview.json comes from
// tools/reference/gen-js-member.mjs: it has the results of value?.[name] in
// V8 for values of every type and for the names of the prototype
// properties.

// memberValue reads a value of the golden file: {"$u":1} is undefined and
// {"$n":"NaN"} is NaN.
func memberValue(v any) any {
	switch x := v.(type) {
	case *Obj[any]:
		if x.Len() == 1 && x.Has("$u") {
			return nil
		}
		if x.Len() == 1 && x.Has("$n") {
			return math.NaN()
		}
		out := NewObj[any]()
		x.Each(func(key string, e any) { out.Set(key, memberValue(e)) })
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = memberValue(e)
		}
		return out
	}
	return v
}

// memberResult prints a result of GetMember in the form of the golden file.
func memberResult(v any) any {
	switch x := v.(type) {
	case nil:
		return map[string]any{"$u": 1.0}
	case Inherited:
		return "inherited"
	case float64:
		if math.IsNaN(x) {
			return map[string]any{"$n": "NaN"}
		}
		return x
	case *Obj[any]:
		out := map[string]any{}
		x.Each(func(key string, e any) { out[key] = memberResult(e) })
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = memberResult(e)
		}
		return out
	}
	if v == Null {
		return nil
	}
	if text, ok := v.(string); ok {
		// A code unit of a string can be half of a surrogate pair. The JSON
		// decoder of Go reads such a unit of the golden file as U+FFFD.
		return strings.ToValidUTF8(text, "\uFFFD")
	}
	return v
}

func TestGetMember(t *testing.T) {
	data, err := os.ReadFile("testdata/member_treeview.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Value  json.RawMessage `json:"value"`
		Name   string          `json:"name"`
		Result any             `json:"result"`
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 1900 {
		t.Fatalf("only %d cases", len(cases))
	}
	inherited := 0
	for _, c := range cases {
		parsed, err := ParseJSON(c.Value)
		if err != nil {
			t.Fatal(err)
		}
		got := memberResult(GetMember(memberValue(parsed), c.Name))
		want := c.Result
		// A function and a prototype object are inherited properties.
		if m, ok := want.(map[string]any); ok && len(m) == 1 && (m["$f"] != nil || m["$p"] != nil) {
			want = "inherited"
			inherited++
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("value %s, name %q: got %v, want %v", c.Value, c.Name, got, want)
		}
	}
	t.Logf("%d cases, %d inherited properties", len(cases), inherited)
}
