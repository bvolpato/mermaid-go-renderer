package jsyaml

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// The golden file has the results of yaml.load(text, { schema:
// yaml.JSON_SCHEMA }) of the js-yaml that mermaid bundles:
// tools/reference/gen-agentflow-yaml.mjs writes it in node. The test loads
// the same texts and compares the documents, with the order of their keys,
// and the reason, the message and the mark of each exception.

type goldenMark struct {
	Line     int `json:"line"`
	Column   int `json:"column"`
	Position int `json:"position"`
	Buffer   any `json:"buffer"`
	Snippet  any `json:"snippet"`
}

type goldenError struct {
	Reason  any         `json:"reason"`
	Message any         `json:"message"`
	Mark    *goldenMark `json:"mark"`
}

type goldenCase struct {
	Input  any             `json:"input"`
	Result json.RawMessage `json:"result"`
	Cyclic bool            `json:"cyclic"`
	Error  *goldenError    `json:"error"`
}

func loadGolden(tb testing.TB) []goldenCase {
	tb.Helper()
	f, err := os.Open(filepath.Join("testdata", "yaml.json.gz"))
	if err != nil {
		tb.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		tb.Fatal(err)
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		tb.Fatal(err)
	}
	var g struct {
		Cases []goldenCase `json:"cases"`
	}
	if err := json.Unmarshal(data, &g); err != nil {
		tb.Fatal(err)
	}
	return g.Cases
}

// text decodes a string of the golden file: a JSON string, or the code
// units of a string with a lone surrogate ({ $s: [units] }).
func text(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case map[string]any:
		list, _ := x["$s"].([]any)
		units := make([]rune, len(list))
		for i, u := range list {
			units[i] = rune(u.(float64))
		}
		return js.FromUTF16Units(units)
	}
	return ""
}

// decode converts a value of the golden file to a dynamic value of
// internal/js. See the encode function of the generator.
func decode(v any) any {
	switch x := v.(type) {
	case nil:
		return js.Null
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = decode(e)
		}
		return out
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
		if _, ok := x["$s"]; ok {
			return text(x)
		}
		pairs, _ := x["$o"].([]any)
		obj := js.NewObj[any]()
		for _, p := range pairs {
			pair := p.([]any)
			obj.Set(text(pair[0]), decode(pair[1]))
		}
		return obj
	}
	return v
}

// diff describes the first difference between two documents, or returns "".
// The order of the keys of an object counts.
func diff(path string, got, want any) string {
	switch w := want.(type) {
	case nil:
		if got != nil {
			return fmt.Sprintf("%s: got %s, want undefined", path, show(got))
		}
		return ""
	case float64:
		g, ok := got.(float64)
		if !ok || math.Float64bits(g) != math.Float64bits(w) && !(math.IsNaN(g) && math.IsNaN(w)) {
			return fmt.Sprintf("%s: got %s, want %s", path, show(got), show(want))
		}
		return ""
	case []any:
		g, ok := got.([]any)
		if !ok {
			return fmt.Sprintf("%s: got %s, want an array", path, show(got))
		}
		for i := 0; i < len(g) && i < len(w); i++ {
			if d := diff(fmt.Sprintf("%s[%d]", path, i), g[i], w[i]); d != "" {
				return d
			}
		}
		if len(g) != len(w) {
			return fmt.Sprintf("%s: length %d, want %d", path, len(g), len(w))
		}
		return ""
	case *js.Obj[any]:
		g, ok := got.(*js.Obj[any])
		if !ok || g == nil {
			return fmt.Sprintf("%s: got %s, want an object", path, show(got))
		}
		gk, wk := g.Keys(), w.Keys()
		if strings.Join(gk, "\x00") != strings.Join(wk, "\x00") {
			return fmt.Sprintf("%s: keys %q, want %q", path, gk, wk)
		}
		for _, k := range wk {
			if d := diff(path+"."+k, g.Get(k), w.Get(k)); d != "" {
				return d
			}
		}
		return ""
	}
	if !js.StrictEquals(got, want) {
		return fmt.Sprintf("%s: got %s, want %s", path, show(got), show(want))
	}
	return ""
}

func show(v any) string {
	if f, ok := v.(float64); ok {
		return js.Str(f)
	}
	if v == nil {
		return "undefined"
	}
	s := js.StringifyJSON(v)
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return s
}

