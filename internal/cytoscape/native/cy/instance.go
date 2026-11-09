// The elements of a cytoscape instance, and the state of each element that
// the renderer and the bounding box functions keep. Ported from cytoscape
// 3.34.0 (MIT license): src/collection/element.mjs (the private state),
// src/collection/index.mjs (restore), src/collection/compounds.mjs and
// src/core/add-remove.mjs.

package cy

import "github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native/internal/bail"

// NodeStyle has the values of the style of a node that the geometry reads.
// The values of the caller are the result of the style sheet of the instance
// for the node: the package has no selector engine.
type NodeStyle struct {
	// Width and Height are pstyle('width').pfValue and
	// pstyle('height').pfValue.
	Width, Height float64
	// Padding is pstyle('padding').pfValue when the node has no children,
	// and ParentPadding when it has: the default style sheet of cytoscape
	// has the rule `:parent { padding: 10 }`.
	Padding, ParentPadding float64
	// Label is pstyle('label').strValue. Empty for none.
	Label string
	// TextValign is pstyle('text-valign').strValue: "top" or "bottom".
	TextValign string
	// FontSize is pstyle('font-size').pfValue.
	FontSize float64
}

// Endpoint is the parsed value of the style property source-endpoint or
// target-endpoint with two numbers, for example "50% 100%": the pfValue of
// each number, and whether its unit is %.
type Endpoint struct {
	PfValue [2]float64
	Percent [2]bool
}

// EdgeStyle has the values of the style of an edge that the geometry reads.
type EdgeStyle struct {
	// Segments tells that the curve style is segments, with the edge
	// distances endpoints. Without it the curve style is straight.
	Segments bool
	// SourceEndpoint and TargetEndpoint are the manual endpoints.
	SourceEndpoint, TargetEndpoint Endpoint
	// SegmentWeights and SegmentDistances are the pfValue lists of the two
	// properties.
	SegmentWeights, SegmentDistances []float64
	// Label is pstyle('label').strValue. Empty for none.
	Label string
	// Width is pstyle('width').pfValue, and FontSize
	// pstyle('font-size').pfValue.
	Width, FontSize float64
}

// labelBounds is _private.labelBounds: the members that the bounding box
// functions read.
type labelBounds struct {
	all *BB
}

// renderedStyle is _private.rstyle: the values that the renderer gives to
// the core. A number that the JavaScript leaves undefined is NaN.
type renderedStyle struct {
	clean bool

	labelX, labelY           float64
	labelWidth, labelHeight  float64
	srcX, srcY, tgtX, tgtY   float64
	midX, midY               float64
	linePts                  []Point
	hasLinePts               bool
	nodeX, nodeY             float64
	nodeW, nodeH, arrowWidth float64
}

// Element is a node or an edge of an instance.
type Element struct {
	cy *Instance
	// ID is data.id.
	ID     string
	isNode bool
	// poolIndex is the index of the element in the instance.
	poolIndex int

	// A node: its style, the node that contains it and the nodes inside it.
	NodeStyle NodeStyle
	parent    *Element
	children  []*Element
	// position is _private.position.
	position Point
	// autoWidth, autoHeight and autoPadding are the size of a compound node.
	// They are undefined until the first update of the compound bounds.
	autoWidth, autoHeight, autoPadding float64
	hasAutoPadding                     bool
	compoundBoundsClean                bool

	// edges are the edges of a node, in the order of cy.add
	// (_private.edges).
	edges []*Element

	// An edge: its style and its ends.
	EdgeStyle      EdgeStyle
	source, target *Element
	// rs is _private.rscratch of an edge.
	rs EdgeScratch
	// segpts is rscratch.segpts.
	segpts []float64

	// The caches of the bounding box.
	bbCache       *BB
	bbCachePosKey int32
	bodyBounds    *BB
	labelBounds   labelBounds

	rstyle renderedStyle
	// labelMeasured tells that the dimensions of the label are computed:
	// the key rscratch.prefixedLabelDimsKey is set.
	labelMeasured bool
}

// Instance is a cytoscape instance with a style and a canvas renderer.
type Instance struct {
	// Nodes and Edges are cy.nodes() and cy.edges(): the elements in the
	// order of cy.add.
	Nodes []*Element
	Edges []*Element
	// elements has the nodes and the edges in the order of cy.add.
	elements []*Element
	byID     map[string]*Element

	hasCompoundNodes bool
	batchCount       int
	// destroyed tells that the renderer is destroyed: it computes nothing
	// from then on.
	destroyed bool

	// MeasureText is context.measureText(text).width of the canvas of the
	// renderer.
	MeasureText func(font, text string) float64
}

// New returns an empty instance.
func New() *Instance {
	return &Instance{byID: map[string]*Element{}}
}

// nanStyle is the rendered style of an element that the renderer did not
// compute yet.
func nanStyle() renderedStyle {
	return renderedStyle{
		labelX: nan, labelY: nan, labelWidth: nan, labelHeight: nan,
		srcX: nan, srcY: nan, tgtX: nan, tgtY: nan, midX: nan, midY: nan,
		nodeX: nan, nodeY: nan, nodeW: nan, nodeH: nan, arrowWidth: nan,
	}
}

