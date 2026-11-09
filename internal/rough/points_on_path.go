// Ported from points-on-path/lib/index.js (points-on-path 0.2.1).

package rough

// pointsOnPath returns one list of points for each subpath of the path data.
// A distance of 0 means that the lists are not simplified.
func pointsOnPath(g *Generator, path string, tolerance, distance float64) [][]*Point {
	segments := parsePath(path)
	normalized := normalize(g, absolutize(segments))
	sets := [][]*Point{}
	currentPoints := []*Point{}
	start := &Point{0, 0}
	pendingCurve := []*Point{}
	appendPendingCurve := func() {
		if len(pendingCurve) >= 4 {
			currentPoints = append(currentPoints, pointsOnBezierCurves(g, pendingCurve, tolerance, 0)...)
		}
		pendingCurve = []*Point{}
	}
	appendPendingPoints := func() {
		appendPendingCurve()
		if len(currentPoints) > 0 {
			sets = append(sets, currentPoints)
			currentPoints = []*Point{}
		}
	}
	for _, seg := range normalized {
		data := seg.data
		switch seg.key {
		case 'M':
			appendPendingPoints()
			start = &Point{data[0], data[1]}
			currentPoints = append(currentPoints, start)
		case 'L':
			appendPendingCurve()
			currentPoints = append(currentPoints, &Point{data[0], data[1]})
		case 'C':
			if len(pendingCurve) == 0 {
				lastPoint := start
				if len(currentPoints) > 0 {
					lastPoint = currentPoints[len(currentPoints)-1]
				}
				pendingCurve = append(pendingCurve, &Point{lastPoint[0], lastPoint[1]})
			}
			pendingCurve = append(pendingCurve, &Point{data[0], data[1]})
			pendingCurve = append(pendingCurve, &Point{data[2], data[3]})
			pendingCurve = append(pendingCurve, &Point{data[4], data[5]})
		case 'Z':
			appendPendingCurve()
			currentPoints = append(currentPoints, &Point{start[0], start[1]})
		}
	}
	appendPendingPoints()
	if !truthyNum(distance) {
		return sets
	}
	out := [][]*Point{}
	for _, set := range sets {
		simplifiedSet := simplify(g, set, distance)
		if len(simplifiedSet) > 0 {
			out = append(out, simplifiedSet)
		}
	}
	return out
}
