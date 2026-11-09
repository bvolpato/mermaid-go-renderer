// Ported from @upsetjs/venn.js 2.0.0, src/layout.js (the part of
// build/venn.esm.js that mermaid 12.2.0 bundles).

package venn

import (
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// maxIterations is parameters.maxIterations after venn() has set it:
// parameters.maxIterations = parameters.maxIterations || 500. nelderMead and
// conjugateGradient get the same parameters object.
const maxIterations = 500

// venn is venn(sets, parameters) with the parameters that the chart and
// layout() give: the default loss function, and distinct not set.
//
// given a list of set objects, and their corresponding overlaps
// updates the (x, y, radius) attribute on each set such that their positions
// roughly correspond to the desired overlaps
func (e *engine) venn(sets []*Area) *js.Obj[*Circle] {
	// add in missing pairwise areas as having 0 size
	areas := e.addMissingAreas(sets)

	// initial layout is done greedily
	circles := e.bestInitialLayout(areas)

	// transform x/y coordinates to a vector to optimize
	setids := circles.Keys()
	initial := []float64{}
	for _, setid := range setids {
		initial = append(initial, circles.Get(setid).X)
		initial = append(initial, circles.Get(setid).Y)
	}

	// optimize initial layout from our loss function
	solution := e.nelderMead(
		func(values []float64) float64 {
			current := js.NewObj[*Circle]()
			for i := 0; i < len(setids); i++ {
				setid := setids[i]
				current.Set(setid, &Circle{
					X:      values[2*i],
					Y:      values[2*i+1],
					Radius: circles.Get(setid).Radius,
				})
			}
			return e.lossFunction(current, areas)
		},
		initial,
		nelderMeadParams{maxIterations: maxIterations},
	)

	// transform solution vector back to x/y points
	positions := solution.x
	for i := 0; i < len(setids); i++ {
		setid := setids[i]
		circles.Get(setid).X = positions[2*i]
		circles.Get(setid).Y = positions[2*i+1]
	}

	return circles
}

// distanceFromIntersectArea is distanceFromIntersectArea(r1, r2, overlap).
//
// Returns the distance necessary for two circles of radius r1 + r2 to
// have the overlap area 'overlap'
func (e *engine) distanceFromIntersectArea(r1, r2, overlap float64) float64 {
	// handle complete overlapped circles
	if js.Min(r1, r2)*js.Min(r1, r2)*pi <= overlap+small {
		return math.Abs(r1 - r2)
	}

	return bisect(func(distance float64) float64 { return e.circleOverlap(r1, r2, distance) - overlap }, 0, r1+r2)
}

// toKey is toKey(arr) of addMissingAreas.
func toKey(arr ...string) string { return strings.Join(arr, ";") }

// addMissingAreas is addMissingAreas(areas, parameters) with distinct not
// set.
//
// Missing pair-wise intersection area data can cause problems:
// treating as an unknown means that sets will be laid out overlapping,
// which isn't what people expect. To reflect that we want disjoint sets
// here, set the overlap to 0 for all missing pairwise set intersections
func (e *engine) addMissingAreas(areas []*Area) []*Area {
	r := make([]*Area, len(areas))
	for i, s := range areas {
		c := *s
		r[i] = &c
	}

	// two circle intersections that aren't defined
	ids := []string{}

	pairs := map[string]bool{}
	for _, area := range r {
		if len(area.Sets) == 1 {
			ids = append(ids, area.Sets[0])
		} else if len(area.Sets) == 2 {
			a := area.Sets[0]
			b := area.Sets[1]
			pairs[toKey(area.Sets...)] = true
			pairs[toKey(b, a)] = true
		}
	}

	// The greedy layout computes about n^5/6 overlaps for n sets. Fail
	// before the pairs take memory when that is more than the bound.
	if n := float64(len(ids)); n*n*n*n*n/6 > float64(e.maxWork) {
		e.step(e.maxWork + 1)
	}

	sortBy(e, ids, func(a, b string) float64 {
		switch c := js.CompareStrings(a, b); {
		case c == 0:
			return 0
		case c < 0:
			return -1
		}
		return +1
	})

	for i := 0; i < len(ids); i++ {
		a := ids[i]
		for j := i + 1; j < len(ids); j++ {
			b := ids[j]
			if !pairs[toKey(a, b)] {
				r = append(r, &Area{Sets: []string{a, b}, Size: 0})
			}
		}
	}
	return r
}

// getDistanceMatrices is getDistanceMatrices(areas, sets, setids).
//
// Returns two matrices, one of the euclidean distances between the sets
// and the other indicating if there are subset or disjoint set relationships
func (e *engine) getDistanceMatrices(areas, sets []*Area, setids *js.Obj[int]) (distances, constraints [][]float64) {
	// initialize an empty distance matrix between all the points
	distances = zerosM(len(sets), len(sets))
	constraints = zerosM(len(sets), len(sets))

	// compute required distances between all the sets such that
	// the areas match
	for _, current := range areas {
		if len(current.Sets) != 2 {
			continue
		}
		left, ok := setids.Lookup(current.Sets[0])
		if !ok {
			throwUndefined("size")
		}
		right, ok := setids.Lookup(current.Sets[1])
		if !ok {
			throwUndefined("size")
		}
		r1 := math.Sqrt(sets[left].Size / pi)
		r2 := math.Sqrt(sets[right].Size / pi)
		distance := e.distanceFromIntersectArea(r1, r2, current.Size)

		distances[right][left] = distance
		distances[left][right] = distance

		// also update constraints to indicate if its a subset or disjoint
		// relationship
		c := 0.0
		if current.Size+1e-10 >= js.Min(sets[left].Size, sets[right].Size) {
			c = 1
		} else if current.Size <= 1e-10 {
			c = -1
		}
		constraints[right][left] = c
		constraints[left][right] = c
	}

	return distances, constraints
}

// constrainedMDSGradient is constrainedMDSGradient(x, fxprime, distances,
// constraints): it computes the gradient and loss simultaneously for our
// constrained MDS optimizer.
func (e *engine) constrainedMDSGradient(x, fxprime []float64, distances, constraints [][]float64) float64 {
	for i := 0; i < len(fxprime); i++ {
		fxprime[i] = 0
	}
	e.step(len(distances)*len(distances)/16 + 1)

	loss := 0.0
	for i := 0; i < len(distances); i++ {
		xi := x[2*i]
		yi := x[2*i+1]
		for j := i + 1; j < len(distances); j++ {
			xj := x[2*j]
			yj := x[2*j+1]
			dij := distances[i][j]
			constraint := constraints[i][j]

			squaredDistance := float64((xj-xi)*(xj-xi)) + float64((yj-yi)*(yj-yi))
			distance := math.Sqrt(squaredDistance)
			delta := squaredDistance - float64(dij*dij)

			if (constraint > 0 && distance <= dij) || (constraint < 0 && distance >= dij) {
				continue
			}

			loss += float64(2 * delta * delta)

			fxprime[2*i] += float64(4 * delta * (xi - xj))
			fxprime[2*i+1] += float64(4 * delta * (yi - yj))

			fxprime[2*j] += float64(4 * delta * (xj - xi))
			fxprime[2*j+1] += float64(4 * delta * (yj - yi))
		}
	}
	return loss
}

// bestInitialLayout is bestInitialLayout(areas, params): it takes the best
// working variant of either constrained MDS or greedy.
func (e *engine) bestInitialLayout(areas []*Area) *js.Obj[*Circle] {
	initial := e.greedyLayout(areas)

	// greedylayout is sufficient for all 2/3 circle cases. try out
	// constrained MDS for higher order problems, take its output
	// if it outperforms. (greedy is aesthetically better on 2/3 circles
	// since it axis aligns)
	if len(areas) >= 8 {
		constrained := e.constrainedMDSLayout(areas)
		constrainedLoss := e.lossFunction(constrained, areas)
		greedyLoss := e.lossFunction(initial, areas)

		if constrainedLoss+1e-8 < greedyLoss {
			initial = constrained
		}
	}
	return initial
}

// constrainedMDSLayout is constrainedMDSLayout(areas, params): it uses the
// constrained MDS variant to generate an initial layout.
func (e *engine) constrainedMDSLayout(areas []*Area) *js.Obj[*Circle] {
	const restarts = 10

	// bidirectionally map sets to a rowid  (so we can create a matrix)
	sets := []*Area{}
	setids := js.NewObj[int]()
	for _, area := range areas {
		if len(area.Sets) == 1 {
			setids.Set(area.Sets[0], len(sets))
			sets = append(sets, area)
		}
	}

	distances, constraints := e.getDistanceMatrices(areas, sets, setids)

	// keep distances bounded, things get messed up otherwise.
	// TODO: proper preconditioner?
	rowNorms := make([]float64, len(distances))
	for i, row := range distances {
		rowNorms[i] = norm2(row)
	}
	norm := norm2(rowNorms) / float64(len(distances))
	for _, row := range distances {
		for i, value := range row {
			row[i] = value / norm
		}
	}

	obj := func(x, fxprime []float64) float64 {
		return e.constrainedMDSGradient(x, fxprime, distances, constraints)
	}

	var best *cgState
	for i := 0; i < restarts; i++ {
		initial := zeros(len(distances) * 2)
		for k := range initial {
			initial[k] = e.random()
		}

		current := conjugateGradient(obj, initial, maxIterations)
		if best == nil || current.fx < best.fx {
			best = current
		}
	}

	positions := best.x

	// translate rows back to (x,y,radius) coordinates
	circles := js.NewObj[*Circle]()
	for i := 0; i < len(sets); i++ {
		set := sets[i]
		circles.Set(set.Sets[0], &Circle{
			X:      float64(positions[2*i] * norm),
			Y:      float64(positions[2*i+1] * norm),
			Radius: math.Sqrt(set.Size / pi),
		})
	}
	return circles
}

// setOverlap is one entry of the lists of setOverlaps and of mostOverlapped
// in greedyLayout: { set, size, weight }.
type setOverlap struct {
	set    string
	size   float64
	weight float64
}

// greedyLayout is greedyLayout(areas, params).
//
// Lays out a Venn diagram greedily, going from most overlapped sets to
// least overlapped, attempting to position each new set such that the
// overlapping areas to already positioned sets are basically right
func (e *engine) greedyLayout(areas []*Area) *js.Obj[*Circle] {
	// define a circle for each set
	circles := js.NewObj[*Circle]()
	setOverlaps := js.NewObj[*[]setOverlap]()
	for _, area := range areas {
		if len(area.Sets) == 1 {
			set := area.Sets[0]
			circles.Set(set, &Circle{
				X:      1e10,
				Y:      1e10,
				Size:   area.Size,
				Radius: math.Sqrt(area.Size / pi),
			})
			setOverlaps.Set(set, &[]setOverlap{})
		}
	}

	pairAreas := []*Area{}
	for _, a := range areas {
		if len(a.Sets) == 2 {
			pairAreas = append(pairAreas, a)
		}
	}
	areas = pairAreas

	// map each set to a list of all the other sets that overlap it
	for _, current := range areas {
		weight := 1.0
		if current.HasWeight {
			weight = current.Weight
		}
		left := current.Sets[0]
		right := current.Sets[1]

		// completely overlapped circles shouldn't be positioned early here
		if current.Size+small >= js.Min(circleOf(circles, left, "size").Size, circleOf(circles, right, "size").Size) {
			weight = 0
		}

		leftOverlaps := setOverlaps.Get(left)
		*leftOverlaps = append(*leftOverlaps, setOverlap{set: right, size: current.Size, weight: weight})
		rightOverlaps := setOverlaps.Get(right)
		*rightOverlaps = append(*rightOverlaps, setOverlap{set: left, size: current.Size, weight: weight})
	}

	// get list of most overlapped sets
	mostOverlapped := []setOverlap{}
	for _, set := range setOverlaps.Keys() {
		size := 0.0
		overlaps := *setOverlaps.Get(set)
		for i := 0; i < len(overlaps); i++ {
			size += float64(overlaps[i].size * overlaps[i].weight)
		}

		mostOverlapped = append(mostOverlapped, setOverlap{set: set, size: size})
	}

	// sort by size desc
	sortOrder := func(a, b setOverlap) float64 {
		return b.size - a.size
	}
	sortBy(e, mostOverlapped, sortOrder)

	// keep track of what sets have been laid out
	positioned := js.NewObj[bool]()
	isPositioned := func(element setOverlap) bool {
		return inObject(positioned, element.set)
	}

	// adds a point to the output
	positionSet := func(point Point, index string) {
		circles.Get(index).X = point.X
		circles.Get(index).Y = point.Y
		positioned.Set(index, true)
	}

	// add most overlapped set at (0,0)
	if len(mostOverlapped) == 0 {
		throwUndefined("set")
	}
	positionSet(Point{X: 0, Y: 0}, mostOverlapped[0].set)

	// get distances between all points. TODO, necessary?
	// answer: probably not
	// var distances = venn.getDistanceMatrices(circles, areas).distances;
	for i := 1; i < len(mostOverlapped); i++ {
		setIndex := mostOverlapped[i].set
		overlap := []setOverlap{}
		for _, element := range *setOverlaps.Get(setIndex) {
			if isPositioned(element) {
				overlap = append(overlap, element)
			}
		}
		set := circles.Get(setIndex)
		sortBy(e, overlap, sortOrder)

		if len(overlap) == 0 {
			// this shouldn't happen anymore with addMissingAreas
			throw("Error", "ERROR: missing pairwise overlap information")
		}

		points := []Point{}
		for j := 0; j < len(overlap); j++ {
			// get appropriate distance from most overlapped already added set
			p1 := circles.Get(overlap[j].set)
			d1 := e.distanceFromIntersectArea(set.Radius, p1.Radius, overlap[j].size)

			// sample positions at 90 degrees for maximum aesthetics
			points = append(points, Point{X: p1.X + d1, Y: p1.Y})
			points = append(points, Point{X: p1.X - d1, Y: p1.Y})
			points = append(points, Point{Y: p1.Y + d1, X: p1.X})
			points = append(points, Point{Y: p1.Y - d1, X: p1.X})

			// if we have at least 2 overlaps, then figure out where the
			// set should be positioned analytically and try those too
			for k := j + 1; k < len(overlap); k++ {
				p2 := circles.Get(overlap[k].set)
				d2 := e.distanceFromIntersectArea(set.Radius, p2.Radius, overlap[k].size)

				extraPoints := circleCircleIntersection(
					&Circle{X: p1.X, Y: p1.Y, Radius: d1},
					&Circle{X: p2.X, Y: p2.Y, Radius: d2},
				)
				points = append(points, extraPoints...)
			}
		}

		// we have some candidate positions for the set, examine loss
		// at each position to figure out where to put it at
		bestLoss := 1e50
		bestPoint := points[0]
		for _, point := range points {
			circles.Get(setIndex).X = point.X
			circles.Get(setIndex).Y = point.Y
			localLoss := e.lossFunction(circles, areas)
			if localLoss < bestLoss {
				bestLoss = localLoss
				bestPoint = point
			}
		}

		positionSet(bestPoint, setIndex)
	}

	return circles
}

// circleOf is circles[set] for a set that the code reads a property of. It
// raises the TypeError of JavaScript when the object has no such circle.
func circleOf(circles *js.Obj[*Circle], set, property string) *Circle {
	c, ok := circles.Lookup(set)
	if !ok || c == nil {
		throwUndefined(property)
	}
	return c
}

// lossFunction is lossFunction(circles, overlaps).
//
// Given a bunch of sets, and the desired overlaps between these sets - computes
// the distance from the actual overlaps to the desired overlaps. Note that
// this method ignores overlaps of more than 2 circles
func (e *engine) lossFunction(circles *js.Obj[*Circle], overlaps []*Area) float64 {
	e.step(len(overlaps))
	output := 0.0

	for _, area := range overlaps {
		if len(area.Sets) == 1 {
			continue
		}
		var overlap float64
		if len(area.Sets) == 2 {
			left := circleOf(circles, area.Sets[0], "radius")
			right := circleOf(circles, area.Sets[1], "radius")
			overlap = e.circleOverlap(left.Radius, right.Radius, distance(left.X, left.Y, right.X, right.Y))
		} else {
			members := make([]*Circle, len(area.Sets))
			for i, d := range area.Sets {
				members[i] = circles.Get(d)
			}
			overlap = e.intersectionArea(members, nil)
		}

		weight := 1.0
		if area.HasWeight {
			weight = area.Weight
		}
		output += float64(weight * (overlap - area.Size) * (overlap - area.Size))
	}

	return output
}

// orientateCircles is orientateCircles(circles, orientation,
// orientationOrder) with no orientationOrder: it orientates a bunch of
// circles to point in orientation.
func (e *engine) orientateCircles(circles []*Circle, orientation float64) {
	sortBy(e, circles, func(a, b *Circle) float64 { return b.Radius - a.Radius })

	// shift circles so largest circle is at (0, 0)
	if len(circles) > 0 {
		largestX := circles[0].X
		largestY := circles[0].Y

		for _, circle := range circles {
			circle.X -= largestX
			circle.Y -= largestY
		}
	}

	if len(circles) == 2 {
		// if the second circle is a subset of the first, arrange so that
		// it is off to one side. hack for https://github.com/benfred/venn.js/issues/120
		dist := distance(circles[0].X, circles[0].Y, circles[1].X, circles[1].Y)
		if dist < math.Abs(circles[1].Radius-circles[0].Radius) {
			circles[1].X = circles[0].X + circles[0].Radius - circles[1].Radius - 1e-10
			circles[1].Y = circles[0].Y
		}
	}

	// rotate circles so that second largest is at an angle of 'orientation'
	// from largest
	if len(circles) > 1 {
		rotation := e.m.Atan2(circles[1].X, circles[1].Y) - orientation
		c := e.m.Cos(rotation)
		s := e.m.Sin(rotation)

		for _, circle := range circles {
			x := circle.X
			y := circle.Y
			circle.X = float64(c*x) - float64(s*y)
			circle.Y = float64(s*x) + float64(c*y)
		}
	}

	// mirror solution if third solution is above plane specified by
	// first two circles
	if len(circles) > 2 {
		angle := e.m.Atan2(circles[2].X, circles[2].Y) - orientation
		for angle < 0 {
			angle += 2 * pi
		}
		for angle > 2*pi {
			angle -= 2 * pi
		}
		if angle > pi {
			slope := circles[1].Y / (1e-10 + circles[1].X)
			for _, circle := range circles {
				d := (circle.X + float64(slope*circle.Y)) / (1 + float64(slope*slope))
				circle.X = float64(2*d) - circle.X
				circle.Y = float64(2*d*slope) - circle.Y
			}
		}
	}
}

// disjointCluster is disjointCluster(circles).
func disjointCluster(circles []*Circle) [][]*Circle {
	// union-find clustering to get disjoint sets
	for _, circle := range circles {
		circle.parent = circle
	}

	// path compression step in union find
	var find func(circle *Circle) *Circle
	find = func(circle *Circle) *Circle {
		if circle.parent != circle {
			circle.parent = find(circle.parent)
		}
		return circle.parent
	}

	union := func(x, y *Circle) {
		xRoot := find(x)
		yRoot := find(y)
		xRoot.parent = yRoot
	}

	// get the union of all overlapping sets
	for i := 0; i < len(circles); i++ {
		for j := i + 1; j < len(circles); j++ {
			maxDistance := circles[i].Radius + circles[j].Radius
			if distance(circles[i].X, circles[i].Y, circles[j].X, circles[j].Y)+1e-10 < maxDistance {
				union(circles[j], circles[i])
			}
		}
	}

	// find all the disjoint clusters and group them together
	disjointClusters := js.NewMap[string, *[]*Circle]()
	for i := 0; i < len(circles); i++ {
		setid := find(circles[i]).parent.SetID
		if !disjointClusters.Has(setid) {
			disjointClusters.Set(setid, &[]*Circle{})
		}
		list := disjointClusters.Get(setid)
		*list = append(*list, circles[i])
	}

	// cleanup bookkeeping
	for _, circle := range circles {
		circle.parent = nil
	}

	// return in more usable form
	ret := [][]*Circle{}
	for _, list := range disjointClusters.Values() {
		ret = append(ret, *list)
	}
	return ret
}

// rangeOf is { max, min } of getBoundingBox.
type rangeOf struct {
	max, min float64
}

// boundingBox is the result of getBoundingBox: { xRange, yRange }.
type boundingBox struct {
	xRange, yRange rangeOf
}

// getBoundingBox is getBoundingBox(circles).
func getBoundingBox(circles []*Circle) boundingBox {
	minMax := func(d func(c *Circle) float64) rangeOf {
		hi := math.Inf(-1)
		for _, c := range circles {
			hi = js.Max(hi, d(c)+c.Radius)
		}
		lo := math.Inf(1)
		for _, c := range circles {
			lo = js.Min(lo, d(c)-c.Radius)
		}
		return rangeOf{max: hi, min: lo}
	}
	return boundingBox{
		xRange: minMax(func(c *Circle) float64 { return c.X }),
		yRange: minMax(func(c *Circle) float64 { return c.Y }),
	}
}

// cluster is one array of disjointCluster with the properties size and
// bounds that normalizeSolution adds.
type cluster struct {
	circles []*Circle
	size    float64
	bounds  boundingBox
}

// normalizeSolution is normalizeSolution(solution, orientation,
// orientationOrder) with no orientationOrder.
func (e *engine) normalizeSolution(solution *js.Obj[*Circle], orientation float64) *js.Obj[*Circle] {
	// work with a list instead of a dictionary, and take a copy so we
	// don't mutate input
	source := fromObjectNotation(solution)
	circles := make([]*Circle, len(source))
	for i, d := range source {
		c := *d
		circles[i] = &c
	}

	// get all the disjoint clusters
	lists := disjointCluster(circles)
	clusters := make([]*cluster, len(lists))

	// orientate all disjoint sets, get sizes
	for i, list := range lists {
		e.orientateCircles(list, orientation)
		bounds := getBoundingBox(list)
		clusters[i] = &cluster{
			circles: list,
			size:    float64((bounds.xRange.max - bounds.xRange.min) * (bounds.yRange.max - bounds.yRange.min)),
			bounds:  bounds,
		}
	}
	sortBy(e, clusters, func(a, b *cluster) float64 { return b.size - a.size })

	// orientate the largest at 0,0, and get the bounds
	if len(clusters) == 0 {
		throwUndefined("bounds")
	}
	circles = clusters[0].circles
	returnBounds := clusters[0].bounds
	spacing := (returnBounds.xRange.max - returnBounds.xRange.min) / 50

	addCluster := func(index int, right, bottom bool) {
		if index >= len(clusters) {
			return
		}
		cluster := clusters[index]

		bounds := cluster.bounds
		var xOffset float64
		var yOffset float64

		if right {
			xOffset = returnBounds.xRange.max - bounds.xRange.min + spacing
		} else {
			xOffset = returnBounds.xRange.max - bounds.xRange.max
			centreing := float64((bounds.xRange.max-bounds.xRange.min)/2) - float64((returnBounds.xRange.max-returnBounds.xRange.min)/2)
			if centreing < 0 {
				xOffset += centreing
			}
		}

		if bottom {
			yOffset = returnBounds.yRange.max - bounds.yRange.min + spacing
		} else {
			yOffset = returnBounds.yRange.max - bounds.yRange.max
			centreing := float64((bounds.yRange.max-bounds.yRange.min)/2) - float64((returnBounds.yRange.max-returnBounds.yRange.min)/2)
			if centreing < 0 {
				yOffset += centreing
			}
		}

		for _, c := range cluster.circles {
			c.X += xOffset
			c.Y += yOffset
			circles = append(circles, c)
		}
	}

	index := 1
	for index < len(clusters) {
		addCluster(index, true, false)
		addCluster(index+1, false, true)
		addCluster(index+2, true, true)
		index += 3

		// have one cluster (in top left). lay out next three relative
		// to it in a grid
		returnBounds = getBoundingBox(circles)
	}

	// convert back to solution form
	return toObjectNotation(circles)
}

// scaleSolution is scaleSolution(solution, width, height, padding,
// scaleToFit) with scaleToFit not set.
//
// Scales a solution from venn.venn or venn.greedyLayout such that it fits in
// a rectangle of width/height - with padding around the borders. also
// centers the diagram in the available space at the same time.
func scaleSolution(solution *js.Obj[*Circle], width, height, padding float64) *js.Obj[*Circle] {
	circles := fromObjectNotation(solution)

	width -= float64(2 * padding)
	height -= float64(2 * padding)

	box := getBoundingBox(circles)
	xRange, yRange := box.xRange, box.yRange

	if xRange.max == xRange.min || yRange.max == yRange.min {
		// console.log('not scaling solution: zero size detected');
		return solution
	}

	xScaling := width / (xRange.max - xRange.min)
	yScaling := height / (yRange.max - yRange.min)

	scaling := js.Min(yScaling, xScaling)
	// while we're at it, center the diagram too
	xOffset := float64((width - float64((xRange.max-xRange.min)*scaling)) / 2)
	yOffset := float64((height - float64((yRange.max-yRange.min)*scaling)) / 2)

	scaled := make([]*Circle, len(circles))
	for i, circle := range circles {
		scaled[i] = &Circle{
			Radius: float64(scaling * circle.Radius),
			X:      padding + xOffset + float64((circle.X-xRange.min)*scaling),
			Y:      padding + yOffset + float64((circle.Y-yRange.min)*scaling),
			SetID:  circle.SetID,
		}
	}
	return toObjectNotation(scaled)
}

// toObjectNotation is toObjectNotation(circles).
func toObjectNotation(circles []*Circle) *js.Obj[*Circle] {
	r := js.NewObj[*Circle]()
	for _, circle := range circles {
		r.Set(circle.SetID, circle)
	}
	return r
}

// fromObjectNotation is fromObjectNotation(solution).
func fromObjectNotation(solution *js.Obj[*Circle]) []*Circle {
	setids := solution.Keys()
	circles := make([]*Circle, len(setids))
	for i, id := range setids {
		c := solution.Get(id)
		c.SetID = id
		circles[i] = c
	}
	return circles
}
