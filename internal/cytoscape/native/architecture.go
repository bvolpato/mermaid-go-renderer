// The fcose layout of an architecture diagram. The statements are those of
// mermaid 12.2.0, diagrams/architecture/architectureRenderer.ts
// (layoutArchitecture, addGroups, addServices, addJunctions, addEdges),
// architectureSeed.ts (withSeededRandom) and svgDraw.ts (the reads of
// drawEdges and drawGroups), with the parts of cytoscape 3.34.0 (MIT license)
// that they reach.

package native

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native/cy"
	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native/fcose"
	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native/internal/bail"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

func isArchitectureDirectionX(x string) bool { return x == "L" || x == "R" }
func isArchitectureDirectionY(x string) bool { return x == "T" || x == "B" }

func isArchitectureDirectionXY(a, b string) bool {
	return isArchitectureDirectionX(a) != isArchitectureDirectionX(b)
}

// endpointOf is the value of the style property source-endpoint or
// target-endpoint for a direction, as cytoscape parses it: '0 50%' for L,
// '100% 50%' for R, '50% 0' for T and '50% 100%' for each other value.
func endpointOf(dir string) cy.Endpoint {
	switch dir {
	case "L":
		return cy.Endpoint{PfValue: [2]float64{0, 0.5}, Percent: [2]bool{false, true}}
	case "R":
		return cy.Endpoint{PfValue: [2]float64{1, 0.5}, Percent: [2]bool{true, true}}
	case "T":
		return cy.Endpoint{PfValue: [2]float64{0.5, 0}, Percent: [2]bool{true, false}}
	}
	return cy.Endpoint{PfValue: [2]float64{0.5, 1}, Percent: [2]bool{true, true}}
}

// The default style of cytoscape for a node and for an edge.
const (
	defaultNodeSize      = 30.0
	defaultFontSize      = 16.0
	defaultParentPadding = 10.0
	edgeWidth            = 3.0
)

// maxNesting is the deepest nesting of groups that the port lays out.
const maxNesting = 12

// finite tells whether x is a number that is not infinite.
func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

// architectureLayout is the state of one call of Architecture.
type architectureLayout struct {
	env  *Env
	in   *ArchitectureInput
	inst *cy.Instance
	// edgeIndex is the index of each edge of the instance in in.Edges.
	edgeIndex []int
	// constraints are the constraints of the layout options, after the
	// validity check of fcose.
	constraints fcose.Constraints

	sameGroupIdealLength  float64
	crossGroupIdealLength float64
	sameGroupElasticity   float64
}

