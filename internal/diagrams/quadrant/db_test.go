package quadrant

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
	if resolved.DiagramType != "quadrantChart" {
		return nil, nil, errors.New("detected type " + resolved.DiagramType)
	}
	ctx := &render.Context{Config: resolved.Config, ConfigState: resolved.State, Sanitizer: sanitize.New()}
	db := newDB(ctx)
	if err := parseDiagram(ctx, db, render.EncodeEntities(resolved.Code)+"\n"); err != nil {
		return nil, nil, err
	}
	return ctx, db, nil
}

// plainText is a QuadrantTextType in the form of the golden file.
func plainText(t QuadrantTextType) any {
	return map[string]any{
		"text":          t.Text,
		"fill":          encode(t.Fill),
		"x":             encode(t.X),
		"y":             encode(t.Y),
		"verticalPos":   t.VerticalPos,
		"horizontalPos": t.HorizontalPos,
		"fontSize":      encode(t.FontSize),
		"rotation":      encode(t.Rotation),
	}
}

// plain is the db in the form of the golden file, after the first
// statements of draw in quadrantRenderer.ts.
func plain(ctx *render.Context, db *DB) any {
	orDefault := func(value any) any {
		if js.IsNullish(value) {
			return 500.0
		}
		return value
	}
	db.SetHeight(orDefault(js.Path(ctx.Config, "quadrantChart", "chartHeight")))
	db.SetWidth(orDefault(js.Path(ctx.Config, "quadrantChart", "chartWidth")))
	data := db.GetQuadrantData()

	points := []any{}
	for _, p := range data.Points {
		points = append(points, map[string]any{
			"x":           encode(p.X),
			"y":           encode(p.Y),
			"fill":        encode(p.Fill),
			"radius":      encode(p.Radius),
			"text":        plainText(p.Text),
			"strokeColor": encode(p.StrokeColor),
			"strokeWidth": encode(p.StrokeWidth),
		})
	}
	quadrants := []any{}
	for _, q := range data.Quadrants {
		quadrants = append(quadrants, map[string]any{
			"text":   plainText(q.Text),
			"x":      encode(q.X),
			"y":      encode(q.Y),
			"width":  encode(q.Width),
			"height": encode(q.Height),
			"fill":   encode(q.Fill),
		})
	}
	axisLabels := []any{}
	for _, l := range data.AxisLabels {
		axisLabels = append(axisLabels, plainText(l))
	}
	borderLines := []any{}
	for _, l := range data.BorderLines {
		borderLines = append(borderLines, map[string]any{
			"strokeFill":  encode(l.StrokeFill),
			"strokeWidth": encode(l.StrokeWidth),
			"x1":          encode(l.X1),
			"y1":          encode(l.Y1),
			"x2":          encode(l.X2),
			"y2":          encode(l.Y2),
		})
	}
	var title any = map[string]any{"$u": 1.0}
	if data.Title != nil {
		title = plainText(*data.Title)
	}
	return map[string]any{
		"title":          db.GetDiagramTitle(),
		"accTitle":       db.GetAccTitle(),
		"accDescription": db.GetAccDescription(),
		"data": map[string]any{
			"points":      points,
			"quadrants":   quadrants,
			"axisLabels":  axisLabels,
			"borderLines": borderLines,
			"title":       title,
		},
	}
}

// TestDB compares the db with the db of real mermaid in Chromium
// (tools/reference/gen-smallcharts-db.mjs), for corpus fixtures, hand-written texts
// and mutated texts. A text that
// mermaid rejects must give an error.
func TestDB(t *testing.T) {
	cases := readDBGolden(t)
	valid := 0
	for _, c := range cases {
		ctx, db, err := parseLikeMermaid(c.Text)
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
		if got := plain(ctx, db); !reflect.DeepEqual(got, c.DB) {
			t.Errorf("text %q:\n got %v\nwant %v", c.Text, got, c.DB)
		}
	}
	t.Logf("%d cases, %d valid", len(cases), valid)
}
