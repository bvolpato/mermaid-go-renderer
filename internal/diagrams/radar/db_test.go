package radar

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"math"
	"os"
	"reflect"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
)

type dbCase struct {
	Text  string  `json:"text"`
	DB    any     `json:"db"`
	Error *string `json:"error"`
}

func readDBGolden(t testing.TB) []dbCase {
	t.Helper()
	f, err := os.Open("testdata/db.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var cases []dbCase
	if err := json.NewDecoder(zr).Decode(&cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

// encode is the encode function of tools/reference/smallcharts-db-page.js:
// undefined is {"$u":1}, and NaN, the infinities and -0 are {"$n":"<text>"}.
func encode(v any) any {
	switch x := v.(type) {
	case nil:
		return map[string]any{"$u": 1.0}
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return map[string]any{"$n": js.Str(x)}
		}
		if x == 0 && math.Signbit(x) {
			return map[string]any{"$n": "-0"}
		}
		return x
	case *js.Obj[any]:
		m := map[string]any{}
		x.Each(func(key string, value any) { m[key] = encode(value) })
		return m
	case []any:
		list := make([]any, len(x))
		for i, e := range x {
			list[i] = encode(e)
		}
		return list
	}
	if v == js.Null {
		return nil
	}
	return v
}

// parseLikeMermaid runs the steps of mermaid.parse(text) and of
// getDiagramFromText(text): the config of the front matter and of the
// directives, the type detection, and the parser on the cleaned text.
func parseLikeMermaid(text string) (*render.Context, *DB, error) {
	site := js.NewObj[any]()
	site.Set("startOnLoad", false)
	resolved, err := config.Resolve(text, site)
	if err != nil {
		return nil, nil, err
	}
	if resolved.DiagramType != "radar" {
		return nil, nil, errors.New("detected type " + resolved.DiagramType)
	}
	ctx := &render.Context{Config: resolved.Config, ConfigState: resolved.State, Sanitizer: sanitize.New()}
	db := newDB(ctx)
	if err := parseDiagram(ctx, db, render.EncodeEntities(resolved.Code)+"\n"); err != nil {
		return nil, nil, err
	}
	return ctx, db, nil
}

// plain is the db in the form of the golden file.
func (db *DB) plain() any {
	axes := []any{}
	for _, a := range db.GetAxes() {
		axes = append(axes, map[string]any{"name": a.Name, "label": a.Label})
	}
	curves := []any{}
	for _, c := range db.GetCurves() {
		entries := []any{}
		for _, e := range c.Entries {
			entries = append(entries, encode(e))
		}
		curves = append(curves, map[string]any{"name": c.Name, "label": c.Label, "entries": entries})
	}
	o := db.GetOptions()
	options := map[string]any{
		"showLegend": o.ShowLegend,
		"ticks":      encode(o.Ticks),
		"max":        nil,
		"min":        encode(o.Min),
		"graticule":  o.Graticule,
	}
	if o.Max != nil {
		options["max"] = encode(*o.Max)
	}
	return map[string]any{
		"title":          db.GetDiagramTitle(),
		"accTitle":       db.GetAccTitle(),
		"accDescription": db.GetAccDescription(),
		"axes":           axes,
		"curves":         curves,
		"options":        options,
		"config":         encode(db.GetConfig()),
	}
}

// TestDB compares the db with the db of real mermaid in Chromium
// (tools/reference/gen-smallcharts-db.mjs), for every text that the parser
// accepts in the parser golden and for texts with a config. A text that
// mermaid rejects must give an error.
func TestDB(t *testing.T) {
	cases := readDBGolden(t)
	valid := 0
	for _, c := range cases {
		_, db, err := parseLikeMermaid(c.Text)
		if c.Error != nil {
			if err == nil {
				t.Errorf("text %q: no error, mermaid fails with %q", c.Text, *c.Error)
			}
			continue
		}
		valid++
		if err != nil {
			t.Errorf("text %q: error %v, mermaid accepts the text", c.Text, err)
			continue
		}
		if got := db.plain(); !reflect.DeepEqual(got, c.DB) {
			t.Errorf("text %q:\n got %v\nwant %v", c.Text, got, c.DB)
		}
	}
	t.Logf("%d cases, %d valid", len(cases), valid)
}
