package railroad

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/langium"
)

// The golden file comes from the real parsers of the four railroad grammars
// in Node.js (tools/reference/gen-railroad-ast.mjs): the corpus fixtures,
// hand-written inputs, random sentences and mutated inputs, each with the
// AST or the fact that the parser fails.

type goldenASTCase struct {
	Text  string `json:"text"`
	Error bool   `json:"error"`
	AST   any    `json:"ast"`
}

type goldenASTGrammar struct {
	Fixtures int             `json:"fixtures"`
	Cases    []goldenASTCase `json:"cases"`
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

// The plain functions give an AST in the form of the golden file: the
// properties that are defined, with the type of the node as $type.

func plainCommon(typ string, c *langium.Common) map[string]any {
	m := map[string]any{"$type": typ}
	if c.Title != nil {
		m["title"] = *c.Title
	}
	if c.AccTitle != nil {
		m["accTitle"] = *c.AccTitle
	}
	if c.AccDescr != nil {
		m["accDescr"] = *c.AccDescr
	}
	return m
}

func plainList[T any](list []T, plain func(T) any) []any {
	out := make([]any, len(list))
	for i, e := range list {
		out[i] = plain(e)
	}
	return out
}

func plainRailroad(ast *railroadAST) any {
	m := plainCommon("Railroad", &ast.Common)
	m["rules"] = plainList(ast.rules, func(r *railroadRule) any {
		return map[string]any{"$type": "RailroadRule", "name": r.name, "definition": plainRailroadExpr(r.definition)}
	})
	return m
}

func plainRailroadExpr(e *railroadExpr) any {
	m := map[string]any{"$type": e.typ}
	switch e.typ {
	case "RailroadSequenceExpr":
		m["elements"] = plainList(e.elements, plainRailroadExpr)
	case "RailroadChoiceExpr":
		m["alternatives"] = plainList(e.alternatives, plainRailroadExpr)
	case "RailroadOptionalExpr", "RailroadOneOrMoreExpr", "RailroadZeroOrMoreExpr":
		m["element"] = plainRailroadExpr(e.element)
	case "RailroadTerminalExpr":
		m["value"] = e.value
	case "RailroadNonTerminalExpr":
		m["name"] = e.name
	case "RailroadSpecialExpr":
		m["text"] = e.text
	}
	return m
}

func plainEbnf(ast *ebnfAST) any {
	m := plainCommon("RailroadEbnf", &ast.Common)
	m["rules"] = plainList(ast.rules, func(r *ebnfRule) any {
		return map[string]any{"$type": "EbnfRule", "name": r.name, "definition": plainEbnfChoice(r.definition)}
	})
	return m
}

func plainEbnfChoice(c *ebnfChoice) any {
	return map[string]any{"$type": "EbnfChoice", "alternatives": plainList(c.alternatives, func(s *ebnfSequence) any {
		return map[string]any{"$type": "EbnfSequence", "elements": plainList(s.elements, func(t *ebnfTerm) any {
			return map[string]any{"$type": "EbnfTerm", "base": plainEbnfPrimary(t.base), "postfixes": plainList(t.postfixes, func(p *ebnfPostfix) any {
				m := map[string]any{"$type": p.typ}
				if p.typ == "EbnfExceptionPostfix" {
					m["except"] = plainEbnfPrimary(p.except)
				} else {
					m["operator"] = p.operator
				}
				return m
			})}
		})}
	})}
}

func plainEbnfPrimary(p *ebnfPrimary) any {
	m := map[string]any{"$type": p.typ}
	switch p.typ {
	case "EbnfTerminal":
		m["value"] = p.value
	case "EbnfNonTerminal":
		m["name"] = p.name
	case "EbnfSpecial":
		m["text"] = p.text
	default:
		m["element"] = plainEbnfChoice(p.element)
	}
	return m
}

func plainAbnf(ast *abnfAST) any {
	m := plainCommon("RailroadAbnf", &ast.Common)
	m["rules"] = plainList(ast.rules, func(r *abnfRule) any {
		return map[string]any{"$type": "AbnfRule", "name": r.name, "definition": plainAbnfAlternation(r.definition)}
	})
	return m
}

func plainAbnfAlternation(a *abnfAlternation) any {
	return map[string]any{"$type": "AbnfAlternation", "alternatives": plainList(a.alternatives, func(c *abnfConcatenation) any {
		return map[string]any{"$type": "AbnfConcatenation", "elements": plainList(c.elements, func(e *abnfElement) any {
			m := map[string]any{"$type": "AbnfElement"}
			if e.hasRepeat {
				m["repeat"] = e.repeat
			}
			p := map[string]any{"$type": e.primary.typ}
			switch e.primary.typ {
			case "AbnfStringLiteral", "AbnfNumVal":
				p["value"] = e.primary.value
			case "AbnfRuleName":
				p["name"] = e.primary.name
			default:
				p["element"] = plainAbnfAlternation(e.primary.element)
			}
			m["primary"] = p
			return m
		})}
	})}
}

func plainPeg(ast *pegAST) any {
	m := plainCommon("RailroadPeg", &ast.Common)
	m["rules"] = plainList(ast.rules, func(r *pegRule) any {
		return map[string]any{"$type": "PegRule", "name": r.name, "definition": plainPegChoice(r.definition)}
	})
	return m
}

func plainPegChoice(c *pegOrderedChoice) any {
	return map[string]any{"$type": "PegOrderedChoice", "alternatives": plainList(c.alternatives, func(s *pegSequence) any {
		return map[string]any{"$type": "PegSequence", "elements": plainList(s.elements, func(p *pegPrefix) any {
			m := map[string]any{"$type": "PegPrefix"}
			if p.operator != "" {
				m["operator"] = p.operator
			}
			suffix := map[string]any{"$type": "PegSuffix"}
			if p.suffix.operator != "" {
				suffix["operator"] = p.suffix.operator
			}
			primary := map[string]any{"$type": p.suffix.primary.typ}
			switch p.suffix.primary.typ {
			case "PegLiteral":
				primary["value"] = p.suffix.primary.value
			case "PegIdentifier":
				primary["name"] = p.suffix.primary.name
			case "PegGroup":
				primary["element"] = plainPegChoice(p.suffix.primary.element)
			default:
				primary["dot"] = p.suffix.primary.dot
			}
			suffix["primary"] = primary
			m["suffix"] = suffix
			return m
		})}
	})}
}

// parsePlain runs the parser of a grammar and returns the AST in the form
// of the golden file.
func parsePlain(grammar, text string) (any, error) {
	switch grammar {
	case "railroad":
		ast, err := parseRailroad(text)
		if err != nil {
			return nil, err
		}
		return plainRailroad(ast), nil
	case "railroadEbnf":
		ast, err := parseEbnf(text)
		if err != nil {
			return nil, err
		}
		return plainEbnf(ast), nil
	case "railroadAbnf":
		ast, err := parseAbnf(text)
		if err != nil {
			return nil, err
		}
		return plainAbnf(ast), nil
	}
	ast, err := parsePeg(text)
	if err != nil {
		return nil, err
	}
	return plainPeg(ast), nil
}

func TestParseASTGolden(t *testing.T) {
	var golden map[string]goldenASTGrammar
	loadGzipJSON(t, "testdata/ast.json.gz", &golden)
	wantFixtures := map[string]int{"railroad": 10, "railroadEbnf": 4, "railroadAbnf": 2, "railroadPeg": 2}
	for _, grammar := range []string{"railroad", "railroadEbnf", "railroadAbnf", "railroadPeg"} {
		g := golden[grammar]
		if len(g.Cases) < 2000 || g.Fixtures != wantFixtures[grammar] {
			t.Fatalf("%s: the golden file has %d cases, %d from fixtures", grammar, len(g.Cases), g.Fixtures)
		}
		accepted, rejected, failed := 0, 0, 0
		for i, c := range g.Cases {
			got, err := parsePlain(grammar, c.Text)
			if c.Error {
				rejected++
				if err == nil {
					failed++
					t.Errorf("%s case %d: the port accepts %q, the real parser fails", grammar, i, c.Text)
				}
				continue
			}
			accepted++
			if err != nil {
				failed++
				t.Errorf("%s case %d: the port fails for %q: %v", grammar, i, c.Text, err)
				continue
			}
			if !reflect.DeepEqual(got, c.AST) {
				failed++
				gotJSON, _ := json.Marshal(got)
				wantJSON, _ := json.Marshal(c.AST)
				t.Errorf("%s case %d: %q:\n got %s\nwant %s", grammar, i, c.Text, gotJSON, wantJSON)
			}
		}
		t.Logf("%s: %d cases: %d with the same AST, %d that both parsers reject, %d failures", grammar, len(g.Cases), accepted, rejected, failed)
	}
}
