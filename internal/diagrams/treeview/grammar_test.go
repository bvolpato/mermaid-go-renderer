package treeview

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// astGolden is testdata/ast.json.gz, which
// tools/reference/gen-treeview-cynefin-wardley-ast.mjs wrote with the real
// parser: texts with the AST of parse('treeView', text), or the error.
type astGolden struct {
	Fixtures int `json:"fixtures"`
	Cases    []struct {
		Text  string  `json:"text"`
		AST   any     `json:"ast"`
		Error *string `json:"error"`
	} `json:"cases"`
}

func readGolden(t testing.TB, name string, v any) {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewDecoder(zr).Decode(v); err != nil {
		t.Fatal(err)
	}
}

// plain is the AST in the form of the golden file.
func (ast *TreeView) plain() any {
	out := map[string]any{"$type": "TreeView"}
	nodes := []any{}
	for _, n := range ast.Nodes {
		node := map[string]any{"$type": "TreeNode", "name": n.Name}
		if n.Indent != nil {
			node["indent"] = float64(*n.Indent)
		}
		if n.ClassAnnotation != nil {
			node["classAnnotation"] = *n.ClassAnnotation
		}
		if n.IconAnnotation != nil {
			node["iconAnnotation"] = *n.IconAnnotation
		}
		if n.DescAnnotation != nil {
			node["descAnnotation"] = *n.DescAnnotation
		}
		nodes = append(nodes, node)
	}
	out["nodes"] = nodes
	if ast.AccDescr != nil {
		out["accDescr"] = *ast.AccDescr
	}
	if ast.AccTitle != nil {
		out["accTitle"] = *ast.AccTitle
	}
	if ast.Title != nil {
		out["title"] = *ast.Title
	}
	return out
}

// TestParse compares the parser with parse('treeView', text) of the parser
// package of mermaid, for the corpus fixtures, hand-written inputs and
// mutated inputs. A text that the real parser rejects must give an error.
func TestParse(t *testing.T) {
	var golden astGolden
	readGolden(t, "ast.json.gz", &golden)
	if len(golden.Cases) < 3000 || golden.Fixtures < 30 {
		t.Fatalf("the golden file has %d cases, %d from fixtures", len(golden.Cases), golden.Fixtures)
	}
	valid := 0
	for _, c := range golden.Cases {
		ast, err := parse(c.Text)
		if c.Error != nil {
			if err == nil {
				t.Errorf("text %q: no error, the real parser fails with %q", c.Text, *c.Error)
			}
			continue
		}
		valid++
		if err != nil {
			t.Errorf("text %q: error %v, the real parser accepts the text", c.Text, err)
			continue
		}
		if got := ast.plain(); !reflect.DeepEqual(got, c.AST) {
			t.Errorf("text %q:\n got %v\nwant %v", c.Text, got, c.AST)
		}
	}
	t.Logf("%d cases, %d that the real parser accepts", len(golden.Cases), valid)
}