// Architecture is layoutArchitecture of mermaid followed by the reads of
// drawEdges, drawGroups and positionNodes.
//
// reason is not empty when the port does not cover the input: res is then
// nil, and the caller runs the JavaScript. This is the case for the option
// randomize, for config values that are not positive numbers, for an edge of
// a group that has members, for an edge from a node to itself, and for an
// input on which cytoscape or the layout throws.
func Architecture(env *Env, in *ArchitectureInput) (res *ArchitectureResult, reason string) {
	defer bail.Recover(&reason)

	if env == nil {
		env = &Env{}
	}
	conf := in.Config
	// A value that is not a positive number makes a style rule invalid, a
	// node without a size, an edge without a length or a layout without an
	// end. No diagram has such a config, and the port does not cover it.
	for _, v := range []float64{conf.IconSize, conf.FontSize, conf.IdealEdgeLengthMultiplier, conf.EdgeElasticity, conf.NodeSeparation} {
		if !(v > 0) || !finite(v) {
			bail.Out("architecture: a config value is not a positive number")
		}
	}
	if !(conf.Padding >= 0) || !finite(conf.Padding) {
		bail.Out("architecture: the padding is not a number")
	}
	if !(conf.NumIter >= 1) || !finite(conf.NumIter) || !finite(conf.Seed) {
		bail.Out("architecture: numIter or the seed is not a number")
	}

	a := &architectureLayout{env: env, in: in, inst: cy.New()}
	a.inst.MeasureText = env.MeasureText

	// addGroups, addServices, addJunctions and addEdges. The style sheet of
	// the renderer gives each node its style: see nodeStyle.
	nodeTypes := make([]string, 0, len(in.Groups)+len(in.Services)+len(in.Junctions))
	nodeIndex := make([]int, 0, cap(nodeTypes))
	for i, group := range in.Groups {
		a.inst.AddNode(group.ID, group.In, a.nodeStyle("group", group.Title))
		nodeTypes = append(nodeTypes, "group")
		nodeIndex = append(nodeIndex, i)
	}
	for i, service := range in.Services {
		a.inst.AddNode(service.ID, service.In, a.nodeStyle("service", service.Title))
		nodeTypes = append(nodeTypes, "service")
		nodeIndex = append(nodeIndex, i)
	}
	for i, junction := range in.Junctions {
		a.inst.AddNode(junction.ID, junction.In, a.nodeStyle("junction", ""))
		nodeTypes = append(nodeTypes, "junction")
		nodeIndex = append(nodeIndex, i)
	}
	for i, parsedEdge := range in.Edges {
		style := cy.EdgeStyle{
			SourceEndpoint: endpointOf(parsedEdge.LhsDir),
			TargetEndpoint: endpointOf(parsedEdge.RhsDir),
			Label:          parsedEdge.Title,
			Width:          edgeWidth,
			FontSize:       defaultFontSize,
		}
		if isArchitectureDirectionXY(parsedEdge.LhsDir, parsedEdge.RhsDir) {
			// The class segments: 'segment-weights': '0' and
			// 'segment-distances': [0.5].
			style.Segments = true
			style.SegmentWeights = []float64{0}
			style.SegmentDistances = []float64{0.5}
		}
		a.inst.AddEdge(parsedEdge.LhsID+"-"+parsedEdge.RhsID, parsedEdge.LhsID, parsedEdge.RhsID, style)
		a.edgeIndex = append(a.edgeIndex, i)
	}
	// cytoscape computes the bounds of a compound node two times for each
	// level around it: the cost doubles with each level of nesting.
	for _, node := range a.inst.Nodes {
		depth := 0
		for p := node.Parent(); p != nil; p = p.Parent() {
			depth++
		}
		if depth > maxNesting {
			bail.Out("architecture: groups nested deeper than %d levels", maxNesting)
		}
	}
	for i, node := range a.inst.Nodes {
		if node.IsParent() && nodeTypes[i] != "group" {
			// The db of mermaid rejects a service inside a service.
			bail.Out("architecture: a service or a junction with members")
		}
	}
	for _, edge := range a.inst.Edges {
		if edge.Source().IsParent() || edge.Target().IsParent() || edge.Source() == edge.Target() {
			bail.Out("architecture: an edge of a group that has members, or from a node to itself")
		}
	}

	// The options of the layout.
	a.sameGroupIdealLength = conf.IdealEdgeLengthMultiplier * conf.IconSize
	a.crossGroupIdealLength = 0.5 * conf.IconSize
	a.sameGroupElasticity = conf.EdgeElasticity
	// fcose: an empty list of a constraint is no constraint.
	a.constraints.HasAlignment = true
	if len(in.Alignment.Vertical) > 0 {
		a.constraints.Vertical = in.Alignment.Vertical
	}
	if len(in.Alignment.Horizontal) > 0 {
		a.constraints.Horizontal = in.Alignment.Horizontal
	}
	for _, r := range in.RelativePlacement {
		if !(r.Gap > 0) || !finite(r.Gap) {
			bail.Out("architecture: a relative placement with a gap that is not a positive number")
		}
		if r.Left != "" || r.Right != "" {
			if r.Left == "" || r.Right == "" {
				bail.Out("architecture: a relative placement with one side")
			}
			a.constraints.Relative = append(a.constraints.Relative, fcose.RelativePlacement{Left: r.Left, Right: r.Right, Gap: r.Gap})
		} else {
			if r.Top == "" || r.Bottom == "" {
				bail.Out("architecture: a relative placement with one side")
			}
			a.constraints.Relative = append(a.constraints.Relative, fcose.RelativePlacement{Top: r.Top, Bottom: r.Bottom, Gap: r.Gap})
		}
	}

	// withSeededRandom(seed, () => layout.run()). The handler of the event
	// layoutstop of the first run bends the edges and runs the layout again.
	a.run()
	a.bendEdges()
	a.run()

	// The await of mermaid: the mutation observer destroys the renderer.
	a.inst.Destroy()

	res = &ArchitectureResult{}
	// drawEdges
	for i, edge := range a.inst.Edges {
		s := edge.SourceEndpoint()
		m := edge.Midpoint()
		t := edge.TargetEndpoint()
		res.Edges = append(res.Edges, ArchitectureEdgeResult{
			ID:             edge.ID,
			Index:          a.edgeIndex[i],
			SourceEndpoint: Point{s.X, s.Y},
			Midpoint:       Point{m.X, m.Y},
			TargetEndpoint: Point{t.X, t.Y},
			HasRScratch:    true,
		})
	}
	// drawGroups
	for i, node := range a.inst.Nodes {
		r := ArchitectureNodeResult{ID: node.ID, Type: nodeTypes[i], Index: nodeIndex[i], X: math.NaN(), Y: math.NaN()}
		if nodeTypes[i] == "group" {
			bb := a.inst.BoundingBox([]*cy.Element{node})
			r.BoundingBox = Box{X1: bb.X1, Y1: bb.Y1, X2: bb.X2, Y2: bb.Y2, W: bb.W, H: bb.H}
		}
		res.Nodes = append(res.Nodes, r)
	}
	// positionNodes
	for i, node := range a.inst.Nodes {
		if nodeTypes[i] == "group" {
			continue
		}
		p := node.Position()
		res.Nodes[i].X, res.Nodes[i].Y = p.X, p.Y
	}

	if env.Trace != nil {
		var values []float64
		for _, n := range res.Nodes {
			if n.Type == "group" {
				values = append(values, n.BoundingBox.X1, n.BoundingBox.Y1, n.BoundingBox.X2, n.BoundingBox.Y2, n.BoundingBox.W, n.BoundingBox.H)
			} else {
				values = append(values, n.X, n.Y)
			}
		}
		for _, e := range res.Edges {
			values = append(values, e.SourceEndpoint.X, e.SourceEndpoint.Y, e.Midpoint.X, e.Midpoint.Y, e.TargetEndpoint.X, e.TargetEndpoint.Y)
		}
		env.Trace("cy.result", values)
	}
	return res, ""
}

