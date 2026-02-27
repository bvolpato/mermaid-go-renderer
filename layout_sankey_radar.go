package mermaid

import (
	"math"
	"sort"
	"strings"
)

var sankeyTableau10 = []string{
	"#4e79a7",
	"#f28e2c",
	"#e15759",
	"#76b7b2",
	"#59a14f",
	"#edc949",
	"#af7aa1",
	"#ff9da7",
	"#9c755f",
	"#bab0ab",
}

type sankeyNodeState struct {
	ID    string
	Value float64
	Rank  int
	X0    float64
	Y0    float64
	X1    float64
	Y1    float64

	In  []*sankeyLinkState
	Out []*sankeyLinkState
}

type sankeyLinkState struct {
	Source *sankeyNodeState
	Target *sankeyNodeState
	Value  float64
	Width  float64
	Y0     float64
	Y1     float64
}

func layoutSankeyFidelity(graph *Graph, theme Theme, config LayoutConfig) Layout {
	if len(graph.SankeyLinks) == 0 {
		return layoutGraphLike(graph, theme, config)
	}

	const (
		width       = 600.0
		height      = 400.0
		nodeWidth   = 10.0
		nodePadding = 25.0
	)

	nodes := make([]*sankeyNodeState, 0, 16)
	nodeByID := map[string]*sankeyNodeState{}
	links := make([]*sankeyLinkState, 0, len(graph.SankeyLinks))

	getNode := func(id string) *sankeyNodeState {
		if n, ok := nodeByID[id]; ok {
			return n
		}
		n := &sankeyNodeState{ID: id}
		nodeByID[id] = n
		nodes = append(nodes, n)
		return n
	}

	for _, l := range graph.SankeyLinks {
		if l.Value <= 0 {
			continue
		}
		source := getNode(l.Source)
		target := getNode(l.Target)
		link := &sankeyLinkState{
			Source: source,
			Target: target,
			Value:  l.Value,
		}
		source.Out = append(source.Out, link)
		target.In = append(target.In, link)
		links = append(links, link)
	}
	if len(nodes) == 0 || len(links) == 0 {
		return layoutGraphLike(graph, theme, config)
	}

	maxRank := assignSankeyRanks(nodes)
	columns := make([][]*sankeyNodeState, maxRank+1)
	for _, node := range nodes {
		if node.Rank < 0 {
			node.Rank = 0
		}
		if node.Rank > maxRank {
			node.Rank = maxRank
		}
		columns[node.Rank] = append(columns[node.Rank], node)
	}

	ky := computeSankeyScale(columns, height, nodePadding)
	if ky <= 0 || math.IsInf(ky, 0) || math.IsNaN(ky) {
		ky = 1
	}

	stepX := 0.0
	if maxRank > 0 {
		stepX = (width - nodeWidth) / float64(maxRank)
	}
	for _, col := range columns {
		y := 0.0
		for _, node := range col {
			node.X0 = float64(node.Rank) * stepX
			node.X1 = node.X0 + nodeWidth
			node.Y0 = y
			node.Y1 = y + node.Value*ky
			y = node.Y1 + nodePadding
		}
	}

	for i := 0; i < 8; i++ {
		computeSankeyLinkBreadths(nodes, ky)
		relaxSankeyRightToLeft(columns, 0.5)
		resolveSankeyCollisions(columns, height, nodePadding)
		computeSankeyLinkBreadths(nodes, ky)
		relaxSankeyLeftToRight(columns, 0.5)
		resolveSankeyCollisions(columns, height, nodePadding)
	}
	computeSankeyLinkBreadths(nodes, ky)

	layout := Layout{
		Kind:          DiagramSankey,
		Width:         width,
		Height:        height,
		ViewBoxX:      0,
		ViewBoxY:      0,
		ViewBoxWidth:  width,
		ViewBoxHeight: height,
	}

	colorByID := map[string]string{}
	for i, node := range nodes {
		color := sankeyTableau10[i%len(sankeyTableau10)]
		colorByID[node.ID] = color
		layout.SankeyNodes = append(layout.SankeyNodes, SankeyNodeLayout{
			ID:    node.ID,
			Value: node.Value,
			X0:    node.X0,
			Y0:    node.Y0,
			X1:    node.X1,
			Y1:    node.Y1,
			Color: color,
		})
	}

	for _, link := range links {
		midX := (link.Source.X1 + link.Target.X0) * 0.5
		path := "M" + formatFloat(link.Source.X1) + "," + formatFloat(link.Y0) +
			"C" + formatFloat(midX) + "," + formatFloat(link.Y0) +
			"," + formatFloat(midX) + "," + formatFloat(link.Y1) +
			"," + formatFloat(link.Target.X0) + "," + formatFloat(link.Y1)
		layout.SankeyLinks = append(layout.SankeyLinks, SankeyLinkLayout{
			SourceID:    link.Source.ID,
			TargetID:    link.Target.ID,
			Value:       link.Value,
			Width:       link.Width,
			X0:          link.Source.X1,
			Y0:          link.Y0,
			X1:          link.Target.X0,
			Y1:          link.Y1,
			Path:        path,
			SourceColor: colorByID[link.Source.ID],
			TargetColor: colorByID[link.Target.ID],
		})
	}

	return layout
}

