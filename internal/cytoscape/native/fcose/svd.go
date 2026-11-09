// Ported from layout-base 2.0.1 (MIT license): src/util/SVD.js.
//
// The singular value decomposition and the function hypot of that file are
// adopted from JamaJS (https://github.com/dragonfly-ai/JamaJS), which is
// under the Apache License, Version 2.0 (http://www.apache.org/licenses/).
// layout-base changed the code so that the input is a 2d array and the
// result is an object with the components.

package fcose

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native/internal/bail"
)

// svdResult has the components of a singular value decomposition.
type svdResult struct {
	U, V [][]float64
	S    []float64
}

// svd is SVD.svd. It changes a. The port covers a square matrix: the two
// callers pass one, and for another shape the JavaScript reads and writes
// array members that do not exist.
func svd(a [][]float64) svdResult {
	m := len(a)
	n := len(a[0])
	if m != n || m < 2 {
		bail.Out("fcose: the singular values of a matrix of %d by %d", m, n)
	}
	for _, row := range a {
		if len(row) != n {
			bail.Out("fcose: the singular values of a matrix with rows of different lengths")
		}
	}
	nu := min(m, n)
	s := make([]float64, min(m+1, n))
	u := make([][]float64, m)
	for i := range u {
		u[i] = make([]float64, nu)
	}
	v := make([][]float64, n)
	for i := range v {
		v[i] = make([]float64, n)
	}
	e := make([]float64, n)
	work := make([]float64, m)
	const wantu = true
	const wantv = true
	nct := min(m-1, n)
	nrt := max(0, min(n-2, m))
	for k := 0; k < max(nct, nrt); k++ {
		if k < nct {
			s[k] = 0
			for i := k; i < m; i++ {
				s[k] = svdHypot(s[k], a[i][k])
			}
			if s[k] != 0.0 {
				if a[k][k] < 0.0 {
					s[k] = -s[k]
				}
				for i := k; i < m; i++ {
					a[i][k] /= s[k]
				}
				a[k][k] += 1.0
			}
			s[k] = -s[k]
		}
		for j := k + 1; j < n; j++ {
			if k < nct && s[k] != 0.0 {
				t := 0.0
				for i := k; i < m; i++ {
					t += float64(a[i][k] * a[i][j])
				}
				t = -t / a[k][k]
				for i := k; i < m; i++ {
					a[i][j] += float64(t * a[i][k])
				}
			}
			e[j] = a[k][j]
		}
		if wantu && k < nct {
			for i := k; i < m; i++ {
				u[i][k] = a[i][k]
			}
		}
		if k < nrt {
			e[k] = 0
			for i := k + 1; i < n; i++ {
				e[k] = svdHypot(e[k], e[i])
			}
			if e[k] != 0.0 {
				if e[k+1] < 0.0 {
					e[k] = -e[k]
				}
				for i := k + 1; i < n; i++ {
					e[i] /= e[k]
				}
				e[k+1] += 1.0
			}
			e[k] = -e[k]
			if k+1 < m && e[k] != 0.0 {
				for i := k + 1; i < m; i++ {
					work[i] = 0.0
				}
				for j := k + 1; j < n; j++ {
					for i := k + 1; i < m; i++ {
						work[i] += float64(e[j] * a[i][j])
					}
				}
				for j := k + 1; j < n; j++ {
					t := -e[j] / e[k+1]
					for i := k + 1; i < m; i++ {
						a[i][j] += float64(t * work[i])
					}
				}
			}
			if wantv {
				for i := k + 1; i < n; i++ {
					v[i][k] = e[i]
				}
			}
		}
	}
	p := min(n, m+1)
	if nct < n {
		s[nct] = a[nct][nct]
	}
	if m < p {
		s[p-1] = 0.0
	}
	if nrt+1 < p {
		e[nrt] = a[nrt][p-1]
	}
	e[p-1] = 0.0
	if wantu {
		for j := nct; j < nu; j++ {
			for i := 0; i < m; i++ {
				u[i][j] = 0.0
			}
			u[j][j] = 1.0
		}
		for k := nct - 1; k >= 0; k-- {
			if s[k] != 0.0 {
				for j := k + 1; j < nu; j++ {
					t := 0.0
					for i := k; i < m; i++ {
						t += float64(u[i][k] * u[i][j])
					}
					t = -t / u[k][k]
					for i := k; i < m; i++ {
						u[i][j] += float64(t * u[i][k])
					}
				}
				for i := k; i < m; i++ {
					u[i][k] = -u[i][k]
				}
				u[k][k] = 1.0 + u[k][k]
				for i := 0; i < k-1; i++ {
					u[i][k] = 0.0
				}
			} else {
				for i := 0; i < m; i++ {
					u[i][k] = 0.0
				}
				u[k][k] = 1.0
			}
		}
	}
	if wantv {
		for k := n - 1; k >= 0; k-- {
			if k < nrt && e[k] != 0.0 {
				for j := k + 1; j < nu; j++ {
					t := 0.0
					for i := k + 1; i < n; i++ {
						t += float64(v[i][k] * v[i][j])
					}
					t = -t / v[k+1][k]
					for i := k + 1; i < n; i++ {
						v[i][j] += float64(t * v[i][k])
					}
				}
			}
			for i := 0; i < n; i++ {
				v[i][k] = 0.0
			}
			v[k][k] = 1.0
		}
	}
	pp := p - 1
	// eps is Math.pow(2.0, -52.0) and tiny is Math.pow(2.0, -966.0): exact
	// powers of two.
	eps := math.Ldexp(1, -52)
	tiny := math.Ldexp(1, -966)
	for p > 0 {
		var k, kase int
		for k = p - 2; k >= -1; k-- {
			if k == -1 {
				break
			}
			if math.Abs(e[k]) <= tiny+float64(eps*(math.Abs(s[k])+math.Abs(s[k+1]))) {
				e[k] = 0.0
				break
			}
		}
		if k == p-2 {
			kase = 4
		} else {
			var ks int
			for ks = p - 1; ks >= k; ks-- {
				if ks == k {
					break
				}
				t := 0.0
				if ks != p {
					t = math.Abs(e[ks])
				}
				if ks != k+1 {
					t += math.Abs(e[ks-1])
				} else {
					t += 0.0
				}
				if math.Abs(s[ks]) <= tiny+float64(eps*t) {
					s[ks] = 0.0
					break
				}
			}
			if ks == k {
				kase = 3
			} else if ks == p-1 {
				kase = 1
			} else {
				kase = 2
				k = ks
			}
		}
		k++
		switch kase {
		case 1:
			f := e[p-2]
			e[p-2] = 0.0
			for j := p - 2; j >= k; j-- {
				t := svdHypot(s[j], f)
				cs := s[j] / t
				sn := f / t
				s[j] = t
				if j != k {
					f = -sn * e[j-1]
					e[j-1] = cs * e[j-1]
				}
				if wantv {
					for i := 0; i < n; i++ {
						t = float64(cs*v[i][j]) + float64(sn*v[i][p-1])
						v[i][p-1] = float64(-sn*v[i][j]) + float64(cs*v[i][p-1])
						v[i][j] = t
					}
				}
			}
		case 2:
			f := e[k-1]
			e[k-1] = 0.0
			for j := k; j < p; j++ {
				t := svdHypot(s[j], f)
				cs := s[j] / t
				sn := f / t
				s[j] = t
				f = -sn * e[j]
				e[j] = cs * e[j]
				if wantu {
					for i := 0; i < m; i++ {
						t = float64(cs*u[i][j]) + float64(sn*u[i][k-1])
						u[i][k-1] = float64(-sn*u[i][j]) + float64(cs*u[i][k-1])
						u[i][j] = t
					}
				}
			}
		case 3:
			scale := max(max(max(max(math.Abs(s[p-1]), math.Abs(s[p-2])), math.Abs(e[p-2])), math.Abs(s[k])), math.Abs(e[k]))
			sp := s[p-1] / scale
			spm1 := s[p-2] / scale
			epm1 := e[p-2] / scale
			sk := s[k] / scale
			ek := e[k] / scale
			b := float64((float64((spm1+sp)*(spm1-sp)) + float64(epm1*epm1)) / 2.0)
			c := float64(sp * epm1 * (sp * epm1))
			shift := 0.0
			if b != 0.0 || c != 0.0 {
				shift = math.Sqrt(float64(b*b) + c)
				if b < 0.0 {
					shift = -shift
				}
				shift = c / (b + shift)
			}
			f := float64((sk+sp)*(sk-sp)) + shift
			g := sk * ek
			for j := k; j < p-1; j++ {
				t := svdHypot(f, g)
				cs := f / t
				sn := g / t
				if j != k {
					e[j-1] = t
				}
				f = float64(cs*s[j]) + float64(sn*e[j])
				e[j] = float64(cs*e[j]) - float64(sn*s[j])
				g = sn * s[j+1]
				s[j+1] = cs * s[j+1]
				if wantv {
					for i := 0; i < n; i++ {
						t = float64(cs*v[i][j]) + float64(sn*v[i][j+1])
						v[i][j+1] = float64(-sn*v[i][j]) + float64(cs*v[i][j+1])
						v[i][j] = t
					}
				}
				t = svdHypot(f, g)
				cs = f / t
				sn = g / t
				s[j] = t
				f = float64(cs*e[j]) + float64(sn*s[j+1])
				s[j+1] = float64(-sn*e[j]) + float64(cs*s[j+1])
				g = sn * e[j+1]
				e[j+1] = cs * e[j+1]
				if wantu && j < m-1 {
					for i := 0; i < m; i++ {
						t = float64(cs*u[i][j]) + float64(sn*u[i][j+1])
						u[i][j+1] = float64(-sn*u[i][j]) + float64(cs*u[i][j+1])
						u[i][j] = t
					}
				}
			}
			e[p-2] = f
		case 4:
			if s[k] <= 0.0 {
				if s[k] < 0.0 {
					s[k] = -s[k]
				} else {
					s[k] = 0.0
				}
				if wantv {
					for i := 0; i <= pp; i++ {
						v[i][k] = -v[i][k]
					}
				}
			}
			for k < pp {
				if s[k] >= s[k+1] {
					break
				}
				t := s[k]
				s[k] = s[k+1]
				s[k+1] = t
				if wantv && k < n-1 {
					for i := 0; i < n; i++ {
						t = v[i][k+1]
						v[i][k+1] = v[i][k]
						v[i][k] = t
					}
				}
				if wantu && k < m-1 {
					for i := 0; i < m; i++ {
						t = u[i][k+1]
						u[i][k+1] = u[i][k]
						u[i][k] = t
					}
				}
				k++
			}
			p--
		}
	}
	return svdResult{U: u, V: v, S: s}
}

// svdHypot is SVD.hypot: sqrt(a^2 + b^2) without under/overflow.
func svdHypot(a, b float64) float64 {
	var r float64
	if math.Abs(a) > math.Abs(b) {
		r = b / a
		r = math.Abs(a) * math.Sqrt(1+float64(r*r))
	} else if b != 0 {
		r = a / b
		r = math.Abs(b) * math.Sqrt(1+float64(r*r))
	} else {
		r = 0.0
	}
	return r
}
