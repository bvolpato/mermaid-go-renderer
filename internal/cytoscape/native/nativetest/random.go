// Random inputs of the two layouts.

package nativetest

import (
	"math"
	"strconv"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native"
)

// rng draws the numbers of an input.
type rng struct{ m *Mulberry32 }

func (r rng) float() float64 { return r.m.Random() }

// between returns an integer in [lo, hi].
func (r rng) between(lo, hi int) int {
	if hi <= lo {
		return lo
	}
	return lo + int(math.Floor(scale(r.float(), float64(hi-lo+1))))
}

// scale is x * n. It is not inlined, so that the product has one rounding
// on every machine.
//
//go:noinline
func scale(x, n float64) float64 { return x * n }

func (r rng) chance(p float64) bool { return r.float() < p }

// Mindmap is a random input of the cose-bilkent layout.
type Mindmap struct {
	// Kind is tree (what the mindmap db builds), forest, general (a graph
	// with cycles) or special (edges in parallel, an edge from a node to
	// itself, nodes without edges, sizes of zero).
	Kind  string
	Nodes []native.Node
	Edges []native.Edge
}

// RandomMindmap returns the input for a seed. Seven of ten inputs are trees
// of 1 to 200 nodes with varied sizes and depths.
func RandomMindmap(seed uint32) Mindmap {
	r := rng{NewMulberry32(seed*2654435761+12345, 0)}
	kinds := []string{"tree", "tree", "tree", "tree", "tree", "tree", "tree", "forest", "general", "special"}
	m := Mindmap{Kind: kinds[seed%10]}
	maxNodes := []int{1, 2, 3, 4, 6, 9, 14, 22, 35, 60, 100, 200}[r.between(0, 11)]
	switch m.Kind {
	case "forest":
		maxNodes = min(maxNodes, 60)
	case "general":
		maxNodes = min(maxNodes, 35)
	case "special":
		maxNodes = min(maxNodes, 14)
	}
	n := r.between(1, maxNodes)
	depthBias := r.float()
	newTree := 0.0
	if m.Kind == "forest" {
		newTree = 0.05 + float64(r.float()*0.2)
	}
	// first is the first node of the current tree.
	first := 0
	for i := 0; i < n; i++ {
		var w, h float64
		if r.chance(0.5) {
			w = 30 + float64(math.Floor(r.float()*4000)/16)
		} else {
			w = float64(r.between(20, 300))
		}
		if r.chance(0.7) {
			h = 34
		} else {
			h = 34 + float64(math.Floor(r.float()*1600)/16)
		}
		if m.Kind == "special" && r.chance(0.15) {
			w = 0
		}
		if m.Kind == "special" && r.chance(0.15) {
			h = 0
		}
		m.Nodes = append(m.Nodes, native.Node{ID: strconv.Itoa(i), Width: w, Height: h})
		if i == 0 {
			continue
		}
		if r.chance(newTree) {
			first = i
			continue
		}
		// A deep tree takes a late node as the parent, a flat tree an early
		// one.
		var p int
		switch {
		case depthBias < 0.33:
			p = r.between(first, min(i-1, first+2))
		case depthBias < 0.66:
			p = r.between(first, i-1)
		default:
			p = max(first, i-1-r.between(0, 2))
		}
		m.Edges = append(m.Edges, native.Edge{ID: "edge_" + strconv.Itoa(p) + "_" + strconv.Itoa(i), Start: strconv.Itoa(p), End: strconv.Itoa(i)})
	}
	switch m.Kind {
	case "general":
		extra := r.between(1, max(1, n/2))
		for k := 0; k < extra; k++ {
			a, b := r.between(0, n-1), r.between(0, n-1)
			m.Edges = append(m.Edges, native.Edge{ID: "x_" + strconv.Itoa(k), Start: strconv.Itoa(a), End: strconv.Itoa(b)})
		}
	case "special":
		switch r.between(0, 3) {
		case 0:
			// An edge in parallel to another one, in one of the two
			// directions.
			if len(m.Edges) > 0 {
				e := m.Edges[r.between(0, len(m.Edges)-1)]
				if r.chance(0.5) {
					m.Edges = append(m.Edges, native.Edge{ID: "p", Start: e.Start, End: e.End})
				} else {
					m.Edges = append(m.Edges, native.Edge{ID: "p", Start: e.End, End: e.Start})
				}
			}
		case 1:
			a := strconv.Itoa(r.between(0, n-1))
			m.Edges = append(m.Edges, native.Edge{ID: "self", Start: a, End: a})
		case 2:
			// Nodes without edges.
			more := r.between(1, 3)
			for k := 0; k < more; k++ {
				m.Nodes = append(m.Nodes, native.Node{ID: "alone" + strconv.Itoa(k), Width: float64(r.between(10, 100)), Height: float64(r.between(10, 60))})
			}
		}
		// The last case changes only the sizes.
	}
	return m
}

