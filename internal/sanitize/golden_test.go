package sanitize

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// The goldens come from mermaid 12.2.0 in Chromium. Regenerate them with
// `node tools/reference/gen-sanitize.mjs`.

func readGolden(t testing.TB, name string, v any) {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
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
		t.Fatal(err)
	}
}

// result is one result of the oracle: a JSON value, a thrown error, or a
// value that JSON cannot hold (undefined, NaN, Infinity).
type result struct {
	raw     json.RawMessage
	throw   bool
	special string // "undefined", "NaN", "Infinity", "-Infinity"
}

func (r *result) UnmarshalJSON(data []byte) error {
	r.raw = append(r.raw[:0], data...)
	if len(data) > 0 && data[0] == '{' {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(data, &obj); err != nil {
			return err
		}
		if _, ok := obj["throw"]; ok {
			r.throw = true
		} else if v, ok := obj["value"]; ok && len(obj) == 1 {
			return json.Unmarshal(v, &r.special)
		}
	}
	return nil
}

// str returns the result as a string. ok is false for any other result.
func (r *result) str() (string, bool) {
	if r.throw || r.special != "" || len(r.raw) == 0 || r.raw[0] != '"' {
		return "", false
	}
	var s string
	if err := json.Unmarshal(r.raw, &s); err != nil {
		return "", false
	}
	return s, true
}

func (r *result) String() string {
	return string(r.raw)
}

// counter counts comparisons and prints the first failures.
type counter struct {
	t      *testing.T
	name   string
	total  int
	failed int
	// parser counts the failures on a text whose parse tree differs from
	// Chromium. They are not errors of this package.
	parser int
}

func (c *counter) check(ok bool, format string, args ...any) {
	c.total++
	if ok {
		return
	}
	c.failed++
	if c.failed <= 12 {
		c.t.Errorf(c.name+": "+format, args...)
	}
}

// checkText is check for a result that depends on the HTML parser. A failure
// is counted apart when the parser of the port builds another tree for the
// text than Chromium does.
func (c *counter) checkText(ok bool, text string, format string, args ...any) {
	if !ok && parserDiverges(c.t, text) {
		c.total++
		c.parser++
		return
	}
	c.check(ok, format, args...)
}

func (c *counter) done() {
	if c.parser > 0 {
		c.t.Logf("%s: %d of %d cases pass, %d differ because the parse tree differs from Chromium, %d fail",
			c.name, c.total-c.failed-c.parser, c.total, c.parser, c.failed)
	} else {
		c.t.Logf("%s: %d of %d cases pass", c.name, c.total-c.failed, c.total)
	}
	if c.failed > 12 {
		c.t.Errorf("%s: %d more failures not shown", c.name, c.failed-12)
	}
}

// chromiumTrees maps a text to the tree that DOMParser builds for it in
// Chromium, in the format of dom.Node.DumpTree.
var chromiumTrees map[string]string

// parserDiverges reports whether the HTML parser of the port builds a tree
// for the text that differs from the tree of Chromium. TestGoldenParse of
// internal/dom lists such inputs by cause.
func parserDiverges(t testing.TB, text string) bool {
	if chromiumTrees == nil {
		var golden struct {
			Parse struct {
				Inputs []string `json:"inputs"`
				Trees  []string `json:"trees"`
			} `json:"parse"`
		}
		readGolden(t, "sanitize_text.json.gz", &golden)
		chromiumTrees = make(map[string]string, len(golden.Parse.Inputs))
		for i, input := range golden.Parse.Inputs {
			chromiumTrees[input] = golden.Parse.Trees[i]
		}
	}
	want, ok := chromiumTrees[text]
	if !ok {
		return false
	}
	doc, err := dom.ParseHTMLDocument(text)
	return err != nil || doc.Root.DumpTree() != want
}

func parseValue(t testing.TB, raw json.RawMessage) any {
	t.Helper()
	v, err := js.ParseJSON(raw)
	if err != nil {
		t.Fatalf("%s: %v", raw, err)
	}
	return v
}

func parseConfig(t testing.TB, raw json.RawMessage) *js.Obj[any] {
	t.Helper()
	if len(raw) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return js.NewObj[any]()
	}
	o, ok := parseValue(t, raw).(*js.Obj[any])
	if !ok {
		t.Fatalf("config %s is not an object", raw)
	}
	return o
}

type urlGolden struct {
	SanitizeUrl struct {
		Inputs []string `json:"inputs"`
		Out    []result `json:"out"`
	} `json:"sanitizeUrl"`
	UrlHref struct {
		Inputs []string `json:"inputs"`
		Out    []result `json:"out"`
	} `json:"urlHref"`
	FormatUrl struct {
		Calls [][]json.RawMessage `json:"calls"`
		Out   []result            `json:"out"`
	} `json:"formatUrl"`
}

