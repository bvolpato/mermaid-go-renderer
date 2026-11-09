package treeview

import (
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// dbCase is one case of testdata/db.json.gz, which
// tools/reference/gen-treeview-cynefin-wardley-db.mjs wrote with mermaid
// 12.2.0 in Chromium: a text and what the db holds after mermaid.render
// parsed it, or the error of the render.
type dbCase struct {
	Name  string  `json:"name"`
	Text  string  `json:"text"`
	DB    any     `json:"db"`
	Error *string `json:"error"`
}

// encode is the encode function of the generator: undefined is {"$u":1},
// and NaN, the infinities and -0 are {"$n":"<text>"}.
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

// parseLikeRender runs the steps of render.Render up to the parser: the
// config of the front matter and of the directives, the type detection, the
// title of the front matter, and the parser on the cleaned text.
func parseLikeRender(text string) (*DB, error) {
	resolved, err := config.Resolve(text, nil)
	if err != nil {
		return nil, err
	}
	code := resolved.Source.Cleaned
	diagramType, err := config.DetectType(code, resolved.Config)
	if err != nil {
		return nil, err
	}
	if diagramType != "treeView" {
		return nil, fmt.Errorf("detected type %s", diagramType)
	}
	ctx := render.NewContext(render.ContextOptions{Config: resolved.Config, ConfigState: resolved.State})
	db := newDB(ctx)
	if resolved.Title != "" {
		db.SetDiagramTitle(resolved.Title)
	}
	if err := parseDiagram(ctx, db, render.EncodeEntities(code)+"\n"); err != nil {
		return nil, err
	}
	return db, nil
}

// plainNode is a node in the form of the golden file. A property that is
// undefined is {"$u":1}.
func plainNode(n *Node) any {
	optional := func(text string, has bool) any {
		if !has {
			return encode(nil)
		}
		return text
	}
	children := []any{}
	for _, child := range n.Children {
		children = append(children, plainNode(child))
	}
	return map[string]any{
		"id":          float64(n.ID),
		"level":       float64(n.Level),
		"name":        n.Name,
		"nodeType":    string(n.NodeType),
		"icon":        optional(n.Icon, n.Icon != ""),
		"cssClass":    optional(n.CSSClass, n.CSSClass != ""),
		"description": optional(n.Description, n.HasDescription),
		"children":    children,
	}
}

// plain is the db in the form of the golden file.
func (db *DB) plain() any {
	return map[string]any{
		"title":          db.GetDiagramTitle(),
		"accTitle":       db.GetAccTitle(),
		"accDescription": db.GetAccDescription(),
		"root":           plainNode(db.GetRoot()),
		"count":          float64(db.GetCount()),
		"config":         encode(db.GetConfig()),
	}
}

// TestDB compares the db with the db of real mermaid in Chromium after a
// render, for the corpus fixtures and for extra inputs, the order of the
// nodes included. A text that mermaid rejects must give an error.
func TestDB(t *testing.T) {
	var cases []dbCase
	readGolden(t, "db.json.gz", &cases)
	if len(cases) < 120 {
		t.Fatalf("the golden file has %d cases", len(cases))
	}
	valid := 0
	for _, c := range cases {
		if c.Error != nil {
			if _, err := render.Render(c.Text, render.Options{}); err == nil {
				t.Errorf("%s: no error, mermaid fails with %q\n%s", c.Name, *c.Error, c.Text)
			}
			continue
		}
		valid++
		db, err := parseLikeRender(c.Text)
		if err != nil {
			t.Errorf("%s: error %v, mermaid accepts the text\n%s", c.Name, err, c.Text)
			continue
		}
		if got := db.plain(); !reflect.DeepEqual(got, c.DB) {
			t.Errorf("%s:\n got %v\nwant %v\n%s", c.Name, got, c.DB, c.Text)
		}
	}
	t.Logf("%d cases, %d that mermaid accepts", len(cases), valid)
}
