// Package fcose is a Go port of the fcose layout as mermaid 12.2.0 runs it
// for an architecture diagram: cytoscape-fcose 2.2.0 with cose-base 2.2.0
// and layout-base 2.0.1 (all MIT license; the singular value decomposition
// of layout-base is from JamaJS, Apache License 2.0).
//
// The port covers the options of mermaid: quality proof, constraints (an
// alignment constraint and a relative placement constraint, no fixed nodes),
// compound nodes, and no animation. The constraints switch the tiling and
// the packing of components off, and the layout is incremental: it starts
// from the positions of the nodes, or from the spectral layout with the
// option randomize. For another path Run reports the reason, and the caller
// runs the JavaScript.
//
// This file is ported from cytoscape-fcose 2.2.0 (MIT license):
// src/fcose/index.js, src/fcose/cose.js and src/fcose/auxiliary.js.
package fcose

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native/internal/bail"
)

// NodeInput is a node of cytoscape as the extension reads it.
type NodeInput struct {
	ID string
	// X and Y are position('x') and position('y') of the node, and W and H
	// its layoutDimensions().
	X, Y, W, H float64
	// Padding is parseInt(css('padding')).
	Padding float64
	// Parent is the node that contains the node, or nil. Children are
	// children() of the node, in order. A node with children is a compound
	// node.
	Parent   *NodeInput
	Children []*NodeInput

	// The spectral layout of the option randomize also reads these.

	// Neighbors are neighborhood().nodes(): the nodes at the other end of
	// each edge of the node, each one time, in the order of the edges.
	Neighbors []*NodeInput
	// Degree is connectedEdges().length.
	Degree int
	// Width and Height are width() and height() of the node.
	Width, Height float64
}

// EdgeInput is an edge of cytoscape with the values of the options
// idealEdgeLength and edgeElasticity for it.
type EdgeInput struct {
	ID, Source, Target string
	IdealLength        float64
	Elasticity         float64
}

// Position is the centre of a node after the layout.
type Position struct {
	X, Y float64
}

// Options are the options of a run, and its observers.
type Options struct {
	// Randomize is options.randomize: the layout starts from the spectral
	// layout and not from the positions of the nodes.
	Randomize bool
	// NumIter is options.numIter and NodeSeparation options.nodeSeparation.
	NumIter        float64
	NodeSeparation float64
	// Constraints are the constraints after the validity check of fcose.
	Constraints Constraints
	// Center is the centre of the bounding box of the elements before the
	// layout: the layout moves its result there.
	Center Position
	// Random is Math.random.
	Random func() float64
	// Trace receives the state of the layout at the points that the oracle
	// records. nil for none.
	Trace func(stage string, values []float64)
	// Cancelled tells that the caller does not wait for the result. The
	// layout then stops with a reason.
	Cancelled func() bool
}

// maxNodes bounds the graphs that the port lays out.
const maxNodes = 5000

// Run is Layout.run of fcose up to the positions that it gives to cytoscape:
// the centre of each node by its id. nodes are eles.nodes(), in order.
//
// reason is not empty when the port does not cover the graph.
func Run(nodes []*NodeInput, edges []EdgeInput, opts Options) (positions map[string]Position, reason string) {
	defer bail.Recover(&reason)

	if !opts.Constraints.HasAlignment {
		// Without a constraint the JavaScript tiles nodes, packs components
		// and reduces trees.
		bail.Out("fcose: a layout without an alignment constraint")
	}
	if !(opts.NumIter >= 0) || math.IsInf(opts.NumIter, 0) {
		bail.Out("fcose: numIter is not a finite number")
	}
	if len(nodes) > maxNodes {
		bail.Out("fcose: more than %d nodes", maxNodes)
	}

	// apply spectral layout
	var spectral *spectralResult
	if opts.Randomize {
		spectral = spectralLayout(nodes, &opts)
	}

	// apply cose layout as postprocessing
	idToLNode := coseLayout(getTopMostNodes(nodes), edges, &opts, spectral)
	// relocate center to original position
	relocateComponent(opts.Center, idToLNode)

	// getPositions: the centre of the rectangle of each layout node.
	positions = make(map[string]Position, len(idToLNode.order))
	for _, node := range idToLNode.order {
		positions[node.id] = Position{X: node.getRect().getCenterX(), Y: node.getRect().getCenterY()}
	}
	return positions, ""
}

// nodeTable is the object idToLNode of the JavaScript: the layout node of
// each id. order has the nodes in the order of their creation.
type nodeTable struct {
	byID  map[string]*Node
	order []*Node
}