// nodeStyle is the style of a node of the instance, from the style sheet of
// layoutArchitecture over the defaults of cytoscape:
//
//	node[label]   { text-valign: bottom, text-halign: center, font-size }
//	.node-service { label: data(label), width: data(width), height: data(height) }
//	.node-junction { width: data(width), height: data(height) }
//	.node-group   { padding }
//
// title is the data member label. Empty stands for undefined.
func (a *architectureLayout) nodeStyle(kind, title string) cy.NodeStyle {
	conf := a.in.Config
	style := cy.NodeStyle{
		Width: defaultNodeSize, Height: defaultNodeSize,
		Padding: 0, ParentPadding: defaultParentPadding,
		TextValign: "top",
		FontSize:   defaultFontSize,
	}
	if title != "" {
		style.TextValign = "bottom"
		style.FontSize = conf.FontSize
	}
	switch kind {
	case "group":
		style.Padding = conf.Padding
		style.ParentPadding = conf.Padding
	case "service":
		style.Label = title
		style.Width, style.Height = conf.IconSize, conf.IconSize
	case "junction":
		style.Width, style.Height = conf.IconSize, conf.IconSize
	}
	return style
}

// random is Math.random inside withSeededRandom for one run of the layout:
// the generator of architectureSeed.ts, or Math.random of the page for the
// seed 0.
func (a *architectureLayout) random() func() float64 {
	seed := a.in.Config.Seed
	if seed == 0 {
		if a.env.Random == nil {
			return func() float64 {
				bail.Out("architecture: the layout calls Math.random, and Random is nil")
				return 0
			}
		}
		return a.env.Random
	}
	state := js.ToUint32(seed)
	return func() float64 {
		state += 0x6d2b79f5
		t := state
		t = (t ^ (t >> 15)) * (t | 1)
		t ^= t + (t^(t>>7))*(t|61)
		return float64(t^(t>>14)) / 4294967296
	}
}

