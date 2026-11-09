// Ported from mermaid 12.2.0, mermaid/src/utils/lineWithOffset.ts (MIT).

package utils

import "math"

// markerOffsets is markerOffsets. The lines are drawn a bit shorter to
// avoid drawing under any transparent markers. The offsets are calculated
// from the dimensions of the markers.
var markerOffsets = map[string]float64{
	"aggregation":    17.25,
	"extension":      17.25,
	"composition":    17.25,
	"dependency":     6,
	"lollipop":       13.5,
	"arrow_point":    4,
	"arrow_barb":     0,
	"arrow_barb_neo": 5.5,
	//arrow_cross: 24,
}

// markerOffsets2 is markerOffsets2.
var markerOffsets2 = map[string]float64{
	"arrow_point":  4,
	"arrow_cross":  12.5,
	"arrow_circle": 12.5,
}

// MarkerOffset is markerOffsets[arrowType]. The second result is false when
// the object has no such key (Object.hasOwn is false).
func MarkerOffset(arrowType string) (float64, bool) {
	v, ok := markerOffsets[arrowType]
	return v, ok
}

// MarkerOffset2 is markerOffsets2[arrowType]. The second result is false
// when the object has no such key.
func MarkerOffset2(arrowType string) (float64, bool) {
	v, ok := markerOffsets2[arrowType]
	return v, ok
}

// markerOffset is markerOffsets[arrowType]. A missing key is undefined
// upstream. It is NaN here: every comparison with it is false and every
// arithmetic result is NaN, as with undefined.
func markerOffset(arrowType string) float64 {
	if v, ok := markerOffsets[arrowType]; ok {
		return v
	}
	return math.NaN()
}

func hasMarkerOffset(arrowType string) bool {
	_, ok := markerOffsets[arrowType]
	return ok
}

// calculateDeltaAndAngle calculates the deltas and angle between two
// points. A nil point is an undefined point.
func calculateDeltaAndAngle(point1, point2 *Point) (angle, deltaX, deltaY float64) {
	if point1 == nil || point2 == nil {
		return 0, 0, 0
	}
	x1, y1 := point1.X, point1.Y
	x2, y2 := point2.X, point2.Y
	deltaX = x2 - x1
	deltaY = y2 - y1
	return atan(deltaY / deltaX), deltaX, deltaY
}

// pointAt is data[i], or nil when the index is out of range.
func pointAt(data []Point, i int) *Point {
	if i < 0 || i >= len(data) {
		return nil
	}
	return &data[i]
}

// LineFunctionsWithOffset is the result of getLineFunctionsWithOffset: the
// x and y accessors of a d3 line. Each takes the datum, its index and the
// whole data. With empty data, where upstream throws a TypeError, an
// accessor returns the coordinate of the datum.
type LineFunctionsWithOffset struct {
	X func(d Point, i int, data []Point) float64
	Y func(d Point, i int, data []Point) float64
}

// GetLineFunctionsWithOffset is getLineFunctionsWithOffset(edge). The
// arguments are edge.arrowTypeStart and edge.arrowTypeEnd; an empty string
// stands for undefined.
func GetLineFunctionsWithOffset(arrowTypeStart, arrowTypeEnd string) LineFunctionsWithOffset {
	return LineFunctionsWithOffset{
		X: func(d Point, i int, data []Point) float64 {
			if len(data) == 0 {
				return d.X
			}
			offset := 0.0
			DIRECTION := "right"
			if data[0].X < data[len(data)-1].X {
				DIRECTION = "left"
			}
			if i == 0 && hasMarkerOffset(arrowTypeStart) {
				angle, deltaX, _ := calculateDeltaAndAngle(pointAt(data, 0), pointAt(data, 1))
				offset = float64(markerOffset(arrowTypeStart) * cos(angle) * sign(deltaX))
			} else if i == len(data)-1 && hasMarkerOffset(arrowTypeEnd) {
				angle, deltaX, _ := calculateDeltaAndAngle(pointAt(data, len(data)-1), pointAt(data, len(data)-2))
				offset = float64(markerOffset(arrowTypeEnd) * cos(angle) * sign(deltaX))
			}

			differenceToEnd := math.Abs(d.X - data[len(data)-1].X)
			differenceInYEnd := math.Abs(d.Y - data[len(data)-1].Y)
			differenceToStart := math.Abs(d.X - data[0].X)
			differenceInYStart := math.Abs(d.Y - data[0].Y)
			startMarkerHeight := markerOffset(arrowTypeStart)
			endMarkerHeight := markerOffset(arrowTypeEnd)
			const extraRoom = 1

			// Adjust the offset if the difference is smaller than the marker height
			if differenceToEnd < endMarkerHeight &&
				differenceToEnd > 0 &&
				differenceInYEnd < endMarkerHeight {
				adjustment := endMarkerHeight + extraRoom - differenceToEnd
				if DIRECTION == "right" {
					adjustment = -adjustment
				}
				// Adjust the offset by the amount needed to fit the marker
				offset -= adjustment
			}

			if differenceToStart < startMarkerHeight &&
				differenceToStart > 0 &&
				differenceInYStart < startMarkerHeight {
				adjustment := startMarkerHeight + extraRoom - differenceToStart
				if DIRECTION == "right" {
					adjustment = -adjustment
				}
				offset += adjustment
			}

			return d.X + offset
		},
		Y: func(d Point, i int, data []Point) float64 {
			if len(data) == 0 {
				return d.Y
			}
			offset := 0.0
			DIRECTION := "up"
			if data[0].Y < data[len(data)-1].Y {
				DIRECTION = "down"
			}
			if i == 0 && hasMarkerOffset(arrowTypeStart) {
				angle, _, deltaY := calculateDeltaAndAngle(pointAt(data, 0), pointAt(data, 1))
				offset = float64(markerOffset(arrowTypeStart) * math.Abs(sin(angle)) * sign(deltaY))
			} else if i == len(data)-1 && hasMarkerOffset(arrowTypeEnd) {
				angle, _, deltaY := calculateDeltaAndAngle(pointAt(data, len(data)-1), pointAt(data, len(data)-2))
				offset = float64(markerOffset(arrowTypeEnd) * math.Abs(sin(angle)) * sign(deltaY))
			}

			differenceToEnd := math.Abs(d.Y - data[len(data)-1].Y)
			differenceInXEnd := math.Abs(d.X - data[len(data)-1].X)
			differenceToStart := math.Abs(d.Y - data[0].Y)
			differenceInXStart := math.Abs(d.X - data[0].X)
			startMarkerHeight := markerOffset(arrowTypeStart)
			endMarkerHeight := markerOffset(arrowTypeEnd)
			const extraRoom = 1

			// Adjust the offset if the difference is smaller than the marker height
			if differenceToEnd < endMarkerHeight &&
				differenceToEnd > 0 &&
				differenceInXEnd < endMarkerHeight {
				adjustment := endMarkerHeight + extraRoom - differenceToEnd
				if DIRECTION == "up" {
					adjustment = -adjustment
				}
				// Adjust the offset by the amount needed to fit the marker
				offset -= adjustment
			}

			if differenceToStart < startMarkerHeight &&
				differenceToStart > 0 &&
				differenceInXStart < startMarkerHeight {
				adjustment := startMarkerHeight + extraRoom - differenceToStart
				if DIRECTION == "up" {
					adjustment = -adjustment
				}
				offset += adjustment
			}
			return d.Y + offset
		},
	}
}

// sign is (delta >= 0 ? 1 : -1).
func sign(delta float64) float64 {
	if delta >= 0 {
		return 1
	}
	return -1
}
