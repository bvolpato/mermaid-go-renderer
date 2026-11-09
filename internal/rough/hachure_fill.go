// Ported from hachure-fill/bin/hachure.js (hachure-fill 0.5.2).

package rough

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// rotatePoints rotates the points in place. A point that is in the list
// twice is rotated twice, as it is in JavaScript.
func rotatePoints(m *mathFuncs, points []*Point, center Point, degrees float64) {
	if len(points) > 0 {
		pi := math.Pi
		cx, cy := center[0], center[1]
		angle := float64((pi / 180) * degrees)
		cos := m.cos(angle)
		sin := m.sin(angle)
		for _, p := range points {
			x, y := p[0], p[1]
			p[0] = float64((x-cx)*cos) - float64((y-cy)*sin) + cx
			p[1] = float64((x-cx)*sin) + float64((y-cy)*cos) + cy
		}
	}
}

func rotateLines(m *mathFuncs, lines [][2]*Point, center Point, degrees float64) {
	points := make([]*Point, 0, 2*len(lines))
	for _, line := range lines {
		points = append(points, line[0], line[1])
	}
	rotatePoints(m, points, center, degrees)
}

func areSamePoints(p1, p2 *Point) bool {
	return p1[0] == p2[0] && p1[1] == p2[1]
}

// hachureLines returns the hachure lines of the polygons. The JavaScript
// function also accepts one polygon. roughjs always gives a list.
func hachureLines(g *Generator, polygons [][]*Point, hachureGap, hachureAngle, hachureStepOffset float64) [][2]*Point {
	m := &g.m
	angle := hachureAngle
	gap := js.Max(hachureGap, 0.1)
	polygonList := polygons
	rotationCenter := Point{0, 0}
	if truthyNum(angle) {
		for _, polygon := range polygonList {
			rotatePoints(m, polygon, rotationCenter, angle)
		}
	}
	lines := straightHachureLines(g, polygonList, gap, hachureStepOffset)
	if truthyNum(angle) {
		for _, polygon := range polygonList {
			rotatePoints(m, polygon, rotationCenter, -angle)
		}
		rotateLines(m, lines, rotationCenter, -angle)
	}
	return lines
}

type hachureEdge struct {
	ymin, ymax, x, islope float64
}

func straightHachureLines(g *Generator, polygons [][]*Point, gap, hachureStepOffset float64) [][2]*Point {
	vertexArray := [][]*Point{}
	for _, polygon := range polygons {
		if len(polygon) == 0 {
			// areSamePoints(vertices[0], ...) reads vertices[0][0].
			throw("TypeError", "Cannot read properties of undefined (reading '0')")
		}
		vertices := append([]*Point{}, polygon...)
		if !areSamePoints(vertices[0], vertices[len(vertices)-1]) {
			vertices = append(vertices, &Point{vertices[0][0], vertices[0][1]})
		}
		if len(vertices) > 2 {
			vertexArray = append(vertexArray, vertices)
		}
	}
	lines := [][2]*Point{}
	gap = js.Max(gap, 0.1)
	// Create sorted edges table
	edges := []*hachureEdge{}
	for _, vertices := range vertexArray {
		for i := 0; i < len(vertices)-1; i++ {
			p1 := vertices[i]
			p2 := vertices[i+1]
			if p1[1] != p2[1] {
				ymin := js.Min(p1[1], p2[1])
				x := p2[0]
				if ymin == p1[1] {
					x = p1[0]
				}
				edges = append(edges, &hachureEdge{
					ymin:   ymin,
					ymax:   js.Max(p1[1], p2[1]),
					x:      x,
					islope: (p2[0] - p1[0]) / (p2[1] - p1[1]),
				})
			}
		}
	}
	js.SortFunc(edges, func(e1, e2 *hachureEdge) float64 {
		if e1.ymin < e2.ymin {
			return -1
		}
		if e1.ymin > e2.ymin {
			return 1
		}
		if e1.x < e2.x {
			return -1
		}
		if e1.x > e2.x {
			return 1
		}
		if e1.ymax == e2.ymax {
			return 0
		}
		return (e1.ymax - e2.ymax) / math.Abs(e1.ymax-e2.ymax)
	})
	if len(edges) == 0 {
		return lines
	}
	// Start scanning
	activeEdges := []*hachureEdge{}
	y := edges[0].ymin
	// The scan ends after the scan line passes the end of every edge.
	for _, edge := range edges {
		g.need((edge.ymax - y) / hachureStepOffset)
	}
	iteration := 0.0
	for len(activeEdges) > 0 || len(edges) > 0 {
		g.step()
		if len(edges) > 0 {
			ix := -1
			for i := 0; i < len(edges); i++ {
				if edges[i].ymin > y {
					break
				}
				ix = i
			}
			removed := edges[:ix+1]
			edges = edges[ix+1:]
			for _, edge := range removed {
				if edge.ymax != edge.ymax || (math.IsInf(edge.ymax, 1) && !math.IsInf(hachureStepOffset, 1)) {
					// The scan line never reaches the end of this
					// edge, so the JavaScript loop does not end.
					hang("a hachure edge has no finite end")
				}
				activeEdges = append(activeEdges, edge)
			}
		}
		kept := activeEdges[:0]
		for _, ae := range activeEdges {
			if ae.ymax <= y {
				continue
			}
			kept = append(kept, ae)
		}
		activeEdges = kept
		js.SortFunc(activeEdges, func(ae1, ae2 *hachureEdge) float64 {
			if ae1.x == ae2.x {
				return 0
			}
			return (ae1.x - ae2.x) / math.Abs(ae1.x-ae2.x)
		})
		// fill between the edges
		if (hachureStepOffset != 1) || (math.Mod(iteration, gap) == 0) {
			if len(activeEdges) > 1 {
				for i := 0; i < len(activeEdges); i = i + 2 {
					nexti := i + 1
					if nexti >= len(activeEdges) {
						break
					}
					ce := activeEdges[i]
					ne := activeEdges[nexti]
					lines = append(lines, [2]*Point{
						{mathRound(ce.x), y},
						{mathRound(ne.x), y},
					})
				}
			}
		}
		next := y + hachureStepOffset
		if (next == y || next != next) && (len(activeEdges) > 0 || len(edges) > 0) {
			// The scan line does not move, or it is NaN. No edge can
			// leave the active list, so the JavaScript loop does not end.
			hang("the hachure scan line does not advance")
		}
		y = next
		for _, ae := range activeEdges {
			ae.x = ae.x + float64(hachureStepOffset*ae.islope)
		}
		iteration++
	}
	return lines
}
