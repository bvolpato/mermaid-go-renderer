// Helpers for the JavaScript semantics that the files of this package
// share. They port no upstream file.

package swimlanes

import (
	"math"
	"strconv"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// sortFunc is Array.prototype.sort(cmp) of Chromium. Some comparators of
// this layout are not strict weak orders (they compare with a threshold),
// so the result depends on the sort algorithm of the engine.
func sortFunc[T any](s []T, cmp func(a, b T) float64) {
	js.SortV8(js.V8PowerSort, s, cmp)
}

// jsError is an error that upstream throws while it lays out. The layout
// functions raise it with panic, and the layout core returns it as an
// error.
type jsError string

func (e jsError) Error() string { return string(e) }

// localeCompare is a.localeCompare(b).
func localeCompare(a, b string) float64 {
	return float64(js.LocaleCompare(a, b))
}

// cat is the array literal [...a, ...b, ...]: a new slice with the points
// of every part.
func cat(parts ...[]Point) []Point {
	n := 0
	for _, p := range parts {
		n += len(p)
	}
	out := make([]Point, 0, n)
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// clonePoints is points.map((p) => ({ ...p })) and [...points].
func clonePoints(points []Point) []Point {
	return append([]Point{}, points...)
}

// boolNum is Number(b) for a boolean.
func boolNum(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// endpointIdsOf is [edge.start, edge.end].filter(Boolean).
func endpointIdsOf(start, end string) []string {
	ids := make([]string, 0, 2)
	if start != "" {
		ids = append(ids, start)
	}
	if end != "" {
		ids = append(ids, end)
	}
	return ids
}

// containsString is ids.includes(id).
func containsString(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

// toFixed is x.toFixed(digits).
func toFixed(x float64, digits int) string {
	var buf [32]byte
	return string(appendFixed(buf[:0], x, digits))
}

// appendFixed appends x.toFixed(digits) to b. The layout builds many keys
// with it, so the common case does not go through the arbitrary-precision
// path of js.ToFixed: strconv rounds the exact value of x to the digits, as
// toFixed does. The two differ for a tie only (toFixed takes the larger
// number, strconv the even one), and x is a tie exactly when x *
// 2^(digits+1) is an odd integer.
func appendFixed(b []byte, x float64, digits int) []byte {
	if x == 0 {
		// (-0).toFixed is "0".
		x = 0
	}
	ax := math.Abs(x)
	if digits == 3 && ax < 1e12 {
		// The keys of the paths have three digits, and the cleanup passes
		// make very many of them. The digits are the integer nearest to
		// ax*1000. scaled is that product, rounded to a float64. Rounding
		// keeps the order of two numbers, and a number that ends in .5 is a
		// float64 here. So scaled and the exact product are on the same
		// side of such a number, unless scaled is that number: then the
		// code after this block decides.
		scaled := float64(ax * 1000)
		whole := math.Floor(scaled)
		if rest := scaled - whole; rest != 0.5 {
			n := uint64(whole)
			if rest > 0.5 {
				n++
			}
			if x < 0 {
				b = append(b, '-')
			}
			b = strconv.AppendUint(b, n/1000, 10)
			f := n % 1000
			return append(b, '.', byte('0'+f/100), byte('0'+f/10%10), byte('0'+f%10))
		}
	}
	if ax < 1e15 {
		scaled := x * float64(uint64(2)<<uint(digits))
		if scaled != math.Trunc(scaled) || math.Mod(scaled, 2) == 0 {
			return strconv.AppendFloat(b, x, 'f', digits, 64)
		}
	}
	return append(b, js.ToFixed(x, digits)...)
}

// pointKey3 is `${x.toFixed(3)},${y.toFixed(3)}` for a point.
func pointKey3(p Point) string {
	var buf [48]byte
	b := appendFixed(buf[:0], p.X, 3)
	b = append(b, ',')
	return string(appendFixed(b, p.Y, 3))
}

// pathKey3 is the key of a path that the cleanup passes use to tell
// candidates apart: its points with three decimals, joined with "|". The
// passes make a key for every candidate path of every edge, so the key is
// written into one buffer, with one string at the end.
func pathKey3(points []Point) string {
	var buf [256]byte
	b := buf[:0]
	for i, p := range points {
		if i > 0 {
			b = append(b, '|')
		}
		b = appendFixed(b, p.X, 3)
		b = append(b, ',')
		b = appendFixed(b, p.Y, 3)
	}
	return string(b)
}