// coseLayout is coseLayout of src/fcose/cose.js: it transfers the nodes, the
// edges and the constraints of cytoscape to a CoSELayout and runs it.
func coseLayout(top []*NodeInput, edges []EdgeInput, opts *Options, spectral *spectralResult) *nodeTable {
	idToLNode := &nodeTable{byID: map[string]*Node{}}

	p := params{
		maxIterations:                 opts.NumIter,
		transformOnConstraintHandling: opts.Randomize,
	}
	coseLayout := newLayout(p)
	coseLayout.trace = opts.Trace
	coseLayout.cancelled = opts.Cancelled
	coseLayout.random = opts.Random
	gm := coseLayout.newGraphManagerForRun()

	// transfer cytoscape nodes to cose nodes
	count := 0
	var processChildrenList func(parent *Graph, children []*NodeInput)
	processChildrenList = func(parent *Graph, children []*NodeInput) {
		for _, theChild := range children {
			count++
			if count > maxNodes {
				bail.Out("fcose: more than %d nodes", maxNodes)
			}
			w, h := theChild.W, theChild.H
			// parseFloat of a number: the sign of a zero goes away.
			if w == 0 {
				w = 0
			}
			if h == 0 {
				h = 0
			}
			loc := PointD{theChild.X - float64(theChild.W/2), theChild.Y - float64(theChild.H/2)}
			if spectral != nil {
				if len(theChild.Children) == 0 {
					// xCoords[nodeIndexes.get(theChild.id())]: undefined is
					// NaN in the difference.
					x, y := nan, nan
					if i, ok := spectral.nodeIndexes[theChild.ID]; ok && i < len(spectral.xCoords) {
						x, y = spectral.xCoords[i], spectral.yCoords[i]
					}
					loc = PointD{x - float64(theChild.W/2), y - float64(theChild.H/2)}
				} else {
					loc.x, loc.y, w, h = spectralBoundingBox(theChild, spectral)
				}
			}
			if !(w >= 0 && h >= 0) || math.IsInf(w, 0) || math.IsInf(h, 0) || math.IsInf(loc.x, 0) || math.IsInf(loc.y, 0) {
				bail.Out("fcose: a node with a size or a position that is not a finite number")
			}
			theNode := parent.addNode(newNode(coseLayout.graphManager, loc, PointD{w, h}))
			// Attach id to the layout node and repulsion value
			theNode.id = theChild.ID
			theNode.hasID = true
			theNode.nodeRepulsion = defaultRepulsionStrength
			// Attach the paddings of cy node to layout node
			theNode.paddingLeft = theChild.Padding
			theNode.hasPadding = true

			// Map the layout node
			if _, dup := idToLNode.byID[theChild.ID]; dup {
				bail.Out("fcose: two nodes with the same id")
			}
			idToLNode.byID[theChild.ID] = theNode
			idToLNode.order = append(idToLNode.order, theNode)

			if math.IsNaN(theNode.rect.x) {
				theNode.rect.x = 0
			}

			if math.IsNaN(theNode.rect.y) {
				theNode.rect.y = 0
			}

			if len(theChild.Children) > 0 {
				theNewGraph := coseLayout.getGraphManager().addGraph(coseLayout.newGraph(), theNode)
				processChildrenList(theNewGraph, theChild.Children)
			}
		}
	}

	// transfer cytoscape edges to cose edges
	processEdges := func() {
		idealLengthTotal := 0.0
		edgeCount := 0
		for _, edge := range edges {
			sourceNode := idToLNode.byID[edge.Source]
			targetNode := idToLNode.byID[edge.Target]
			if sourceNode != nil && targetNode != nil && sourceNode != targetNode && len(sourceNode.getEdgesBetween(targetNode)) == 0 {
				e1 := gm.addEdge(coseLayout.newEdge(), sourceNode, targetNode)
				e1.id = edge.ID
				e1.idealLength = edge.IdealLength
				e1.edgeElasticity = edge.Elasticity
				idealLengthTotal += e1.idealLength
				edgeCount++
			}
		}
		// we need to update the ideal edge length constant with the avg.
		// ideal length value after processing edges. in case there is no
		// edge, the option of mermaid is a function, so the value is 50.
		if edgeCount > 0 {
			coseLayout.params.defaultEdgeLength = idealLengthTotal / float64(edgeCount)
		} else {
			coseLayout.params.defaultEdgeLength = 50
		}
		// we need to update these constant values based on the ideal edge length constant
		coseLayout.params.minRepulsionDist = coseLayout.params.defaultEdgeLength / 10.0
	}

	processChildrenList(gm.addRoot(), top)
	processEdges()
	// transfer cytoscape constraints to cose layout
	coseLayout.constraints = opts.Constraints

	if opts.Trace != nil {
		opts.Trace("cose.graph", coseLayout.traceGraph())
	}

	coseLayout.runLayout()

	if opts.Trace != nil {
		opts.Trace("cose.final", coseLayout.traceRects())
	}

	return idToLNode
}

// relocateComponent is aux.relocateComponent for a layout of the quality
// proof: it moves the result so that the centre of its bounding box is
// originalCenter.
func relocateComponent(originalCenter Position, componentResult *nodeTable) {
	minXCoord := math.Inf(1)
	maxXCoord := math.Inf(-1)
	minYCoord := math.Inf(1)
	maxYCoord := math.Inf(-1)
	// calculate current bounding box
	for _, node := range componentResult.order {
		leftX := node.getRect().x
		rightX := node.getRect().x + node.getRect().width
		topY := node.getRect().y
		bottomY := node.getRect().y + node.getRect().height

		if leftX < minXCoord {
			minXCoord = leftX
		}
		if rightX > maxXCoord {
			maxXCoord = rightX
		}
		if topY < minYCoord {
			minYCoord = topY
		}
		if bottomY > maxYCoord {
			maxYCoord = bottomY
		}
	}
	// find difference between current and original center
	diffOnX := originalCenter.X - float64((maxXCoord+minXCoord)/2)
	diffOnY := originalCenter.Y - float64((maxYCoord+minYCoord)/2)
	// move component to original center
	for _, node := range componentResult.order {
		node.setCenter(node.getCenterX()+diffOnX, node.getCenterY()+diffOnY)
	}
}
