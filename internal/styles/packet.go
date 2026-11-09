// Ported from mermaid 12.2.0, mermaid/src/diagrams/packet/styles.ts (MIT).

package styles

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// defaultPacketStyleOptions is the constant of the same name.
func defaultPacketStyleOptions() *js.Obj[any] {
	return objOf(
		"byteFontSize", "10px",
		"startByteColor", "black",
		"endByteColor", "black",
		"labelColor", "black",
		"labelFontSize", "12px",
		"titleColor", "black",
		"titleFontSize", "14px",
		"blockStrokeColor", "black",
		"blockStrokeWidth", "1",
		"blockFillColor", "#efefef",
	)
}

// packetStyles reads its options from the packet theme variable.
func packetStyles(in *options) string {
	o := &options{base: cleanAndMerge(defaultPacketStyleOptions(), in.Get("packet"))}

	return `
	.packetByte {
		font-size: ` + o.s("byteFontSize") + `;
	}
	.packetByte.start {
		fill: ` + o.s("startByteColor") + `;
	}
	.packetByte.end {
		fill: ` + o.s("endByteColor") + `;
	}
	.packetLabel {
		fill: ` + o.s("labelColor") + `;
		font-size: ` + o.s("labelFontSize") + `;
	}
	.packetTitle {
		fill: ` + o.s("titleColor") + `;
		font-size: ` + o.s("titleFontSize") + `;
	}
	.packetBlock {
		stroke: ` + o.s("blockStrokeColor") + `;
		stroke-width: ` + o.s("blockStrokeWidth") + `;
		fill: ` + o.s("blockFillColor") + `;
	}
	`
}
