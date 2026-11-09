// Ported from mermaid 12.2.0,
// packages/parser/src/language/architecture/architecture.langium,
// arch.langium and valueConverter.ts (MIT).

package architecture

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/langium"
)

// Architecture is the AST of the entry rule Architecture.
type Architecture struct {
	langium.Common
	Groups     []GroupAST
	Services   []ServiceAST
	Junctions  []JunctionAST
	Edges      []EdgeAST
	Alignments []AlignmentAST
}

// GroupAST is the AST of the rule Group. A nil pointer is a property that
// the text does not set: undefined in the AST.
type GroupAST struct {
	ID    string
	Icon  *string
	Title *string
	In    *string
}

// ServiceAST is the AST of the rule Service.
type ServiceAST struct {
	ID       string
	IconText *string
	Icon     *string
	Title    *string
	In       *string
}

// JunctionAST is the AST of the rule Junction.
type JunctionAST struct {
	ID string
	In *string
}

// EdgeAST is the AST of the rule Edge.
type EdgeAST struct {
	LhsID    string
	LhsGroup bool
	LhsDir   string
	LhsInto  bool
	Title    *string
	RhsInto  bool
	RhsDir   string
	RhsID    string
	RhsGroup bool
}

// AlignmentAST is the AST of the rule Alignment.
type AlignmentAST struct {
	// Direction is "row" or "column".
	Direction string
	Members   []string
}

var lexer = langium.MustLexer("architecture")

// parseAST is parse('architecture', text) of the parser package. The
// grammar is:
//
//	entry Architecture:
//	    NEWLINE*
//	    "architecture-beta"
//	    (
//	        NEWLINE
//	        | TitleAndAccessibilities
//	        | Statement
//	    )*
//	;
//
//	fragment Statement:
//	    groups+=Group
//	    | services+=Service
//	    | junctions+=Junction
//	    | edges+=Edge
//	    | alignments+=Alignment
//	;
func parseAST(text string) (*Architecture, error) {
	p, err := lexer.Parse(text)
	if err != nil {
		return nil, err
	}
	ast := &Architecture{
		Groups:     []GroupAST{},
		Services:   []ServiceAST{},
		Junctions:  []JunctionAST{},
		Edges:      []EdgeAST{},
		Alignments: []AlignmentAST{},
	}
	p.Skip("NEWLINE")
	if _, err := p.Expect("architecture-beta"); err != nil {
		return nil, err
	}
	for {
		switch {
		case p.At("NEWLINE"):
			p.Next()
		case p.AtTitleAndAccessibilities():
			if err := p.TitleAndAccessibilities(&ast.Common); err != nil {
				return nil, err
			}
		case p.At("group"):
			g, err := group(p)
			if err != nil {
				return nil, err
			}
			ast.Groups = append(ast.Groups, g)
		case p.At("service"):
			s, err := service(p)
			if err != nil {
				return nil, err
			}
			ast.Services = append(ast.Services, s)
		case p.At("junction"):
			j, err := junction(p)
			if err != nil {
				return nil, err
			}
			ast.Junctions = append(ast.Junctions, j)
		case p.At("ID"):
			e, err := edge(p)
			if err != nil {
				return nil, err
			}
			ast.Edges = append(ast.Edges, e)
		case p.At("align"):
			a, err := alignment(p)
			if err != nil {
				return nil, err
			}
			ast.Alignments = append(ast.Alignments, a)
		default:
			if err := p.End(); err != nil {
				return nil, err
			}
			return ast, nil
		}
	}
}

// id is one ID token: the value of the default converter.
func id(p *langium.Parser) (string, error) {
	t, err := p.Expect("ID")
	if err != nil {
		return "", err
	}
	return langium.ConvertID(t.Image), nil
}

