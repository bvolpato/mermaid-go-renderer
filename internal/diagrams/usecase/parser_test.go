package usecase

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// The golden file testdata/parser.json.gz has what the real lexer, parser,
// CST visitor and model builder of mermaid make of a text, and the results
// of JSON.parse of V8: tools/reference/gen-usecase-parser.mjs writes it in
// plain node. The tests run the same texts through this port and compare
// everything.

type parserGolden struct {
	TokenTypes []string         `json:"tokenTypes"`
	Cases      []parserCase     `json:"cases"`
	JSON       []jsonGoldenCase `json:"json"`
}

type parserCase struct {
	Text string `json:"text"`
	// Tokens has [type, start offset, length, line, column] for each token.
	Tokens [][]int `json:"tokens"`
	// LexError is [offset, line, column, length, message].
	LexError  []any          `json:"lexError"`
	Model     map[string]any `json:"model"`
	Error     *string        `json:"error"`
	ErrorName string         `json:"errorName"`
}

type jsonGoldenCase struct {
	Text    string  `json:"text"`
	Value   any     `json:"value"`
	Message *string `json:"message"`
	// Ordered is [startLine, startColumn, value, propertyOrder].
	Ordered []any `json:"ordered"`
	// OrderedError is [startLine, startColumn, name, message, line, column].
	OrderedError []any `json:"orderedError"`
}

var (
	parserGoldenOnce sync.Once
	parserGoldenData *parserGolden
)

func loadParserGolden(tb testing.TB) *parserGolden {
	tb.Helper()
	parserGoldenOnce.Do(func() {
		f, err := os.Open(filepath.Join("testdata", "parser.json.gz"))
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
		var g parserGolden
		if err := json.Unmarshal(data, &g); err != nil {
			tb.Fatal(err)
		}
		parserGoldenData = &g
	})
	return parserGoldenData
}

// newParserDB returns a db for a parser test: its sanitizer changes nothing,
// as the commit of the generator keeps the texts of the model as they are.
func newParserDB() *DB {
	d := &DB{}
	d.Sanitize = func(text string) string { return text }
	d.state = d.createModel()
	return d
}

// ---- the Go side as JSON data ----

// wf is the text as encoding/json reads it from the golden file: a lone
// surrogate is U+FFFD there.
func wf(s string) string { return js.ToWellFormed(s) }

func stringList(list []string) []any {
	out := make([]any, len(list))
	for i, s := range list {
		out[i] = wf(s)
	}
	return out
}

// optString is `value ?? null` for an optional string that is "" when it
// is undefined.
func optString(s string) any {
	if s == "" {
		return nil
	}
	return wf(s)
}

// orderedJSON is the function ordered of the generator: an object is
// { $o: [[key, value], ...] } in the order of its properties.
func orderedJSON(v any) any {
	switch x := v.(type) {
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) || (x == 0 && math.Signbit(x)) {
			return map[string]any{"$n": numberText(x)}
		}
		return x
	case string:
		return wf(x)
	case bool:
		return x
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = orderedJSON(e)
		}
		return out
	case []string:
		return stringList(x)
	case *js.Obj[any]:
		entries := []any{}
		x.Each(func(k string, e any) { entries = append(entries, []any{wf(k), orderedJSON(e)}) })
		return map[string]any{"$o": entries}
	case *js.Obj[[]string]:
		entries := []any{}
		x.Each(func(k string, e []string) { entries = append(entries, []any{wf(k), stringList(e)}) })
		return map[string]any{"$o": entries}
	}
	if js.IsNullish(v) {
		return nil
	}
	panic(fmt.Sprintf("orderedJSON: unexpected %T", v))
}

// numberText is the text of a number that JSON cannot hold, as the
// generator writes it.
func numberText(x float64) string {
	if x == 0 && math.Signbit(x) {
		return "-0"
	}
	return js.Str(x)
}

