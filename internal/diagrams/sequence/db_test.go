package sequence

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// readGzipJSON reads a golden file of testdata as a dynamic JavaScript value.
func readGzipJSON(t testing.TB, name string) any {
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
	v, err := js.ParseJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// parseToDB runs the steps of mermaid.parse and getDiagramFromText: the
// config of the front matter and of the directives, the type detection, the
// db, the init hook and the parser. The title of the front matter is not
// set, as in getDiagramFromText(text) without metadata.
func parseToDB(text string) (*db, error) {
	resolved, err := config.Resolve(text, nil)
	if err != nil {
		return nil, err
	}
	ctx := render.NewContext(render.ContextOptions{
		Config:      resolved.Config,
		ConfigState: resolved.State,
	})
	diagramType, err := config.DetectType(resolved.Source.Cleaned, resolved.Config)
	if err != nil {
		return nil, err
	}
	def, ok := render.Lookup(diagramType)
	if !ok {
		return nil, fmt.Errorf("diagram type %q is not registered", diagramType)
	}
	d := def.New(ctx)
	def.Init(ctx, d)
	ctx.Config = ctx.ConfigState.GetConfig()
	if err := def.Parse(ctx, d, render.EncodeEntities(resolved.Source.Cleaned)+"\n"); err != nil {
		return nil, err
	}
	return d.(*db), nil
}

func obj(pairs ...any) *js.Obj[any] {
	o := js.NewObj[any]()
	for i := 0; i+1 < len(pairs); i += 2 {
		o.Set(pairs[i].(string), pairs[i+1])
	}
	return o
}

func strs(list []string) []any {
	out := make([]any, len(list))
	for i, s := range list {
		out[i] = s
	}
	return out
}

func entries(m *js.Map[string, int]) []any {
	out := []any{}
	m.Each(func(k string, v int) {
		out = append(out, []any{k, float64(v)})
	})
	return out
}

// dumpDB returns the content of the db in the form of the golden file (see
// tools/reference/sequence-db-page.js). A property that is undefined is left
// out, as JSON.stringify does.
func dumpDB(d *db) *js.Obj[any] {
	boxes := []any{}
	boxIndex := map[*box]int{}
	for i, b := range d.boxes {
		boxIndex[b] = i
		o := obj("wrap", b.wrap, "fill", b.fill, "actorKeys", strs(b.actorKeys))
		if !b.nameUndefined {
			o.Set("name", b.name)
		}
		boxes = append(boxes, o)
	}
	actors := []any{}
	d.actors.Each(func(key string, a *actor) {
		o := obj(
			"name", a.name,
			"wrap", a.wrap,
			"links", a.links,
			"properties", a.properties,
			"actorCnt", js.Null,
			"rectData", js.Null,
		)
		if a.descriptionRaw != nil {
			o.Set("description", a.descriptionRaw)
		} else {
			o.Set("description", a.description)
		}
		if a.typRaw != nil {
			o.Set("type", a.typRaw)
		} else {
			o.Set("type", a.typ)
		}
		if a.box != nil {
			o.Set("box", float64(boxIndex[a.box]))
		}
		if a.prevActor != "" {
			o.Set("prevActor", a.prevActor)
		}
		if a.nextActor != "" {
			o.Set("nextActor", a.nextActor)
		}
		actors = append(actors, []any{key, o})
	})
	messages := []any{}
	for _, m := range d.messages {
		o := obj("id", m.id, "wrap", m.wrap, "type", float64(m.typ))
		if m.hasFrom {
			o.Set("from", m.from)
		}
		if m.hasTo {
			o.Set("to", m.to)
		}
		switch m.kind {
		case kindSequenceIndex:
			auto := obj("visible", m.auto.visible)
			if m.auto.hasStart {
				auto.Set("start", jsonNumber(m.auto.start))
			}
			if m.auto.hasStep {
				auto.Set("step", jsonNumber(m.auto.step))
			}
			o.Set("message", auto)
		case kindNote:
			o.Set("message", m.message)
			o.Set("placement", float64(m.placement))
		default:
			o.Set("message", m.message)
			o.Set("activate", m.activate)
			o.Set("centralConnection", float64(m.centralConnection))
		}
		messages = append(messages, o)
	}
	return obj(
		"type", "sequence",
		"boxes", boxes,
		"actors", actors,
		"actorKeys", strs(d.getActorKeys()),
		"messages", messages,
		"createdActors", entries(d.createdActors),
		"destroyedActors", entries(d.destroyedActors),
		"title", d.GetDiagramTitle(),
		"accTitle", d.GetAccTitle(),
		"accDescription", d.GetAccDescription(),
		"hasAtLeastOneBox", d.hasAtLeastOneBox(),
		"hasAtLeastOneBoxWithTitle", d.hasAtLeastOneBoxWithTitle(),
		"autoWrap", d.autoWrap(),
	)
}

// jsonNumber is a number as JSON.stringify prints it: NaN and the
// infinities are null.
func jsonNumber(v float64) any {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return js.Null
	}
	return v
}

