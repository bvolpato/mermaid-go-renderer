package js

import (
	"encoding/json"
	"math"
	"os"
	"strconv"
	"testing"
)

type v8Golden struct {
	Nums []struct {
		Bits, Str, Round string
		F0, F2, F5       *string
	}
	Strs []struct {
		S, PF, PI, PI16, Num string
	}
	Hypot []struct{ X, Y, H string }
}

func fromBits(t *testing.T, s string) float64 {
	t.Helper()
	u, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		t.Fatal(err)
	}
	return math.Float64frombits(u)
}

func sameFloat(a, b float64) bool {
	if math.IsNaN(a) && math.IsNaN(b) {
		return true
	}
	return math.Float64bits(a) == math.Float64bits(b)
}

// TestAgainstV8 compares number formatting and parsing with values recorded
// from V8 by tools/reference/gen-jsnum.mjs.
func TestAgainstV8(t *testing.T) {
	data, err := os.ReadFile("testdata/v8_numbers.json")
	if err != nil {
		t.Fatal(err)
	}
	var g v8Golden
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	for _, n := range g.Nums {
		x := fromBits(t, n.Bits)
		if got := Str(x); got != n.Str {
			t.Errorf("Str(%v) = %q, want %q", x, got, n.Str)
		}
		if got, want := Round(x), fromBits(t, n.Round); got != want {
			t.Errorf("Round(%v) = %v, want %v", x, got, want)
		}
		for digits, want := range map[int]*string{0: n.F0, 2: n.F2, 5: n.F5} {
			if want == nil {
				continue
			}
			if got := ToFixed(x, digits); got != *want {
				t.Errorf("ToFixed(%v, %d) = %q, want %q", x, digits, got, *want)
			}
		}
	}
	for _, s := range g.Strs {
		if got, want := ParseFloat(s.S), fromBits(t, s.PF); !sameFloat(got, want) {
			t.Errorf("ParseFloat(%q) = %v, want %v", s.S, got, want)
		}
		if got, want := ParseInt(s.S, 0), fromBits(t, s.PI); !sameFloat(got, want) {
			t.Errorf("ParseInt(%q) = %v, want %v", s.S, got, want)
		}
		if got, want := ParseInt(s.S, 16), fromBits(t, s.PI16); !sameFloat(got, want) {
			t.Errorf("ParseInt(%q, 16) = %v, want %v", s.S, got, want)
		}
		if got, want := Number(s.S), fromBits(t, s.Num); !sameFloat(got, want) {
			t.Errorf("Number(%q) = %v, want %v", s.S, got, want)
		}
	}
	for _, h := range g.Hypot {
		x, y := fromBits(t, h.X), fromBits(t, h.Y)
		if got, want := Hypot(x, y), fromBits(t, h.H); got != want {
			t.Errorf("Hypot(%v, %v) = %v, want %v", x, y, got, want)
		}
	}
}

func TestParseJSONRejectsTrailingText(t *testing.T) {
	if _, err := ParseJSON([]byte(`{"a":1} graph TD`)); err == nil {
		t.Fatal("ParseJSON accepted text after the value")
	}
	if _, err := ParseJSON([]byte("{\"a\":1} \n")); err != nil {
		t.Fatalf("ParseJSON rejected trailing white space: %v", err)
	}
}

func TestRoundNegativeZero(t *testing.T) {
	if r := Round(-0.2); r != 0 || !math.Signbit(r) {
		t.Fatalf("Round(-0.2) = %v, want -0", r)
	}
	if r := Round(-0.5); r != 0 || !math.Signbit(r) {
		t.Fatalf("Round(-0.5) = %v, want -0", r)
	}
}

func TestObjOrder(t *testing.T) {
	o := NewObj[int]()
	for i, k := range []string{"b", "10", "a", "2", "01", "-1", "0", "4294967295"} {
		o.Set(k, i)
	}
	want := []string{"0", "2", "10", "b", "a", "01", "-1", "4294967295"}
	if got := o.Keys(); !equal(got, want) {
		t.Fatalf("Keys() = %v, want %v", got, want)
	}
	o.Delete("a")
	o.Delete("2")
	o.Set("a", 9)
	o.Set("2", 9)
	want = []string{"0", "2", "10", "b", "01", "-1", "4294967295", "a"}
	if got := o.Keys(); !equal(got, want) {
		t.Fatalf("after delete and set: Keys() = %v, want %v", got, want)
	}
}

func TestMapOrder(t *testing.T) {
	m := NewMap[string, int]()
	for i, k := range []string{"b", "10", "a", "2"} {
		m.Set(k, i)
	}
	m.Delete("10")
	m.Set("10", 7)
	want := []string{"b", "a", "2", "10"}
	if got := m.Keys(); !equal(got, want) {
		t.Fatalf("Keys() = %v, want %v", got, want)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestJSONRoundTrip(t *testing.T) {
	in := `{"b":1,"10":"x","a":[1.5,null,{"z":true}],"2":{"k":"<&>\n"}}`
	v, err := ParseJSON([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	// JSON.stringify(JSON.parse(in)) in V8.
	want := `{"2":{"k":"<&>\n"},"10":"x","b":1,"a":[1.5,null,{"z":true}]}`
	if got := StringifyJSON(v); got != want {
		t.Fatalf("StringifyJSON = %s, want %s", got, want)
	}
	if got := ToString(Path(v, "a")); got != "1.5,,[object Object]" {
		t.Fatalf("ToString(array) = %q", got)
	}
}
