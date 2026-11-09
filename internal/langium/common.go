// Ported from mermaid 12.2.0,
// packages/parser/src/language/common/common.langium (the fragment
// TitleAndAccessibilities) and
// packages/mermaid/src/diagrams/common/populateCommonDb.ts (MIT).

package langium

// Common holds the properties that the fragment TitleAndAccessibilities
// assigns. A nil pointer is a property that the text does not set: undefined
// in the AST.
type Common struct {
	AccDescr *string
	AccTitle *string
	Title    *string
}

// AtTitleAndAccessibilities reports whether the next token starts the
// fragment TitleAndAccessibilities.
func (p *Parser) AtTitleAndAccessibilities() bool {
	return p.At("ACC_DESCR", "ACC_TITLE", "TITLE")
}

// TitleAndAccessibilities is the fragment
//
//	((accDescr=ACC_DESCR | accTitle=ACC_TITLE | title=TITLE) EOL)+
//
// It assigns the properties to common. A property that the text sets two
// times keeps the last value.
func (p *Parser) TitleAndAccessibilities(common *Common) error {
	if !p.AtTitleAndAccessibilities() {
		return p.Unexpected("ACC_DESCR", "ACC_TITLE", "TITLE")
	}
	for p.AtTitleAndAccessibilities() {
		t := p.Next()
		var value string
		var ok bool
		var target **string
		switch t.Type.Name {
		case "ACC_DESCR":
			value, ok = ConvertAccDescr(t.Image)
			target = &common.AccDescr
		case "ACC_TITLE":
			value, ok = ConvertAccTitle(t.Image)
			target = &common.AccTitle
		default:
			value, ok = ConvertTitle(t.Image)
			target = &common.Title
		}
		if !ok {
			// The common converter returns undefined and the default
			// converter of Langium then keeps the text of the token.
			value = t.Image
		}
		*target = &value
		if err := p.EOL(); err != nil {
			return err
		}
	}
	return nil
}

// CommonDB is the part of a diagram db that populateCommonDb fills. The
// CommonDB of the render package implements it.
type CommonDB interface {
	SetAccDescription(text string)
	SetAccTitle(text string)
	SetDiagramTitle(text string)
}

// Populate is populateCommonDb(ast, db): it gives each property that is not
// empty to the db.
func (c *Common) Populate(db CommonDB) {
	if c.AccDescr != nil && *c.AccDescr != "" {
		db.SetAccDescription(*c.AccDescr)
	}
	if c.AccTitle != nil && *c.AccTitle != "" {
		db.SetAccTitle(*c.AccTitle)
	}
	if c.Title != nil && *c.Title != "" {
		db.SetDiagramTitle(*c.Title)
	}
}