// run is layout.run() of fcose (src/fcose/index.js) for the options of
// mermaid.
func (a *architectureLayout) run() {
	inst := a.inst
	conf := a.in.Config
	eles := inst.Elements()
	trace := a.env.Trace

	if len(inst.Nodes) > 0 {
		// store component center
		boundingBox := inst.BoundingBox(eles)
		if trace != nil {
			trace("cy.center", []float64{boundingBox.X1, boundingBox.Y1, boundingBox.X2, boundingBox.Y2, boundingBox.W, boundingBox.H})
		}
		center := fcose.Position{X: boundingBox.X1 + float64(boundingBox.W/2), Y: boundingBox.Y1 + float64(boundingBox.H/2)}

		// What coseLayout and spectralLayout read of each node. The order of
		// the reads is the order of processChildrenList: the top nodes, and
		// the children of each node.
		inputs := make(map[*cy.Element]*fcose.NodeInput, len(inst.Nodes))
		var build func(node *cy.Element, parent *fcose.NodeInput) *fcose.NodeInput
		build = func(node *cy.Element, parent *fcose.NodeInput) *fcose.NodeInput {
			w, h := node.LayoutDimensions()
			pos := node.Position()
			n := &fcose.NodeInput{ID: node.ID, X: pos.X, Y: pos.Y, W: w, H: h, Padding: node.CSSPadding(), Parent: parent, Degree: node.Degree()}
			inputs[node] = n
			for _, child := range node.Children() {
				n.Children = append(n.Children, build(child, n))
			}
			return n
		}
		for _, node := range inst.Nodes {
			if node.Parent() == nil {
				build(node, nil)
			}
		}
		nodes := make([]*fcose.NodeInput, len(inst.Nodes))
		for i, node := range inst.Nodes {
			n := inputs[node]
			nodes[i] = n
			if conf.Randomize {
				n.Width, n.Height = node.Width(), node.Height()
				for _, other := range node.NeighborNodes() {
					n.Neighbors = append(n.Neighbors, inputs[other])
				}
			}
		}
		edges := make([]fcose.EdgeInput, len(inst.Edges))
		for i, edge := range inst.Edges {
			e := fcose.EdgeInput{ID: edge.ID, Source: edge.Source().ID, Target: edge.Target().ID}
			// idealEdgeLength and edgeElasticity of layoutArchitecture.
			if edge.Source().Parent() == edge.Target().Parent() {
				e.IdealLength = a.sameGroupIdealLength
				e.Elasticity = a.sameGroupElasticity
			} else {
				e.IdealLength = a.crossGroupIdealLength
				e.Elasticity = 0.001
			}
			edges[i] = e
		}

		positions, reason := fcose.Run(nodes, edges, fcose.Options{
			Randomize:      conf.Randomize,
			NumIter:        conf.NumIter,
			NodeSeparation: conf.NodeSeparation,
			Constraints:    a.constraints,
			Center:         center,
			Random:         a.random(),
			Trace:          trace,
			Cancelled:      a.env.Cancelled,
		})
		if reason != "" {
			bail.Out("%s", reason)
		}

		// layoutPositions: the positions of the nodes without children, and
		// the fit of the viewport, which asks for the bounding box of all
		// elements.
		inst.StartBatch()
		for _, node := range inst.Nodes {
			if node.IsParent() {
				continue
			}
			pos, ok := positions[node.ID]
			if !ok {
				bail.Out("architecture: a node without a position")
			}
			node.SetPosition(cy.Point{X: pos.X, Y: pos.Y})
		}
		inst.EndBatch()
		inst.BoundingBox(eles)
	}
	if trace != nil {
		trace("cy.fit", inst.TraceState())
	}
}

// bendEdges is the handler of the event layoutstop of layoutArchitecture:
// once the positions of the services are set, it adjusts the XY edges to
// have a 90deg bend.
func (a *architectureLayout) bendEdges() {
	inst := a.inst
	inst.StartBatch()
	for i, edge := range inst.Edges {
		sPos := edge.Source().Position()
		tPos := edge.Target().Position()
		sX, sY := sPos.X, sPos.Y
		tX, tY := tPos.X, tPos.Y
		if sX != tX && sY != tY {
			sEP := edge.SourceEndpoint()
			tEP := edge.TargetEndpoint()
			sourceDir := a.in.Edges[a.edgeIndex[i]].LhsDir
			var pointX, pointY float64
			if isArchitectureDirectionY(sourceDir) {
				pointX, pointY = sEP.X, tEP.Y
			} else {
				pointX, pointY = tEP.X, sEP.Y
			}
			distances, weights := getSegmentWeights(sEP, tEP, pointX, pointY)
			// cytoscape throws for a style value that is not a number.
			if !finite(distances) || !finite(weights) {
				bail.Out("architecture: a bend that is not a number")
			}
			if a.env.Trace != nil {
				a.env.Trace("cy.bypass", []float64{distances})
				a.env.Trace("cy.bypass", []float64{weights})
			}
			edge.SetSegments(distances, weights)
		}
	}
	inst.EndBatch()
}

// getSegmentWeights is getSegmentWeights of layoutArchitecture.
func getSegmentWeights(source, target cy.Point, pointX, pointY float64) (distances, weights float64) {
	sX, sY := source.X, source.Y
	tX, tY := target.X, target.Y

	d := (pointY - sY + (sX-pointX)*(sY-tY)/(sX-tX)) / math.Sqrt(1+js.Pow((sY-tY)/(sX-tX), 2))
	w := math.Sqrt(js.Pow(pointY-sY, 2) + js.Pow(pointX-sX, 2) - js.Pow(d, 2))

	distAB := math.Sqrt(js.Pow(tX-sX, 2) + js.Pow(tY-sY, 2))
	w = w / distAB

	delta1 := float64((tX-sX)*(pointY-sY)) - float64((tY-sY)*(pointX-sX))
	switch {
	case delta1 >= 0:
		delta1 = 1
	case delta1 < 0:
		delta1 = -1
	}
	delta2 := float64((tX-sX)*(pointX-sX)) + float64((tY-sY)*(pointY-sY))
	switch {
	case delta2 >= 0:
		delta2 = 1
	case delta2 < 0:
		delta2 = -1
	}

	d = math.Abs(d) * delta1
	w = w * delta2

	return d, w
}