// TestGoldenSanitizeUrl compares SanitizeUrl with sanitizeUrl of
// @braintree/sanitize-url 7.1.2.
func TestGoldenSanitizeUrl(t *testing.T) {
	var golden urlGolden
	readGolden(t, "urls.json.gz", &golden)
	c := &counter{t: t, name: "sanitizeUrl"}
	for i, input := range golden.SanitizeUrl.Inputs {
		want, _ := golden.SanitizeUrl.Out[i].str()
		// A numeric character reference can give a lone surrogate. JSON
		// cannot carry it: the golden has U+FFFD in its place.
		got := js.ToWellFormed(SanitizeUrl(input))
		c.check(got == want, "%q\n  got  %q\n  want %q", input, got, want)
	}
	c.done()
}

// TestGoldenURLHref compares the URL parser with new URL(input).href. Only
// the schemes http and https are ported: sanitizeUrl needs no other.
func TestGoldenURLHref(t *testing.T) {
	var golden urlGolden
	readGolden(t, "urls.json.gz", &golden)
	c := &counter{t: t, name: "new URL"}
	skipped := 0
	for i, input := range golden.UrlHref.Inputs {
		want, parsed := golden.UrlHref.Out[i].str()
		got, ok := parseHTTPURL(input)
		if parsed && !(len(want) >= 5 && (want[:5] == "http:" || want[:6] == "https:")) {
			// A URL with another scheme. The port reports failure.
			if ok {
				c.check(false, "%q: got %q for a scheme that is not ported", input, got)
			} else {
				skipped++
			}
			continue
		}
		c.check(ok == parsed && got == want, "%q\n  got  %q %v\n  want %q %v", input, got, ok, want, parsed)
	}
	c.done()
	t.Logf("new URL: %d inputs with another scheme are not compared", skipped)
}

// TestGoldenFormatUrl compares FormatUrl with formatUrl of utils.ts.
func TestGoldenFormatUrl(t *testing.T) {
	var golden urlGolden
	readGolden(t, "urls.json.gz", &golden)
	c := &counter{t: t, name: "formatUrl"}
	for i, call := range golden.FormatUrl.Calls {
		var link string
		if err := json.Unmarshal(call[0], &link); err != nil {
			t.Fatal(err)
		}
		got, ok := FormatUrl(link, parseConfig(t, call[1]))
		// See TestGoldenSanitizeUrl for the lone surrogate.
		got = js.ToWellFormed(got)
		want := &golden.FormatUrl.Out[i]
		if want.special == "undefined" {
			c.check(!ok, "%q %s: got %q, want undefined", link, call[1], got)
			continue
		}
		wantStr, _ := want.str()
		c.check(ok && got == wantStr, "%q %s\n  got  %q\n  want %q", link, call[1], got, wantStr)
	}
	c.done()
}

// toJSON converts a Go result of a helper to the JSON form of the oracle.
func toJSON(v any) string {
	switch x := v.(type) {
	case float64:
		switch {
		case math.IsNaN(x):
			return `{"value":"NaN"}`
		case math.IsInf(x, 1):
			return `{"value":"Infinity"}`
		case math.IsInf(x, -1):
			return `{"value":"-Infinity"}`
		}
		return js.Str(x)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "error: " + err.Error()
	}
	return string(bytes.TrimSpace(buf.Bytes()))
}

// sameJSON compares two JSON texts by value.
func sameJSON(a, b string) bool {
	var va, vb any
	if json.Unmarshal([]byte(a), &va) != nil || json.Unmarshal([]byte(b), &vb) != nil {
		return false
	}
	return reflect.DeepEqual(va, vb)
}

