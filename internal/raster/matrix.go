// Ported from skia/src/core/SkMatrix.cpp and SkM44.cpp (Skia branch
// chrome/m156).

package raster

// Matrix is the affine part of SkMatrix, in float32:
//
//	| sx kx tx |
//	| ky sy ty |
type Matrix struct {
	sx, kx, tx float32
	ky, sy, ty float32
}

func (m *Matrix) isIdentity() bool {
	return m.sx == 1 && m.kx == 0 && m.tx == 0 && m.ky == 0 && m.sy == 1 && m.ty == 0
}

// isScaleTranslate is SkMatrix::isScaleTranslate.
func (m *Matrix) isScaleTranslate() bool { return m.kx == 0 && m.ky == 0 }

// rectStaysRect is SkMatrix::rectStaysRect: the matrix maps a rectangle to
// a rectangle.
func (m *Matrix) rectStaysRect() bool {
	if m.kx == 0 && m.ky == 0 {
		return m.sx != 0 && m.sy != 0
	}
	if m.sx == 0 && m.sy == 0 {
		return m.kx != 0 && m.ky != 0
	}
	return false
}

// mapPoints is SkMatrix::mapPoints.
func (m *Matrix) mapPoints(dst, src []Point) {
	switch {
	case m.isIdentity():
		copy(dst, src)
	case m.kx == 0 && m.ky == 0:
		for i, p := range src {
			dst[i] = Point{float32(p.X*m.sx) + m.tx, float32(p.Y*m.sy) + m.ty}
		}
	default:
		for i, p := range src {
			dst[i] = Point{
				float32(p.X*m.sx) + float32(p.Y*m.kx) + m.tx,
				float32(p.Y*m.sy) + float32(p.X*m.ky) + m.ty,
			}
		}
	}
}

func (m *Matrix) mapPoint(p Point) Point {
	var out [1]Point
	m.mapPoints(out[:], []Point{p})
	return out[0]
}

// mapVector is SkMatrix::mapVector: the translation does not apply.
func (m *Matrix) mapVector(v Point) Point {
	t := *m
	t.tx, t.ty = 0, 0
	return t.mapPoint(v)
}

// mapRect is SkMatrix::mapRect: the bounds of the mapped corners.
func (m *Matrix) mapRect(r Rect) Rect {
	if m.isScaleTranslate() {
		l := float32(r.L*m.sx) + m.tx
		t := float32(r.T*m.sy) + m.ty
		rr := float32(r.R*m.sx) + m.tx
		b := float32(r.B*m.sy) + m.ty
		return Rect{l, t, rr, b}.sorted()
	}
	var q [4]Point
	m.mapPoints(q[:], []Point{{r.L, r.T}, {r.R, r.T}, {r.R, r.B}, {r.L, r.B}})
	out := Rect{q[0].X, q[0].Y, q[0].X, q[0].Y}
	for _, p := range q[1:] {
		out.L, out.R = min(out.L, p.X), max(out.R, p.X)
		out.T, out.B = min(out.T, p.Y), max(out.B, p.Y)
	}
	return out
}

// invert returns the inverse matrix, computed in float64.
func (m *Matrix) invert() (Matrix, bool) {
	a, b, c, d := float64(m.sx), float64(m.kx), float64(m.ky), float64(m.sy)
	det := float64(a*d) - float64(b*c)
	if det == 0 || det != det {
		return Matrix{}, false
	}
	inv := 1 / det
	tx, ty := float64(m.tx), float64(m.ty)
	return Matrix{
		sx: float32(d * inv), kx: float32(-b * inv), tx: float32((float64(b*ty) - float64(d*tx)) * inv),
		ky: float32(-c * inv), sy: float32(a * inv), ty: float32((float64(c*tx) - float64(a*ty)) * inv),
	}, true
}

// Affine is blink::AffineTransform: [a c e; b d f] in float64. The painter
// accumulates the transforms of the document in it, as the property trees
// of Blink do, and converts the result to a float32 Matrix for Skia.
type Affine struct {
	A, B, C, D, E, F float64
}

// IdentityAffine is the identity transform.
var IdentityAffine = Affine{A: 1, D: 1}

// Mul returns m * o: o applies first.
func (m Affine) Mul(o Affine) Affine {
	return Affine{
		A: float64(o.A*m.A) + float64(o.B*m.C),
		B: float64(o.A*m.B) + float64(o.B*m.D),
		C: float64(o.C*m.A) + float64(o.D*m.C),
		D: float64(o.C*m.B) + float64(o.D*m.D),
		E: float64(o.E*m.A) + float64(o.F*m.C) + m.E,
		F: float64(o.E*m.B) + float64(o.F*m.D) + m.F,
	}
}

// Translate returns m with a translation applied first.
func (m Affine) Translate(tx, ty float64) Affine {
	return m.Mul(Affine{A: 1, D: 1, E: tx, F: ty})
}

// Scale returns m with a scale applied first.
func (m Affine) Scale(sx, sy float64) Affine {
	return m.Mul(Affine{A: sx, D: sy})
}

// IsIdentity reports whether m is the identity.
func (m Affine) IsIdentity() bool { return m == IdentityAffine }

// Invert returns the inverse transform.
func (m Affine) Invert() (Affine, bool) {
	det := float64(m.A*m.D) - float64(m.B*m.C)
	if det == 0 || det != det {
		return Affine{}, false
	}
	return Affine{
		A: m.D / det, B: -m.B / det, C: -m.C / det, D: m.A / det,
		E: (float64(m.C*m.F) - float64(m.D*m.E)) / det,
		F: (float64(m.B*m.E) - float64(m.A*m.F)) / det,
	}, true
}

// MapPoint maps a point.
func (m Affine) MapPoint(x, y float64) (float64, float64) {
	return float64(m.A*x) + float64(m.C*y) + m.E, float64(m.B*x) + float64(m.D*y) + m.F
}

// toMatrix converts to the float32 matrix of Skia.
func (m Affine) toMatrix() Matrix {
	return Matrix{sx: float32(m.A), kx: float32(m.C), tx: float32(m.E), ky: float32(m.B), sy: float32(m.D), ty: float32(m.F)}
}
