package git

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"
)

// readGolden reads a gzip-compressed JSON golden file.
func readGolden(t testing.TB, name string, v any) {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
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
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatal(err)
	}
}

// astJSON converts the AST to the form of the golden file: the properties
// that the Langium parser assigns, with $type on each statement.
func astJSON(ast *gitGraphAST) map[string]any {
	out := map[string]any{}
	setString := func(m map[string]any, key string, v *string) {
		if v != nil {
			m[key] = *v
		}
	}
	tags := func(list []string) []any {
		l := make([]any, len(list))
		for i, s := range list {
			l[i] = s
		}
		return l
	}
	setString(out, "accDescr", ast.accDescr)
	setString(out, "accTitle", ast.accTitle)
	setString(out, "title", ast.title)
	setString(out, "dir", ast.dir)
	statements := make([]any, len(ast.statements))
	for i, statement := range ast.statements {
		m := map[string]any{"$type": statement.astType()}
		switch s := statement.(type) {
		case *commitAST:
			setString(m, "id", s.id)
			setString(m, "message", s.message)
			m["tags"] = tags(s.tags)
			setString(m, "type", s.typ)
		case *branchAST:
			m["name"] = s.name
			if s.order != nil {
				m["order"] = *s.order
			}
		case *mergeAST:
			m["branch"] = s.branch
			setString(m, "id", s.id)
			m["tags"] = tags(s.tags)
			setString(m, "type", s.typ)
		case *checkoutAST:
			m["branch"] = s.branch
		case *cherryPickingAST:
			setString(m, "id", s.id)
			m["tags"] = tags(s.tags)
			setString(m, "parent", s.parent)
		}
		statements[i] = m
	}
	out["statements"] = statements
	return out
}

// TestParserGolden compares the parser with the Langium parser of
// @mermaid-js/parser. The golden file comes from
// tools/reference/gen-gitgraph-parser.mjs.
func TestParserGolden(t *testing.T) {
	var golden struct {
		Cases []struct {
			Text  string         `json:"text"`
			AST   map[string]any `json:"ast"`
			Error bool           `json:"error"`
		} `json:"cases"`
	}
	readGolden(t, "parser.json.gz", &golden)
	if len(golden.Cases) < 9000 {
		t.Fatalf("golden file has %d cases", len(golden.Cases))
	}
	failed := 0
	for i, c := range golden.Cases {
		ast, err := parseGitGraphText(c.Text)
		switch {
		case c.Error && err == nil:
			failed++
			t.Errorf("case %d: upstream reports an error, got %v\ntext: %q", i, astJSON(ast), c.Text)
		case !c.Error && err != nil:
			failed++
			t.Errorf("case %d: upstream parses, got error %v\ntext: %q", i, err, c.Text)
		case !c.Error:
			if got := astJSON(ast); !reflect.DeepEqual(got, c.AST) {
				failed++
				t.Errorf("case %d:\n got: %v\nwant: %v\ntext: %q", i, got, c.AST, c.Text)
			}
		}
		if failed >= 20 {
			t.Fatal("too many failures")
		}
	}
}