// add is the part of restore for each element: the checks of the id.
func (cy *Instance) add(e *Element) {
	if e.ID == "" {
		bail.Out("cytoscape: an element with an empty id")
	}
	if _, dup := cy.byID[e.ID]; dup {
		bail.Out("cytoscape: a second element with the same id")
	}
	e.cy = cy
	e.poolIndex = len(cy.elements)
	e.rstyle = nanStyle()
	e.autoWidth, e.autoHeight, e.autoPadding = nan, nan, nan
	cy.byID[e.ID] = e
	cy.elements = append(cy.elements, e)
}

// AddNode is cy.add of a node without a position: the node is at (0, 0).
// parent is data.parent, the id of the node that contains the node. Empty
// for none. A parent that does not exist, and the node itself, are no
// parent.
func (cy *Instance) AddNode(id, parent string, style NodeStyle) *Element {
	e := &Element{ID: id, isNode: true, NodeStyle: style}
	cy.add(e)
	cy.Nodes = append(cy.Nodes, e)
	if parent != "" {
		// A parent is specified.
		p := cy.byID[parent]
		if p != nil && !p.isNode {
			bail.Out("cytoscape: a node inside an edge")
		}
		if p != nil && p != e {
			// connect with children
			p.children = append(p.children, e)
			e.parent = p

			// let the core know we have a compound graph
			cy.hasCompoundNodes = true
		}
	}
	return e
}

// AddEdge is cy.add of an edge. cytoscape throws for an end that does not
// exist.
func (cy *Instance) AddEdge(id, source, target string, style EdgeStyle) *Element {
	src, tgt := cy.byID[source], cy.byID[target]
	if src == nil || tgt == nil || !src.isNode || !tgt.isNode {
		bail.Out("cytoscape: an edge to a node that does not exist")
	}
	e := &Element{ID: id, source: src, target: tgt, EdgeStyle: style}
	cy.add(e)
	e.rs = EdgeScratch{
		StartX: nan, StartY: nan, EndX: nan, EndY: nan,
		ArrowStartX: nan, ArrowStartY: nan, ArrowEndX: nan, ArrowEndY: nan,
		MidX: nan, MidY: nan,
	}
	cy.Edges = append(cy.Edges, e)
	src.edges = append(src.edges, e)
	if tgt != src {
		tgt.edges = append(tgt.edges, e)
	}
	return e
}

// Degree is node.connectedEdges().length.
func (e *Element) Degree() int { return len(e.edges) }

// NeighborNodes is node.neighborhood().nodes(): the node at the other end of
// each edge of the node, each one time, in the order of the edges.
func (e *Element) NeighborNodes() []*Element {
	var out []*Element
	for _, edge := range e.edges {
		other := edge.source
		if e == edge.source {
			other = edge.target
		}
		seen := false
		for _, n := range out {
			if n == other {
				seen = true
				break
			}
		}
		if !seen {
			out = append(out, other)
		}
	}
	return out
}

// Width and Height are node.width() and node.height().
func (e *Element) Width() float64  { return e.width() }
func (e *Element) Height() float64 { return e.height() }

// IsNode tells whether the element is a node.
func (e *Element) IsNode() bool { return e.isNode }

// IsParent is ele.isParent(): the node has children.
func (e *Element) IsParent() bool { return e.isNode && len(e.children) > 0 }

// Parent is the node that contains the node, or nil.
func (e *Element) Parent() *Element { return e.parent }

// Children is ele.children().
func (e *Element) Children() []*Element { return e.children }

// Source and Target are the ends of an edge.
func (e *Element) Source() *Element { return e.source }
func (e *Element) Target() *Element { return e.target }

// Elements is the collection of all elements, in the order of cy.add.
func (cy *Instance) Elements() []*Element { return cy.elements }

// StartBatch and EndBatch are cy.startBatch() and cy.endBatch().
func (cy *Instance) StartBatch() { cy.batchCount++ }

func (cy *Instance) EndBatch() {
	if cy.batchCount > 0 {
		cy.batchCount--
	}
}

func (cy *Instance) batching() bool { return cy.batchCount > 0 }

// Destroy destroys the renderer, as the mutation observer of the container
// does after mermaid removed the container.
func (cy *Instance) Destroy() { cy.destroyed = true }

// TraceState is the state of the instance that the tests compare with the
// JavaScript after a fit: for each node its position, the size of its
// content when it is a compound node, and the position and the size of its
// label; for each edge the points of its renderer scratch and its label.
func (cy *Instance) TraceState() []float64 {
	var out []float64
	for _, n := range cy.Nodes {
		out = append(out, n.position.X, n.position.Y, n.autoWidth, n.autoHeight, n.autoPadding,
			n.rstyle.labelX, n.rstyle.labelY, n.rstyle.labelWidth, n.rstyle.labelHeight)
	}
	for _, e := range cy.Edges {
		rs := &e.rs
		out = append(out, rs.StartX, rs.StartY, rs.EndX, rs.EndY, rs.ArrowStartX, rs.ArrowStartY, rs.ArrowEndX, rs.ArrowEndY, rs.MidX, rs.MidY,
			e.rstyle.labelX, e.rstyle.labelY, e.rstyle.labelWidth, e.rstyle.labelHeight)
	}
	return out
}
