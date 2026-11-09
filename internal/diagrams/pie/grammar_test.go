package pie

import (
	"compress/gzip"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"
)

type astCase struct {
	Text  string `json:"text"`
	AST   any    `json:"ast"`
	Error string `json:"error"`
}

func readASTGolden(t testing.TB) []astCase {
	t.Helper()
	f, err := os.Open("testdata/ast.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var cases []astCase
	if err := json.NewDecoder(zr).Decode(&cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

// number is a JavaScript number in the form of the golden: -0 is
// {"$n": "-0"}, because JSON has no -0.
func number(v float64) any {
	if v == 0 && math.Signbit(v) {
		return map[string]any{"$n": "-0"}
	}
	return v
}

// plain is the AST in the form of the golden file.
func (ast *Pie) plain() any {
	out := map[string]any{"$type": "Pie", "showData": ast.ShowData}
	sections := []any{}
	for _, s := range ast.Sections {
		sections = append(sections, map[string]any{"$type": "PieSection", "label": s.Label, "value": number(s.Value)})
	}
	out["sections"] = sections
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

// TestParse compares the parser with parse('pie', text) of the parser
// package of mermaid, for corpus fixtures, hand-written inputs and mutated
// inputs. A text that the real parser rejects must give an error.
func TestParse(t *testing.T) {
	cases := readASTGolden(t)
	valid := 0
	for _, c := range cases {
		ast, err := parse(c.Text)
		if c.Error != "" {
			if err == nil {
				t.Errorf("text %q: no error, the real parser fails with %q", c.Text, c.Error)
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
	t.Logf("%d cases, %d valid", len(cases), valid)
}
