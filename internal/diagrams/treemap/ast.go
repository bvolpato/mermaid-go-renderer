// Ported from mermaid 12.2.0,
// packages/parser/src/language/treemap/treemap.langium and valueConverter.ts:
// the grammar of the treemap diagram and the values of its tokens (MIT).

package treemap

import (
	"regexp"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/langium"
)

// treemapAST is the AST node Treemap.
type treemapAST struct {
	langium.Common
	// rows is TreemapRows.
	rows []*treemapRow
}

// treemapRow is one element of TreemapRows: a TreemapRow, or the
// ClassDefStatement that the value converter makes of the rule ClassDef.
type treemapRow struct {
	// indent is the length of the INDENTATION token, or nil.
	indent *int
	// item is set for a TreemapRow with an item.
	item *treemapItem
	// classDef is set for a ClassDefStatement.
	classDef *classDefStatement
}

// treemapItem is the AST node Item: a Leaf or a Section.
type treemapItem struct {
	// leaf is true for the type Leaf and false for the type Section.
	leaf bool
	name string
	// value is the value of a Leaf.
	value float64
	// classSelector is "" when the item has none.
	classSelector string
}

// classDefStatement is the object that the value converter returns for the
// rule ClassDef.
type classDefStatement struct {
	className string
	// styleText is "" when the statement has none: undefined upstream.
	styleText string
}

// jsSpace is the class \s of a JavaScript regular expression.
const jsSpace = `[\t\n\v\f\r \x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}]`

// classDefRegex is classDefRegex of valueConverter.ts:
// /classDef\s+([A-Z_a-z]\w+)(?:\s+([^\n\r;]*))?;?/. It extracts the class
// name and the style text of a CLASS_DEF token.
var classDefRegex = regexp.MustCompile(`classDef` + jsSpace + `+([A-Z_a-z][0-9A-Z_a-z]+)(?:` + jsSpace + `+([^\n\r;]*))?;?`)

// parseAST is parse('treemap', text) of the parser package: the lexer of
// the grammar and a parser for the rule Treemap. It returns an error where
// the real parser reports a lexer error or a parser error.
//
// The lexer has no token for a line break: NL is hidden. It gives every run
// of spaces and tabs as an INDENTATION token, because INDENTATION comes
// before the hidden WS. So the grammar reads rows by their tokens only.
func parseAST(text string) (*treemapAST, error) {
	lexer, err := langium.NewLexer("treemap")
	if err != nil {
		return nil, err
	}
	p, err := lexer.Parse(text)
	if err != nil {
		return nil, err
	}

	// Treemap:
	//     TREEMAP_KEYWORD
	//     (
	//         TitleAndAccessibilities
	//         | TreemapRows+=TreemapRow
	//     )*;
	ast := &treemapAST{}
	if _, err := p.Expect("TREEMAP_KEYWORD"); err != nil {
		return nil, err
	}
	for {
		switch {
		case p.At("ACC_DESCR", "ACC_TITLE", "TITLE"):
			titleAndAccessibilities(p, &ast.Common)
		case p.At("INDENTATION", "STRING2", "CLASS_DEF"):
			row, err := parseTreemapRow(p)
			if err != nil {
				return nil, err
			}
			ast.rows = append(ast.rows, row)
		default:
			if err := p.End(); err != nil {
				return nil, err
			}
			return ast, nil
		}
	}
}

// titleAndAccessibilities is the fragment of the treemap grammar:
//
//	((accDescr=ACC_DESCR | accTitle=ACC_TITLE | title=TITLE))+
//
// Other than the fragment of common.langium, it has no end of line.
func titleAndAccessibilities(p *langium.Parser, common *langium.Common) {
	for p.At("ACC_DESCR", "ACC_TITLE", "TITLE") {
		t := p.Next()
		var value string
		var ok bool
		var target **string
		switch t.Type.Name {
		case "ACC_DESCR":
			value, ok = langium.ConvertAccDescr(t.Image)
			target = &common.AccDescr
		case "ACC_TITLE":
			value, ok = langium.ConvertAccTitle(t.Image)
			target = &common.AccTitle
		default:
			value, ok = langium.ConvertTitle(t.Image)
			target = &common.Title
		}
		if !ok {
			// The common converter returns undefined and the default
			// converter of Langium then keeps the text of the token.
			value = t.Image
		}
		*target = &value
	}
}

// parseTreemapRow is the rule
//
//	TreemapRow:
//	    indent=INDENTATION? (item=Item | ClassDef);
func parseTreemapRow(p *langium.Parser) (*treemapRow, error) {
	row := &treemapRow{}
	if t := p.Accept("INDENTATION"); t != nil {
		// The converter of INDENTATION returns input.length.
		n := js.UTF16Len(t.Image)
		row.indent = &n
	}
	if t := p.Accept("CLASS_DEF"); t != nil {
		// ClassDef returns string: CLASS_DEF. The converter makes a
		// ClassDefStatement of the text.
		row.classDef = &classDefStatement{}
		if match := classDefRegex.FindStringSubmatch(t.Image); match != nil {
			row.classDef.className = match[1]
			// styleText: match[2] || undefined
			row.classDef.styleText = match[2]
		}
		return row, nil
	}
	if !p.At("STRING2") {
		return nil, p.Unexpected("STRING2", "CLASS_DEF")
	}

	// Item returns Item:
	//     Leaf | Section;
	// Section returns Section:
	//     name=STRING2 (STYLE_SEPARATOR classSelector=ID2)?;
	// Leaf returns Leaf:
	//     name=STRING2 INDENTATION? (SEPARATOR | COMMA) INDENTATION? value=MyNumber (STYLE_SEPARATOR classSelector=ID2)?;
	//
	// Both alternatives start with STRING2. The item is a Leaf when a
	// separator follows, with or without an INDENTATION in front. In all
	// other cases the INDENTATION starts the next row.
	item := &treemapItem{}
	row.item = item
	name := p.Next()
	// The converter of STRING2 removes the quotes.
	item.name = js.StringSlice(name.Image, 1, js.UTF16Len(name.Image)-1)
	k := 1
	if p.At("INDENTATION") {
		k = 2
	}
	if next := p.LA(k).Type.Name; next == "SEPARATOR" || next == "COMMA" {
		item.leaf = true
		p.Accept("INDENTATION")
		p.Next()
		p.Accept("INDENTATION")
		// MyNumber returns number: NUMBER2. The converter of NUMBER2
		// removes the commas and calls parseFloat.
		number, err := p.Expect("NUMBER2")
		if err != nil {
			return nil, err
		}
		item.value = js.ParseFloat(strings.ReplaceAll(number.Image, ",", ""))
	}
	if p.Accept("STYLE_SEPARATOR") != nil {
		id, err := p.Expect("ID2")
		if err != nil {
			return nil, err
		}
		item.classSelector = id.Image
	}
	return row, nil
}