func dumpModel(m *fields) map[string]any {
	actors := []any{}
	m.actors.Each(func(key string, a *Actor) {
		actors = append(actors, []any{
			wf(key), wf(a.ID), wf(a.Label), a.LabelType, a.Type, optString(a.Icon), a.Business,
			optString(a.Stereotype), optString(a.ParentID), stringList(a.Classes), stringList(a.Styles),
		})
	})
	useCases := []any{}
	m.useCases.Each(func(key string, u *UseCase) {
		useCases = append(useCases, []any{
			wf(key), wf(u.ID), wf(u.Label), u.LabelType, u.Shape, u.Business,
			optString(u.Stereotype), optString(u.ParentID), stringList(u.Classes), stringList(u.Styles),
		})
	})
	systemBoundaries := []any{}
	m.systemBoundaries.Each(func(key string, b *SystemBoundary) {
		systemBoundaries = append(systemBoundaries, []any{
			wf(key), wf(b.ID), wf(b.Label), b.LabelType, b.Type, stringList(b.Members), stringList(b.Classes), stringList(b.Styles),
		})
	})
	relationships := []any{}
	for _, r := range m.relationships {
		var label, labelType any
		if r.HasLabel {
			label, labelType = wf(r.Label), r.LabelType
		}
		relationships = append(relationships, []any{
			wf(r.ID), r.ExplicitID, wf(r.Source), wf(r.Target), r.Type, float64(r.ArrowType), label, labelType,
			r.Minlen, stringList(r.Classes), stringList(r.Styles), r.Animate, optString(r.Animation),
		})
	}
	notes := []any{}
	m.notes.Each(func(key string, n *UsecaseNote) {
		notes = append(notes, []any{key, n.ID, wf(n.Target), wf(n.Label), n.LabelType})
	})
	jsonNodes := []any{}
	m.jsonNodes.Each(func(key string, j *UsecaseJsonNode) {
		jsonNodes = append(jsonNodes, []any{
			wf(key), wf(j.ID), orderedJSON(j.Value), orderedJSON(j.PropertyOrder), stringList(j.Classes), stringList(j.Styles),
		})
	})
	classDefs := []any{}
	m.classDefs.Each(func(key string, c *ClassDef) {
		classDefs = append(classDefs, []any{wf(key), wf(c.ID), stringList(c.Styles)})
	})
	symbols := []any{}
	m.symbols.Each(func(id string, kind string) {
		symbols = append(symbols, []any{wf(id), kind})
	})
	return map[string]any{
		"actors":              actors,
		"useCases":            useCases,
		"systemBoundaries":    systemBoundaries,
		"relationships":       relationships,
		"notes":               notes,
		"jsonNodes":           jsonNodes,
		"classDefs":           classDefs,
		"symbols":             symbols,
		"direction":           m.direction,
		"relationshipCounter": float64(m.relationshipCounter),
		"noteCounter":         float64(m.noteCounter),
		"accTitle":            wf(m.accTitle),
		"accDescription":      wf(m.accDescription),
	}
}

// ---- comparison ----

// jsonDiff returns a description of the first difference between two JSON
// values, or "". The order of the elements of a list counts.
func jsonDiff(path string, got, want any) string {
	if reflect.DeepEqual(got, want) {
		return ""
	}
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			break
		}
		for k, wv := range w {
			gv, ok := g[k]
			if !ok {
				return fmt.Sprintf("%s.%s: missing, want %s", path, k, showJSON(wv))
			}
			if d := jsonDiff(path+"."+k, gv, wv); d != "" {
				return d
			}
		}
		for k, gv := range g {
			if _, ok := w[k]; !ok {
				return fmt.Sprintf("%s.%s: got %s, want it absent", path, k, showJSON(gv))
			}
		}
	case []any:
		g, ok := got.([]any)
		if !ok {
			break
		}
		for i := 0; i < len(g) && i < len(w); i++ {
			if d := jsonDiff(path+"["+strconv.Itoa(i)+"]", g[i], w[i]); d != "" {
				return d
			}
		}
		if len(g) != len(w) {
			return fmt.Sprintf("%s: length %d, want %d", path, len(g), len(w))
		}
	}
	return fmt.Sprintf("%s: got %s, want %s", path, showJSON(got), showJSON(want))
}

func showJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	if len(b) > 400 {
		return string(b[:400]) + "..."
	}
	return string(b)
}

func quoteText(text string) string {
	return strings.ReplaceAll(strings.ReplaceAll(text, "\r", "\\r"), "\n", "\\n")
}

// compareLexer compares the tokens or the first lexer error of a text with
// the real lexer. It returns the first difference, or "".
func compareLexer(g *parserGolden, c *parserCase) string {
	text := js.UTF16Units(c.Text)
	tokens, lexErr := tokenize(text)
	if c.LexError != nil {
		if lexErr == nil {
			return fmt.Sprintf("got %d tokens, want the lexer error %v", len(tokens), c.LexError)
		}
		got := []any{float64(lexErr.offset), float64(lexErr.line), float64(lexErr.column), float64(lexErr.length), wf(lexErr.message)}
		return jsonDiff("lexError", got, c.LexError)
	}
	if lexErr != nil {
		return fmt.Sprintf("got the lexer error %q, want %d tokens", lexErr.message, len(c.Tokens))
	}
	for i := 0; i < len(tokens) && i < len(c.Tokens); i++ {
		tok, want := tokens[i], c.Tokens[i]
		got := []int{0, tok.startOffset, tok.endOffset - tok.startOffset + 1, tok.startLine, tok.startColumn}
		wantName := g.TokenTypes[want[0]]
		if tokenTypeNames[tok.typ] != wantName || !reflect.DeepEqual(got[1:], want[1:]) {
			return fmt.Sprintf("token %d: got %s %v, want %s %v", i, tokenTypeNames[tok.typ], got[1:], wantName, want[1:])
		}
		if image := js.FromUTF16Units(text[tok.startOffset : tok.endOffset+1]); tok.image != image {
			return fmt.Sprintf("token %d: image %q, want %q", i, tok.image, image)
		}
	}
	if len(tokens) != len(c.Tokens) {
		return fmt.Sprintf("got %d tokens, want %d", len(tokens), len(c.Tokens))
	}
	return ""
}