func clip(s string) string {
	if len(s) > 300 {
		return s[:300] + "..."
	}
	return s
}

// compare loads the text of a case and returns the first difference with
// the result of js-yaml, or "".
func compare(c *goldenCase) (problem string) {
	defer func() {
		if p := recover(); p != nil {
			problem = fmt.Sprintf("panic: %v", p)
		}
	}()
	doc, err := Load(text(c.Input))
	switch {
	case c.Cyclic:
		if !errors.Is(err, ErrCyclic) {
			return fmt.Sprintf("got %s, %v, want a document that contains itself", show(doc), err)
		}
		return ""
	case c.Error != nil:
		var ex *Exception
		if !errors.As(err, &ex) {
			return fmt.Sprintf("got %s, %v, want the exception %q", show(doc), err, text(c.Error.Reason))
		}
		if ex.Reason != text(c.Error.Reason) {
			return fmt.Sprintf("reason: got %q, want %q", ex.Reason, text(c.Error.Reason))
		}
		wantMark := c.Error.Mark
		switch {
		case wantMark == nil && ex.Mark != nil:
			return "got a mark, want none"
		case wantMark != nil && ex.Mark == nil:
			return "got no mark"
		case wantMark != nil:
			if ex.Mark.Line != wantMark.Line || ex.Mark.Column != wantMark.Column || ex.Mark.Position != wantMark.Position {
				return fmt.Sprintf("mark: got line %d column %d position %d, want line %d column %d position %d (%s)",
					ex.Mark.Line, ex.Mark.Column, ex.Mark.Position, wantMark.Line, wantMark.Column, wantMark.Position, ex.Reason)
			}
			if ex.Mark.Buffer != text(wantMark.Buffer) {
				return fmt.Sprintf("buffer: got %q, want %q", ex.Mark.Buffer, text(wantMark.Buffer))
			}
			if ex.Mark.Snippet != text(wantMark.Snippet) {
				return fmt.Sprintf("snippet:\n got %q\nwant %q", ex.Mark.Snippet, text(wantMark.Snippet))
			}
		}
		if ex.Message != text(c.Error.Message) {
			return fmt.Sprintf("message:\n got %q\nwant %q", ex.Message, text(c.Error.Message))
		}
		return ""
	}
	if err != nil {
		return fmt.Sprintf("got the error %v, want a document", err)
	}
	var raw any
	if e := json.Unmarshal(c.Result, &raw); e != nil {
		return e.Error()
	}
	return diff("doc", doc, decode(raw))
}

// TestLoadGolden compares Load with js-yaml for every text of the golden
// file.
func TestLoadGolden(t *testing.T) {
	cases := loadGolden(t)
	var ok, errorCases, cyclic int
	failed := 0
	for i := range cases {
		c := &cases[i]
		if problem := compare(c); problem != "" {
			failed++
			if failed <= 25 {
				t.Errorf("case %d: %s\n--- text ---\n%q", i, problem, clip(text(c.Input)))
			}
			continue
		}
		ok++
		switch {
		case c.Error != nil:
			errorCases++
		case c.Cyclic:
			cyclic++
		}
	}
	t.Logf("cases: %d of %d agree (%d exceptions, %d documents that contain themselves)", ok, len(cases), errorCases, cyclic)
	if failed > 25 {
		t.Errorf("%d more cases differ", failed-25)
	}
	if len(cases) == 0 {
		t.Error("no cases")
	}
}

// TestLoadNoPanic loads cut and joined texts of the golden file. The
// requirement is: no panic and no hang.
func TestLoadNoPanic(t *testing.T) {
	cases := loadGolden(t)
	n := 0
	for i := range cases {
		a := text(cases[i].Input)
		b := text(cases[(i*7+3)%len(cases)].Input)
		for _, input := range []string{a + b, b + "\n" + a, a[:len(a)/2] + b, strings.Repeat(a, 3)} {
			input = strings.ToValidUTF8(input, "�")
			func() {
				defer func() {
					if p := recover(); p != nil {
						t.Errorf("panic: %v\n--- text ---\n%q", p, clip(input))
					}
				}()
				_, _ = Load(input)
			}()
			n++
		}
	}
	t.Logf("%d texts: no panic", n)
}