// FakeMeasure is a MeasureText for random inputs: a width that depends on
// the font size and on the text, with a fraction.
func FakeMeasure(font, text string) float64 {
	size := 16.0
	if i := strings.Index(font, "px"); i > 0 {
		j := strings.LastIndexByte(font[:i], ' ')
		if f, err := strconv.ParseFloat(font[j+1:i], 64); err == nil {
			size = f
		}
	}
	sum := 0
	for _, b := range []byte(text) {
		sum += int(b)
	}
	return float64(size*float64(0.5*float64(len(text)))) + float64(float64(sum%17)/16)
}

// Architecture is a random input of the fcose layout.
type Architecture struct {
	// Kind is grid (the constraints follow from a placement of the nodes on
	// a grid, as in mermaid), or wild (constraints that can contradict each
	// other, and edges that mermaid does not make).
	Kind  string
	Input native.ArchitectureInput
}

var (
	archOpposite = map[string]string{"L": "R", "R": "L", "T": "B", "B": "T"}
	archShift    = map[string][2]int{"L": {-1, 0}, "R": {1, 0}, "T": {0, 1}, "B": {0, -1}}
	archDirs     = []string{"L", "R", "T", "B"}
)

// RandomArchitecture returns the input for a seed: services, junctions,
// nested groups, edges of every direction, and the alignment and relative
// placement constraints of the grid on which the nodes of each component
// sit.
func RandomArchitecture(seed uint32) Architecture {
	r := rng{NewMulberry32(seed*2246822519+977, 0)}
	a := Architecture{Kind: "grid"}
	if seed%8 == 7 {
		a.Kind = "wild"
	}
	in := &a.Input
	maxNodes := []int{1, 2, 3, 4, 5, 6, 8, 10, 12, 16, 20, 30, 40}[r.between(0, 12)]
	n := r.between(1, maxNodes)
	iconSize := 80.0
	if r.chance(0.2) {
		iconSize = []float64{40, 64, 100, 37.5}[r.between(0, 3)]
	}
	mult := 1.5
	if r.chance(0.2) {
		mult = []float64{1, 2.5, 0.7}[r.between(0, 2)]
	}
	in.Config = native.ArchitectureConfig{
		IconSize: iconSize, FontSize: 16, Padding: 40,
		IdealEdgeLengthMultiplier: mult, EdgeElasticity: 0.45,
		NodeSeparation: 75, NumIter: 2500, Seed: 1,
	}
	if r.chance(0.2) {
		in.Config.Padding = float64(r.between(0, 60))
	}
	if r.chance(0.15) {
		in.Config.FontSize = []float64{12, 20, 14.5}[r.between(0, 2)]
	}
	if r.chance(0.1) {
		in.Config.Seed = float64(r.between(0, 1000))
	}
	if r.chance(0.1) {
		in.Config.NumIter = []float64{100, 500, 1000}[r.between(0, 2)]
	}
	if r.chance(0.1) {
		in.Config.EdgeElasticity = []float64{0.1, 0.9}[r.between(0, 1)]
	}
	if r.chance(0.12) {
		in.Config.Randomize = true
	}

	groupCount := 0
	if !r.chance(0.3) {
		groupCount = r.between(1, max(1, min(8, n/2)))
	}
	for g := 0; g < groupCount; g++ {
		group := native.ArchitectureNode{ID: "g" + strconv.Itoa(g)}
		if r.chance(0.8) {
			group.Title = "Group " + strconv.Itoa(g)
		}
		if g > 0 && r.chance(0.4) {
			group.In = "g" + strconv.Itoa(r.between(0, g-1))
		}
		in.Groups = append(in.Groups, group)
	}
	ids := make([]string, n)
	groupOf := map[string]string{}
	for i := 0; i < n; i++ {
		inGroup := ""
		if groupCount > 0 && r.chance(0.7) {
			inGroup = "g" + strconv.Itoa(r.between(0, groupCount-1))
		}
		if r.chance(0.15) {
			ids[i] = "j" + strconv.Itoa(i)
			in.Junctions = append(in.Junctions, native.ArchitectureNode{ID: ids[i], In: inGroup})
		} else {
			ids[i] = "s" + strconv.Itoa(i)
			s := native.ArchitectureNode{ID: ids[i], Icon: "server", In: inGroup}
			if r.chance(0.85) {
				s.Title = "Service " + strconv.Itoa(i)
				if r.chance(0.2) {
					s.Title += " with a long title"
				}
			}
			in.Services = append(in.Services, s)
		}
		groupOf[ids[i]] = inGroup
	}

	used := map[string]bool{}
	addEdge := func(from, to, fromDir, toDir string) {
		key := from + "-" + to
		if from == to || used[key] {
			return
		}
		used[key] = true
		e := native.ArchitectureEdge{LhsID: from, RhsID: to, LhsDir: fromDir, RhsDir: toDir}
		if r.chance(0.15) {
			e.Title = "edge " + strconv.Itoa(len(in.Edges))
		}
		in.Edges = append(in.Edges, e)
	}
	gap := iconSize * mult
	in.Alignment.Horizontal = [][]string{}
	in.Alignment.Vertical = [][]string{}
	for at := 0; at < n; {
		size := min(n-at, r.between(1, 9))
		type cell [2]int
		cells := map[cell]string{{0, 0}: ids[at]}
		pos := map[string]cell{ids[at]: {0, 0}}
		placed := []string{ids[at]}
		for k := 1; k < size; k++ {
			id := ids[at+k]
			for tries := 0; tries < 50; tries++ {
				from := placed[r.between(0, len(placed)-1)]
				dir := archDirs[r.between(0, 3)]
				p := cell{pos[from][0] + archShift[dir][0], pos[from][1] + archShift[dir][1]}
				if _, taken := cells[p]; !taken {
					cells[p] = id
					pos[id] = p
					placed = append(placed, id)
					addEdge(from, id, dir, archOpposite[dir])
					break
				}
			}
		}
		// Alignments: the nodes of a row and of a column, sometimes by
		// group.
		rows := map[string][]string{}
		cols := map[string][]string{}
		var rowKeys, colKeys []string
		for _, id := range placed {
			for axis, m := range []map[string][]string{rows, cols} {
				key := strconv.Itoa(pos[id][1-axis]) + "/all"
				if !r.chance(0.8) {
					key = strconv.Itoa(pos[id][1-axis]) + "/" + groupOf[id]
				}
				if _, ok := m[key]; !ok {
					if axis == 0 {
						rowKeys = append(rowKeys, key)
					} else {
						colKeys = append(colKeys, key)
					}
				}
				m[key] = append(m[key], id)
			}
		}
		for _, key := range rowKeys {
			if len(rows[key]) > 1 {
				in.Alignment.Horizontal = append(in.Alignment.Horizontal, rows[key])
			}
		}
		for _, key := range colKeys {
			if len(cols[key]) > 1 {
				in.Alignment.Vertical = append(in.Alignment.Vertical, cols[key])
			}
		}
		// Relative placements: a search from the first cell, as mermaid does.
		queue := []cell{{0, 0}}
		visited := map[cell]bool{}
		for len(queue) > 0 {
			curr := queue[0]
			queue = queue[1:]
			visited[curr] = true
			currID := cells[curr]
			for _, dir := range archDirs {
				np := cell{curr[0] + archShift[dir][0], curr[1] + archShift[dir][1]}
				newID, ok := cells[np]
				if !ok || visited[np] {
					continue
				}
				queue = append(queue, np)
				c := native.RelativePlacement{Gap: gap}
				switch dir {
				case "L":
					c.Left, c.Right = newID, currID
				case "R":
					c.Right, c.Left = newID, currID
				case "T":
					c.Top, c.Bottom = newID, currID
				default:
					c.Bottom, c.Top = newID, currID
				}
				in.RelativePlacement = append(in.RelativePlacement, c)
			}
		}
		// Edges that bend, and edges between cells that are not neighbours.
		if r.chance(0.5) {
			extra := r.between(0, 3)
			for k := 0; k < extra; k++ {
				addEdge(placed[r.between(0, len(placed)-1)], placed[r.between(0, len(placed)-1)], archDirs[r.between(0, 3)], archDirs[r.between(0, 3)])
			}
		}
		at += size
	}
	// Edges between components.
	if n > 1 && r.chance(0.4) {
		extra := r.between(1, 3)
		for k := 0; k < extra; k++ {
			addEdge(ids[r.between(0, n-1)], ids[r.between(0, n-1)], archDirs[r.between(0, 3)], archDirs[r.between(0, 3)])
		}
	}
	if a.Kind == "wild" {
		switch r.between(0, 4) {
		case 0:
			// A constraint that contradicts another one.
			if len(in.RelativePlacement) > 0 {
				c := in.RelativePlacement[r.between(0, len(in.RelativePlacement)-1)]
				c.Left, c.Right = c.Right, c.Left
				c.Top, c.Bottom = c.Bottom, c.Top
				in.RelativePlacement = append(in.RelativePlacement, c)
			}
		case 1:
			// An edge of a group, and an edge from a node to itself.
			if groupCount > 0 {
				in.Edges = append(in.Edges, native.ArchitectureEdge{LhsID: ids[0], RhsID: "g0", LhsDir: "R", RhsDir: "L"})
			} else {
				in.Edges = append(in.Edges, native.ArchitectureEdge{LhsID: ids[0], RhsID: ids[0], LhsDir: "R", RhsDir: "L"})
			}
		case 2:
			// A node in two alignments, and a constraint on a group.
			if len(in.Alignment.Horizontal) > 0 && n > 2 {
				in.Alignment.Horizontal = append(in.Alignment.Horizontal, []string{in.Alignment.Horizontal[0][0], ids[n-1]})
			} else if groupCount > 0 {
				in.Alignment.Vertical = append(in.Alignment.Vertical, []string{"g0", ids[0]})
			}
		case 3:
			// A group without members, and a group inside a later group.
			in.Groups = append(in.Groups, native.ArchitectureNode{ID: "empty", Title: "Empty"})
			if groupCount > 1 {
				in.Groups[0].In = "g1"
			}
		case 4:
			// Config values at the edge of what is a number.
			in.Config.Padding = []float64{0, 12.5, 0.1}[r.between(0, 2)]
			in.Config.NodeSeparation = 10
			in.Config.IconSize = []float64{1, 300, 80}[r.between(0, 2)]
		}
	}
	return a
}
