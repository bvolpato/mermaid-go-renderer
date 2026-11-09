// Ported from mermaid 12.2.0, diagrams/block/renderHelpers.ts (MIT).

package block

import (
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/graphlib"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/edges"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

// getNodeFromBlock is getNodeFromBlock(block, db, positioned): the node of
// the unified renderer for a block.
func getNodeFromBlock(ctx *render.Context, block *Block, db *DB, positioned bool) *ru.Node {
	vertex := block

	classStr := "default"
	if len(vertex.Classes) > 0 {
		classStr = strings.Join(vertex.Classes, " ")
	}
	classStr = classStr + " flowchart-label"
	cssCompiledStyles := []string{}
	for _, className := range vertex.Classes {
		if class, ok := db.GetClasses().Lookup(className); ok {
			cssCompiledStyles = append(cssCompiledStyles, class.Styles...)
		}
	}

	// We create a SVG label, either by delegating to addHtmlLabel or manually
	radius := 0.0
	shape := "rect"
	// padding is undefined (NaN) for every shape but the composite.
	padding := undef
	// Set the shape based parameters
	switch vertex.Type {
	case typeRound:
		radius = 5
		shape = "rect"
	case typeComposite:
		radius = 0
		shape = "composite"
		padding = 0
	case typeSquare:
		shape = "rect"
	case typeDiamond:
		shape = "question"
	case typeHexagon:
		shape = "hexagon"
	case typeBlockArrow:
		shape = "block_arrow"
	case typeOdd:
		shape = "rect_left_inv_arrow"
	case typeLeanRight:
		shape = "lean_right"
	case typeLeanLeft:
		shape = "lean_left"
	case typeTrapezoid:
		shape = "trapezoid"
	case typeInvTrapezoid:
		shape = "inv_trapezoid"
	case typeRectLeftInv:
		shape = "rect_left_inv_arrow"
	case typeCircle:
		shape = "circle"
	case typeEllipse:
		// Ellipses are broken, see https://github.com/mermaid-js/mermaid/issues/5976
		shape = "ellipse"
	case typeStadium:
		shape = "stadium"
	case typeSubroutine:
		shape = "subroutine"
	case typeCylinder:
		shape = "cylinder"
	case typeGroup:
		shape = "rect"
	case typeDoubleCircle:
		shape = "doublecircle"
	default:
		shape = "rect"
	}

	// vertex?.styles ?? []
	vertexStyles := vertex.Styles
	if vertexStyles == nil {
		vertexStyles = []string{}
	}
	style, labelStyle := utils.GetStylesFromArray(vertexStyles)

	// Use vertex id as text in the box if no text is provided by the graph definition
	vertexText, hasVertexText := vertex.Label, vertex.HasLabel

	// vertex.size ?? { width: 0, height: 0, x: 0, y: 0 }
	var bounds Size
	if vertex.Size != nil {
		bounds = *vertex.Size
	}
	dbDiagramID := db.GetDiagramId()
	// Add the node
	node := ru.NewNode(vertex.ID)
	node.LabelStyle = labelStyle
	node.Shape = shape
	node.Label, node.HasLabel = vertexText, hasVertexText
	if hasVertexText {
		node.Set("labelText", vertexText)
	}
	node.Rx = radius
	node.Ry = radius
	node.Set("class", classStr)
	node.CSSClasses, node.CSSClassesSet = classStr, true
	node.CSSStyles, node.CSSStylesSet = vertexStyles, true
	node.CSSCompiledStyles = cssCompiledStyles
	node.Set("style", style)
	if dbDiagramID != "" {
		node.DomID = dbDiagramID + "-" + vertex.idString()
	} else {
		node.DomID = vertex.ID
	}
	node.IsGroup = false
	if vertex.Directions != nil {
		directions := make([]any, len(vertex.Directions))
		for i, direction := range vertex.Directions {
			directions[i] = direction
		}
		node.Set("directions", directions)
	}
	node.Width = ru.Or(bounds.Width, ru.Undef)
	node.Height = ru.Or(bounds.Height, ru.Undef)
	node.WrappingWidth = ru.Or(bounds.Width, math.Inf(1))
	node.X = bounds.X
	node.Y = bounds.Y
	node.Set("positioned", positioned)
	node.Intersect = nil
	// padding ?? getConfig()?.block?.padding ?? 0
	if isUndef(padding) {
		padding = blockPadding(db, 0)
	}
	node.Padding = padding
	node.Set("widthInColumns", nn(vertex.WidthInColumns, 1))
	node.ColorIndex = vertex.ColorIndex
	if look := ctx.Cfg("look"); !js.IsNullish(look) {
		node.Look = js.ToString(look)
	}
	return node
}

// calculateBlockSize is calculateBlockSize(elem, block, db): it draws the
// block to measure it, and removes the drawing.
func calculateBlockSize(ctx *render.Context, elem *dom.Node, block *Block, db *DB) error {
	node := getNodeFromBlock(ctx, block, db, false)
	if block.Type == typeGroup {
		return nil
	}

	// Add the element to the DOM to size it
	config := ctx.Config
	nodeEl, err := ru.InsertNode(ctx, elem, node, &ru.ShapeRenderOptions{Config: config, Padding: ru.Undef})
	if err != nil {
		return err
	}
	// nodeEl.node()?.getBBox() ?? { width: 0, height: 0 }
	var boundingBox dom.Rect
	if nodeEl != nil {
		boundingBox = nodeEl.BBox()
	}
	obj := db.getBlock(block.key())
	if obj == nil {
		return typeError("Cannot set properties of undefined (setting 'size')")
	}
	obj.Size = &Size{Width: boundingBox.Width, Height: boundingBox.Height, X: 0, Y: 0}
	db.SetBlock(obj)
	nodeEl.Remove()
	return nil
}

// actionFun is ActionFun: the operation of performOperations.
type actionFun func(ctx *render.Context, elem *dom.Node, block *Block, db *DB) error

// insertBlockPositioned is insertBlockPositioned(elem, block, db): it draws
// the block at its place.
func insertBlockPositioned(ctx *render.Context, elem *dom.Node, block *Block, db *DB) error {
	node := getNodeFromBlock(ctx, block, db, true)
	// Add the element to the DOM to size it
	obj := db.getBlock(block.key())
	if obj == nil {
		return typeError("Cannot read properties of undefined (reading 'type')")
	}
	if obj.Type != typeSpace {
		config := ctx.Config
		el, err := ru.InsertNode(ctx, elem, node, &ru.ShapeRenderOptions{Config: config, Padding: ru.Undef})
		if err != nil {
			return err
		}
		// Only where a slot was actually assigned -- composites. `stampColorSlot` falls back
		// to `colorIndex ?? 0`, so calling it unconditionally would stamp every plain block
		// with slot 0 and paint the whole diagram one colour.
		if !ru.IsUndef(node.ColorIndex) {
			ru.StampColorSlot(el, node.ColorIndex, config.Get("theme"), js.Path(config, "themeVariables", "borderColorArray"))
		}
		block.Intersect = node.Intersect
		// The intersect function of the question shape reads
		// this.calcIntersect (see blockIntersect).
		block.intersectUsesThis = node.Intersect != nil && node.Shape == "question"
		ru.PositionNode(ctx, ru.PositionedNode{
			ID:     node.ID,
			X:      node.X,
			Y:      node.Y,
			Width:  node.Width,
			Height: node.Height,
			Diff:   node.Diff,
		})
	}
	return nil
}

// performOperations is performOperations(elem, blocks, db, operation): it
// runs the operation for every block of the hierarchy, a block before the
// blocks inside it.
func performOperations(ctx *render.Context, elem *dom.Node, blocks []*Block, db *DB, operation actionFun) error {
	for _, block := range blocks {
		if err := operation(ctx, elem, block, db); err != nil {
			return err
		}
		if block.hasChildren() {
			if err := performOperations(ctx, elem, block.Children, db, operation); err != nil {
				return err
			}
		}
	}
	return nil
}

// calculateBlockSizes is calculateBlockSizes(elem, blocks, db).
func calculateBlockSizes(ctx *render.Context, elem *dom.Node, blocks []*Block, db *DB) error {
	return performOperations(ctx, elem, blocks, db, calculateBlockSize)
}

// insertBlocks is insertBlocks(elem, blocks, db).
func insertBlocks(ctx *render.Context, elem *dom.Node, blocks []*Block, db *DB) error {
	return performOperations(ctx, elem, blocks, db, insertBlockPositioned)
}

// graphNode is the label of a node of the graph of insertEdges:
// { width, height, intersect }.
type graphNode struct {
	width, height float64
	intersect     func(point ru.Point) ru.Point
	// intersectUsesThis is Block.intersectUsesThis of the block.
	intersectUsesThis bool
}

// jsThrow is the panic value that carries a JavaScript exception out of a
// function of another package that has no error result.
type jsThrow struct{ err error }

// blockIntersect is the node that insertEdge gets for an end of an edge:
// the graph node label, of which insertEdge calls the intersect function.
//
// Upstream moves the intersect function of the drawn node to the graph
// node label and calls it there. The function of the question shape (the
// diamond block) reads this.calcIntersect, and the label has no such
// property: the call throws a TypeError. So an edge to or from a diamond
// block fails the render, and it does here.
func blockIntersect(n *graphNode) *ru.Node {
	node := &ru.Node{}
	if n == nil || n.intersect == nil {
		return node
	}
	if n.intersectUsesThis {
		node.Intersect = func(ru.Point) ru.Point {
			panic(jsThrow{err: typeError("this.calcIntersect is not a function")})
		}
		return node
	}
	node.Intersect = n.intersect
	return node
}

// insertEdge calls insertEdge of the unified renderer. It returns the
// exception of an intersect function as an error.
func insertEdge(ctx *render.Context, elem *dom.Node, edge *ru.Edge, startNode, endNode *graphNode, id string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			thrown, ok := r.(jsThrow)
			if !ok {
				panic(r)
			}
			err = thrown.err
		}
	}()
	_, err = edges.InsertEdge(ctx, elem, edge, nil, "block", blockIntersect(startNode), blockIntersect(endNode), id, false, "")
	return err
}

