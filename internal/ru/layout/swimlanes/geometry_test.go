package swimlanes

import (
	"math/rand"
	"reflect"
	"testing"
)

// randomPath returns a path with orthogonal segments, diagonal segments,
// repeated points and points on a line.
func randomPath(rng *rand.Rand) []Point {
	points := make([]Point, rng.Intn(12))
	x, y := float64(rng.Intn(200)), float64(rng.Intn(200))
	for i := range points {
		switch rng.Intn(5) {
		case 0:
			x += float64(rng.Intn(60) - 30)
		case 1:
			y += float64(rng.Intn(60) - 30)
		case 2:
			x += float64(rng.Intn(60) - 30)
			y += float64(rng.Intn(60) - 30)
		case 3:
			// Less than the default epsilon: the points are the same.
			x += 0.0004
		}
		points[i] = Point{X: x, Y: y}
	}
	return points
}

// TestSegmentScratch compares the segments in the scratch, and the count of
// the segments, with orthogonalSegmentsForPoints. One scratch gives the
// segments of all paths, so the test finds a segment that stays from the
// path before.
func TestSegmentScratch(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	var scratch segmentScratch
	for i := 0; i < 20000; i++ {
		points := randomPath(rng)
		want := orthogonalSegmentsForPoints(points)
		got := scratch.of(points)
		if len(got) != len(want) {
			t.Fatalf("path %v: %d segments in the scratch, want %d", points, len(got), len(want))
		}
		for j := range want {
			if *got[j] != *want[j] {
				t.Fatalf("path %v: segment %d is %+v, want %+v", points, j, *got[j], *want[j])
			}
		}
		if count := orthogonalSegmentCount(points); count != len(want) {
			t.Fatalf("path %v: orthogonalSegmentCount is %d, want %d", points, count, len(want))
		}
		if count := orthogonalSegmentCount(points, mgEpsLocal); count != len(orthogonalSegmentsForPoints(points, mgEpsLocal)) {
			t.Fatalf("path %v: orthogonalSegmentCount with an epsilon is %d", points, count)
		}
	}
}

// TestSimplifyPolylineResult checks what the callers of simplifyPolyline
// count on: it does not change its input, and an append to its result does
// not write to memory that another slice has.
func TestSimplifyPolylineResult(t *testing.T) {
	rng := rand.New(rand.NewSource(4))
	for i := 0; i < 20000; i++ {
		points := randomPath(rng)
		before := clonePoints(points)
		result := simplifyPolyline(points)
		if !reflect.DeepEqual(points, before) {
			t.Fatalf("simplifyPolyline changed its input %v to %v", before, points)
		}
		if len(points) < 3 {
			continue
		}
		if cap(result) != len(result) {
			t.Fatalf("path %v: the result has room for %d points and has %d", points, cap(result), len(result))
		}
	}
}
