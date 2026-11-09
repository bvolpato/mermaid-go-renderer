// Ported from @upsetjs/venn.js 2.0.0, build/venn.esm.js: the functions of
// the fmin package that the build holds (bisect, nelderMead,
// wolfeLineSearch, conjugateGradient and the vector helpers).

package venn

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// bisect is bisect(f, a, b, parameters) with no parameters.
//
// finds the zeros of a function, given two starting points (which must
// have opposite signs
func bisect(f func(float64) float64, a, b float64) float64 {
	const maxIterations = 100
	const tolerance = 1e-10
	fA := f(a)
	fB := f(b)
	delta := b - a

	if fA*fB > 0 {
		throw("Error", "Initial bisect points must have opposite signs")
	}

	if fA == 0 {
		return a
	}
	if fB == 0 {
		return b
	}

	for i := 0; i < maxIterations; i++ {
		delta = float64(delta / 2)
		mid := a + delta
		fMid := f(mid)

		if fMid*fA >= 0 {
			a = mid
		}

		if math.Abs(delta) < tolerance || fMid == 0 {
			return mid
		}
	}
	return a + delta
}

// need some basic operations on vectors, rather than adding a dependency,
// just define here

// zeros is zeros(x).
func zeros(x int) []float64 { return make([]float64, x) }

// zerosM is zerosM(x, y).
func zerosM(x, y int) [][]float64 {
	r := make([][]float64, x)
	for i := range r {
		r[i] = zeros(y)
	}
	return r
}

// dot is dot(a, b).
func dot(a, b []float64) float64 {
	ret := 0.0
	for i := 0; i < len(a); i++ {
		ret += float64(a[i] * b[i])
	}
	return ret
}

// norm2 is norm2(a).
func norm2(a []float64) float64 {
	return math.Sqrt(dot(a, a))
}

// scale is scale(ret, value, c).
func scale(ret, value []float64, c float64) {
	for i := 0; i < len(value); i++ {
		ret[i] = float64(value[i] * c)
	}
}

// weightedSum is weightedSum(ret, w1, v1, w2, v2).
func weightedSum(ret []float64, w1 float64, v1 []float64, w2 float64, v2 []float64) {
	for j := 0; j < len(ret); j++ {
		ret[j] = float64(w1*v1[j]) + float64(w2*v2[j])
	}
}

// vector is an array of numbers with the properties fx and id, as the
// points of the simplex of nelderMead are.
type vector struct {
	x  []float64
	fx float64
	id int
}

// nelderMeadParams are the parameters of nelderMead that the callers of
// venn.js set. A zero value is an undefined parameter.
type nelderMeadParams struct {
	maxIterations float64
	minErrorDelta float64
}

// orFloat is a || b for numbers.
func orFloat(a, b float64) float64 {
	if a != 0 && !math.IsNaN(a) {
		return a
	}
	return b
}

// nelderMead is nelderMead(f, x0, parameters): it minimizes a function using
// the downhill simplex method. It returns { fx, x }.
func (e *engine) nelderMead(f func(x []float64) float64, x0 []float64, parameters nelderMeadParams) *vector {
	maxIterations := orFloat(parameters.maxIterations, float64(len(x0)*200))
	const nonZeroDelta = 1.05
	const zeroDelta = 0.001
	minErrorDelta := orFloat(parameters.minErrorDelta, 1e-6)
	minTolerance := orFloat(parameters.minErrorDelta, 1e-5)
	// rho, chi, psi and sigma have their defaults. The weights that the
	// steps make from them are exact, so constants give the same numbers.
	const rho = 1.0
	const chi = 2.0
	const psi = -0.5
	const sigma = 0.5
	var maxDiff float64

	// initialize simplex.
	N := len(x0)
	simplex := make([]*vector, N+1)
	simplex[0] = &vector{x: x0}
	simplex[0].fx = f(x0)
	simplex[0].id = 0
	for i := 0; i < N; i++ {
		point := append([]float64(nil), x0...)
		if point[i] != 0 && !math.IsNaN(point[i]) {
			point[i] = point[i] * nonZeroDelta
		} else {
			point[i] = zeroDelta
		}
		simplex[i+1] = &vector{x: point}
		simplex[i+1].fx = f(point)
		simplex[i+1].id = i + 1
	}

	updateSimplex := func(value *vector) {
		for i := 0; i < len(value.x); i++ {
			simplex[N].x[i] = value.x[i]
		}
		simplex[N].fx = value.fx
	}

	sortOrder := func(a, b *vector) float64 { return a.fx - b.fx }

	centroid := &vector{x: append([]float64(nil), x0...)}
	reflected := &vector{x: append([]float64(nil), x0...)}
	contracted := &vector{x: append([]float64(nil), x0...)}
	expanded := &vector{x: append([]float64(nil), x0...)}

	for iteration := 0.0; iteration < maxIterations; iteration++ {
		sortBy(e, simplex, sortOrder)

		maxDiff = 0
		for i := 0; i < N; i++ {
			maxDiff = js.Max(maxDiff, math.Abs(simplex[0].x[i]-simplex[1].x[i]))
		}

		if math.Abs(simplex[0].fx-simplex[N].fx) < minErrorDelta && maxDiff < minTolerance {
			break
		}

		// compute the centroid of all but the worst point in the simplex
		for i := 0; i < N; i++ {
			centroid.x[i] = 0
			for j := 0; j < N; j++ {
				centroid.x[i] += simplex[j].x[i]
			}
			centroid.x[i] /= float64(N)
		}

		// reflect the worst point past the centroid  and compute loss at reflected
		// point
		worst := simplex[N]
		weightedSum(reflected.x, 1+rho, centroid.x, -rho, worst.x)
		reflected.fx = f(reflected.x)

		// if the reflected point is the best seen, then possibly expand
		if reflected.fx < simplex[0].fx {
			weightedSum(expanded.x, 1+chi, centroid.x, -chi, worst.x)
			expanded.fx = f(expanded.x)
			if expanded.fx < reflected.fx {
				updateSimplex(expanded)
			} else {
				updateSimplex(reflected)
			}
		} else if reflected.fx >= simplex[N-1].fx {
			// if the reflected point is worse than the second worst, we need to
			// contract
			shouldReduce := false

			if reflected.fx > worst.fx {
				// do an inside contraction
				weightedSum(contracted.x, 1+psi, centroid.x, -psi, worst.x)
				contracted.fx = f(contracted.x)
				if contracted.fx < worst.fx {
					updateSimplex(contracted)
				} else {
					shouldReduce = true
				}
			} else {
				// do an outside contraction
				weightedSum(contracted.x, 1-psi*rho, centroid.x, psi*rho, worst.x)
				contracted.fx = f(contracted.x)
				if contracted.fx < reflected.fx {
					updateSimplex(contracted)
				} else {
					shouldReduce = true
				}
			}

			if shouldReduce {
				// if we don't contract here, we're done
				if sigma >= 1 {
					break
				}

				// do a reduction
				for i := 1; i < len(simplex); i++ {
					weightedSum(simplex[i].x, 1-sigma, simplex[0].x, sigma, simplex[i].x)
					simplex[i].fx = f(simplex[i].x)
				}
			}
		} else {
			updateSimplex(reflected)
		}
	}

	sortBy(e, simplex, sortOrder)
	return &vector{fx: simplex[0].fx, x: simplex[0].x}
}

