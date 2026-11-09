package cynefin

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// astGolden is testdata/ast.json.gz, which
// tools/reference/gen-treeview-cynefin-wardley-ast.mjs wrote with the real
// parser: texts with the AST of parse('cynefin', text), or the error.
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
func (ast *Cynefin) plain() any {
	out := map[string]any{"$type": "Cynefin"}
	domains := []any{}
	for _, block := range ast.Domains {
		items := []any{}
		for _, item := range block.Items {
			items = append(items, map[string]any{"$type": "DomainItem", "label": item.Label})
		}
		domains = append(domains, map[string]any{"$type": "DomainBlock", "domain": block.Domain, "items": items})
	}
	out["domains"] = domains
	transitions := []any{}
	for _, t := range ast.Transitions {
		transition := map[string]any{"$type": "Transition", "from": t.From, "to": t.To}
		if t.Label != nil {
			transition["label"] = *t.Label
		}
		transitions = append(transitions, transition)
	}
	out["transitions"] = transitions
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

// TestParse compares the parser with parse('cynefin', text) of the parser
// package of mermaid, for the corpus fixtures, hand-written inputs and
// mutated inputs. A text that the real parser rejects must give an error.
func TestParse(t *testing.T) {
	var golden astGolden
	readGolden(t, "ast.json.gz", &golden)
	if len(golden.Cases) < 2600 || golden.Fixtures < 24 {
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
