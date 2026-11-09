package c4

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
)

// dbCase is one case of testdata/db.json.gz, which
// tools/reference/gen-c4-db.mjs writes from real mermaid in the browser.
type dbCase struct {
	Name  string  `json:"name"`
	Text  string  `json:"text"`
	Error *string `json:"error"`
	Type  string  `json:"type"`
	// DB is JSON.stringify of the getters of the db.
	DB string `json:"db"`
}

func loadDBCases(t testing.TB) []dbCase {
	t.Helper()
	data, err := os.ReadFile("testdata/db.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	var cases []dbCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

// parseForTest runs the steps of mermaid.parse(text): the config of the
// text, the detection of the diagram type, db.clear(), init and the parser.
func parseForTest(text string) (*db, error) {
	resolved, err := config.Resolve(text, nil)
	if err != nil {
		return nil, err
	}
	diagramType, err := config.DetectType(resolved.Source.Cleaned, resolved.Config)
	if err != nil {
		return nil, err
	}
	if diagramType != "c4" {
		return nil, &render.UnsupportedError{Type: diagramType}
	}
	ctx := &render.Context{
		ID:          render.DefaultID,
		Type:        diagramType,
		Config:      resolved.Config,
		ConfigState: resolved.State,
		Sanitizer:   sanitize.New(),
	}
	d := newDB(ctx)
	initDiagram(ctx, d)
	if err := parse(ctx, d, render.EncodeEntities(resolved.Source.Cleaned)+"\n"); err != nil {
		return nil, err
	}
	return d, nil
}

// dumpDB is the object that the page script of the generator stringifies.
func dumpDB(d *db) string {
	list := func(bags []*js.Obj[any]) []any {
		out := make([]any, len(bags))
		for i, bag := range bags {
			out[i] = bag
		}
		return out
	}
	o := js.NewObj[any]()
	o.Set("c4Type", d.getC4Type())
	o.Set("title", d.getTitle())
	o.Set("accTitle", d.GetAccTitle())
	o.Set("accDescription", d.GetAccDescription())
	o.Set("c4ShapeInRow", d.getC4ShapeInRow())
	o.Set("c4BoundaryInRow", d.getC4BoundaryInRow())
	o.Set("wrap", d.autoWrap())
	o.Set("currentBoundaryParse", d.getCurrentBoundaryParse())
	o.Set("parentBoundaryParse", d.getParentBoundaryParse())
	o.Set("shapes", list(d.getC4ShapeArray()))
	o.Set("boundaries", list(d.getBoundaries()))
	o.Set("rels", list(d.getRels()))
	return js.StringifyJSON(o)
}

// TestDBGolden compares the db after the parse with the db of real mermaid
// for every case. For a text that mermaid rejects, the port must return an
// error too. The error texts are not compared.
func TestDBGolden(t *testing.T) {
	cases := loadDBCases(t)
	if len(cases) < 500 {
		t.Fatalf("only %d cases", len(cases))
	}
	failed := 0
	for _, c := range cases {
		d, err := parseForTest(c.Text)
		switch {
		case c.Error != nil && err == nil:
			failed++
			t.Errorf("%s: mermaid fails with %q, the port parses the text", c.Name, *c.Error)
		case c.Error == nil && err != nil:
			failed++
			t.Errorf("%s: the port fails with %v, mermaid parses the text", c.Name, err)
		case c.Error == nil:
			if got := dumpDB(d); got != c.DB {
				failed++
				t.Errorf("%s:\n got  %s\n want %s", c.Name, got, c.DB)
			}
		}
		if failed > 10 {
			t.Fatal("too many failures")
		}
	}
	t.Logf("%d cases", len(cases))
}
