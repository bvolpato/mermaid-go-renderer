// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p2layers/CoffmanGrahamLayerer.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package p2layers

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("CoffmanGrahamLayerer", func() lgraph.Processor { return NewCoffmanGrahamLayerer() })
}

// integer is a java.lang.Integer object. compareNodesInTopo compares two of
// them by identity, as the Java code does.
type integer struct {
	value int
}

// CoffmanGrahamLayerer is the phase CoffmanGrahamLayerer: a layering with a
// bound on the number of nodes in a layer.
type CoffmanGrahamLayerer struct {
	nodeMark []bool
	edgeMark []bool

	inDeg  []int
	outDeg []int

	topoOrd []int

	// inTopo is the ArrayListMultimap<LNode, Integer> of the Java class.
	inTopo map[*lgraph.LNode][]*integer

	// boxedValues is the cache of Integer.valueOf: the values from -128 to
	// 127 have one object each.
	boxedValues [256]*integer

	// depth is the depth of the recursion of dfs. It is not in the Java
	// class: it finds a cycle, where the JavaScript code fails with a stack
	// overflow.
	depth int
}

// NewCoffmanGrahamLayerer is new CoffmanGrahamLayerer().
func NewCoffmanGrahamLayerer() *CoffmanGrahamLayerer { return &CoffmanGrahamLayerer{} }

// valueOf is Integer.valueOf(i).
func (l *CoffmanGrahamLayerer) valueOf(i int) *integer {
	if i > -129 && i < 128 {
		rebase := i + 128
		result := l.boxedValues[rebase]
		if result == nil {
			result = &integer{i}
			l.boxedValues[rebase] = result
		}
		return result
	}
	return &integer{i}
}

// Process is process(layeredGraph, progressMonitor).
func (l *CoffmanGrahamLayerer) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	if len(layeredGraph.LayerlessNodes) == 0 {
		return
	}

	// the maximum number of allowed nodes per layer
	w := prop.Get(layeredGraph, lopts.LayeringCoffmanGrahamLayerBound)

	// initialization, assign indexes and initialize arrays
	index := 0
	edgeIndex := 0
	for _, n := range layeredGraph.LayerlessNodes {
		n.ID = index
		index++
		for _, port := range n.Ports {
			for _, e := range port.OutgoingEdges {
				e.ID = edgeIndex
				edgeIndex++
			}
		}
	}
	l.nodeMark = make([]bool, index)
	l.edgeMark = make([]bool, edgeIndex)
	l.inDeg = make([]int, index)
	l.outDeg = make([]int, index)
	l.topoOrd = make([]int, index)
	l.inTopo = map[*lgraph.LNode][]*integer{}

	// The ids are indices now. An edge to or from a node that is not a
	// layerless node of the graph would read the arrays at the id of that
	// node, or at the id of an edge without a number.
	for _, n := range layeredGraph.LayerlessNodes {
		for _, port := range n.Ports {
			for _, e := range port.OutgoingEdges {
				l.checkEndpoint(layeredGraph.LayerlessNodes, e.Target)
			}
			for _, e := range port.IncomingEdges {
				l.checkEndpoint(layeredGraph.LayerlessNodes, e.Source)
				if e.ID < 0 || e.ID >= edgeIndex {
					lgraph.Bail("CoffmanGrahamLayerer: an incoming edge is not an outgoing edge of its source")
				}
			}
		}
	}

	// --------------------------
	// #1 Remove transitive edges
	// --------------------------
	l.transitiveReduction(layeredGraph)

	// -------------------------------
	// #2 Compute topological ordering
	// -------------------------------
	sources := gwt.NewPriorityQueue(l.compareNodesInTopo)

	// for each node, determine its current in-degree and remember initial
	// sources
	for _, v := range layeredGraph.LayerlessNodes {
		for _, port := range v.Ports {
			for _, e := range port.IncomingEdges {
				if !l.edgeMark[e.ID] {
					l.inDeg[v.ID]++
				}
			}
		}
		if l.inDeg[v.ID] == 0 {
			sources.Offer(v)
		}
	}

	// compute topological ordering
	i := 0
	for !sources.IsEmpty() {
		v, _ := sources.Poll()
		// assign number of topological order
		l.topoOrd[v.ID] = i
		i++

		// update the rest of the graph
		for _, port := range v.Ports {
			for _, e := range port.OutgoingEdges {
				if l.edgeMark[e.ID] {
					continue
				}
				tgt := e.Target.Node()
				l.inDeg[tgt.ID]--
				l.inTopo[tgt] = append(l.inTopo[tgt], l.valueOf(l.topoOrd[v.ID]))
				if l.inDeg[tgt.ID] == 0 {
					sources.Offer(tgt) // 'tgt' is added according to its priority
				}
			}
		}
	}

	// --------------------------
	// #3 Actual layer assignment
	// --------------------------
	// note that this time we start with sinks and work our way to the
	// original graph's sources

	// set of candidate nodes, that is nodes for which all successors have
	// been placed highest priority first
	sinks := gwt.NewPriorityQueue(func(n1, n2 *lgraph.LNode) int {
		return -gwt.IntCompare(l.topoOrd[n1.ID], l.topoOrd[n2.ID])
	})
	for _, v := range layeredGraph.LayerlessNodes {
		for _, port := range v.Ports {
			for _, e := range port.OutgoingEdges {
				if !l.edgeMark[e.ID] {
					l.outDeg[v.ID]++
				}
			}
		}
		if l.outDeg[v.ID] == 0 {
			sinks.Offer(v)
		}
	}

	// assign the layers
	var layers []*lgraph.Layer
	currentLayer := createLayer(layeredGraph, &layers)
	for !sinks.IsEmpty() {
		// select a node for which all outgoing nodes have been placed,
		// and with maximum value in the topological sort
		u, _ := sinks.Poll()

		// start a new layer if the current one is already full,
		// or an in-layer edge would be introduced
		if isLayerFull(currentLayer, w) || !canAdd(u, currentLayer) {
			currentLayer = createLayer(layeredGraph, &layers)
		}

		// place the node in the layer
		u.SetLayer(currentLayer)

		// update out-degrees and collect the new sinks
		for _, port := range u.Ports {
			for _, e := range port.IncomingEdges {
				if l.edgeMark[e.ID] {
					continue
				}
				src := e.Source.Node()
				l.outDeg[src.ID]--
				if l.outDeg[src.ID] == 0 {
					sinks.Offer(src)
				}
			}
		}
	}

	// the layers were created in inverse order
	for j := len(layers) - 1; j >= 0; j-- {
		layeredGraph.Layers = append(layeredGraph.Layers, layers[j])
	}

	// clear layerless nodes
	clear(layeredGraph.LayerlessNodes)
	layeredGraph.LayerlessNodes = layeredGraph.LayerlessNodes[:0]
}

