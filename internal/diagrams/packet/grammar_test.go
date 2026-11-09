package packet

import (
	"compress/gzip"
	"encoding/json"
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

// plain is the AST in the form of the golden file.
func (ast *Packet) plain() any {
	out := map[string]any{"$type": "Packet"}
	blocks := []any{}
	for _, b := range ast.Blocks {
		block := map[string]any{"$type": "PacketBlock", "label": b.Label}
		if b.Start != nil {
			block["start"] = *b.Start
		}
		if b.End != nil {
			block["end"] = *b.End
		}
		if b.Bits != nil {
			block["bits"] = *b.Bits
		}
		blocks = append(blocks, block)
	}
	out["blocks"] = blocks
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

// TestParse compares the parser with parse('packet', text) of the parser
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
