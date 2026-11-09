// Ported from d3-shape 3.2.0, d3-shape/src/math.js and d3-shape/src/path.js
// (ISC).

package shape

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/internal/jsm"
	"github.com/bvolpato/mermaid-go-renderer/internal/d3/path"
)

// These are variables, not constants, so that Go computes with float64
// values as JavaScript does.
var (
	epsilon float64 = 1e-12
	pi      float64 = math.Pi
	halfPi          = pi / 2
	tau             = float64(2 * pi)
)

func acos(x float64) float64 {
	if x > 1 {
		return 0
	}
	if x < -1 {
		return pi
	}
	return jsm.Acos(x)
}

func asin(x float64) float64 {
	if x >= 1 {
		return halfPi
	}
	if x <= -1 {
		return -halfPi
	}
	return jsm.Asin(x)
}

// digits is the digits setting of a shape generator (withPath in path.js).
// The default is 3. A negative value is the null of d3: no rounding.
type digits struct {
	n int
}

func defaultDigits() digits { return digits{n: 3} }

// newPath is the function that withPath returns: a new path that rounds to
// the digits of the generator.
func (d digits) newPath() *path.Path {
	if d.n < 0 {
		return path.New()
	}
	p, _ := path.NewRound(float64(d.n))
	return p
}
