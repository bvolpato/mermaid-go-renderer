// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/EndLabelPostprocessor.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package labels

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodespacing/cellsystem"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("EndLabelPostprocessor", func() lgraph.Processor { return NewEndLabelPostprocessor() })
}

// EndLabelPostprocessor is the processor EndLabelPostprocessor: when the
// nodes have their final positions, it moves the label cells that
// EndLabelPreprocessor made by the position of their node and gives the end
// labels their positions.
type EndLabelPostprocessor struct{}

// NewEndLabelPostprocessor is new EndLabelPostprocessor().
func NewEndLabelPostprocessor() *EndLabelPostprocessor { return &EndLabelPostprocessor{} }

// Process is process(layeredGraph, monitor).
func (p *EndLabelPostprocessor) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	// We iterate over each node's label cells and offset and place them
	for _, layer := range layeredGraph.Layers {
		for _, node := range layer.Nodes {
			if (node.Type == lgraph.NodeTypeNormal || node.Type == lgraph.NodeTypeExternalPort) && prop.Has(node, iprops.EndLabels) {
				p.processNode(node)
			}
		}
	}
}

func (*EndLabelPostprocessor) processNode(node *lgraph.LNode) {
	// The node should have a non-empty list of label cells, or something
	// went TERRIBLY WRONG!!!
	endLabelCells := endLabelCellsOf(node)

	nodePos := &node.Pos

	endLabelCells.Each(func(_ *lgraph.LPort, labelCell *cellsystem.LabelCell) bool {
		if labelCell == nil {
			lgraph.Bail("EndLabelPostprocessor: a port without a label cell in the property endLabels")
		}
		labelCellRect := labelCell.CellRectangle()
		labelCellRect.Move(nodePos)

		labelCell.ApplyLabelLayout()
		return true
	})

	// Remove label cells
	prop.Unset(node, iprops.EndLabels)
}
