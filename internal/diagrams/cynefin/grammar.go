// Ported from mermaid 12.2.0,
// packages/parser/src/language/cynefin/cynefin.langium (MIT).

package cynefin

import "github.com/bvolpato/mermaid-go-renderer/internal/langium"

// Cynefin is the AST of the entry rule Cynefin.
type Cynefin struct {
	langium.Common
	Domains     []DomainBlock
	Transitions []Transition
}

// DomainBlock is the AST of the rule DomainBlock.
type DomainBlock struct {
	Domain string
	Items  []DomainItem
}

// DomainItem is the AST of the rule DomainItem.
type DomainItem struct {
	Label string
}

// Transition is the AST of the rule Transition. Label is nil when the text
// has no label: undefined in the AST.
type Transition struct {
	From  string
	To    string
	Label *string
}

var lexer = langium.MustLexer("cynefin")

// parse is parse('cynefin', text) of the parser package. The grammar is:
//
//	entry Cynefin:
//	  NEWLINE*
//	  ('cynefin-beta' | 'cynefin-beta:')
//	  NEWLINE*
//	  (TitleAndAccessibilities | domains+=DomainBlock | transitions+=Transition | NEWLINE)*
//	;
//	DomainBlock:
//	  domain=DOMAIN_NAME NEWLINE* (items+=DomainItem NEWLINE*)*
//	;
//	DomainItem:
//	  label=STRING
//	;
//	Transition:
//	  from=DOMAIN_NAME '-->' to=DOMAIN_NAME (':' label=STRING)? EOL
//	;
func parse(text string) (*Cynefin, error) {
	p, err := lexer.Parse(text)
	if err != nil {
		return nil, err
	}
	ast := &Cynefin{Domains: []DomainBlock{}, Transitions: []Transition{}}
	p.Skip("NEWLINE")
	switch {
	case p.At("cynefin-beta", "cynefin-beta:"):
		p.Next()
	default:
		return nil, p.Unexpected("cynefin-beta", "cynefin-beta:")
	}
	p.Skip("NEWLINE")
	for {
		switch {
		case p.AtTitleAndAccessibilities():
			if err := p.TitleAndAccessibilities(&ast.Common); err != nil {
				return nil, err
			}
		case p.At("DOMAIN_NAME"):
			// A domain block and a transition start with the same token.
			// The real parser reads ahead to the end of the transition. A
			// domain name in front of "-->" that starts no complete
			// transition is an error in both rules, because nothing takes
			// "-->" after a domain block.
			if p.LA(2).Type.Name == "-->" {
				transition, err := transition(p)
				if err != nil {
					return nil, err
				}
				ast.Transitions = append(ast.Transitions, transition)
			} else {
				ast.Domains = append(ast.Domains, domainBlock(p))
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

// domainBlock is the rule DomainBlock. The next token is a DOMAIN_NAME.
func domainBlock(p *langium.Parser) DomainBlock {
	block := DomainBlock{Domain: p.Next().Image, Items: []DomainItem{}}
	p.Skip("NEWLINE")
	for p.At("STRING") {
		block.Items = append(block.Items, DomainItem{Label: langium.ConvertString(p.Next().Image)})
		p.Skip("NEWLINE")
	}
	return block
}

// transition is the rule Transition. The next tokens are a DOMAIN_NAME and
// "-->".
func transition(p *langium.Parser) (Transition, error) {
	t := Transition{From: p.Next().Image}
	p.Next()
	to, err := p.Expect("DOMAIN_NAME")
	if err != nil {
		return t, err
	}
	t.To = to.Image
	if p.Accept(":") != nil {
		label, err := p.Expect("STRING")
		if err != nil {
			return t, err
		}
		value := langium.ConvertString(label.Image)
		t.Label = &value
	}
	return t, p.EOL()
}