// TestGoldenHelpers compares the helpers of common.ts and utils.ts with the
// real functions.
func TestGoldenHelpers(t *testing.T) {
	var golden struct {
		Location  Location `json:"location"`
		Functions []struct {
			Name  string              `json:"name"`
			Calls [][]json.RawMessage `json:"calls"`
			Out   []result            `json:"out"`
		} `json:"functions"`
	}
	readGolden(t, "helpers.json.gz", &golden)
	str := func(raw json.RawMessage) string {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		return s
	}
	arg := func(call []json.RawMessage, i int) string {
		if i >= len(call) {
			return ""
		}
		return str(call[i])
	}
	numbers := func(call []json.RawMessage) []float64 {
		out := make([]float64, len(call))
		for i, raw := range call {
			out[i] = js.ToNumber(parseValue(t, raw))
		}
		return out
	}
	for _, fn := range golden.Functions {
		c := &counter{t: t, name: fn.Name}
		for i, call := range fn.Calls {
			var got any
			switch fn.Name {
			case "getRows":
				got = GetRows(arg(call, 0))
			case "hasBreaks":
				got = HasBreaks(arg(call, 0))
			case "splitBreaks":
				got = SplitBreaks(arg(call, 0))
			case "breakToPlaceholder":
				got = BreakToPlaceholder(arg(call, 0))
			case "placeholderToBreak":
				got = PlaceholderToBreak(arg(call, 0))
			case "parseGenericTypes":
				got = ParseGenericTypes(arg(call, 0))
			case "processSet":
				got = ProcessSet(arg(call, 0))
			case "countOccurrence":
				got = CountOccurrence(arg(call, 0), arg(call, 1))
			case "shouldCombineSets":
				got = ShouldCombineSets(arg(call, 0), arg(call, 1))
			case "hasKatex":
				got = HasKatex(arg(call, 0))
			case "evaluate":
				var v any
				if len(call) > 0 {
					v = parseValue(t, call[0])
				}
				got = Evaluate(v)
			case "getEffectiveHtmlLabels":
				got = GetEffectiveHtmlLabels(parseConfig(t, call[0]))
			case "getMax":
				got = GetMax(numbers(call)...)
			case "getMin":
				got = GetMin(numbers(call)...)
			case "getUrl":
				var useAbsolute bool
				if err := json.Unmarshal(call[0], &useAbsolute); err != nil {
					t.Fatal(err)
				}
				got = GetUrl(useAbsolute, golden.Location)
			case "cssEscape":
				got = CSSEscape(arg(call, 0))
			case "encodeEntities":
				got = EncodeEntities(arg(call, 0))
			case "decodeEntities":
				got = DecodeEntities(arg(call, 0))
			case "removeScript":
				got = RemoveScript(arg(call, 0))
			case "sanitizeMore":
				got = New().SanitizeMore(arg(call, 0), parseConfig(t, call[1]))
			case "sanitizeTextOrArray":
				config := parseConfig(t, call[1])
				switch a := parseValue(t, call[0]).(type) {
				case string:
					got = New().SanitizeTextOrArray(a, config)
				case []any:
					got = New().SanitizeTextArray(a, config)
				}
			default:
				t.Fatalf("no test for the function %s", fn.Name)
			}
			gotJSON := toJSON(got)
			ok := sameJSON(gotJSON, fn.Out[i].String())
			switch fn.Name {
			case "removeScript", "sanitizeMore":
				c.checkText(ok, arg(call, 0), "%s\n  got  %s\n  want %s", call, gotJSON, fn.Out[i].String())
			default:
				c.check(ok, "%s\n  got  %s\n  want %s", call, gotJSON, fn.Out[i].String())
			}
		}
		c.done()
	}
}

// TestGoldenSanitizeText compares SanitizeText with sanitizeText of
// common.ts for every security level, htmlLabels value and dompurifyConfig
// of the golden, in the two states that a page can have: before and after
// setupDompurifyHooks.
func TestGoldenSanitizeText(t *testing.T) {
	var golden struct {
		Texts  map[string][]string `json:"texts"`
		Groups []struct {
			Config json.RawMessage   `json:"config"`
			Texts  string            `json:"texts"`
			On     []json.RawMessage `json:"on"`
			Off    []*result         `json:"off"`
			Fresh  []json.RawMessage `json:"fresh"`
		} `json:"groups"`
	}
	readGolden(t, "sanitize_text.json.gz", &golden)
	on := &counter{t: t, name: "sanitizeText with hooks"}
	off := &counter{t: t, name: "sanitizeText on a new page"}
	// compare checks one call against one result of the oracle.
	compare := func(c *counter, s *Sanitizer, text string, config *js.Obj[any], rawConfig json.RawMessage, want *result) {
		got, err := s.SanitizeTextErr(text, config)
		switch {
		case want.throw:
			c.check(err != nil, "%q %s: got %q, want an error", text, rawConfig, got)
		default:
			wantStr, _ := want.str()
			c.checkText(err == nil && got == wantStr, text, "%q %s\n  got  %q %v\n  want %q", text, rawConfig, got, err, wantStr)
		}
	}
	for _, g := range golden.Groups {
		texts := golden.Texts[g.Texts]
		config := parseConfig(t, g.Config)
		// The state after setupDompurifyHooks: one Sanitizer for the group.
		withHooks := New()
		withHooks.RemoveScript("")
		wantOn := make([]result, len(texts))
		for i, text := range texts {
			if string(g.On[i]) == "0" {
				// The result equals the input.
				quoted, _ := json.Marshal(text)
				wantOn[i].raw = quoted
			} else if err := wantOn[i].UnmarshalJSON(g.On[i]); err != nil {
				t.Fatal(err)
			}
			compare(on, withHooks, text, config, g.Config, &wantOn[i])
		}
		// The state of a new page. For a config that does not install the
		// hooks, the oracle recorded every text, and stored the differences
		// from the result with hooks.
		if g.Off != nil {
			clean := New()
			for i, text := range texts {
				want := &wantOn[i]
				if g.Off[i] != nil {
					want = g.Off[i]
				}
				compare(off, clean, text, config, g.Config, want)
			}
			if clean.HooksSetUp() {
				t.Errorf("config %s installed the hooks", g.Config)
			}
		}
		// For a config that installs the hooks, the oracle recorded a sample
		// of the texts, each on a new module instance.
		for _, raw := range g.Fresh {
			var pair []json.RawMessage
			if err := json.Unmarshal(raw, &pair); err != nil {
				t.Fatal(err)
			}
			var index int
			if err := json.Unmarshal(pair[0], &index); err != nil {
				t.Fatal(err)
			}
			var want result
			if err := want.UnmarshalJSON(pair[1]); err != nil {
				t.Fatal(err)
			}
			compare(off, New(), texts[index], config, g.Config, &want)
		}
	}
	on.done()
	off.done()
}