func assignSankeyRanks(nodes []*sankeyNodeState) int {
	indegree := map[*sankeyNodeState]int{}
	for _, node := range nodes {
		indegree[node] = len(node.In)
		sumIn := 0.0
		for _, in := range node.In {
			sumIn += in.Value
		}
		sumOut := 0.0
		for _, out := range node.Out {
			sumOut += out.Value
		}
		node.Value = math.Max(sumIn, sumOut)
		if node.Value <= 0 {
			node.Value = 1
		}
	}

	queue := make([]*sankeyNodeState, 0, len(nodes))
	for _, node := range nodes {
		if indegree[node] == 0 {
			queue = append(queue, node)
		}
	}
	if len(queue) == 0 {
		queue = append(queue, nodes...)
	}

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		for _, out := range node.Out {
			if out.Target.Rank < node.Rank+1 {
				out.Target.Rank = node.Rank + 1
			}
			indegree[out.Target]--
			if indegree[out.Target] == 0 {
				queue = append(queue, out.Target)
			}
		}
	}

	maxRank := 0
	for _, node := range nodes {
		if node.Rank > maxRank {
			maxRank = node.Rank
		}
	}
	for _, node := range nodes {
		if len(node.Out) == 0 {
			node.Rank = maxRank
		}
	}
	for _, node := range nodes {
		if node.Rank > maxRank {
			maxRank = node.Rank
		}
	}
	return maxRank
}

func computeSankeyScale(columns [][]*sankeyNodeState, height, nodePadding float64) float64 {
	ky := math.Inf(1)
	for _, col := range columns {
		if len(col) == 0 {
			continue
		}
		sum := 0.0
		for _, node := range col {
			sum += node.Value
		}
		if sum <= 0 {
			continue
		}
		available := height - float64(len(col)-1)*nodePadding
		if available <= 0 {
			continue
		}
		ky = math.Min(ky, available/sum)
	}
	if math.IsInf(ky, 1) {
		return 1
	}
	return ky
}

func computeSankeyLinkBreadths(nodes []*sankeyNodeState, ky float64) {
	for _, node := range nodes {
		sort.Slice(node.Out, func(i, j int) bool {
			if node.Out[i].Target.Y0 == node.Out[j].Target.Y0 {
				return node.Out[i].Target.ID < node.Out[j].Target.ID
			}
			return node.Out[i].Target.Y0 < node.Out[j].Target.Y0
		})
		sy := 0.0
		for _, out := range node.Out {
			out.Width = out.Value * ky
			out.Y0 = node.Y0 + sy + out.Width*0.5
			sy += out.Width
		}

		sort.Slice(node.In, func(i, j int) bool {
			if node.In[i].Source.Y0 == node.In[j].Source.Y0 {
				return node.In[i].Source.ID < node.In[j].Source.ID
			}
			return node.In[i].Source.Y0 < node.In[j].Source.Y0
		})
		ty := 0.0
		for _, in := range node.In {
			in.Width = in.Value * ky
			in.Y1 = node.Y0 + ty + in.Width*0.5
			ty += in.Width
		}
	}
}

