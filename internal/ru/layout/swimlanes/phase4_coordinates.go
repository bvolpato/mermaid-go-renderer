// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/phase4.coordinates.ts (MIT).

package swimlanes

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// CoordOptions are the options of assignCoordinates. A gap that is
// undefined is NaN.
type CoordOptions struct {
	// LayerGap is the vertical distance between layers.
	LayerGap float64
	// NodeGap is the horizontal gap between siblings inside a lane.
	NodeGap float64
	// LaneGap is the horizontal gap between lanes (clusters).
	LaneGap float64
	// Direction is the layout direction, or "" when it is undefined.
	Direction string
	// LaneOrder is nil when it is undefined.
	LaneOrder []string
}

// nullLaneKey is the key of the null lane in the laneWidth record of
// upstream: `(laneWidth as any).null`. A lane with the id "null" shares it.
const nullLaneKey = "null"

// assignCoordinates is Sugiyama phase 4. opts can be nil.
func assignCoordinates(ordered *OrderedLayers, gWithDummies *Graph, opts *CoordOptions) *Coordinates {
	if opts == nil {
		opts = &CoordOptions{LayerGap: ru.Undef, NodeGap: ru.Undef, LaneGap: ru.Undef}
	}
	layerGap := ru.NN(opts.LayerGap, COORDINATES_DEFAULT_LAYER_GAP)
	nodeGap := ru.NN(opts.NodeGap, COORDINATES_DEFAULT_NODE_GAP)
	laneGap := ru.NN(opts.LaneGap, nodeGap*2)
	direction := opts.Direction
	if direction == "" {
		direction = "TB"
	}
	isHorizontal := direction == "LR" || direction == "RL"

	layers := ordered.Layers

	x := map[NodeId]float64{}
	y := map[NodeId]float64{}

	getWidth := func(id NodeId) float64 {
		node := gWithDummies.NodeById.Get(id)
		if node == nil {
			return 0
		}
		return ru.NN(node.Width, 0)
	}
	getHeight := func(id NodeId) float64 {
		node := gWithDummies.NodeById.Get(id)
		if node == nil {
			return 0
		}
		return ru.NN(node.Height, 0)
	}
	topLaneOf := createTopLaneResolver(gWithDummies)
	laneOrderGlobal := resolveTopLaneOrder(gWithDummies, opts.LaneOrder)

	layerHeights := make([]float64, len(layers))
	for i, layer := range layers {
		m := 0.0
		for _, v := range layer {
			m = js.Max(m, getHeight(v))
		}
		layerHeights[i] = m
	}

	// LR/RL transforms turn width into horizontal span, so widen layer gaps
	// up front.
	extraLayerGaps := []float64{}
	if isHorizontal {
		maxWidth := func(layer []NodeId) float64 {
			m := 0.0
			for _, v := range layer {
				m = js.Max(m, getWidth(v))
			}
			return m
		}
		for i := 0; i+1 < len(layers); i++ {
			thisLayerMaxWidth := maxWidth(layers[i])
			nextLayerMaxWidth := maxWidth(layers[i+1])
			thisLayerMaxHeight := layerHeights[i]
			nextLayerMaxHeight := layerHeights[i+1]

			normalSpacing := float64(thisLayerMaxHeight/2) + float64(nextLayerMaxHeight/2)
			requiredSpacing := float64((thisLayerMaxWidth + nextLayerMaxWidth) / 2)
			extraNeeded := js.Max(0, requiredSpacing-normalSpacing-layerGap)
			extraLayerGaps = append(extraLayerGaps, extraNeeded)
		}
	}

	// The lane "" is the null lane.
	lanesUsedSet := map[string]bool{}
	for _, layer := range layers {
		for _, id := range layer {
			lanesUsedSet[topLaneOf(id)] = true
		}
	}
	hasNullLane := lanesUsedSet[""]
	lanesUsed := []string{}
	for _, L := range laneOrderGlobal {
		if lanesUsedSet[L] {
			lanesUsed = append(lanesUsed, L)
		}
	}
	laneOrderColumns := []string{}
	if hasNullLane {
		laneOrderColumns = append(laneOrderColumns, "")
	}
	laneOrderColumns = append(laneOrderColumns, lanesUsed...)

	laneWidth := map[string]float64{}
	for _, L := range lanesUsed {
		laneWidth[L] = 0
	}
	if hasNullLane {
		laneWidth[nullLaneKey] = 0
	}
	sumWidths := func(ids []NodeId) float64 {
		s := 0.0
		for _, id := range ids {
			s = s + getWidth(id)
		}
		return s
	}
	for _, layer := range layers {
		perLane := js.NewObj[[]string]()
		nullIds := []string{}
		for _, id := range layer {
			L := topLaneOf(id)
			if L == "" {
				nullIds = append(nullIds, id)
			} else {
				perLane.Set(L, append(perLane.Get(L), id))
			}
		}
		perLane.Each(func(L string, ids []string) {
			total := sumWidths(ids) + float64(nodeGap*float64(max(0, len(ids)-1)))
			laneWidth[L] = js.Max(laneWidth[L], total)
		})
		if hasNullLane && len(nullIds) > 0 {
			totalNull := sumWidths(nullIds) + float64(nodeGap*float64(max(0, len(nullIds)-1)))
			laneWidth[nullLaneKey] = js.Max(laneWidth[nullLaneKey], totalNull)
		}
	}

	centerX := map[string]float64{}
	{
		widths := make([]float64, len(laneOrderColumns))
		for i, L := range laneOrderColumns {
			if L == "" {
				widths[i] = laneWidth[nullLaneKey]
			} else {
				widths[i] = laneWidth[L]
			}
		}
		sum := 0.0
		for _, w := range widths {
			sum = sum + w
		}
		totalW := sum + float64(laneGap*float64(max(0, len(laneOrderColumns)-1)))
		cursor := -totalW / 2
		for i := 0; i < len(laneOrderColumns); i++ {
			L := laneOrderColumns[i]
			w := widths[i]
			cx := cursor + float64(w/2)
			centerX[L] = cx
			cursor += w
			if i < len(laneOrderColumns)-1 {
				cursor += laneGap
			}
		}
	}

	yOffset := 0.0
	for li, layer := range layers {
		layerH := layerHeights[li]

		byLane := map[string][]NodeId{}
		for _, id := range layer {
			laneId := topLaneOf(id)
			byLane[laneId] = append(byLane[laneId], id)
		}

		for _, L := range laneOrderColumns {
			nodesInLane := byLane[L]
			if len(nodesInLane) == 0 {
				continue
			}
			cx := centerX[L]
			if len(nodesInLane) == 1 {
				id := nodesInLane[0]
				x[id] = cx
				y[id] = yOffset + float64(layerH/2)
			} else {
				// Preserve phase 3 order while spreading nodes around the lane
				// center.
				widths := make([]float64, len(nodesInLane))
				total := 0.0
				for i, id := range nodesInLane {
					widths[i] = getWidth(id)
					total = total + widths[i]
				}
				total = total + float64(nodeGap*float64(len(nodesInLane)-1))
				start := cx - float64(total/2)
				for i, id := range nodesInLane {
					w := widths[i]
					x[id] = start + float64(w/2)
					y[id] = yOffset + float64(layerH/2)
					start += w + nodeGap
				}
			}
		}

		extraGap := 0.0
		if li < len(extraLayerGaps) {
			extraGap = extraLayerGaps[li]
		}
		yOffset += layerH + layerGap + extraGap
	}

	// Align dummy chains for each original edge: set dummy x to the midpoint
	// between src and dst.
	byRef := js.NewMap[string, []*EdgeRef]()
	for _, e := range gWithDummies.Edges {
		rid := e.Ref.ID
		byRef.Set(rid, append(byRef.Get(rid), e))
	}
	for _, chainEdges := range byRef.Values() {
		if len(chainEdges) == 0 {
			continue
		}
		ref := chainEdges[0].Ref
		src := ref.Start
		dst := ref.End
		// src == null || dst == null. An edge of the graph view always has
		// both ends.
		midX := js.Round((x[src] + x[dst]) / 2)
		involved := js.NewSet[NodeId]()
		for _, e := range chainEdges {
			involved.Add(e.Src)
			involved.Add(e.Dst)
		}
		for _, vid := range involved.Values() {
			if vid == src || vid == dst {
				continue
			}
			node := gWithDummies.NodeById.Get(vid)
			if node != nil && node.IsDummy {
				x[vid] = midX
			}
		}
	}

	return &Coordinates{X: x, Y: y}
}
