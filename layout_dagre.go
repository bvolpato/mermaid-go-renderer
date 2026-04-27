package mermaid

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/dagre"
)

func layoutGraphLikeDagre(astGraph *Graph, theme Theme, config LayoutConfig) Layout {
	if len(astGraph.NodeOrder) == 0 && !(astGraph.Kind == DiagramC4 && strings.TrimSpace(astGraph.C4Title) != "") {
		return layoutGeneric(astGraph, theme)
	}

	layout := Layout{Kind: astGraph.Kind}

	// C4 title-only diagram
	if len(astGraph.NodeOrder) == 0 {
		layout.Texts = append(layout.Texts, LayoutText{
			X:      200,
			Y:      20,
			Value:  astGraph.C4Title,
			Anchor: "middle",
			Size:   max(theme.FontSize+14, 30),
			Weight: "700",
			Color:  theme.PrimaryTextColor,
		})
		layout.Width = 400
		layout.Height = 60
		layout.ViewBoxX = 0
		layout.ViewBoxY = -10
		layout.ViewBoxWidth = layout.Width
		layout.ViewBoxHeight = layout.Height + 10
		addGraphPrimitives(&layout, theme)
		return layout
	}

	dg := dagre.NewGraph()

	dir := "TB"
	if astGraph.Direction == DirectionBottomTop {
		dir = "BT"
	}
	if astGraph.Direction == DirectionLeftRight {
		dir = "LR"
	}
	if astGraph.Direction == DirectionRightLeft {
		dir = "RL"
	}

	ranksep := 50.0
	nodesep := 50.0
	if config.RankSpacing > 0 {
		ranksep = config.RankSpacing
	}
	if config.NodeSpacing > 0 {
		nodesep = config.NodeSpacing
	}
	marginx := 8.0
	marginy := 8.0

	switch astGraph.Kind {
	case DiagramFlowchart:
		marginx = 8
		marginy = 8
	case DiagramState:
		nodesep = max(14, nodesep*0.55)
		if len(astGraph.FlowSubgraphs) > 0 {
			ranksep = max(52, ranksep*0.7)
		}
		marginx = 8
		marginy = 8
	case DiagramC4:
		nodesep = max(26, nodesep*0.6)
		ranksep = max(100, ranksep*1.25)
		marginx = 150
		marginy = 150
	case DiagramRequirement:
		nodesep = max(10, nodesep*0.2)
		ranksep = max(185, ranksep*3.7)
		marginx = 8
		marginy = 8
	case DiagramER:
		marginx = 8
		marginy = 8
	}

	dg.SetGraph(dagre.GraphLabel{
		RankDir: dir,
		NodeSep: nodesep,
		EdgeSep: 10,
		RankSep: ranksep,
		MarginX: marginx,
		MarginY: marginy,
	})

	// Track composite state IDs
	compositeStateIDs := map[string]struct{}{}
	if astGraph.Kind == DiagramState {
		for _, subgraph := range astGraph.FlowSubgraphs {
			if strings.TrimSpace(subgraph.ID) != "" {
				compositeStateIDs[subgraph.ID] = struct{}{}
			}
		}
	}

	// Add nodes
	for _, v := range astGraph.NodeOrder {
		node := astGraph.Nodes[v]

		// Composite state nodes: small placeholder, dagre computes real size
		if _, isComposite := compositeStateIDs[v]; isComposite {
			dg.SetNode(v, &dagre.NodeLabel{Width: 1, Height: 1})
			continue
		}

		// State start/end circles
		if astGraph.Kind == DiagramState &&
			(node.Shape == ShapeCircle || node.Shape == ShapeDoubleCircle) &&
			strings.TrimSpace(node.Label) == "" {
			dg.SetNode(v, &dagre.NodeLabel{Width: 14, Height: 14})
			continue
		}

		w, h := dagreNodeSize(astGraph, node, theme, config)
		dg.SetNode(v, &dagre.NodeLabel{Width: w, Height: h})
	}

	// Add edges
	for i, e := range astGraph.Edges {
		if e.From == "" || e.To == "" {
			continue
		}
		minLen := 1
		if strings.Contains(e.Label, "\n") {
			minLen = 2
		}
		edgeName := fmt.Sprintf("%d", i)

		labelW := 0.0
		labelH := 0.0
		if e.Label != "" {
			labelW = measureTextWidth(e.Label, config.FastTextMetrics) + 8
			labelH = 20.0
		}

		dg.SetEdge(dagre.Edge{V: e.From, W: e.To, Name: edgeName}, &dagre.EdgeLabel{
			MinLen:   minLen,
			Weight:   1,
			Width:    labelW,
			Height:   labelH,
			LabelPos: "c",
		})
	}

	// Register subgraphs (flowchart subgraphs, composite states)
	for _, sg := range astGraph.FlowSubgraphs {
		dg.SetNode(sg.ID, &dagre.NodeLabel{Width: 0, Height: 0})
		for _, child := range sg.NodeIDs {
			dg.SetParent(child, sg.ID)
		}
	}

	dagre.Layout(dg)

	// Build back the visual layout
	minX := math.MaxFloat64
	minY := math.MaxFloat64
	maxX := -math.MaxFloat64
	maxY := -math.MaxFloat64

	for _, v := range astGraph.NodeOrder {
		dn := dg.Node(v)
		if dn == nil {
			continue
		}
		tlX := dn.X - dn.Width/2
		tlY := dn.Y - dn.Height/2

		minX = min(minX, tlX)
		minY = min(minY, tlY)
		maxX = max(maxX, dn.X+dn.Width/2)
		maxY = max(maxY, dn.Y+dn.Height/2)

		astNode := astGraph.Nodes[v]
		shape := astNode.Shape
		label := astNode.Label

		if _, isComposite := compositeStateIDs[v]; isComposite {
			shape = ShapeHidden
			label = ""
		}

		layout.Nodes = append(layout.Nodes, NodeLayout{
			ID:          v,
			Label:       label,
			Shape:       shape,
			X:           tlX,
			Y:           tlY,
			W:           dn.Width,
			H:           dn.Height,
			Fill:        astNode.Fill,
			Stroke:      astNode.Stroke,
			StrokeWidth: astNode.StrokeWidth,
		})
	}

	// Build node index for edge endpoint computation
	nodeIndex := map[string]NodeLayout{}
	for _, node := range layout.Nodes {
		nodeIndex[node.ID] = node
	}

	// Subgraphs as clusters
	for _, sg := range astGraph.FlowSubgraphs {
		dn := dg.Node(sg.ID)
		if dn == nil || dn.Width == 0 || dn.Height == 0 {
			continue
		}
		tlX := dn.X - dn.Width/2
		tlY := dn.Y - dn.Height/2
		clusterW := dn.Width
		clusterH := dn.Height

		if astGraph.Kind == DiagramFlowchart && len(sg.NodeIDs) > 0 {
			minChildX := math.Inf(1)
			minChildY := math.Inf(1)
			maxChildX := math.Inf(-1)
			maxChildY := math.Inf(-1)
			for _, childID := range sg.NodeIDs {
				child, ok := nodeIndex[childID]
				if !ok {
					continue
				}
				minChildX = min(minChildX, child.X)
				minChildY = min(minChildY, child.Y)
				maxChildX = max(maxChildX, child.X+child.W)
				maxChildY = max(maxChildY, child.Y+child.H)
			}
			if !math.IsInf(minChildX, 1) && !math.IsInf(minChildY, 1) {
				clusterPadX := 30.0
				clusterPadTop := 15.0
				clusterPadBottom := 15.0
				tlX = minChildX - clusterPadX
				tlY = minChildY - clusterPadTop
				clusterW = (maxChildX - minChildX) + clusterPadX*2
				clusterH = (maxChildY - minChildY) + clusterPadTop + clusterPadBottom
				minLabelW := measureTextWidth(sg.Label, config.FastTextMetrics) + 28.0
				if clusterW < minLabelW {
					extra := minLabelW - clusterW
					tlX -= extra / 2
					clusterW = minLabelW
				}
			}
		}

		minX = min(minX, tlX)
		minY = min(minY, tlY)
		maxX = max(maxX, tlX+clusterW)
		maxY = max(maxY, tlY+clusterH)

		layout.Rects = append(layout.Rects, LayoutRect{
			Class:         "cluster",
			X:             tlX,
			Y:             tlY,
			W:             clusterW,
			H:             clusterH,
			RX:            6,
			RY:            6,
			Fill:          "rgba(255, 255, 222, 0.5)",
			Stroke:        "rgba(170, 170, 51, 0.2)",
			StrokeWidth:   1,
			StrokeOpacity: 1,
		})
		labelY := tlY + 13
		if astGraph.Kind == DiagramFlowchart {
			labelY = tlY + 18
		}
		layout.Texts = append(layout.Texts, LayoutText{
			Class:            "cluster-label",
			X:                tlX + clusterW/2,
			Y:                labelY,
			Value:            sg.Label,
			Anchor:           "middle",
			Size:             max(11, theme.FontSize-1),
			Color:            theme.PrimaryTextColor,
			DominantBaseline: "middle",
		})
	}

	// Edges via dagre path points
	for i, e := range astGraph.Edges {
		if e.From == "" || e.To == "" {
			continue
		}
		edgeName := fmt.Sprintf("%d", i)
		dl := dg.EdgeByKey(dagre.Edge{V: e.From, W: e.To, Name: edgeName})
		if dl == nil {
			continue
		}

		var dBuilder strings.Builder
		for pi, p := range dl.Points {
			if pi == 0 {
				dBuilder.WriteString("M " + strconv.FormatFloat(p.X, 'f', 2, 64) + " " + strconv.FormatFloat(p.Y, 'f', 2, 64))
			} else {
				dBuilder.WriteString(" L " + strconv.FormatFloat(p.X, 'f', 2, 64) + " " + strconv.FormatFloat(p.Y, 'f', 2, 64))
			}
		}

		if astGraph.Kind == DiagramFlowchart {
			// Flowcharts: use proper CSS classes and markers
			strokeWidth := 2.0
			dashArray := ""
			thicknessClass := "edge-thickness-normal"
			patternClass := "edge-pattern-solid"

			if e.Style == EdgeDotted {
				dashArray = "5,4"
				patternClass = "edge-pattern-dotted"
			} else if e.Style == EdgeThick {
				strokeWidth = 3
				thicknessClass = "edge-thickness-thick"
			}

			markerStart := e.MarkerStart
			markerEnd := e.MarkerEnd
			if markerStart == "" && e.ArrowStart {
				markerStart = "my-svg_flowchart-v2-pointStart"
			}
			if markerEnd == "" && (e.ArrowEnd || e.Directed) {
				markerEnd = "my-svg_flowchart-v2-pointEnd"
			}

			pathID := "L_" + sanitizeID(e.From, e.From) + "_" + sanitizeID(e.To, e.To) + "_" + strconv.Itoa(i)
			layout.Paths = append(layout.Paths, LayoutPath{
				ID:          pathID,
				D:           dBuilder.String(),
				Class:       thicknessClass + " " + patternClass + " flowchart-link",
				Fill:        "none",
				Stroke:      theme.LineColor,
				StrokeWidth: strokeWidth,
				DashArray:   dashArray,
				MarkerStart: markerStart,
				MarkerEnd:   markerEnd,
			})
		} else {
			edgeClass := "edgePath"
			path := LayoutPath{
				ID:          e.From + "-" + e.To + "-" + strconv.Itoa(i),
				D:           dBuilder.String(),
				Class:       edgeClass,
				MarkerStart: e.MarkerStart,
				MarkerEnd:   e.MarkerEnd,
			}

			if e.Style == EdgeDotted {
				path.DashArray = "3,3"
			} else if e.Style == EdgeThick {
				path.StrokeWidth = 3
			}

			layout.Paths = append(layout.Paths, path)
		}

		// Emit edge layout entries (used for edge label positioning in render.go)
		from, okFrom := nodeIndex[e.From]
		to, okTo := nodeIndex[e.To]
		if okFrom && okTo {
			x1, y1, x2, y2 := edgeEndpoints(from, to, astGraph.Direction)
			layout.Edges = append(layout.Edges, EdgeLayout{
				From:        e.From,
				To:          e.To,
				Label:       e.Label,
				D:           dagreEdgePath(dl.Points),
				X1:          x1,
				Y1:          y1,
				X2:          x2,
				Y2:          y2,
				Style:       e.Style,
				ArrowStart:  e.ArrowStart,
				ArrowEnd:    e.ArrowEnd || e.Directed,
				MarkerStart: e.MarkerStart,
				MarkerEnd:   e.MarkerEnd,
			})
		}

		// Edge labels — for flowcharts, addGraphPrimitives handles edge labels
		// via the edgeLabels/edgeLabel rendering path, so skip here to avoid
		// duplicate text rendering.
		if e.Label != "" && dl.HasX && dl.HasY && astGraph.Kind != DiagramFlowchart {
			textL := LayoutText{
				X:      dl.X,
				Y:      dl.Y,
				Value:  e.Label,
				Anchor: "middle",
				Size:   max(11, theme.FontSize-2),
				Color:  theme.PrimaryTextColor,
			}
			layout.Texts = append(layout.Texts, textL)
		}
	}

	viewBoxPad := 10.0
	if astGraph.Kind == DiagramFlowchart {
		viewBoxPad = 8.0
	}
	layout.ViewBoxX = minX - viewBoxPad
	layout.ViewBoxY = minY - viewBoxPad
	layout.ViewBoxWidth = (maxX - minX) + viewBoxPad*2
	layout.ViewBoxHeight = (maxY - minY) + viewBoxPad*2

	if astGraph.Kind == DiagramC4 {
		layout.ViewBoxY = minY - 70
		layout.ViewBoxHeight = (maxY - minY) + 80
	}

	// State, ER, and Requirement diagrams: mmdc always uses viewBox origin at 0,0.
	if astGraph.Kind == DiagramState || astGraph.Kind == DiagramER || astGraph.Kind == DiagramRequirement {
		layout.ViewBoxWidth = maxX + viewBoxPad
		layout.ViewBoxHeight = maxY + viewBoxPad
		layout.ViewBoxX = 0
		layout.ViewBoxY = 0
	}

	layout.Width = layout.ViewBoxWidth
	layout.Height = layout.ViewBoxHeight

	// C4 title (after width is computed so X aligns to center)
	if astGraph.Kind == DiagramC4 && strings.TrimSpace(astGraph.C4Title) != "" {
		layout.Texts = append(layout.Texts, LayoutText{
			X:      layout.Width / 2,
			Y:      20,
			Value:  astGraph.C4Title,
			Anchor: "middle",
			Size:   max(theme.FontSize+14, 30),
			Weight: "700",
			Color:  theme.PrimaryTextColor,
		})
	}

	applyAspectRatio(&layout, config.PreferredAspectRatio)
	addGraphPrimitives(&layout, theme)

	return layout
}

