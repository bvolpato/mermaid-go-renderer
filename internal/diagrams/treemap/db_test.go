package treemap

import (
	"encoding/json"
	"math"
	"os"
	"reflect"
	"strconv"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// dbNode is a TreemapNode in testdata/db.json. A number is the hex text of
// its bits, and a property that is undefined is absent.
type dbNode struct {
	Name              string    `json:"name"`
	Children          *[]dbNode `json:"children,omitempty"`
	Value             *string   `json:"value,omitempty"`
	ClassSelector     *string   `json:"classSelector,omitempty"`
	CSSCompiledStyles *[]string `json:"cssCompiledStyles,omitempty"`
}

type dbClass struct {
	ID         string   `json:"id"`
	Styles     []string `json:"styles"`
	TextStyles []string `json:"textStyles"`
}

// dbCase is one case of testdata/db.json, which
// tools/reference/gen-sankey-treemap-db.mjs wrote with mermaid 12.2.0 in
// Chromium: a diagram text and the db that
// mermaid.mermaidAPI.getDiagramFromText returns for it, or the error.
type dbCase struct {
	Name           string               `json:"name"`
	Text           string               `json:"text"`
	Error          string               `json:"error"`
	RenderError    string               `json:"renderError"`
	Type           string               `json:"type"`
	Title          string               `json:"title"`
	AccTitle       string               `json:"accTitle"`
	AccDescription string               `json:"accDescription"`
	Root           *dbNode              `json:"root"`
	Nodes          [][2]any             `json:"nodes"`
	OuterNodes     []string             `json:"outerNodes"`
	FirstRoot      *string              `json:"firstRoot"`
	Classes        [][2]json.RawMessage `json:"classes"`
	Config         json.RawMessage      `json:"config"`
}

func loadDBCases(t testing.TB) []dbCase {
	t.Helper()
	data, err := os.ReadFile("testdata/db.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []dbCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

// parseDiagram does what mermaid.mermaidAPI.getDiagramFromText does after
// mermaid.render set the config: it finds the diagram type of the text and
// parses the text into a new db. It does not give the title of the front
// matter to the db.
func parseDiagram(text string) (*render.Context, *DB, error) {
	resolved, err := config.Resolve(text, nil)
	if err != nil {
		return nil, nil, err
	}
	diagramType, err := config.DetectType(resolved.Source.Cleaned, resolved.Config)
	if err != nil {
		return nil, nil, err
	}
	ctx := &render.Context{ID: render.DefaultID, Type: diagramType, Config: resolved.Config, ConfigState: resolved.State, Title: resolved.Title}
	db := NewDB(ctx)
	if err := parse(ctx, db, render.EncodeEntities(resolved.Source.Cleaned)+"\n"); err != nil {
		return ctx, nil, err
	}
	return ctx, db, nil
}

func bitsText(v float64) string {
	if math.IsNaN(v) {
		return "7ff8000000000000"
	}
	s := strconv.FormatUint(math.Float64bits(v), 16)
	for len(s) < 16 {
		s = "0" + s
	}
	return s
}

// dump converts a node to the form of the golden file.
func dump(n *Node) dbNode {
	out := dbNode{Name: n.Name}
	if n.Children != nil {
		children := []dbNode{}
		for _, child := range n.Children {
			children = append(children, dump(child))
		}
		out.Children = &children
	}
	if n.HasValue {
		v := bitsText(n.Value)
		out.Value = &v
	}
	if n.ClassSelector != "" {
		out.ClassSelector = &n.ClassSelector
	}
	if n.CSSCompiledStyles != nil {
		out.CSSCompiledStyles = &n.CSSCompiledStyles
	}
	return out
}

func toJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return err.Error()
	}
	return string(data)
}

// TestDBAgainstMermaid compares the db with the db of mermaid in Chromium
// for the corpus fixtures and for hand-written texts, and requires an error
// where mermaid fails.
func TestDBAgainstMermaid(t *testing.T) {
	cases := loadDBCases(t)
	if len(cases) < 60 {
		t.Fatalf("only %d cases", len(cases))
	}
	parsed, rejected := 0, 0
	for _, c := range cases {
		ctx, db, err := parseDiagram(c.Text)
		if c.Error != "" {
			rejected++
			if err == nil {
				t.Errorf("%s: no error, mermaid fails with %q", c.Name, c.Error)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: error %v, mermaid parses the text", c.Name, err)
			continue
		}
		parsed++
		if ctx.Type != c.Type {
			t.Errorf("%s: diagram type %q, want %q", c.Name, ctx.Type, c.Type)
		}
		if db.GetDiagramTitle() != c.Title || db.GetAccTitle() != c.AccTitle || db.GetAccDescription() != c.AccDescription {
			t.Errorf("%s: title, accTitle, accDescription = %q, %q, %q, want %q, %q, %q", c.Name,
				db.GetDiagramTitle(), db.GetAccTitle(), db.GetAccDescription(), c.Title, c.AccTitle, c.AccDescription)
		}

		if got, want := toJSON(dump(db.GetRoot())), toJSON(c.Root); got != want {
			t.Errorf("%s: root\n got  %s\n want %s", c.Name, got, want)
		}
		nodes := [][2]any{}
		for _, n := range db.GetNodes() {
			nodes = append(nodes, [2]any{n.Name, float64(db.levels[n])})
		}
		if !reflect.DeepEqual(nodes, c.Nodes) {
			t.Errorf("%s: nodes and levels = %v, want %v", c.Name, nodes, c.Nodes)
		}
		outer := []string{}
		for _, n := range db.outerNodes {
			outer = append(outer, n.Name)
		}
		if !reflect.DeepEqual(outer, c.OuterNodes) {
			t.Errorf("%s: outer nodes = %q, want %q", c.Name, outer, c.OuterNodes)
		}
		if (db.root == nil) != (c.FirstRoot == nil) || (db.root != nil && db.root.Name != *c.FirstRoot) {
			t.Errorf("%s: the first root differs", c.Name)
		}

		classes := db.GetClasses()
		if classes.Len() != len(c.Classes) {
			t.Errorf("%s: %d classes, want %d", c.Name, classes.Len(), len(c.Classes))
		} else {
			for i, key := range classes.Keys() {
				var wantKey string
				var want dbClass
				if err := json.Unmarshal(c.Classes[i][0], &wantKey); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(c.Classes[i][1], &want); err != nil {
					t.Fatal(err)
				}
				def := classes.Get(key)
				got := dbClass{ID: def.ID, Styles: def.Styles, TextStyles: def.TextStyles}
				if key != wantKey || !reflect.DeepEqual(got, want) {
					t.Errorf("%s: class %d = %q %+v, want %q %+v", c.Name, i, key, got, wantKey, want)
				}
			}
		}

		// getConfig(): the same properties in the same order.
		wantConfig, err := js.ParseJSON(c.Config)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := js.StringifyJSON(db.GetConfig()), js.StringifyJSON(wantConfig); got != want {
			t.Errorf("%s: config\n got  %s\n want %s", c.Name, got, want)
		}

		_, renderErr := render.Render(c.Text, render.Options{})
		if (renderErr != nil) != (c.RenderError != "") {
			t.Errorf("%s: render error %v, mermaid has %q", c.Name, renderErr, c.RenderError)
		}
	}
	t.Logf("%d cases: %d with the same db, %d that mermaid and the port reject", len(cases), parsed, rejected)
}
