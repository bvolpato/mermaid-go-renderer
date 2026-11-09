package architecture

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/langium"
)

// The golden file comes from the real parser of the architecture grammar in
// Node.js (tools/reference/gen-architecture-ast.mjs): the corpus fixtures,
// hand-written inputs, generated inputs and mutated inputs, each with the
// AST or the fact that the parser fails.

type astCase struct {
	Text  string `json:"text"`
	AST   any    `json:"ast"`
	Error bool   `json:"error"`
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

func setOptional(m map[string]any, key string, v *string) {
	if v != nil {
		m[key] = *v
	}
}

// plain is the AST in the form of the golden file.
func (ast *Architecture) plain() any {
	out := map[string]any{}
	setOptional(out, "title", ast.Title)
	setOptional(out, "accTitle", ast.AccTitle)
	setOptional(out, "accDescr", ast.AccDescr)
	groups := []any{}
	for _, g := range ast.Groups {
		m := map[string]any{"id": g.ID}
		setOptional(m, "icon", g.Icon)
		setOptional(m, "title", g.Title)
		setOptional(m, "in", g.In)
		groups = append(groups, m)
	}
	out["groups"] = groups
	services := []any{}
	for _, s := range ast.Services {
		m := map[string]any{"id": s.ID}
		setOptional(m, "iconText", s.IconText)
		setOptional(m, "icon", s.Icon)
		setOptional(m, "title", s.Title)
		setOptional(m, "in", s.In)
		services = append(services, m)
	}
	out["services"] = services
	junctions := []any{}
	for _, j := range ast.Junctions {
		m := map[string]any{"id": j.ID}
		setOptional(m, "in", j.In)
		junctions = append(junctions, m)
	}
	out["junctions"] = junctions
	edges := []any{}
	for _, e := range ast.Edges {
		m := map[string]any{
			"lhsId": e.LhsID, "lhsGroup": e.LhsGroup, "lhsDir": e.LhsDir, "lhsInto": e.LhsInto,
			"rhsInto": e.RhsInto, "rhsDir": e.RhsDir, "rhsId": e.RhsID, "rhsGroup": e.RhsGroup,
		}
		setOptional(m, "title", e.Title)
		edges = append(edges, m)
	}
	out["edges"] = edges
	alignments := []any{}
	for _, a := range ast.Alignments {
		members := []any{}
		for _, m := range a.Members {
			members = append(members, m)
		}
		alignments = append(alignments, map[string]any{"direction": a.Direction, "members": members})
	}
	out["alignments"] = alignments
	return out
}

// TestParserGolden compares the hand-written parser with the real parser:
// the same AST for a text that parses, and an error for a text that the
// real parser rejects.
func TestParserGolden(t *testing.T) {
	var cases []astCase
	loadGzipJSON(t, "testdata/ast.json.gz", &cases)
	if len(cases) < 2000 {
		t.Fatalf("the golden file has %d cases", len(cases))
	}
	parsed, rejected, failed := 0, 0, 0
	for i, c := range cases {
		ast, err := parseAST(c.Text)
		switch {
		case c.Error:
			rejected++
			if err == nil {
				failed++
				t.Errorf("case %d: the real parser rejects the text, the port accepts it\ninput: %q", i, c.Text)
				continue
			}
			var perr *langium.Error
			if !errors.As(err, &perr) {
				failed++
				t.Errorf("case %d: the error is %T, want *langium.Error", i, err)
			}
		case err != nil:
			parsed++
			failed++
			t.Errorf("case %d: the real parser accepts the text, the port fails: %v\ninput: %q", i, err, c.Text)
		default:
			parsed++
			if got := ast.plain(); !reflect.DeepEqual(got, c.AST) {
				failed++
				gj, _ := json.Marshal(got)
				wj, _ := json.Marshal(c.AST)
				t.Errorf("case %d: AST differs\ninput: %q\n got: %s\nwant: %s", i, c.Text, gj, wj)
			}
		}
		if failed > 20 {
			t.Fatal("too many differences")
		}
	}
	t.Logf("%d cases: %d parse, %d fail; %d differences", len(cases), parsed, rejected, failed)
}
