// Ported from roughjs 4.6.6, roughjs/bin/geometry.js (MIT).

package rough

import "math"

// lineLength is the length of the line from p1 to p2.
func lineLength(m *mathFuncs, p1, p2 *Point) float64 {
	return math.Sqrt(m.pow(p1[0]-p2[0], 2) + m.pow(p1[1]-p2[1], 2))
}