// cgState is the object { x, fx, fxprime } of conjugateGradient.
type cgState struct {
	x       []float64
	fx      float64
	fxprime []float64
}

// wolfeLineSearch is wolfeLineSearch(f, pk, current, next, a, c1, c2) with
// c1 and c2 undefined.
//
// searches along line 'pk' for a point that satifies the wolfe conditions
// See 'Numerical Optimization' by Nocedal and Wright p59-60
// f : objective function
// pk : search direction
// current: object containing current gradient/loss
// next: output: contains next gradient/loss
// returns a: step size taken
func wolfeLineSearch(f func(x, fxprime []float64) float64, pk []float64, current, next *cgState, a float64) float64 {
	phi0 := current.fx
	phiPrime0 := dot(current.fxprime, pk)
	phi := phi0
	phiOld := phi0
	phiPrime := phiPrime0
	a0 := 0.0

	a = orFloat(a, 1)
	c1 := 1e-6
	c2 := 0.1

	zoom := func(aLo, aHigh, phiLo float64) float64 {
		for iteration := 0; iteration < 16; iteration++ {
			a = (aLo + aHigh) / 2
			weightedSum(next.x, 1.0, current.x, a, pk)
			next.fx = f(next.x, next.fxprime)
			phi = next.fx
			phiPrime = dot(next.fxprime, pk)

			if phi > phi0+float64(c1*a*phiPrime0) || phi >= phiLo {
				aHigh = a
			} else {
				if math.Abs(phiPrime) <= -c2*phiPrime0 {
					return a
				}

				if phiPrime*(aHigh-aLo) >= 0 {
					aHigh = aLo
				}

				aLo = a
				phiLo = phi
			}
		}

		return 0
	}

	for iteration := 0; iteration < 10; iteration++ {
		weightedSum(next.x, 1.0, current.x, a, pk)
		next.fx = f(next.x, next.fxprime)
		phi = next.fx
		phiPrime = dot(next.fxprime, pk)
		if phi > phi0+float64(c1*a*phiPrime0) || (iteration != 0 && phi >= phiOld) {
			return zoom(a0, a, phiOld)
		}

		if math.Abs(phiPrime) <= -c2*phiPrime0 {
			return a
		}

		if phiPrime >= 0 {
			return zoom(a, a0, phi)
		}

		phiOld = phi
		a0 = a
		a *= 2
	}

	return a
}

// conjugateGradient is conjugateGradient(f, initial, params). maxIterations
// is params.maxIterations: 0 stands for undefined.
func conjugateGradient(f func(x, fxprime []float64) float64, initial []float64, maxIterationsParam float64) *cgState {
	// allocate all memory up front here, keep out of the loop for perfomance
	// reasons
	current := &cgState{x: append([]float64(nil), initial...), fx: 0, fxprime: append([]float64(nil), initial...)}
	next := &cgState{x: append([]float64(nil), initial...), fx: 0, fxprime: append([]float64(nil), initial...)}
	yk := append([]float64(nil), initial...)
	a := 1.0

	maxIterations := orFloat(maxIterationsParam, float64(len(initial)*20))

	current.fx = f(current.x, current.fxprime)
	pk := append([]float64(nil), current.fxprime...)
	scale(pk, current.fxprime, -1)

	for i := 0.0; i < maxIterations; i++ {
		a = wolfeLineSearch(f, pk, current, next, a)

		if a == 0 || math.IsNaN(a) {
			// faiiled to find point that satifies wolfe conditions.
			// reset direction for next iteration
			scale(pk, current.fxprime, -1)
		} else {
			// update direction using Polak-Ribiere CG method
			weightedSum(yk, 1, next.fxprime, -1, current.fxprime)

			deltaK := dot(current.fxprime, current.fxprime)
			betaK := js.Max(0, dot(yk, next.fxprime)/deltaK)

			weightedSum(pk, betaK, pk, -1, next.fxprime)

			current, next = next, current
		}

		if norm2(current.fxprime) <= 1e-5 {
			break
		}
	}

	return current
}
