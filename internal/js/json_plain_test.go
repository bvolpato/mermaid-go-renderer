package js

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"testing"
)

// jsonShape prints a parsed value with its types and the bits of its
// numbers, so that two values with the same text are the same value.
func jsonShape(v any) string {
	var sb strings.Builder
	var write func(v any)
	write = func(v any) {
		switch x := v.(type) {
		case nil:
			sb.WriteString("undefined")
		case *Obj[any]:
			sb.WriteByte('{')
			for _, k := range x.Keys() {
				fmt.Fprintf(&sb, "%q:", k)
				write(x.Get(k))
				sb.WriteByte(',')
			}
			sb.WriteByte('}')
		case []any:
			if x == nil {
				sb.WriteString("nil-array")
			}
			sb.WriteByte('[')
			for _, e := range x {
				write(e)
				sb.WriteByte(',')
			}
			sb.WriteByte(']')
		case float64:
			fmt.Fprintf(&sb, "num:%x", math.Float64bits(x))
		case string:
			fmt.Fprintf(&sb, "%q", x)
		default:
			fmt.Fprintf(&sb, "%T:%v", v, v)
		}
	}
	write(v)
	return sb.String()
}

// randomJSON makes a text that is JSON, or nearly JSON.
func randomJSON(rng *rand.Rand, depth int) string {
	ws := func() string { return []string{"", "", "", " ", "\n", "\t", "\r\n ", "\v", " "}[rng.Intn(9)] }
	str := func() string {
		return []string{`""`, `"a"`, `"key"`, `"x y"`, `"width"`, `"0"`, `"10"`, `"a\nb"`, `"é"`, `"é"`, `"😀"`,
			`"\ud83d"`, "\"\x7f\"", "\"a\x01\"", `"a\"b"`, `"__proto__"`, `"a/b"`, `"\/"`, "\"\xff\"", `"unterminated`,
			`"tab	"`, `"a longer property name than usual"`}[rng.Intn(22)]
	}
	switch n := rng.Intn(16); {
	case n < 3 && depth > 0:
		var sb strings.Builder
		sb.WriteString("{" + ws())
		for i, count := 0, rng.Intn(5); i < count; i++ {
			if i > 0 {
				sb.WriteString("," + ws())
			}
			sb.WriteString(str() + ws() + ":" + ws() + randomJSON(rng, depth-1) + ws())
		}
		if rng.Intn(30) == 0 {
			sb.WriteString(",")
		}
		return sb.String() + "}"
	case n < 5 && depth > 0:
		var sb strings.Builder
		sb.WriteString("[" + ws())
		for i, count := 0, rng.Intn(5); i < count; i++ {
			if i > 0 {
				sb.WriteString(ws() + "," + ws())
			}
			sb.WriteString(randomJSON(rng, depth-1))
		}
		if rng.Intn(30) == 0 {
			return sb.String()
		}
		return sb.String() + ws() + "]"
	case n < 8:
		return str()
	case n < 13:
		return []string{"0", "-0", "1", "-1", "12.5", "1e3", "1E+3", "1.5e-7", "0.1", "123456789012345678901234567890",
			"1e400", "-1e400", "01", "1.", ".5", "-", "+1", "1e", "0x10", "NaN", "Infinity", "4.9e-324", "1e-400",
			"9007199254740993", "0.30000000000000004", "1e5x", "2 3"}[rng.Intn(27)]
	}
	return []string{"true", "false", "null", "tru", "nul", "undefined", "True", "", "truex"}[rng.Intn(9)]
}

// TestPlainJSONAgainstDecoder compares ParseJSON, which reads plain texts
// itself, with the decoder of encoding/json alone: the same value or an
// error for the same texts.
func TestPlainJSONAgainstDecoder(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	plain, valid := 0, 0
	check := func(text string) {
		want, wantErr := parseJSONDecoder([]byte(text))
		got, gotErr := ParseJSON([]byte(text))
		if (gotErr == nil) != (wantErr == nil) {
			t.Fatalf("ParseJSON(%q): error %v, the decoder gives %v", text, gotErr, wantErr)
		}
		if wantErr != nil {
			if gotErr.Error() != wantErr.Error() {
				t.Fatalf("ParseJSON(%q): error %q, the decoder gives %q", text, gotErr, wantErr)
			}
			return
		}
		valid++
		if g, w := jsonShape(got), jsonShape(want); g != w {
			t.Fatalf("ParseJSON(%q) = %s, the decoder gives %s", text, g, w)
		}
		p := plainJSON{data: []byte(text)}
		if _, ok := p.value(); ok && p.end() {
			plain++
		}
	}
	for i := 0; i < 60000; i++ {
		text := randomJSON(rng, 4)
		switch rng.Intn(8) {
		case 0:
			text = " " + text + "\n"
		case 1:
			text += []string{" x", ",", "]", "}", " 1", "\x00"}[rng.Intn(6)]
		}
		check(text)
	}
	// Deep nesting, on both sides of the limit of the plain parser.
	for _, depth := range []int{maxPlainDepth - 1, maxPlainDepth, maxPlainDepth + 1, 2000} {
		check(strings.Repeat("[", depth) + "1" + strings.Repeat("]", depth))
		check(strings.Repeat(`{"a":`, depth) + "1" + strings.Repeat("}", depth))
	}
	if plain < 10000 || valid-plain < 2000 {
		t.Errorf("%d valid texts, %d of them plain", valid, plain)
	}
	t.Logf("%d valid texts, %d of them plain", valid, plain)
}

// TestStringifyJSONStrings checks the strings that StringifyJSON writes as
// they are against the ones that it writes character by character.
func TestStringifyJSONStrings(t *testing.T) {
	for in, want := range map[string]string{
		"":                 `""`,
		"plain text":       `"plain text"`,
		"a\"b":             `"a\"b"`,
		"a\\b":             `"a\\b"`,
		"a\nb\tc":          `"a\nb\tc"`,
		"\x01":             `"\u0001"`,
		"é<>&":             `"é<>&"`,
		"<>&'":             `"<>&'"`,
		"\x7f":             "\"\x7f\"",
		"\xff":             "\"�\"",
		"a/b":              `"a/b"`,
		" ":                "\" \"",
		"emoji \U0001F600": "\"emoji \U0001F600\"",
	} {
		if got := StringifyJSON(in); got != want {
			t.Errorf("StringifyJSON(%q) = %s, want %s", in, got, want)
		}
	}
}
