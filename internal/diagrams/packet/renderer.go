// Ported from mermaid 12.2.0, diagrams/packet/renderer.ts (MIT).

package packet

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// draw is draw(_text, id, _version, diagram).
func draw(ctx *render.Context, rdb render.DB, _ string) error {
	db := rdb.(*DB)
	config := db.GetConfig()
	rowHeight, paddingY, bitWidth, bitsPerRow := config.Get("rowHeight"), config.Get("paddingY"), config.Get("bitWidth"), config.Get("bitsPerRow")
	words := db.GetPacket()
	title := db.GetDiagramTitle()
	totalRowHeight := js.ToNumber(js.Add(rowHeight, paddingY))
	titleSpace := js.ToNumber(rowHeight)
	if title != "" {
		titleSpace = 0
	}
	svgHeight := float64(totalRowHeight*float64(len(words)+1)) - titleSpace
	svgWidth := float64(js.ToNumber(bitWidth)*js.ToNumber(bitsPerRow)) + 2
	svg := ctx.SelectSvgElement()

	svg.Attr("viewBox", "0 0 "+js.Str(svgWidth)+" "+js.Str(svgHeight))
	render.ConfigureSvgSize(svg, svgHeight, svgWidth, js.Truthy(config.Get("useMaxWidth")))

	for word, packet := range words {
		drawWord(svg, packet, float64(word), config)
	}

	svg.Append("text").
		Text(title).
		Attr("x", svgWidth/2).
		Attr("y", svgHeight-float64(totalRowHeight/2)).
		Attr("dominant-baseline", "middle").
		Attr("text-anchor", "middle").
		Attr("class", "packetTitle")
	return nil
}

// drawWord is drawWord(svg, word, rowNumber, config).
func drawWord(svg *dom.Node, word PacketWord, rowNumber float64, config *js.Obj[any]) {
	rowHeight := config.Get("rowHeight")
	paddingX := js.ToNumber(config.Get("paddingX"))
	paddingY := config.Get("paddingY")
	bitWidth := js.ToNumber(config.Get("bitWidth"))
	bitsPerRow := js.ToNumber(config.Get("bitsPerRow"))
	showBits := js.Truthy(config.Get("showBits"))
	bitOrder := config.Get("bitOrder")

	group := svg.Append("g")
	wordY := js.ToNumber(js.Add(float64(rowNumber*js.ToNumber(js.Add(rowHeight, paddingY))), paddingY))
	descending := js.StrictEquals(bitOrder, "descending")
	for _, block := range word {
		bits := block.End - block.Start + 1
		// Column the block starts at within its row. `descending` mirrors the row, so the row
		// reads from its highest bit down to its lowest, as hardware registers are drawn.
		firstColumn := math.Mod(block.Start, bitsPerRow)
		column := firstColumn
		if descending {
			column = bitsPerRow - firstColumn - bits
		}
		blockX := float64(column*bitWidth) + 1
		width := float64(bits*bitWidth) - paddingX
		// Block rectangle
		group.Append("rect").
			Attr("x", blockX).
			Attr("y", wordY).
			Attr("width", width).
			Attr("height", rowHeight).
			Attr("class", "packetBlock")

		// Block label
		group.Append("text").
			Attr("x", blockX+float64(width/2)).
			Attr("y", wordY+float64(js.ToNumber(rowHeight)/2)).
			Attr("class", "packetLabel").
			Attr("dominant-baseline", "middle").
			Attr("text-anchor", "middle").
			Text(block.Label)

		if !showBits {
			continue
		}
		// A mirrored block runs from its highest bit on the left down to its lowest on the right.
		leadingBit, trailingBit := block.Start, block.End
		if descending {
			leadingBit, trailingBit = block.End, block.Start
		}

		// Start byte count
		isSingleBlock := bits == 1
		bitNumberY := wordY - 2
		startX, startAnchor := blockX+0, "start"
		if isSingleBlock {
			startX, startAnchor = blockX+float64(width/2), "middle"
		}
		group.Append("text").
			Attr("x", startX).
			Attr("y", bitNumberY).
			Attr("class", "packetByte start").
			Attr("dominant-baseline", "auto").
			Attr("text-anchor", startAnchor).
			Text(js.Str(leadingBit))

		// Draw end byte count if it is not the same as start byte count
		if !isSingleBlock {
			group.Append("text").
				Attr("x", blockX+width).
				Attr("y", bitNumberY).
				Attr("class", "packetByte end").
				Attr("dominant-baseline", "auto").
				Attr("text-anchor", "end").
				Text(js.Str(trailingBit))
		}
	}
}
