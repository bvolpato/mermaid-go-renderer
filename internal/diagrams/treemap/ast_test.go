package treemap

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"testing"
)

// The golden file comes from the real parser of the treemap grammar in
// Node.js (tools/reference/gen-treemap-ast.mjs): the corpus fixtures,
// hand-written inputs and mutated inputs, each with the AST or the fact
// that the parser fails.

type goldenItem struct {
	Leaf          bool    `json:"leaf"`
	Name          string  `json:"name"`
	Value         *string `json:"value"` // the bits of the number, in hex
	ClassSelector *string `json:"classSelector"`
}

type goldenClassDef struct {
	ClassName string  `json:"className"`
	StyleText *string `json:"styleText"`
}

type goldenRow struct {
	Indent   *int            `json:"indent"`
	Item     *goldenItem     `json:"item"`
	ClassDef *goldenClassDef `json:"classDef"`
}

type goldenAST struct {
	Title    *string     `json:"title"`
	AccTitle *string     `json:"accTitle"`
	AccDescr *string     `json:"accDescr"`
	Rows     []goldenRow `json:"rows"`
}

type goldenASTCase struct {
	Text  string     `json:"text"`
	Error bool       `json:"error"`
	AST   *goldenAST `json:"ast"`
}

func loadGzipJSON(t testing.TB, name string, into any) {
	t.Helper()
	f, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewDecoder(zr).Decode(into); err != nil {
		t.Fatal(err)
	}
}

func optional(s *string) string {
	if s == nil {
		return "undefined"
	}
	return strconv.Quote(*s)
}

// astDiff returns the first difference between the AST of the port and the
// AST of the real parser, or "".
func astDiff(got *treemapAST, want *goldenAST) string {
	for _, p := range []struct {
		name      string
		got, want *string
	}{
		{"title", got.Title, want.Title},
		{"accTitle", got.AccTitle, want.AccTitle},
		{"accDescr", got.AccDescr, want.AccDescr},
	} {
		if optional(p.got) != optional(p.want) {
			return fmt.Sprintf("%s = %s, want %s", p.name, optional(p.got), optional(p.want))
		}
	}
	if len(got.rows) != len(want.Rows) {
		return fmt.Sprintf("%d rows, want %d", len(got.rows), len(want.Rows))
	}
	for i, row := range got.rows {
		w := want.Rows[i]
		switch {
		case (row.indent == nil) != (w.Indent == nil):
			return fmt.Sprintf("row %d: indent defined = %v, want %v", i, row.indent != nil, w.Indent != nil)
		case row.indent != nil && *row.indent != *w.Indent:
			return fmt.Sprintf("row %d: indent = %d, want %d", i, *row.indent, *w.Indent)
		case (row.classDef == nil) != (w.ClassDef == nil):
			return fmt.Sprintf("row %d: is a ClassDefStatement = %v, want %v", i, row.classDef != nil, w.ClassDef != nil)
		case (row.item == nil) != (w.Item == nil):
			return fmt.Sprintf("row %d: has an item = %v, want %v", i, row.item != nil, w.Item != nil)
		}
		if c := row.classDef; c != nil {
			// An empty style text is undefined in the AST.
			styleText := "undefined"
			if c.styleText != "" {
				styleText = strconv.Quote(c.styleText)
			}
			if c.className != w.ClassDef.ClassName || styleText != optional(w.ClassDef.StyleText) {
				return fmt.Sprintf("row %d: classDef %q %s, want %q %s", i, c.className, styleText, w.ClassDef.ClassName, optional(w.ClassDef.StyleText))
			}
		}
		if item := row.item; item != nil {
			if item.leaf != w.Item.Leaf || item.name != w.Item.Name {
				return fmt.Sprintf("row %d: item leaf %v name %q, want leaf %v name %q", i, item.leaf, item.name, w.Item.Leaf, w.Item.Name)
			}
			if item.leaf != (w.Item.Value != nil) {
				return fmt.Sprintf("row %d: the golden item has a value = %v for leaf = %v", i, w.Item.Value != nil, item.leaf)
			}
			if item.leaf {
				u, err := strconv.ParseUint(*w.Item.Value, 16, 64)
				if err != nil {
					return err.Error()
				}
				wantValue := math.Float64frombits(u)
				if math.Float64bits(item.value) != u && !(math.IsNaN(item.value) && math.IsNaN(wantValue)) {
					return fmt.Sprintf("row %d: value = %v, want %v", i, item.value, wantValue)
				}
			}
			classSelector := "undefined"
			if item.classSelector != "" {
				classSelector = strconv.Quote(item.classSelector)
			}
			if classSelector != optional(w.Item.ClassSelector) {
				return fmt.Sprintf("row %d: classSelector = %s, want %s", i, classSelector, optional(w.Item.ClassSelector))
			}
		}
	}
	return ""
}

func TestParseASTGolden(t *testing.T) {
	var golden struct {
		Fixtures int             `json:"fixtures"`
		Cases    []goldenASTCase `json:"cases"`
	}
	loadGzipJSON(t, "testdata/ast.json.gz", &golden)
	if len(golden.Cases) < 2500 || golden.Fixtures != 52 {
		t.Fatalf("golden file has %d cases, %d from fixtures", len(golden.Cases), golden.Fixtures)
	}
	accepted, rejected := 0, 0
	for i, c := range golden.Cases {
		ast, err := parseAST(c.Text)
		if c.Error {
			rejected++
			if err == nil {
				t.Errorf("case %d: the port accepts %q, the real parser fails", i, c.Text)
			}
			continue
		}
		accepted++
		if err != nil {
			t.Errorf("case %d: the port fails for %q: %v", i, c.Text, err)
			continue
		}
		if diff := astDiff(ast, c.AST); diff != "" {
			t.Errorf("case %d: %q: %s", i, c.Text, diff)
		}
	}
	t.Logf("%d cases: %d with the same AST, %d that both parsers reject", len(golden.Cases), accepted, rejected)
}