func relaxSankeyRightToLeft(columns [][]*sankeyNodeState, alpha float64) {
	for rank := len(columns) - 2; rank >= 0; rank-- {
		for _, node := range columns[rank] {
			if len(node.Out) == 0 {
				continue
			}
			sum := 0.0
			total := 0.0
			for _, out := range node.Out {
				center := out.Y1
				sum += center * out.Value
				total += out.Value
			}
			if total == 0 {
				continue
			}
			desiredCenter := sum / total
			currentCenter := (node.Y0 + node.Y1) * 0.5
			delta := (desiredCenter - currentCenter) * alpha
			node.Y0 += delta
			node.Y1 += delta
		}
	}
}

func relaxSankeyLeftToRight(columns [][]*sankeyNodeState, alpha float64) {
	for rank := 1; rank < len(columns)-1; rank++ {
		for _, node := range columns[rank] {
			if len(node.In) == 0 {
				continue
			}
			sum := 0.0
			total := 0.0
			for _, in := range node.In {
				center := in.Y0
				sum += center * in.Value
				total += in.Value
			}
			if total == 0 {
				continue
			}
			desiredCenter := sum / total
			currentCenter := (node.Y0 + node.Y1) * 0.5
			delta := (desiredCenter - currentCenter) * alpha
			node.Y0 += delta
			node.Y1 += delta
		}
	}
}

func resolveSankeyCollisions(columns [][]*sankeyNodeState, height, nodePadding float64) {
	for _, col := range columns {
		if len(col) == 0 {
			continue
		}
		sort.Slice(col, func(i, j int) bool {
			if col[i].Y0 == col[j].Y0 {
				return col[i].ID < col[j].ID
			}
			return col[i].Y0 < col[j].Y0
		})

		y := 0.0
		for _, node := range col {
			if node.Y0 < y {
				delta := y - node.Y0
				node.Y0 += delta
				node.Y1 += delta
			}
			y = node.Y1 + nodePadding
		}

		overflow := y - nodePadding - height
		if overflow <= 0 {
			continue
		}
		col[len(col)-1].Y0 -= overflow
		col[len(col)-1].Y1 -= overflow
		for i := len(col) - 2; i >= 0; i-- {
			limit := col[i+1].Y0 - nodePadding
			if col[i].Y1 > limit {
				delta := col[i].Y1 - limit
				col[i].Y0 -= delta
				col[i].Y1 -= delta
			}
		}
		if col[0].Y0 < 0 {
			shift := -col[0].Y0
			for _, node := range col {
				node.Y0 += shift
				node.Y1 += shift
			}
		}
	}
}

