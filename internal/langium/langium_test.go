package langium

import (
	"compress/gzip"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// readGolden reads a gzipped JSON file that tools/reference/gen-langium.mjs
// wrote.
func readGolden(t testing.TB, name string, into any) {
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
	if err := json.NewDecoder(zr).Decode(into); err != nil {
		t.Fatal(err)
	}
}

type lexCase struct {
	Text   string   `json:"text"`
	Tokens [][3]int `json:"tokens"`
	Hidden [][3]int `json:"hidden"`
	Errors [][2]int `json:"errors"`
}

// TestTokenize compares the lexer with the real Chevrotain lexer of every
// Langium grammar: the token types, the offsets and the lengths of the
// tokens and of the hidden tokens, and the ranges of the errors.
func TestTokenize(t *testing.T) {
	var golden map[string][]lexCase
	readGolden(t, "tokens.json.gz", &golden)
	if len(golden) != len(Grammars()) {
		t.Fatalf("the golden has %d grammars and the package has %d", len(golden), len(Grammars()))
	}
	total := 0
	for _, grammar := range Grammars() {
		lexer, err := NewLexer(grammar)
		if err != nil {
			t.Fatal(err)
		}
		pack := func(tokens []Token) [][3]int {
			out := [][3]int{}
			for _, tok := range tokens {
				out = append(out, [3]int{tok.Type.Index, tok.Offset, js.UTF16Len(tok.Image)})
			}
			return out
		}
		failed := 0
		for _, c := range golden[grammar] {
			total++
			result := lexer.Tokenize(c.Text)
			errs := [][2]int{}
			for _, e := range result.Errors {
				errs = append(errs, [2]int{e.Offset, e.Length})
			}
			if c.Tokens == nil {
				c.Tokens = [][3]int{}
			}
			if c.Hidden == nil {
				c.Hidden = [][3]int{}
			}
			if c.Errors == nil {
				c.Errors = [][2]int{}
			}
			if !reflect.DeepEqual(pack(result.Tokens), c.Tokens) || !reflect.DeepEqual(pack(result.Hidden), c.Hidden) || !reflect.DeepEqual(errs, c.Errors) {
				failed++
				if failed <= 3 {
					t.Errorf("grammar %s, text %q:\n got tokens %v hidden %v errors %v\nwant tokens %v hidden %v errors %v",
						grammar, c.Text, pack(result.Tokens), pack(result.Hidden), errs, c.Tokens, c.Hidden, c.Errors)
				}
			}
		}
		if failed > 0 {
			t.Errorf("grammar %s: %d of %d cases differ", grammar, failed, len(golden[grammar]))
		}
	}
	t.Logf("%d lexer cases", total)
}

// TestConvert compares the value converters with the real converters.
func TestConvert(t *testing.T) {
	var golden []struct {
		Grammar string          `json:"grammar"`
		Rule    string          `json:"rule"`
		Input   string          `json:"input"`
		Value   json.RawMessage `json:"value"`
	}
	readGolden(t, "convert.json.gz", &golden)
	for _, c := range golden {
		var got any
		switch c.Rule {
		case "ACC_DESCR":
			got, _ = ConvertAccDescr(c.Input)
		case "ACC_TITLE":
			got, _ = ConvertAccTitle(c.Input)
		case "TITLE":
			got, _ = ConvertTitle(c.Input)
		case "STRING":
			got = ConvertString(c.Input)
		case "ID":
			got = ConvertID(c.Input)
		case "INT":
			got = ConvertInt(c.Input)
		case "NUMBER", "NUMBER_PIE":
			got = ConvertNumber(c.Input)
		case "BOOLEAN":
			got = ConvertBoolean(c.Input)
		case "GRATICULE":
			got = c.Input
		default:
			t.Fatalf("no converter for rule %s", c.Rule)
		}
		var want any
		if err := json.Unmarshal(c.Value, &want); err != nil {
			t.Fatal(err)
		}
		// {"$n": "-0"} is the number -0.
		if m, ok := want.(map[string]any); ok && m["$n"] == "-0" {
			want = math.Copysign(0, -1)
		}
		equal := reflect.DeepEqual(got, want)
		if g, ok := got.(float64); ok {
			w, isNumber := want.(float64)
			equal = isNumber && math.Float64bits(g) == math.Float64bits(w)
		}
		if !equal {
			t.Errorf("%s %s %q: got %#v, want %#v", c.Grammar, c.Rule, c.Input, got, want)
		}
	}
	t.Logf("%d converter cases", len(golden))
}

func TestParserHelpers(t *testing.T) {
	lexer := MustLexer("pie")
	p, err := lexer.Parse("\n\npie showData %% c\n  title  My   pie \n accDescr {\n  a\n  b\n}\n\n\"x\": 4")
	if err != nil {
		t.Fatal(err)
	}
	if n := p.Skip("NEWLINE"); n != 2 {
		t.Fatalf("Skip: got %d", n)
	}
	if _, err := p.Expect("pie"); err != nil {
		t.Fatal(err)
	}
	if p.Accept("showData") == nil || p.Accept("showData") != nil {
		t.Fatal("Accept")
	}
	if len(p.Hidden()) != 1 || p.Hidden()[0].Type.Name != "SINGLE_LINE_COMMENT" {
		t.Fatalf("Hidden: got %v", p.Hidden())
	}
	p.Skip("NEWLINE")
	var common Common
	if !p.AtTitleAndAccessibilities() {
		t.Fatal("AtTitleAndAccessibilities")
	}
	if err := p.TitleAndAccessibilities(&common); err != nil {
		t.Fatal(err)
	}
	if common.Title == nil || *common.Title != "My pie" || common.AccDescr == nil || *common.AccDescr != "a\nb" || common.AccTitle != nil {
		t.Fatalf("TitleAndAccessibilities: got %+v", common)
	}
	if tok := p.LA(2); tok.Type.Name != ":" || tok.Line != 10 || tok.Column != 4 {
		t.Fatalf("LA(2): got %+v", tok)
	}
	if _, err := p.Expect(":"); err == nil || !strings.Contains(err.Error(), "Parse error on line 10, column 1: Expecting token of type ':' but found `\"x\"`.") {
		t.Fatalf("Expect: got %v", err)
	}
	if err := p.End(); err == nil {
		t.Fatal("End: no error")
	}
	p.Next()
	p.Next()
	p.Next()
	if err := p.EOL(); err != nil || p.End() != nil || !p.At(EOF) {
		t.Fatal("EOL at the end")
	}
	if _, err := p.Expect(":"); err == nil || !strings.Contains(err.Error(), "line ?, column ?") {
		t.Fatalf("Expect at the end: got %v", err)
	}

	if _, err := lexer.Parse("pie\n\"a\": 1 # 2\n"); err == nil || err.Error() != "Parsing failed: Lexer error on line 2, column 8: unexpected character: ->#<- at offset: 11, skipped 1 characters. " {
		t.Fatalf("lexer error: got %v", err)
	}
	if _, err := NewLexer("nothing"); err == nil {
		t.Fatal("NewLexer: no error for an unknown grammar")
	}
}

// FuzzTokenize requires that the lexers do not panic and do not hang.
func FuzzTokenize(f *testing.F) {
	f.Add("pie showData\n\"a\": 1\n")
	f.Add("radar-beta\naxis A[\"x\"], B\ncurve c{1,2}\n")
	f.Add("accDescr {\n\x00\xff\xed\xa0\x80")
	f.Fuzz(func(t *testing.T, text string) {
		if len(text) > 2000 {
			return
		}
		for _, grammar := range Grammars() {
			result := MustLexer(grammar).Tokenize(text)
			end := 0
			for _, tok := range result.Tokens {
				if tok.Offset < end {
					t.Fatalf("grammar %s: token offsets go back", grammar)
				}
				end = tok.Offset
			}
		}
	})
}
