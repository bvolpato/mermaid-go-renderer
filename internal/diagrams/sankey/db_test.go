package sankey

import (
	"encoding/json"
	"math"
	"os"
	"reflect"
	"strconv"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// dbCase is one case of testdata/db.json, which
// tools/reference/gen-sankey-treemap-db.mjs wrote with mermaid 12.2.0 in
// Chromium: a diagram text and the db that
// mermaid.mermaidAPI.getDiagramFromText returns for it, or the error. A
// number is the hex text of its bits.
type dbCase struct {
	Name           string `json:"name"`
	Text           string `json:"text"`
	Error          string `json:"error"`
	RenderError    string `json:"renderError"`
	Type           string `json:"type"`
	Title          string `json:"title"`
	AccTitle       string `json:"accTitle"`
	AccDescription string `json:"accDescription"`
	Graph          *struct {
		Nodes []struct {
			ID string `json:"id"`
		} `json:"nodes"`
		Links []struct {
			Source string `json:"source"`
			Target string `json:"target"`
			Value  string `json:"value"`
		} `json:"links"`
	} `json:"graph"`
	Nodes []string    `json:"nodes"`
	Links [][3]string `json:"links"`
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

// TestDBAgainstMermaid compares the db with the db of mermaid in Chromium
// for the corpus fixtures and for hand-written texts, and requires an error
// where mermaid fails.
func TestDBAgainstMermaid(t *testing.T) {
	cases := loadDBCases(t)
	if len(cases) < 50 {
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

		var nodes []string
		for _, n := range db.GetNodes() {
			nodes = append(nodes, n.ID)
		}
		if !reflect.DeepEqual(nodes, c.Nodes) {
			t.Errorf("%s: nodes = %q, want %q", c.Name, nodes, c.Nodes)
		}
		var links [][3]string
		for _, l := range db.GetLinks() {
			links = append(links, [3]string{l.Source.ID, l.Target.ID, bitsText(l.Value)})
		}
		if !reflect.DeepEqual(links, c.Links) {
			t.Errorf("%s: links = %q, want %q", c.Name, links, c.Links)
		}

		graph := db.GetGraph()
		if len(graph.Nodes) != len(c.Graph.Nodes) || len(graph.Links) != len(c.Graph.Links) {
			t.Errorf("%s: graph with %d nodes and %d links, want %d and %d", c.Name, len(graph.Nodes), len(graph.Links), len(c.Graph.Nodes), len(c.Graph.Links))
			continue
		}
		for i, n := range graph.Nodes {
			if n.ID != c.Graph.Nodes[i].ID {
				t.Errorf("%s: graph node %d = %q, want %q", c.Name, i, n.ID, c.Graph.Nodes[i].ID)
			}
		}
		for i, l := range graph.Links {
			w := c.Graph.Links[i]
			if l.Source != w.Source || l.Target != w.Target || bitsText(l.Value) != w.Value {
				t.Errorf("%s: graph link %d = %q, %q, %s, want %q, %q, %s", c.Name, i, l.Source, l.Target, bitsText(l.Value), w.Source, w.Target, w.Value)
			}
		}

		// mermaid.render fails for a text that parses when the layout
		// throws, for example for a circular link.
		_, renderErr := render.Render(c.Text, render.Options{})
		if (renderErr != nil) != (c.RenderError != "") {
			t.Errorf("%s: render error %v, mermaid has %q", c.Name, renderErr, c.RenderError)
		}
	}
	t.Logf("%d cases: %d with the same db, %d that mermaid and the port reject", len(cases), parsed, rejected)
}
