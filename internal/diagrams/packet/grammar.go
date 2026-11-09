// Ported from mermaid 12.2.0,
// packages/parser/src/language/packet/packet.langium (MIT).

package packet

import "github.com/bvolpato/mermaid-go-renderer/internal/langium"

// Packet is the AST of the entry rule Packet.
type Packet struct {
	langium.Common
	Blocks []PacketBlock
}

// PacketBlock is the AST of the rule PacketBlock. A nil pointer is a
// property that the text does not set.
type PacketBlock struct {
	Start *float64
	End   *float64
	Bits  *float64
	Label string
}

var lexer = langium.MustLexer("packet")

// parse is parse('packet', text) of the parser package. The grammar is:
//
//	entry Packet:
//	  NEWLINE*
//	  ("packet"| "packet-beta")
//	  (TitleAndAccessibilities | blocks+=PacketBlock | NEWLINE)*
//	;
//	PacketBlock:
//	  (start=INT('-' end=INT)? | '+' bits=INT)
//	  ':' label=STRING
//	  EOL
//	;
func parse(text string) (*Packet, error) {
	p, err := lexer.Parse(text)
	if err != nil {
		return nil, err
	}
	ast := &Packet{Blocks: []PacketBlock{}}
	p.Skip("NEWLINE")
	if !p.At("packet", "packet-beta") {
		return nil, p.Unexpected("packet", "packet-beta")
	}
	p.Next()
	for {
		switch {
		case p.AtTitleAndAccessibilities():
			if err := p.TitleAndAccessibilities(&ast.Common); err != nil {
				return nil, err
			}
		case p.At("INT", "+"):
			block, err := packetBlock(p)
			if err != nil {
				return nil, err
			}
			ast.Blocks = append(ast.Blocks, block)
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

func packetBlock(p *langium.Parser) (PacketBlock, error) {
	var block PacketBlock
	integer := func() (*float64, error) {
		t, err := p.Expect("INT")
		if err != nil {
			return nil, err
		}
		v := langium.ConvertInt(t.Image)
		return &v, nil
	}
	var err error
	if p.Accept("+") != nil {
		if block.Bits, err = integer(); err != nil {
			return block, err
		}
	} else {
		if block.Start, err = integer(); err != nil {
			return block, err
		}
		if p.Accept("-") != nil {
			if block.End, err = integer(); err != nil {
				return block, err
			}
		}
	}
	if _, err := p.Expect(":"); err != nil {
		return block, err
	}
	label, err := p.Expect("STRING")
	if err != nil {
		return block, err
	}
	block.Label = langium.ConvertString(label.Image)
	return block, p.EOL()
}
