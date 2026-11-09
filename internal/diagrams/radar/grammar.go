// Ported from mermaid 12.2.0, packages/parser/src/language/radar/radar.langium
// (MIT).

package radar

import "github.com/bvolpato/mermaid-go-renderer/internal/langium"

// Radar is the AST of the entry rule Radar.
type Radar struct {
	langium.Common
	Axes    []Axis
	Curves  []Curve
	Options []Option
}

// Axis is the AST of the rule Axis. A nil Label is a label that the text
// does not set.
type Axis struct {
	Name  string
	Label *string
}

// Curve is the AST of the rule Curve.
type Curve struct {
	Name    string
	Label   *string
	Entries []Entry
}

// Entry is the AST of the rules NumberEntry and DetailedEntry. Axis is the
// text of the cross-reference to an axis, or nil for a NumberEntry.
type Entry struct {
	Axis  *string
	Value float64
}

// Option is the AST of the rule Option. Value is a bool for showLegend, a
// float64 for ticks, max and min, and a string for graticule.
type Option struct {
	Name  string
	Value any
}

var lexer = langium.MustLexer("radar")

// parse is parse('radar', text) of the parser package. The grammar is:
//
//	entry Radar:
//	    NEWLINE*
//	    ('radar-beta' | 'radar-beta:' | 'radar-beta' ':')
//	    NEWLINE*
//	    (
//	        TitleAndAccessibilities
//	        | 'axis' axes+=Axis (',' axes+=Axis)*
//	        | 'curve' curves+=Curve (',' curves+=Curve)*
//	        | options+=Option (',' options+=Option)*
//	        | NEWLINE
//	    )*
//	;
func parse(text string) (*Radar, error) {
	p, err := lexer.Parse(text)
	if err != nil {
		return nil, err
	}
	ast := &Radar{Axes: []Axis{}, Curves: []Curve{}, Options: []Option{}}
	p.Skip("NEWLINE")
	// The first and the third alternative start with the same token. The
	// real parser looks at the token after it.
	switch {
	case p.Accept("radar-beta:") != nil:
	case p.Accept("radar-beta") != nil:
		p.Accept(":")
	default:
		return nil, p.Unexpected("radar-beta", "radar-beta:")
	}
	p.Skip("NEWLINE")
	for {
		switch {
		case p.AtTitleAndAccessibilities():
			if err := p.TitleAndAccessibilities(&ast.Common); err != nil {
				return nil, err
			}
		case p.Accept("axis") != nil:
			for {
				a, err := axis(p)
				if err != nil {
					return nil, err
				}
				ast.Axes = append(ast.Axes, a)
				if p.Accept(",") == nil {
					break
				}
			}
		case p.Accept("curve") != nil:
			for {
				c, err := curve(p)
				if err != nil {
					return nil, err
				}
				ast.Curves = append(ast.Curves, c)
				if p.Accept(",") == nil {
					break
				}
			}
		case p.At("showLegend", "ticks", "max", "min", "graticule"):
			for {
				o, err := option(p)
				if err != nil {
					return nil, err
				}
				ast.Options = append(ast.Options, o)
				if p.Accept(",") == nil {
					break
				}
			}
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

// label is the fragment Label: '[' label=STRING ']'. It returns nil when
// the next token is not '[': the callers have (Label)?.
func label(p *langium.Parser) (*string, error) {
	if p.Accept("[") == nil {
		return nil, nil
	}
	t, err := p.Expect("STRING")
	if err != nil {
		return nil, err
	}
	if _, err := p.Expect("]"); err != nil {
		return nil, err
	}
	text := langium.ConvertString(t.Image)
	return &text, nil
}

// axis is the rule Axis: name=ID (Label)?.
func axis(p *langium.Parser) (Axis, error) {
	name, err := p.Expect("ID")
	if err != nil {
		return Axis{}, err
	}
	l, err := label(p)
	return Axis{Name: langium.ConvertID(name.Image), Label: l}, err
}

// curve is the rule Curve: name=ID (Label)? '{' Entries '}'.
func curve(p *langium.Parser) (Curve, error) {
	name, err := p.Expect("ID")
	if err != nil {
		return Curve{}, err
	}
	c := Curve{Name: langium.ConvertID(name.Image)}
	if c.Label, err = label(p); err != nil {
		return c, err
	}
	if _, err := p.Expect("{"); err != nil {
		return c, err
	}
	if c.Entries, err = entries(p); err != nil {
		return c, err
	}
	_, err = p.Expect("}")
	return c, err
}

// entries is the fragment Entries:
//
//	NEWLINE* entries+=NumberEntry (',' NEWLINE* entries+=NumberEntry)* NEWLINE* |
//	NEWLINE* entries+=DetailedEntry (',' NEWLINE* entries+=DetailedEntry)* NEWLINE*
//
// Both alternatives start with NEWLINE*. The real parser looks past these
// tokens: a NUMBER selects the first alternative and an ID the second.
func entries(p *langium.Parser) ([]Entry, error) {
	p.Skip("NEWLINE")
	var entry func() (Entry, error)
	switch {
	case p.At("NUMBER"):
		entry = func() (Entry, error) { return numberEntry(p) }
	case p.At("ID"):
		entry = func() (Entry, error) { return detailedEntry(p) }
	default:
		return nil, p.Unexpected("NUMBER", "ID")
	}
	list := []Entry{}
	for {
		e, err := entry()
		if err != nil {
			return nil, err
		}
		list = append(list, e)
		if p.Accept(",") == nil {
			break
		}
		p.Skip("NEWLINE")
	}
	p.Skip("NEWLINE")
	return list, nil
}

// numberEntry is the rule NumberEntry: value=NUMBER.
func numberEntry(p *langium.Parser) (Entry, error) {
	value, err := p.Expect("NUMBER")
	if err != nil {
		return Entry{}, err
	}
	return Entry{Value: langium.ConvertNumber(value.Image)}, nil
}

// detailedEntry is the rule DetailedEntry: axis=[Axis:ID] ':'? value=NUMBER.
func detailedEntry(p *langium.Parser) (Entry, error) {
	ref, err := p.Expect("ID")
	if err != nil {
		return Entry{}, err
	}
	p.Accept(":")
	value, err := p.Expect("NUMBER")
	if err != nil {
		return Entry{}, err
	}
	refText := langium.ConvertID(ref.Image)
	return Entry{Axis: &refText, Value: langium.ConvertNumber(value.Image)}, nil
}

// option is the rule Option:
//
//	name='showLegend' value=BOOLEAN
//	| name='ticks' value=NUMBER
//	| name='max' value=NUMBER
//	| name='min' value=NUMBER
//	| name='graticule' value=GRATICULE
func option(p *langium.Parser) (Option, error) {
	name := p.Next()
	switch name.Type.Name {
	case "showLegend":
		value, err := p.Expect("BOOLEAN")
		if err != nil {
			return Option{}, err
		}
		return Option{Name: name.Image, Value: langium.ConvertBoolean(value.Image)}, nil
	case "ticks", "max", "min":
		value, err := p.Expect("NUMBER")
		if err != nil {
			return Option{}, err
		}
		return Option{Name: name.Image, Value: langium.ConvertNumber(value.Image)}, nil
	case "graticule":
		value, err := p.Expect("GRATICULE")
		if err != nil {
			return Option{}, err
		}
		return Option{Name: name.Image, Value: value.Image}, nil
	}
	return Option{}, p.Unexpected("showLegend", "ticks", "max", "min", "graticule")
}