// diffValue compares two dynamic values. Objects are compared by key, and
// the order of the keys must be equal for the objects named in ordered. It
// returns the path of the first difference, or "".
func diffValue(path string, got, want any, ordered bool) string {
	switch w := want.(type) {
	case *js.Obj[any]:
		g, ok := got.(*js.Obj[any])
		if !ok {
			return fmt.Sprintf("%s: got %s, want %s", path, js.StringifyJSON(got), js.StringifyJSON(want))
		}
		if g.Len() != w.Len() || (ordered && strings.Join(g.Keys(), "\x00") != strings.Join(w.Keys(), "\x00")) {
			return fmt.Sprintf("%s: got keys %q, want %q", path, g.Keys(), w.Keys())
		}
		for _, k := range w.Keys() {
			if !g.Has(k) {
				return fmt.Sprintf("%s: got keys %q, want %q", path, g.Keys(), w.Keys())
			}
			if d := diffValue(path+"."+k, g.Get(k), w.Get(k), ordered || k == "links" || k == "properties"); d != "" {
				return d
			}
		}
		return ""
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			return fmt.Sprintf("%s: got %s, want %s", path, js.StringifyJSON(got), js.StringifyJSON(want))
		}
		for i := range w {
			if d := diffValue(fmt.Sprintf("%s[%d]", path, i), g[i], w[i], ordered); d != "" {
				return d
			}
		}
		return ""
	}
	if js.StringifyJSON(got) != js.StringifyJSON(want) {
		return fmt.Sprintf("%s: got %s, want %s", path, js.StringifyJSON(got), js.StringifyJSON(want))
	}
	return ""
}

// TestDBAgainstMermaid parses every case of testdata/db.json.gz and compares
// the db with the db of real mermaid. tools/reference/gen-sequence-db.mjs
// writes the file: the fixtures of the sequence family, mutations of them,
// and hand-written inputs.
func TestDBAgainstMermaid(t *testing.T) {
	golden, ok := readGzipJSON(t, "db.json.gz").([]any)
	if !ok {
		t.Fatal("db.json.gz is not an array")
	}
	if len(golden) < 800 {
		t.Fatalf("db.json.gz has %d cases", len(golden))
	}
	pass, errorCases := 0, 0
	for _, c := range golden {
		rec := c.(*js.Obj[any])
		name := js.ToString(rec.Get("name"))
		text, hasText := rec.Get("text").(string)
		if !hasText {
			// A fixture of the corpus: the name is its path.
			data, err := os.ReadFile(filepath.Join("..", "..", "..", filepath.FromSlash(name)))
			if err != nil {
				t.Fatal(err)
			}
			text = string(data)
		}
		wantErr, hasErr := rec.Get("error").(string)
		d, err := parseToDB(text)
		if hasErr {
			errorCases++
			if err == nil {
				t.Errorf("%s: no error, mermaid fails with %q\n%s", name, wantErr, text)
				continue
			}
			// The message of an exception of the db, of the parser and of
			// the lexer is the same text. The text of a YAML error is not.
			got := err.Error()
			var cfgErr *config.Error
			if errors.As(err, &cfgErr) {
				got = cfgErr.Message
			}
			if isYAMLError(wantErr) {
				pass++
				continue
			}
			if got != wantErr {
				t.Errorf("%s: error\n got  %q\n want %q\n%s", name, got, wantErr, text)
				continue
			}
			pass++
			continue
		}
		if err != nil {
			t.Errorf("%s: error %q, mermaid has none\n%s", name, err, text)
			continue
		}
		want := js.NewObj[any]()
		rec.Each(func(k string, v any) {
			if k != "name" && k != "text" {
				want.Set(k, v)
			}
		})
		if diff := diffValue("db", dumpDB(d), want, false); diff != "" {
			t.Errorf("%s: %s\n%s", name, diff, text)
			continue
		}
		pass++
	}
	t.Logf("%d cases, %d pass, %d of them with an error in mermaid", len(golden), pass, errorCases)
}

// isYAMLError reports whether a message of mermaid is a YAMLException of
// js-yaml: the reason, the position in parentheses and a snippet.
func isYAMLError(message string) bool {
	first, _, _ := strings.Cut(message, "\n")
	open := strings.LastIndex(first, " (")
	return open > 0 && strings.HasSuffix(first, ")") && strings.Contains(first[open:], ":")
}

// TestCSSSupportsColor compares cssSupportsColor with
// window.CSS.supports('color', value) of Chromium, for the values of
// testdata/css_supports_color.json. tools/reference/gen-sequence-csscolor.mjs
// writes the file.
func TestCSSSupportsColor(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "css_supports_color.json"))
	if err != nil {
		t.Fatal(err)
	}
	v, err := js.ParseJSON(bytes.TrimSpace(data))
	if err != nil {
		t.Fatal(err)
	}
	cases := v.([]any)
	if len(cases) < 600 {
		t.Fatalf("%d cases", len(cases))
	}
	known := 0
	for _, c := range cases {
		rec := c.(*js.Obj[any])
		value := rec.Get("value").(string)
		want := rec.Get("supports").(bool)
		if cssColorOutOfScope(value) {
			continue
		}
		known++
		if got := cssSupportsColor(value); got != want {
			t.Errorf("cssSupportsColor(%q) = %v, Chromium says %v", value, got, want)
		}
	}
	t.Logf("%d values, %d in the scope of cssSupportsColor", len(cases), known)
}

// cssColorOutOfScope reports the values with syntax that cssSupportsColor
// documents as not supported: math functions, the relative color syntax,
// substitution functions other than var(), and a function without the
// closing parenthesis, which parseBoxData never tests.
func cssColorOutOfScope(value string) bool {
	for _, s := range []string{"calc(", "min(", "env(", "attr(", "from "} {
		if strings.Contains(value, s) {
			return true
		}
	}
	return strings.Count(value, "(") > strings.Count(value, ")")
}
