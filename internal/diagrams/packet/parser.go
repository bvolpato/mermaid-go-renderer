// Ported from mermaid 12.2.0, diagrams/packet/parser.ts (MIT).

package packet

import (
	"fmt"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

const maxPacketSize = 10_000

// parseDiagram is parser.parse.
func parseDiagram(_ *render.Context, rdb render.DB, text string) error {
	ast, err := parse(text)
	if err != nil {
		return err
	}
	return populate(ast, rdb.(*DB))
}

// populate is populate(ast, db).
func populate(ast *Packet, db *DB) error {
	ast.Populate(db)
	lastBit := -1.0
	word := PacketWord{}
	row := 1.0
	bitsPerRow := db.GetConfig().Get("bitsPerRow")
	// row * bitsPerRow of upstream, with the number conversion of "*".
	rowBits := func() float64 { return row * js.ToNumber(bitsPerRow) }

	for _, astBlock := range ast.Blocks {
		start, end, bits, label := astBlock.Start, astBlock.End, astBlock.Bits, astBlock.Label
		if start != nil && end != nil && *end < *start {
			return fmt.Errorf("Packet block %s - %s is invalid. End must be greater than start.", js.Str(*start), js.Str(*end))
		}
		if start == nil {
			v := lastBit + 1
			start = &v
		}
		if *start != lastBit+1 {
			endOrStart := *start
			if end != nil {
				endOrStart = *end
			}
			return fmt.Errorf("Packet block %s - %s is not contiguous. It should start from %s.",
				js.Str(*start), js.Str(endOrStart), js.Str(lastBit+1))
		}
		if bits != nil && *bits == 0 {
			return fmt.Errorf("Packet block %s is invalid. Cannot have a zero bit field.", js.Str(*start))
		}
		if end == nil {
			one := 1.0
			if bits != nil {
				one = *bits
			}
			v := *start + one - 1
			end = &v
		}
		if bits == nil {
			v := *end - *start + 1
			bits = &v
		}
		lastBit = *end

		block := Block{Start: *start, End: *end, Bits: *bits, Label: label}
		for js.Compare(float64(len(word)), "<=", js.Add(bitsPerRow, 1.0)) && len(db.GetPacket()) < maxPacketSize {
			fitting, nextBlock, err := getNextFittingBlock(block, rowBits())
			if err != nil {
				return err
			}
			word = append(word, fitting)
			if fitting.End+1 == rowBits() {
				db.PushWord(word)
				word = PacketWord{}
				row++
			}
			if nextBlock == nil {
				break
			}
			block = *nextBlock
		}
	}
	db.PushWord(word)
	return nil
}

// getNextFittingBlock is getNextFittingBlock(block, row, bitsPerRow). The
// argument rowBits is row * bitsPerRow.
func getNextFittingBlock(block Block, rowBits float64) (Block, *Block, error) {
	if block.Start > block.End {
		return Block{}, nil, fmt.Errorf("Block start %s is greater than block end %s.", js.Str(block.Start), js.Str(block.End))
	}

	if block.End+1 <= rowBits {
		return block, nil, nil
	}

	rowEnd := rowBits - 1
	rowStart := rowBits
	return Block{
		Start: block.Start,
		End:   rowEnd,
		Label: block.Label,
		Bits:  rowEnd - block.Start,
	}, &Block{
		Start: rowStart,
		End:   block.End,
		Label: block.Label,
		Bits:  block.End - rowStart,
	}, nil
}
