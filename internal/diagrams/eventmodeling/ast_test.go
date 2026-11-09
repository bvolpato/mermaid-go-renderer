package eventmodeling

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// The golden file has the AST of the real parser of the parser package
// (parse('eventmodeling', text)) for corpus fixtures, hand-written inputs
// and mutated inputs, or the fact that the parser fails:
// tools/reference/gen-eventmodeling-ast.mjs writes it.

type astGolden struct {
	Fixtures    int       `json:"fixtures"`
	HandWritten int       `json:"handWritten"`
	Cases       []astCase `json:"cases"`
}

type astCase struct {
	Text  string `json:"text"`
	Error bool   `json:"error"`
	AST   any    `json:"ast"`
}

func loadASTGolden(tb testing.TB) *astGolden {
	tb.Helper()
	f, err := os.Open("testdata/ast.json.gz")
	if err != nil {
		tb.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		tb.Fatal(err)
	}
	var g astGolden
	if err := json.NewDecoder(zr).Decode(&g); err != nil {
		tb.Fatal(err)
	}
	return &g
}

// plainAST is the AST in the plain form of the golden file.
func plainAST(ast *EventModel) map[string]any {
	out := map[string]any{}
	opt := func(target map[string]any, key string, value *string) {
		if value != nil {
			target[key] = *value
		}
	}
	strs := func(list []string) []any {
		out := make([]any, len(list))
		for i, s := range list {
			out[i] = s
		}
		return out
	}
	opt(out, "accDescr", ast.AccDescr)
	opt(out, "accTitle", ast.AccTitle)
	opt(out, "title", ast.Title)
	frames := []any{}
	for _, f := range ast.Frames {
		frame := map[string]any{
			"type":             f.Type,
			"name":             f.Name,
			"modelEntityType":  f.ModelEntityType,
			"entityIdentifier": f.EntityIdentifier,
			"sourceFrames":     strs(f.SourceFrames),
			"line":             float64(f.Line),
		}
		opt(frame, "dataReference", f.DataReference)
		opt(frame, "dataType", f.DataType)
		opt(frame, "dataInlineValue", f.DataInlineValue)
		frames = append(frames, frame)
	}
	out["frames"] = frames
	modelEntities := []any{}
	for _, e := range ast.ModelEntities {
		modelEntities = append(modelEntities, map[string]any{"name": e.Name})
	}
	out["modelEntities"] = modelEntities
	dataEntities := []any{}
	for _, e := range ast.DataEntities {
		entity := map[string]any{"name": e.Name, "dataBlockValue": e.DataBlockValue}
		opt(entity, "dataType", e.DataType)
		dataEntities = append(dataEntities, entity)
	}
	out["dataEntities"] = dataEntities
	noteEntities := []any{}
	for _, e := range ast.NoteEntities {
		note := map[string]any{"sourceFrame": e.SourceFrame, "dataBlockValue": e.DataBlockValue}
		opt(note, "dataType", e.DataType)
		noteEntities = append(noteEntities, note)
	}
	out["noteEntities"] = noteEntities
	statements := func(list []*EmGwtStatement) []any {
		out := make([]any, len(list))
		for i, s := range list {
			out[i] = s.EntityIdentifier
		}
		return out
	}
	gwtEntities := []any{}
	for _, e := range ast.GwtEntities {
		gwtEntities = append(gwtEntities, map[string]any{
			"sourceFrame":     e.SourceFrame,
			"givenStatements": statements(e.GivenStatements),
			"whenStatements":  statements(e.WhenStatements),
			"thenStatements":  statements(e.ThenStatements),
		})
	}
	out["gwtEntities"] = gwtEntities
	return out
}

func TestParserAgainstLangium(t *testing.T) {
	g := loadASTGolden(t)
	if len(g.Cases) < 3000 {
		t.Fatalf("only %d cases", len(g.Cases))
	}
	accepted, rejected, failures := 0, 0, 0
	for i, c := range g.Cases {
		ast, err := parse(c.Text)
		switch {
		case c.Error && err == nil:
			failures++
			if failures <= 20 {
				t.Errorf("case %d: no error, the real parser rejects the text\n%q", i, c.Text)
			}
		case c.Error:
			rejected++
		case err != nil:
			failures++
			if failures <= 20 {
				t.Errorf("case %d: error %v, the real parser accepts the text\n%q", i, err, c.Text)
			}
		default:
			if got := plainAST(ast); !reflect.DeepEqual(got, c.AST) {
				failures++
				if failures <= 20 {
					gotJSON, _ := json.Marshal(got)
					wantJSON, _ := json.Marshal(c.AST)
					t.Errorf("case %d: AST\n got  %s\n want %s\n%q", i, gotJSON, wantJSON, c.Text)
				}
				continue
			}
			accepted++
		}
	}
	t.Logf("%d cases (%d from fixtures, %d hand-written): %d with the same AST, %d that both reject, %d failures",
		len(g.Cases), g.Fixtures, g.HandWritten-g.Fixtures, accepted, rejected, failures)
}
