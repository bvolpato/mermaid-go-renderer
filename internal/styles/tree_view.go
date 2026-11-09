// Ported from mermaid 12.2.0, mermaid/src/diagrams/treeView/styles.ts (MIT).

package styles

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// defaultTreeViewDiagramStyles is the constant of the same name.
func defaultTreeViewDiagramStyles() *js.Obj[any] {
	return objOf(
		"labelFontSize", "16px",
		"labelColor", "black",
		"lineColor", "black",
		"iconColor", "#546e7a",
		"descriptionColor", "#6a9955",
		"highlightBg", "rgba(255, 193, 7, 0.15)",
		"highlightStroke", "#ffc107",
	)
}

// treeViewStyles reads its options from the treeView theme variable.
func treeViewStyles(in *options) string {
	o := &options{base: cleanAndMerge(defaultTreeViewDiagramStyles(), in.Get("treeView"))}
	return `
    .treeView-node-label {
        font-size: ` + o.s("labelFontSize") + `;
        fill: ` + o.s("labelColor") + `;
        white-space: pre;
    }
    .treeView-node-dir {
        font-weight: bold;
    }
    .treeView-node-line {
        stroke: ` + o.s("lineColor") + `;
    }
    .treeView-node-icon {
        color: ` + o.s("iconColor") + `;
    }
    .treeView-node-description {
        font-size: ` + o.s("labelFontSize") + `;
        fill: ` + o.s("descriptionColor") + `;
        font-style: italic;
        white-space: pre;
    }
    .treeView-highlight-bg {
        fill: ` + o.s("highlightBg") + `;
        stroke: ` + o.s("highlightStroke") + `;
        stroke-width: 1;
    }
    `
}