// dagreNodeSize computes width/height for a node based on diagram type.
func dagreNodeSize(g *Graph, node Node, theme Theme, config LayoutConfig) (float64, float64) {
	if g.Kind == DiagramFlowchart {
		return mermaidFlowchartNodeSize(node, config)
	}

	minW := 50.0
	maxW := 300.0
	paddingW := 40.0
	baseHeight := 40.0

	switch g.Kind {
	case DiagramState:
		minW = 49
		maxW = 170
		paddingW = 13
		baseHeight = 40
	case DiagramC4:
		minW = 216
		maxW = 420
		paddingW = 56
		baseHeight = 60
	case DiagramRequirement:
		minW = 160
		maxW = 320
		paddingW = 30
		baseHeight = 76
	}

	labelWidth := measureTextWidth(node.Label, config.FastTextMetrics)

	// Multi-line labels for C4 and Requirement
	if g.Kind == DiagramC4 || g.Kind == DiagramRequirement {
		labelWidth = 0
		for _, line := range splitLinesPreserve(node.Label) {
			labelWidth = max(labelWidth, measureTextWidth(line, config.FastTextMetrics))
		}
	}

	w := clamp(labelWidth+paddingW, minW, maxW)

	h := baseHeight
	if g.Kind == DiagramC4 {
		lineCount := max(1, len(splitLinesPreserve(node.Label)))
		if node.Shape == ShapePerson {
			h = 105
		} else if lineCount > 2 {
			h = max(h, 60+float64(lineCount-2)*20)
		}
	} else if g.Kind == DiagramRequirement {
		lineCount := max(1, len(splitLinesPreserve(node.Label)))
		h = max(76, float64(lineCount)*max(15, theme.FontSize*1.1)+24)
	}

	return w, h
}