// in is ('in' in=ID)?.
func in(p *langium.Parser) (*string, error) {
	if p.Accept("in") == nil {
		return nil, nil
	}
	v, err := id(p)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// archIcon is icon=ARCH_ICON?.
func archIcon(p *langium.Parser) *string {
	t := p.Accept("ARCH_ICON")
	if t == nil {
		return nil
	}
	v := convertArchIcon(t.Image)
	return &v
}

// archTitle is title=ARCH_TITLE?.
func archTitle(p *langium.Parser) *string {
	t := p.Accept("ARCH_TITLE")
	if t == nil {
		return nil
	}
	v := convertArchTitle(t.Image)
	return &v
}

// group is the rule
//
//	Group:
//	    'group' id=ID icon=ARCH_ICON? title=ARCH_TITLE? ('in' in=ID)? EOL
//	;
func group(p *langium.Parser) (GroupAST, error) {
	var g GroupAST
	var err error
	if _, err = p.Expect("group"); err != nil {
		return g, err
	}
	if g.ID, err = id(p); err != nil {
		return g, err
	}
	g.Icon = archIcon(p)
	g.Title = archTitle(p)
	if g.In, err = in(p); err != nil {
		return g, err
	}
	return g, p.EOL()
}

// service is the rule
//
//	Service:
//	    'service' id=ID (iconText=STRING | icon=ARCH_ICON)? title=ARCH_TITLE? ('in' in=ID)? EOL
//	;
func service(p *langium.Parser) (ServiceAST, error) {
	var s ServiceAST
	var err error
	if _, err = p.Expect("service"); err != nil {
		return s, err
	}
	if s.ID, err = id(p); err != nil {
		return s, err
	}
	if t := p.Accept("STRING"); t != nil {
		v := langium.ConvertString(t.Image)
		s.IconText = &v
	} else {
		s.Icon = archIcon(p)
	}
	s.Title = archTitle(p)
	if s.In, err = in(p); err != nil {
		return s, err
	}
	return s, p.EOL()
}

// junction is the rule
//
//	Junction:
//	    'junction' id=ID ('in' in=ID)? EOL
//	;
func junction(p *langium.Parser) (JunctionAST, error) {
	var j JunctionAST
	var err error
	if _, err = p.Expect("junction"); err != nil {
		return j, err
	}
	if j.ID, err = id(p); err != nil {
		return j, err
	}
	if j.In, err = in(p); err != nil {
		return j, err
	}
	return j, p.EOL()
}

// edge is the rule Edge with its fragments:
//
//	Edge:
//	    lhsId=ID lhsGroup?=ARROW_GROUP? Arrow rhsId=ID rhsGroup?=ARROW_GROUP? EOL
//	;
//
//	fragment Arrow:
//	    LeftPort lhsInto?=ARROW_INTO? ('--' | '-' title=ARCH_TITLE '-') rhsInto?=ARROW_INTO? RightPort
//	;
//
//	fragment LeftPort:
//	    ':'lhsDir=ARROW_DIRECTION
//	;
//
//	fragment RightPort:
//	    rhsDir=ARROW_DIRECTION':'
//	;
func edge(p *langium.Parser) (EdgeAST, error) {
	var e EdgeAST
	var err error
	if e.LhsID, err = id(p); err != nil {
		return e, err
	}
	e.LhsGroup = p.Accept("ARROW_GROUP") != nil

	// LeftPort
	if _, err = p.Expect(":"); err != nil {
		return e, err
	}
	t, err := p.Expect("ARROW_DIRECTION")
	if err != nil {
		return e, err
	}
	e.LhsDir = t.Image

	e.LhsInto = p.Accept("ARROW_INTO") != nil
	switch {
	case p.Accept("--") != nil:
	case p.Accept("-") != nil:
		t, err := p.Expect("ARCH_TITLE")
		if err != nil {
			return e, err
		}
		v := convertArchTitle(t.Image)
		e.Title = &v
		if _, err = p.Expect("-"); err != nil {
			return e, err
		}
	default:
		return e, p.Unexpected("--", "-")
	}
	e.RhsInto = p.Accept("ARROW_INTO") != nil

	// RightPort
	if t, err = p.Expect("ARROW_DIRECTION"); err != nil {
		return e, err
	}
	e.RhsDir = t.Image
	if _, err = p.Expect(":"); err != nil {
		return e, err
	}

	if e.RhsID, err = id(p); err != nil {
		return e, err
	}
	e.RhsGroup = p.Accept("ARROW_GROUP") != nil
	return e, p.EOL()
}

// alignment is the rule
//
//	Alignment:
//	    'align' direction=('row' | 'column') members+=ID (members+=ID)+ EOL
//	;
func alignment(p *langium.Parser) (AlignmentAST, error) {
	a := AlignmentAST{Members: []string{}}
	if _, err := p.Expect("align"); err != nil {
		return a, err
	}
	switch {
	case p.Accept("row") != nil:
		a.Direction = "row"
	case p.Accept("column") != nil:
		a.Direction = "column"
	default:
		return a, p.Unexpected("row", "column")
	}
	member, err := id(p)
	if err != nil {
		return a, err
	}
	a.Members = append(a.Members, member)
	// (members+=ID)+
	for {
		if member, err = id(p); err != nil {
			return a, err
		}
		a.Members = append(a.Members, member)
		if !p.At("ID") {
			break
		}
	}
	return a, p.EOL()
}

// convertArchIcon is the converter of the terminal ARCH_ICON:
// input.replace(/[()]/g, ”).trim().
func convertArchIcon(input string) string {
	input = strings.ReplaceAll(input, "(", "")
	input = strings.ReplaceAll(input, ")", "")
	return js.TrimSpace(input)
}

// convertArchTitle is the converter of the terminal ARCH_TITLE.
func convertArchTitle(input string) string {
	// input.replace(/^\[|]$/g, '').trim()
	result := strings.TrimPrefix(input, "[")
	result = strings.TrimSuffix(result, "]")
	result = js.TrimSpace(result)
	// Check if wrapped in quotes and remove only outer quotes
	if (strings.HasPrefix(result, `"`) && strings.HasSuffix(result, `"`)) ||
		(strings.HasPrefix(result, "'") && strings.HasSuffix(result, "'")) {
		// result.slice(1, -1): empty for a text of one quote.
		if len(result) < 2 {
			result = ""
		} else {
			result = result[1 : len(result)-1]
		}
		// Unescape escaped quotes
		result = strings.ReplaceAll(result, `\"`, `"`)
		result = strings.ReplaceAll(result, `\'`, "'")
	}
	return js.TrimSpace(result)
}
