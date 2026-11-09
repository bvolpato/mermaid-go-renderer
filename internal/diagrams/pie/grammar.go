// Ported from mermaid 12.2.0, packages/parser/src/language/pie/pie.langium
// (MIT).

package pie

import "github.com/bvolpato/mermaid-go-renderer/internal/langium"

// Pie is the AST of the entry rule Pie.
type Pie struct {
	langium.Common
	ShowData bool
	Sections []PieSection
}

// PieSection is the AST of the rule PieSection.
type PieSection struct {
	Label string
	Value float64
}

var lexer = langium.MustLexer("pie")

// parse is parse('pie', text) of the parser package. The grammar is:
//
//	entry Pie:
//	  NEWLINE*
//	  "pie" showData?="showData"?
//	  (TitleAndAccessibilities | sections+=PieSection | NEWLINE)*
//	;
//	PieSection:
//	  label=STRING ":" value=NUMBER_PIE EOL
//	;
func parse(text string) (*Pie, error) {
	p, err := lexer.Parse(text)
	if err != nil {
		return nil, err
	}
	ast := &Pie{Sections: []PieSection{}}
	p.Skip("NEWLINE")
	if _, err := p.Expect("pie"); err != nil {
		return nil, err
	}
	ast.ShowData = p.Accept("showData") != nil
	for {
		switch {
		case p.AtTitleAndAccessibilities():
			if err := p.TitleAndAccessibilities(&ast.Common); err != nil {
				return nil, err
			}
		case p.At("STRING"):
			section, err := pieSection(p)
			if err != nil {
				return nil, err
			}
			ast.Sections = append(ast.Sections, section)
		case p.At("NEWLINE"):
			p.Next()
		default:
			if err := p.End(); err != nil {
				return nil, err
			}
			return ast, nil
		}
	}
}

func pieSection(p *langium.Parser) (PieSection, error) {
	label, err := p.Expect("STRING")
	if err != nil {
		return PieSection{}, err
	}
	if _, err := p.Expect(":"); err != nil {
		return PieSection{}, err
	}
	value, err := p.Expect("NUMBER_PIE")
	if err != nil {
		return PieSection{}, err
	}
	if err := p.EOL(); err != nil {
		return PieSection{}, err
	}
	return PieSection{
		Label: langium.ConvertString(label.Image),
		Value: langium.ConvertNumber(value.Image),
	}, nil
}
