// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/SortByInputModelProcessor.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package preserveorder

import (
	"runtime"
	"strconv"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("SortByInputModelProcessor", func() lgraph.Processor { return NewSortByInputModelProcessor() })
}

// SortByInputModelProcessor is the processor SortByInputModelProcessor: it
// sorts the nodes of each layer and the ports of each node by their model
// order, the order in the input graph. Outgoing ports are sorted by the
// order of their edges. Incoming ports are sorted by the order of their
// nodes in the previous layer.
type SortByInputModelProcessor struct{}

// NewSortByInputModelProcessor is new SortByInputModelProcessor().
func NewSortByInputModelProcessor() *SortByInputModelProcessor { return &SortByInputModelProcessor{} }

// Process is process(graph, progressMonitor).
func (*SortByInputModelProcessor) Process(graph *lgraph.LGraph, ctx *lgraph.Context) {
	defer BailOnPanic("SortByInputModelProcessor")
	// The Java code makes the title of the progress monitor with the value
	// of this option. The option has no cloneable default, so the read
	// leaves no trace.
	layerIndex := 0
	for _, layer := range graph.Layers {
		previousLayerIndex := 0
		if layerIndex != 0 {
			previousLayerIndex = layerIndex - 1
		}
		previousLayer := graph.Layers[previousLayerIndex]
		for _, node := range layer.Nodes {
			if prop.Get(node, lopts.PortConstraints) != core.PortConstraintsFixedOrder &&
				prop.Get(node, lopts.PortConstraints) != core.PortConstraintsFixedPos {
				// Special case: If two edges (of the same node) have the
				// same target node they should be next to each other.
				// Therefore all ports that connect to the same node should
				// have the same (their minimal) model order. Get minimal
				// model order for target node
				cmp := NewModelOrderPortComparator(previousLayer,
					prop.Get(graph, lopts.ConsiderModelOrderStrategy),
					LongEdgeTargetNodePreprocessing(node),
					prop.Get(graph, lopts.ConsiderModelOrderPortModelOrder))
				gwt.SortFunc(node.Ports, cmp.Compare)
			}
		}
		// Sort nodes.
		cmp := NewModelOrderNodeComparator(previousLayer,
			prop.Get(graph, lopts.ConsiderModelOrderStrategy),
			prop.Get(graph, lopts.ConsiderModelOrderLongEdgeStrategy))
		gwt.SortFunc(layer.Nodes, cmp.Compare)
		layerIndex++
	}
}

// LongEdgeTargetNodePreprocessing is
// SortByInputModelProcessor.longEdgeTargetNodePreprocessing(node): it
// calculates the long edge target of each port with outgoing edges, saves it
// on the port, and saves the map of the long edge targets of the node on the
// node. It returns that map.
func LongEdgeTargetNodePreprocessing(node *lgraph.LNode) *gwt.Map[*lgraph.LNode, int] {
	targetNodeModelOrder := gwt.NewMap[*lgraph.LNode, int]()
	if prop.Has(node, iprops.TargetNodeModelOrder) {
		return prop.Get(node, iprops.TargetNodeModelOrder)
	}
	for _, p := range node.Ports {
		if len(p.OutgoingEdges) == 0 {
			continue
		}
		targetNode := GetTargetNode(p)
		prop.Set(p, iprops.LongEdgeTargetNode, targetNode)
		if targetNode != nil {
			previousOrder := 2147483647
			if targetNodeModelOrder.ContainsKey(targetNode) {
				previousOrder = targetNodeModelOrder.Get(targetNode)
			}
			edge := p.OutgoingEdges[0]
			if !prop.Get(edge, iprops.Reversed) {
				targetNodeModelOrder.Put(targetNode, min(modelOrder(edge), previousOrder))
			}
		}
	}
	prop.Set(node, iprops.TargetNodeModelOrder, targetNodeModelOrder)
	return targetNodeModelOrder
}

// GetTargetNode is SortByInputModelProcessor.getTargetNode(port): the target
// node of the first outgoing edge of the port, through long edge dummy
// nodes. It returns nil when there is none.
func GetTargetNode(port *lgraph.LPort) *lgraph.LNode {
	var node *lgraph.LNode
	edge := port.OutgoingEdges[0]
	for {
		node = edge.Target.Node()
		// If the dummy node has a target return it.
		if prop.Has(node, iprops.LongEdgeTarget) {
			return prop.Get(node, iprops.LongEdgeTarget).Node()
		}
		// If not the current node might be the target node or one has to
		// iterate manually through to it.
		if node.Type != lgraph.NodeTypeNormal {
			outgoing := firstOutgoingEdge(node)
			if outgoing == nil {
				return nil
			}
			edge = outgoing
		}
		if node.Type == lgraph.NodeTypeNormal {
			break
		}
	}
	return node
}

// firstOutgoingEdge returns the first edge of getOutgoingEdges(), or nil.
func firstOutgoingEdge(node *lgraph.LNode) *lgraph.LEdge {
	for _, p := range node.Ports {
		if len(p.OutgoingEdges) != 0 {
			return p.OutgoingEdges[0]
		}
	}
	return nil
}

// BailOnPanic turns a run-time panic of a ported method into lgraph.Bail.
// Call it with defer at the start of a processor.
//
// A Go run-time panic (a nil pointer, an index out of range) stands for a
// state where the Java code throws, or where the JavaScript of elkjs goes on
// with undefined values. The port does not follow elkjs there, so the layout
// must fall back.
//
// TODO(core): move to lgraph. The crossing minimization packages use it.
func BailOnPanic(processor string) {
	r := recover()
	if r == nil {
		return
	}
	if b, ok := r.(*lgraph.BailError); ok {
		panic(b)
	}
	lgraph.Bail("%s: %v%s", processor, r, panicSite())
}

// panicSite names the function that panicked, for the reason of a Bail.
func panicSite() string {
	pcs := make([]uintptr, 32)
	n := runtime.Callers(2, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	afterPanic := false
	for {
		frame, more := frames.Next()
		if strings.HasPrefix(frame.Function, "runtime.") {
			afterPanic = true
		} else if afterPanic {
			return " (" + frame.Function + ":" + strconv.Itoa(frame.Line) + ")"
		}
		if !more {
			return ""
		}
	}
}