func layoutRadarFidelity(graph *Graph, theme Theme, config LayoutConfig) Layout {
	if len(graph.RadarAxes) == 0 || len(graph.RadarCurves) == 0 {
		return layoutGraphLike(graph, theme, config)
	}

	const (
		chartWidth      = 600.0
		chartHeight     = 600.0
		marginTop       = 50.0
		marginRight     = 50.0
		marginBottom    = 50.0
		marginLeft      = 50.0
		axisScaleFactor = 1.0
		axisLabelFactor = 1.05
		curveTension    = 0.17
	)

	totalWidth := chartWidth + marginLeft + marginRight
	totalHeight := chartHeight + marginTop + marginBottom
	radius := math.Min(chartWidth, chartHeight) * 0.5

	ticks := graph.RadarTicks
	if ticks <= 0 {
		ticks = 5
	}

	minValue := 0.0
	if graph.RadarMin != nil {
		minValue = *graph.RadarMin
	}

	maxValue := minValue + 1
	if graph.RadarMax != nil {
		maxValue = *graph.RadarMax
	} else {
		found := false
		for _, curve := range graph.RadarCurves {
			for _, entry := range curve.Entries {
				if !found || entry > maxValue {
					maxValue = entry
					found = true
				}
			}
		}
		if !found {
			maxValue = minValue + 1
		}
	}
	if maxValue <= minValue {
		maxValue = minValue + 1
	}

	layout := Layout{
		Kind:                  DiagramRadar,
		Width:                 totalWidth,
		Height:                totalHeight,
		ViewBoxX:              0,
		ViewBoxY:              0,
		ViewBoxWidth:          totalWidth,
		ViewBoxHeight:         totalHeight,
		RadarTitle:            graph.RadarTitle,
		RadarShowLegend:       graph.RadarShowLegend,
		RadarTicks:            ticks,
		RadarGraticule:        graph.RadarGraticule,
		RadarLegendX:          ((chartWidth*0.5 + marginRight) * 3.0) / 4.0,
		RadarLegendY:          (-(chartHeight*0.5 + marginTop) * 3.0) / 4.0,
		RadarLegendLineHeight: 20.0,
	}
	if layout.RadarGraticule == "" {
		layout.RadarGraticule = "circle"
	}
	for i := 1; i <= ticks; i++ {
		layout.RadarGraticuleRadii = append(layout.RadarGraticuleRadii, radius*float64(i)/float64(ticks))
	}

	numAxes := len(graph.RadarAxes)
	for i, axis := range graph.RadarAxes {
		angle := 2*math.Pi*float64(i)/float64(numAxes) - math.Pi*0.5
		layout.RadarAxes = append(layout.RadarAxes, RadarAxisLayout{
			Label: axis.Label,
			LineX: radius * axisScaleFactor * math.Cos(angle),
			LineY: radius * axisScaleFactor * math.Sin(angle),
			TextX: radius * axisLabelFactor * math.Cos(angle),
			TextY: radius * axisLabelFactor * math.Sin(angle),
		})
	}

	for i, curve := range graph.RadarCurves {
		if len(curve.Entries) != numAxes {
			continue
		}
		points := make([]Point, 0, numAxes)
		for j, entry := range curve.Entries {
			angle := 2*math.Pi*float64(j)/float64(numAxes) - math.Pi*0.5
			r := radarRelativeRadius(entry, minValue, maxValue, radius)
			points = append(points, Point{
				X: r * math.Cos(angle),
				Y: r * math.Sin(angle),
			})
		}
		curveLayout := RadarCurveLayout{
			Label: curve.Label,
			Class: "radarCurve-" + intString(i),
		}
		if layout.RadarGraticule == "polygon" {
			curveLayout.Path = radarPointsString(points)
			curveLayout.Polygon = true
		} else {
			curveLayout.Path = closedRoundCurve(points, curveTension)
		}
		layout.RadarCurves = append(layout.RadarCurves, curveLayout)
		layout.RadarLegend = append(layout.RadarLegend, curve.Label)
	}

	return layout
}

func radarRelativeRadius(value, minValue, maxValue, radius float64) float64 {
	clipped := math.Min(math.Max(value, minValue), maxValue)
	return radius * (clipped - minValue) / (maxValue - minValue)
}

func closedRoundCurve(points []Point, tension float64) string {
	if len(points) == 0 {
		return ""
	}
	n := len(points)
	d := "M" + formatFloat(points[0].X) + "," + formatFloat(points[0].Y)
	for i := 0; i < n; i++ {
		p0 := points[(i-1+n)%n]
		p1 := points[i]
		p2 := points[(i+1)%n]
		p3 := points[(i+2)%n]

		cp1x := p1.X + (p2.X-p0.X)*tension
		cp1y := p1.Y + (p2.Y-p0.Y)*tension
		cp2x := p2.X - (p3.X-p1.X)*tension
		cp2y := p2.Y - (p3.Y-p1.Y)*tension

		d += " C" + formatFloat(cp1x) + "," + formatFloat(cp1y) +
			" " + formatFloat(cp2x) + "," + formatFloat(cp2y) +
			" " + formatFloat(p2.X) + "," + formatFloat(p2.Y)
	}
	return d + " Z"
}

func radarPointsString(points []Point) string {
	parts := make([]string, 0, len(points))
	for _, point := range points {
		parts = append(parts, formatFloat(point.X)+","+formatFloat(point.Y))
	}
	return strings.Join(parts, " ")
}
