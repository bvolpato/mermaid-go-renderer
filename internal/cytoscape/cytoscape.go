// The Go API. Each function stands for one use of cytoscape in mermaid
// 12.2.0:
//
//   - CoseBilkent: rendering-util/layout-algorithms/cose-bilkent/
//     cytoscape-setup.ts and layout.ts (createCytoscapeInstance,
//     extractPositionedNodes, extractPositionedEdges).
//   - Architecture: diagrams/architecture/architectureRenderer.ts
//     (layoutArchitecture) and the reads of svgDraw.ts and positionNodes.
//   - Run: the same sequence for other elements, styles and layouts.

package cytoscape

import (
	"context"
	"errors"
	"math"

	"github.com/dop251/goja"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Point is a position in the coordinates of the layout.
type Point struct {
	X, Y float64
}

// Box is the result of boundingBox() of an element.
type Box struct {
	X1, Y1, X2, Y2, W, H float64
}

// ---- cose-bilkent ----

// Node is a node of the cose-bilkent layout: the fields of a layout node of
// mermaid that the layout reads.
type Node struct {
	ID string
	// Width and Height are the size that the layout gives to the node
	// (mermaid replaces layoutDimensions of each node with these values).
	Width, Height float64
	// X and Y are the position before the layout: node.x ?? 0 and
	// node.y ?? 0. The layout starts from random positions, so they do not
	// change the result of mermaid's options.
	X, Y float64
}

// Edge is an edge of the cose-bilkent layout: edge.id, edge.start and
// edge.end of mermaid.
type Edge struct {
	ID, Start, End string
}

// PositionedNode is a node after the layout. X and Y are the centre.
type PositionedNode struct {
	ID   string
	X, Y float64
}

// PositionedEdge is an edge after the layout: the points that mermaid takes
// from the renderer scratch of cytoscape. A value that cytoscape did not
// compute is NaN (undefined in JavaScript).
type PositionedEdge struct {
	ID, Source, Target string
	StartX, StartY     float64
	MidX, MidY         float64
	EndX, EndY         float64
}

// CoseBilkentResult is LayoutResult of the cose-bilkent layout.
type CoseBilkentResult struct {
	Nodes []PositionedNode
	Edges []PositionedEdge
}

// CoseBilkent is executeCoseBilkentLayout: it creates the cytoscape instance
// as createCytoscapeInstance does, runs the layout cose-bilkent with the
// options of mermaid (quality proof, no animation) and returns the positions
// of the nodes and the points of the edges.
//
// The result has the nodes and the edges in the order of the input. An
// element that cytoscape cannot add is an error, as it is in mermaid: an id
// that is empty or that exists, and an edge whose source or target does not
// exist.
//
// A graph that is a forest, as the tree of a mindmap is, gets its first
// positions on circles around the roots and draws no random number. For
// another graph the layout prunes the leaves, puts them back beside their
// neighbours and draws from Math.random when two or four sides are free: so
// env.Random must be the generator of the render.
func CoseBilkent(ctx context.Context, env Env, nodes []Node, edges []Edge) (*CoseBilkentResult, error) {
	if o := observer.Load(); o != nil && o.CoseBilkent != nil {
		env = o.CoseBilkent(env, nodes, edges)
	}
	return coseBilkentByMode(ctx, env, nodes, edges)
}

// coseBilkent runs the JavaScript.
func coseBilkent(ctx context.Context, env Env, opts runtimeOptions, nodes []Node, edges []Edge) (*CoseBilkentResult, error) {
	countInterpreter()
	var out *CoseBilkentResult
	err := withRuntime(ctx, env, opts, func(ctx context.Context, rt *jsRuntime) error {
		vm := rt.vm
		jsNodes := make([]any, len(nodes))
		for i, n := range nodes {
			jsNodes[i] = rt.object("id", n.ID, "width", n.Width, "height", n.Height, "x", n.X, "y", n.Y)
		}
		jsEdges := make([]any, len(edges))
		for i, e := range edges {
			jsEdges[i] = rt.object("id", e.ID, "start", e.Start, "end", e.End)
		}
		res, err := rt.invoke(ctx, "coseBilkent", rt.object("nodes", vm.NewArray(jsNodes...), "edges", vm.NewArray(jsEdges...)))
		if err != nil {
			return err
		}
		out = &CoseBilkentResult{}
		for _, n := range rt.items(res, "nodes") {
			out.Nodes = append(out.Nodes, PositionedNode{ID: str(n, "id"), X: num(n, "x"), Y: num(n, "y")})
		}
		for _, e := range rt.items(res, "edges") {
			out.Edges = append(out.Edges, PositionedEdge{
				ID: str(e, "id"), Source: str(e, "source"), Target: str(e, "target"),
				StartX: num(e, "startX"), StartY: num(e, "startY"),
				MidX: num(e, "midX"), MidY: num(e, "midY"),
				EndX: num(e, "endX"), EndY: num(e, "endY"),
			})
		}
		return nil
	})
	return out, err
}

// ---- fcose for the architecture diagram ----

// ArchitectureConfig has the values of db.getConfigField that
// layoutArchitecture reads.
type ArchitectureConfig struct {
	IconSize                  float64
	FontSize                  float64
	Padding                   float64
	IdealEdgeLengthMultiplier float64
	EdgeElasticity            float64
	NodeSeparation            float64
	NumIter                   float64
	Randomize                 bool
	// Seed is the seed of withSeededRandom. With 0 the layout draws from
	// Env.Random.
	Seed float64
}

// ArchitectureNode is a service, a junction or a group of the architecture
// db. An empty string stands for a property that is undefined: the grammar
// has no empty title, icon or group id.
type ArchitectureNode struct {
	ID string
	// Icon is service.icon or group.icon. A junction has none.
	Icon string
	// Title is service.title or group.title: the label in cytoscape.
	Title string
	// In is the id of the group that contains the node.
	In string
}

// ArchitectureEdge is an edge of the architecture db.
type ArchitectureEdge struct {
	LhsID, RhsID string
	// LhsDir and RhsDir are "L", "R", "T" or "B".
	LhsDir, RhsDir string
	Title          string
}

// AlignmentConstraint is the alignmentConstraint option of fcose: lists of
// node ids that share a y coordinate (Horizontal) or an x coordinate
// (Vertical). It is the result of getAlignments.
type AlignmentConstraint struct {
	Horizontal [][]string
	Vertical   [][]string
}

// RelativePlacement is one entry of the relativePlacementConstraint option
// of fcose, as getRelativeConstraints builds it: either Left and Right are
// set, or Top and Bottom.
type RelativePlacement struct {
	Left, Right string
	Top, Bottom string
	Gap         float64
}

// ArchitectureInput is the input of layoutArchitecture.
type ArchitectureInput struct {
	Config    ArchitectureConfig
	Groups    []ArchitectureNode
	Services  []ArchitectureNode
	Junctions []ArchitectureNode
	Edges     []ArchitectureEdge
	Alignment AlignmentConstraint
	// RelativePlacement is the list in the order of getRelativeConstraints.
	// The order of the entries changes the layout.
	RelativePlacement []RelativePlacement
}

// ArchitectureNodeResult is what mermaid reads from a node after the layout.
type ArchitectureNodeResult struct {
	ID string
	// Type is "group", "service" or "junction".
	Type string
	// Index is the index of the node in Groups, Services or Junctions.
	Index int
	// BoundingBox is node.boundingBox(). mermaid reads it for a group only,
	// and so does this package.
	BoundingBox Box
	// X and Y are node.position(), for a service and a junction.
	X, Y float64
}

// ArchitectureEdgeResult is what drawEdges reads from an edge.
type ArchitectureEdgeResult struct {
	// ID is the id in cytoscape: LhsID + "-" + RhsID.
	ID string
	// Index is the index of the edge in ArchitectureInput.Edges.
	Index          int
	SourceEndpoint Point
	Midpoint       Point
	TargetEndpoint Point
	// HasRScratch is the test `edge[0]._private.rscratch` of drawEdges.
	HasRScratch bool
}

// ArchitectureResult has the nodes and edges of the cytoscape instance in
// the order of cy.nodes() and cy.edges(): the groups, the services, the
// junctions, and the edges, each in the order of the input.
type ArchitectureResult struct {
	Nodes []ArchitectureNodeResult
	Edges []ArchitectureEdgeResult
}

// Architecture is layoutArchitecture followed by the reads of drawEdges,
// drawGroups and positionNodes: it creates the instance with the style sheet
// of the architecture renderer, runs fcose, bends the edges that connect two
// sides at a right angle, runs fcose again, and reads the endpoints of the
// edges, the bounding boxes of the groups and the positions of the other
// nodes.
//
// The labels of the services take part in the bounding box of a group, so
// the layout measures them with Env.MeasureText.
//
// An element that cytoscape cannot add is an error, as it is in mermaid. Two
// edges between the same two nodes in the same direction are such a case:
// they have the same id.
func Architecture(ctx context.Context, env Env, in *ArchitectureInput) (*ArchitectureResult, error) {
	if o := observer.Load(); o != nil && o.Architecture != nil && in != nil {
		env = o.Architecture(env, in)
	}
	return architectureByMode(ctx, env, in)
}

func architecture(ctx context.Context, env Env, opts runtimeOptions, in *ArchitectureInput) (*ArchitectureResult, error) {
	if in == nil {
		return nil, errors.New("cytoscape: the architecture input is nil")
	}
	countInterpreter()
	var out *ArchitectureResult
	err := withRuntime(ctx, env, opts, func(ctx context.Context, rt *jsRuntime) error {
		vm := rt.vm
		nodeList := func(list []ArchitectureNode) *goja.Object {
			items := make([]any, len(list))
			for i, n := range list {
				items[i] = rt.object("id", n.ID, "icon", optional(n.Icon), "title", optional(n.Title), "in", optional(n.In))
			}
			return vm.NewArray(items...)
		}
		edges := make([]any, len(in.Edges))
		for i, e := range in.Edges {
			edges[i] = rt.object("lhsId", e.LhsID, "rhsId", e.RhsID, "lhsDir", e.LhsDir, "rhsDir", e.RhsDir, "title", optional(e.Title))
		}
		idLists := func(lists [][]string) *goja.Object {
			items := make([]any, len(lists))
			for i, l := range lists {
				ids := make([]any, len(l))
				for j, id := range l {
					ids[j] = id
				}
				items[i] = vm.NewArray(ids...)
			}
			return vm.NewArray(items...)
		}
		placements := make([]any, len(in.RelativePlacement))
		for i, p := range in.RelativePlacement {
			// getRelativeConstraints writes the two sides first and the gap
			// last. fcose reads the entries by name.
			if p.Left != "" || p.Right != "" {
				placements[i] = rt.object("left", p.Left, "right", p.Right, "gap", p.Gap)
			} else {
				placements[i] = rt.object("top", p.Top, "bottom", p.Bottom, "gap", p.Gap)
			}
		}
		c := in.Config
		input := rt.object(
			"config", rt.object(
				"iconSize", c.IconSize, "fontSize", c.FontSize, "padding", c.Padding,
				"idealEdgeLengthMultiplier", c.IdealEdgeLengthMultiplier, "edgeElasticity", c.EdgeElasticity,
				"randomize", c.Randomize, "nodeSeparation", c.NodeSeparation, "numIter", c.NumIter, "seed", c.Seed,
			),
			"groups", nodeList(in.Groups),
			"services", nodeList(in.Services),
			"junctions", nodeList(in.Junctions),
			"edges", vm.NewArray(edges...),
			"alignmentConstraint", rt.object("horizontal", idLists(in.Alignment.Horizontal), "vertical", idLists(in.Alignment.Vertical)),
			"relativePlacementConstraint", vm.NewArray(placements...),
		)
		res, err := rt.invoke(ctx, "architecture", input)
		if err != nil {
			return err
		}
		out = &ArchitectureResult{}
		for _, n := range rt.items(res, "nodes") {
			r := ArchitectureNodeResult{ID: str(n, "id"), Type: str(n, "type"), Index: index(n, "index"), X: math.NaN(), Y: math.NaN()}
			if bb, ok := n.Get("boundingBox").(*goja.Object); ok {
				r.BoundingBox = box(bb)
			} else {
				r.X, r.Y = num(n, "x"), num(n, "y")
			}
			out.Nodes = append(out.Nodes, r)
		}
		for _, e := range rt.items(res, "edges") {
			out.Edges = append(out.Edges, ArchitectureEdgeResult{
				ID:             str(e, "id"),
				Index:          index(e, "index"),
				SourceEndpoint: point(e, "sourceEndpoint"),
				Midpoint:       point(e, "midpoint"),
				TargetEndpoint: point(e, "targetEndpoint"),
				HasRScratch:    boolean(e, "hasRscratch"),
			})
		}
		return nil
	})
	return out, err
}

func box(bb *goja.Object) Box {
	return Box{X1: num(bb, "x1"), Y1: num(bb, "y1"), X2: num(bb, "x2"), Y2: num(bb, "y2"), W: num(bb, "w"), H: num(bb, "h")}
}

// ---- the general call ----

// Element is the argument of one cy.add call.
type Element struct {
	// Group is "nodes" or "edges".
	Group string
	// Data is the data object: id, and parent for a node in a compound node,
	// source and target for an edge, and what the style sheet maps.
	Data *js.Obj[any]
	// Classes is the space-separated class list. Empty for none.
	Classes string
	// Position is the position of a node. nil for none.
	Position *Point
}

// Graph is the input of Run. Style, Layout and the entries of Layouts are
// dynamic values of the package js (nil, bool, float64, string, []any,
// *js.Obj[any]), as mermaid would write them in JavaScript.
type Graph struct {
	// Style is the style option of the constructor: a list of
	// { selector, style } objects. nil leaves the option out.
	Style any
	// Layout is the layout option of the constructor, which cytoscape runs
	// on the empty instance. nil leaves the option out (cytoscape then runs
	// the grid layout).
	Layout any
	// Elements are added one by one, in order.
	Elements []Element
	// DataDimensions gives each node the layout size data.width and
	// data.height, as the cose-bilkent setup of mermaid does. Without it a
	// layout takes the size from the style.
	DataDimensions bool
	// Layouts are the options of cy.layout(options).run(), run in order. A
	// value of type PerElement stands for a function of the element.
	Layouts []*js.Obj[any]
	// Seed runs each layout with Math.random replaced by the generator of
	// withSeededRandom (architectureSeed.ts). 0 keeps Env.Random.
	Seed float64
	// Settle runs the timers and animation frames that wait after the
	// layouts, at most this number of them, with a virtual clock. mermaid
	// reads its results before any of them runs, so the default is 0.
	Settle int
}

// NodeResult is the geometry of a node after Run.
type NodeResult struct {
	ID string
	// Position is node.position().
	Position Point
	// BoundingBox is node.boundingBox().
	BoundingBox Box
	// IsParent tells that the node is a compound node.
	IsParent bool
}

// EdgeResult is the geometry of an edge after Run.
type EdgeResult struct {
	ID, Source, Target string
	// RScratch has startX, startY, midX, midY, endX and endY of the renderer
	// scratch, read before the calls below. A value that cytoscape did not
	// compute is NaN.
	RScratch struct{ Start, Mid, End Point }
	// SourceEndpoint, Midpoint and TargetEndpoint are the results of the
	// methods of the same names.
	SourceEndpoint Point
	Midpoint       Point
	TargetEndpoint Point
}

// Result is the result of Run: the elements of the instance in the order of
// cy.nodes() and cy.edges().
type Result struct {
	Nodes []NodeResult
	Edges []EdgeResult
	// PendingTasks is the number of timers and animation frames that did
	// not run.
	PendingTasks int
}

// Run runs cytoscape with the given elements, style and layouts in the way
// mermaid uses it, and returns the positions and bounding boxes of the nodes
// and the endpoints of the edges.
//
// The sequence is the one of mermaid: a container with display:none, the
// constructor, the removal of the container, the elements, the layouts, and
// then the microtask in which the renderer is destroyed. The geometry is
// read after that, so it is the geometry that the layouts and their own
// bounding box requests left behind. CoseBilkent and Architecture are this
// sequence with the exact statements of the two mermaid callers: use them
// for those diagrams.
func Run(ctx context.Context, env Env, g *Graph) (*Result, error) {
	if g == nil {
		return nil, errors.New("cytoscape: the graph is nil")
	}
	var out *Result
	err := withRuntime(ctx, env, runtimeOptions{}, func(ctx context.Context, rt *jsRuntime) error {
		vm := rt.vm
		spec := vm.NewObject()
		set := func(name string, v any) error {
			jv, err := rt.toValue(v)
			if err != nil {
				return err
			}
			return spec.Set(name, jv)
		}
		if g.Style != nil {
			if err := set("style", g.Style); err != nil {
				return err
			}
		}
		if g.Layout != nil {
			if err := set("layout", g.Layout); err != nil {
				return err
			}
		}
		elements := make([]any, len(g.Elements))
		for i, e := range g.Elements {
			data, err := rt.toValue(e.Data)
			if err != nil {
				return err
			}
			el := rt.object("group", e.Group, "data", data)
			if e.Classes != "" {
				_ = el.Set("classes", e.Classes)
			}
			if e.Position != nil {
				_ = el.Set("position", rt.object("x", e.Position.X, "y", e.Position.Y))
			}
			elements[i] = el
		}
		layouts := make([]any, len(g.Layouts))
		for i, l := range g.Layouts {
			lv, err := rt.toValue(l)
			if err != nil {
				return err
			}
			layouts[i] = lv
		}
		_ = spec.Set("elements", vm.NewArray(elements...))
		_ = spec.Set("layouts", vm.NewArray(layouts...))
		_ = spec.Set("dataDimensions", g.DataDimensions)
		_ = spec.Set("seed", g.Seed)
		_ = spec.Set("settle", g.Settle)

		res, err := rt.invoke(ctx, "run", spec)
		if err != nil {
			return err
		}
		out = &Result{PendingTasks: max(index(res, "pendingTasks"), 0)}
		for _, n := range rt.items(res, "nodes") {
			r := NodeResult{ID: str(n, "id"), Position: Point{X: num(n, "x"), Y: num(n, "y")}, IsParent: boolean(n, "isParent")}
			if bb, ok := n.Get("boundingBox").(*goja.Object); ok {
				r.BoundingBox = box(bb)
			}
			out.Nodes = append(out.Nodes, r)
		}
		for _, e := range rt.items(res, "edges") {
			r := EdgeResult{
				ID: str(e, "id"), Source: str(e, "source"), Target: str(e, "target"),
				SourceEndpoint: point(e, "sourceEndpoint"),
				Midpoint:       point(e, "midpoint"),
				TargetEndpoint: point(e, "targetEndpoint"),
			}
			r.RScratch.Start = Point{X: num(e, "startX"), Y: num(e, "startY")}
			r.RScratch.Mid = Point{X: num(e, "midX"), Y: num(e, "midY")}
			r.RScratch.End = Point{X: num(e, "endX"), Y: num(e, "endY")}
			out.Edges = append(out.Edges, r)
		}
		return nil
	})
	return out, err
}