// compareParser compares the model or the error of a text with the real
// parser. It returns the first difference, or "".
func compareParser(c *parserCase) (problem string) {
	defer func() {
		if p := recover(); p != nil {
			problem = fmt.Sprintf("panic: %v", p)
		}
	}()
	d := newParserDB()
	err := parse(d, c.Text)
	if c.Error != nil {
		if err == nil {
			return fmt.Sprintf("got a model, want the error %q", *c.Error)
		}
		if got := wf(err.Error()); got != *c.Error {
			return fmt.Sprintf("error:\n got %q\nwant %q", got, *c.Error)
		}
		name := "Error"
		if _, ok := err.(*jsonError); ok {
			name = "UsecaseJsonError"
		}
		wantName := c.ErrorName
		if wantName == "" {
			wantName = "Error"
		}
		if name != wantName {
			return fmt.Sprintf("error name: got %s, want %s", name, wantName)
		}
		return ""
	}
	if err != nil {
		return fmt.Sprintf("got the error %q, want a model", err)
	}
	return jsonDiff("model", dumpModel(d.state), c.Model)
}

// TestParserAgainstMermaid compares, for every text of the golden file, the
// tokens of the lexer and the model that the parser commits with the real
// lexer, parser, visitor and builder. A text that upstream rejects must
// fail here with the same message.
func TestParserAgainstMermaid(t *testing.T) {
	g := loadParserGolden(t)
	var lexOK, parseOK, models, errorCases, shown int
	for i := range g.Cases {
		c := &g.Cases[i]
		if c.Error != nil {
			errorCases++
		} else {
			models++
		}
		problem := compareLexer(g, c)
		if problem == "" {
			lexOK++
			problem = compareParser(c)
			if problem == "" {
				parseOK++
				continue
			}
		}
		if shown < 25 {
			t.Errorf("case %d: %s\n    text: %s", i, problem, quoteText(c.Text))
		} else {
			t.Fail()
		}
		shown++
	}
	t.Logf("%d texts: the tokens agree for %d, the model or the error agrees for %d (%d texts have a model upstream, %d have an error)",
		len(g.Cases), lexOK, parseOK, models, errorCases)
}

// TestJSONAgainstV8 compares JSON.parse of this port (the value, or the
// message of the SyntaxError) and parseOrderedJsonObject with V8 and with
// upstream, for the JSON texts of the golden file.
func TestJSONAgainstV8(t *testing.T) {
	g := loadParserGolden(t)
	var ok, invalid, shown int
	for i := range g.JSON {
		c := &g.JSON[i]
		problem := func() (problem string) {
			defer func() {
				if p := recover(); p != nil {
					problem = fmt.Sprintf("panic: %v", p)
				}
			}()
			value, message, valid := jsonParse(js.UTF16Units(c.Text))
			if c.Message != nil {
				if valid {
					return fmt.Sprintf("JSON.parse: got a value, want the error %q", *c.Message)
				}
				if got := wf(message); got != *c.Message {
					return fmt.Sprintf("JSON.parse error:\n got %q\nwant %q", got, *c.Message)
				}
			} else {
				if !valid {
					return fmt.Sprintf("JSON.parse: got the error %q, want a value", message)
				}
				if d := jsonDiff("value", orderedJSON(value), c.Value); d != "" {
					return d
				}
			}

			if c.Ordered != nil {
				parsed, err := parseOrderedJSONObject(c.Text, int(c.Ordered[0].(float64)), int(c.Ordered[1].(float64)))
				if err != nil {
					return fmt.Sprintf("parseOrderedJsonObject: got the error %q, want a value", err)
				}
				if d := jsonDiff("ordered.value", orderedJSON(parsed.value), c.Ordered[2]); d != "" {
					return d
				}
				return jsonDiff("ordered.propertyOrder", orderedJSON(parsed.propertyOrder), c.Ordered[3])
			}
			_, err := parseOrderedJSONObject(c.Text, int(c.OrderedError[0].(float64)), int(c.OrderedError[1].(float64)))
			if err == nil {
				return fmt.Sprintf("parseOrderedJsonObject: got a value, want the error %v", c.OrderedError[3])
			}
			jsonErr, isJSONError := err.(*jsonError)
			if !isJSONError {
				return fmt.Sprintf("parseOrderedJsonObject: got the error %q, want a UsecaseJsonError", err)
			}
			got := []any{"UsecaseJsonError", wf(jsonErr.message), float64(jsonErr.line), float64(jsonErr.column)}
			return jsonDiff("orderedError", got, c.OrderedError[2:])
		}()
		if c.Message != nil {
			invalid++
		}
		if problem == "" {
			ok++
			continue
		}
		if shown < 25 {
			t.Errorf("JSON case %d: %s\n    text: %s", i, problem, quoteText(c.Text))
		} else {
			t.Fail()
		}
		shown++
	}
	t.Logf("%d JSON texts: %d agree (%d are not valid JSON)", len(g.JSON), ok, invalid)
}