// insertEdges is insertEdges(elem, edges, blocks, db, id): it draws the
// edges and their labels. An edge is a line from the center of its source
// block over the middle to the center of its target block, cut at the
// outlines of the two blocks.
func insertEdges(ctx *render.Context, elem *dom.Node, edgeList []*Block, blocks []*Block, db *DB, id string) error {
	g := graphlib.New(graphlib.Options{
		Multigraph: true,
		Compound:   true,
	})
	graphLabel := js.NewObj[any]()
	graphLabel.Set("rankdir", "TB")
	graphLabel.Set("nodesep", 10.0)
	graphLabel.Set("ranksep", 10.0)
	graphLabel.Set("marginx", 8.0)
	graphLabel.Set("marginy", 8.0)
	g.SetGraph(graphLabel)

	for _, block := range blocks {
		if block.Size != nil {
			g.SetNode(block.idString(), &graphNode{
				width:             block.Size.Width,
				height:            block.Size.Height,
				intersect:         block.Intersect,
				intersectUsesThis: block.intersectUsesThis,
			})
		}
	}

	for _, edge := range edgeList {
		// elem, e, edge, clusterDb, diagramType, graph;
		if edge.Start != "" && edge.End != "" {
			startBlock := db.GetBlock(edge.Start)
			endBlock := db.GetBlock(edge.End)

			if startBlock != nil && startBlock.Size != nil && endBlock != nil && endBlock.Size != nil {
				start := startBlock.Size
				end := endBlock.Size
				points := []ru.Point{
					{X: start.X, Y: start.Y},
					{X: start.X + float64((end.X-start.X)/2), Y: start.Y + float64((end.Y-start.Y)/2)},
					{X: end.X, Y: end.Y},
				}
				prefixedEdgeID := edge.ID
				if id != "" {
					prefixedEdgeID = id + "-" + edge.ID
				}

				thicknessClass := "edge-thickness-normal"
				if edge.Thickness == "thick" {
					thicknessClass = "edge-thickness-thick"
				}
				patternClass := "edge-pattern-solid"
				if edge.Pattern == "dotted" {
					patternClass = "edge-pattern-dotted"
				}
				dynamicClasses := thicknessClass + " " + patternClass + " flowchart-link LS-a1 LE-b1"

				// { ...edge, id: prefixedEdgeId, arrowTypeEnd, arrowTypeStart, points, classes: dynamicClasses }
				// The block edge has no style: insertEdge prints the text
				// "undefined" for it.
				drawnEdge := edgeOfBlock(edge)
				drawnEdge.ID = prefixedEdgeID
				drawnEdge.Points = points
				drawnEdge.Classes = dynamicClasses
				drawnEdge.StyleUndefined = true
				startNode, _ := g.Node(edge.Start).(*graphNode)
				endNode, _ := g.Node(edge.End).(*graphNode)
				if err := insertEdge(ctx, elem, drawnEdge, startNode, endNode, id); err != nil {
					return err
				}
				if edge.Label != "" {
					labelEdge := edgeOfBlock(edge)
					labelEdge.LabelStyleText, labelEdge.LabelStyleIsText = "stroke: #333; stroke-width: 1.5px;fill:none;", true
					labelEdge.Points = points
					labelEdge.Classes = dynamicClasses
					if _, err := edges.InsertEdgeLabel(ctx, elem, labelEdge); err != nil {
						return err
					}
					positionedEdge := edgeOfBlock(edge)
					positionedEdge.X = points[1].X
					positionedEdge.Y = points[1].Y
					if err := edges.PositionEdgeLabel(ctx, positionedEdge, &edges.Paths{
						OriginalPath:    points,
						HasOriginalPath: true,
					}); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// edgeOfBlock is { ...edge }: the edge of the unified renderer with the
// properties of the block edge.
func edgeOfBlock(edge *Block) *ru.Edge {
	e := ru.NewEdge(edge.ID)
	e.Start = edge.Start
	e.End = edge.End
	e.Label, e.HasLabel = edge.Label, edge.HasLabel
	e.Type = edge.Type
	e.Thickness = edge.Thickness
	e.Pattern = edge.Pattern
	e.ArrowTypeEnd = edge.ArrowTypeEnd
	e.ArrowTypeStart = edge.ArrowTypeStart
	if edge.Directions != nil {
		directions := make([]any, len(edge.Directions))
		for i, direction := range edge.Directions {
			directions[i] = direction
		}
		e.Set("directions", directions)
	}
	return e
}
