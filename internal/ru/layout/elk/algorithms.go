// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/elk/algorithms.ts (MIT).

package elk

// ELK_ALGORITHMS are the ELK algorithms selectable as `layout: <name>`, in
// addition to the plain `elk` name, which maps to `elk.layered`.
var ELK_ALGORITHMS = []string{
	"elk.stress",
	"elk.force",
	"elk.mrtree",
	"elk.sporeOverlap",
	"elk.box",
	"elk.rectpacking",
}