// checkEndpoint stops the layout when the node of the port is not the
// layerless node with its id.
func (l *CoffmanGrahamLayerer) checkEndpoint(nodes []*lgraph.LNode, port *lgraph.LPort) {
	if port == nil || port.Node() == nil {
		lgraph.Bail("CoffmanGrahamLayerer: an edge has no node at one end")
	}
	n := port.Node()
	if n.ID < 0 || n.ID >= len(nodes) || nodes[n.ID] != n {
		lgraph.Bail("CoffmanGrahamLayerer: an edge leads to a node that is not a layerless node of the graph")
	}
}

func isLayerFull(layer *lgraph.Layer, w int) bool {
	return len(layer.Nodes) >= w
}

func canAdd(n *lgraph.LNode, l *lgraph.Layer) bool {
	for _, port := range n.Ports {
		for _, e := range port.OutgoingEdges {
			v := e.Target.Node()
			if v.Layer() == l {
				return false
			}
		}
	}
	return true
}

func createLayer(graph *lgraph.LGraph, layers *[]*lgraph.Layer) *lgraph.Layer {
	aLayer := lgraph.NewLayer(graph)
	*layers = append(*layers, aLayer)
	return aLayer
}

// compareNodesInTopo compares two nodes by the numbers of their
// predecessors in the topological order, from the last number to the first.
//
// The Java code compares two Integer objects with !=, which compares the
// objects and not the numbers. Two equal numbers from 128 are two objects,
// so the comparison stops at them with the result 0. The method also tells
// with hasNext that a list ended, and hasNext is true for each list that the
// loop read a number of.
func (l *CoffmanGrahamLayerer) compareNodesInTopo(u, v *lgraph.LNode) int {
	inListU := l.inTopo[u]
	inLsitV := l.inTopo[v]
	itU := len(inListU)
	itV := len(inLsitV)

	// find the node with the lower associated maximum value in 'inTopo'
	// break ties by ignoring all equal maxima
	for itU > 0 && itV > 0 {
		itU--
		itV--
		iu := inListU[itU]
		iv := inLsitV[itV]
		if iu != iv {
			return gwt.IntCompare(iu.value, iv.value)
		}
	}

	itUHasNext := itU < len(inListU)
	itVHasNext := itV < len(inLsitV)
	if !itUHasNext && !itVHasNext {
		return 0 // u == v
	} else if !itUHasNext {
		return -1 // u < v
	}
	return 1 // u > v
}

func (l *CoffmanGrahamLayerer) transitiveReduction(graph *lgraph.LGraph) {
	for _, start := range graph.LayerlessNodes {
		clear(l.nodeMark)
		for _, port := range start.Ports {
			for _, out := range port.OutgoingEdges {
				l.depth = 0
				l.dfs(start, out.Target.Node())
			}
		}
	}
}

func (l *CoffmanGrahamLayerer) dfs(start, v *lgraph.LNode) {
	if l.nodeMark[v.ID] {
		return
	}
	if l.depth > len(l.nodeMark) {
		lgraph.Bail("CoffmanGrahamLayerer: the graph has a cycle")
	}
	l.depth++

	for _, port := range v.Ports {
		for _, out := range port.OutgoingEdges {
			w := out.Target.Node()
			for _, wport := range w.Ports {
				for _, transitive := range wport.IncomingEdges {
					if transitive.Source.Node() == start {
						l.edgeMark[transitive.ID] = true
					}
				}
			}
			l.dfs(start, w)
		}
	}

	l.depth--
	l.nodeMark[v.ID] = true
}
