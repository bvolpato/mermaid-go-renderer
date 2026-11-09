// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/InnermostNodeMarginCalculator.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package nodesize

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/adapters"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodespacing"
)

func init() {
	lgraph.Register("InnermostNodeMarginCalculator", func() lgraph.Processor { return NewInnermostNodeMarginCalculator() })
}

// InnermostNodeMarginCalculator is the processor
// InnermostNodeMarginCalculator: it sets the inner node margins.
//
// The margin of a node is the space around it that must not overlap other
// elements. This processor computes the space for the ports, the port
// labels and the node labels. Later processors extend the margins for self
// loops, comment boxes and the end labels of edges.
//
// Before it, the ports have their positions, and the node labels and port
// labels have their positions.
type InnermostNodeMarginCalculator struct{}

// NewInnermostNodeMarginCalculator is new InnermostNodeMarginCalculator().
func NewInnermostNodeMarginCalculator() *InnermostNodeMarginCalculator {
	return &InnermostNodeMarginCalculator{}
}

// Process is process(layeredGraph, monitor).
func (*InnermostNodeMarginCalculator) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	// Calculate the margins using ELK's utility methods. What is not
	// included in the margins yet are is space required for self loops and
	// comment boxes. We will deal with all of those later.
	nodespacing.GetNodeMarginCalculator(adapters.AdaptLGraphTransparent(layeredGraph, false)).
		ExcludeEdgeHeadTailLabels().Process()
}
