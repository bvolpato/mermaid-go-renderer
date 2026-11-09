// Ported from Chromium 156.0.8078.4, in part: the matrix of the SVG
// transform attribute, as Chromium computes it for layout. The types and
// the functions of this file follow the parts of Chromium that their
// comments name:
//
//   - gfx::Transform, gfx::AxisTransform2d, gfx::Matrix44 and
//     gfx::SinCosDegrees: ui/gfx/geometry (BSD-3-Clause).
//   - base::DegToRad: base/numerics/angle_conversions.h (BSD-3-Clause).
//   - AffineTransform::FromTransform:
//     platform/transforms/affine_transform.cc of Blink (BSD-2-Clause, with
//     the copyright of Apple and Google in the file).
//   - ComputedStyle::ApplyTransform: core/style/computed_style.cc of Blink
//     (LGPL-2.0-or-later).
//
// The Blink paths are in third_party/blink/renderer. The license of each
// file is the license in its header at that version. See NOTICE in this
// directory.

package browser

import (
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js/crmath"
)

// This file is the matrix of the SVG transform attribute, as Chromium
// computes it for layout. Blink maps the attribute to the CSS property
// transform and applies the functions one after the other to a
// gfx::Transform. The result is not the matrix of the SVG DOM
// (transform.baseVal), which AffineTransform builds in double:
//
//   - While the transform is a scale and a translation only, gfx::Transform
//     keeps it as a gfx::AxisTransform2d in float32. Translations and scales
//     add up in float32 there.
//   - A rotation, a skew or a matrix function turns it into a gfx::Matrix44
//     in double.
//   - The sine and the cosine of a multiple of 45 degrees are exact
//     constants (gfx::SinCosDegrees).

// gfxTransform is gfx::Transform, for the 2D functions of a transform list.
type gfxTransform struct {
	// full is full_matrix_: the transform is in m. Otherwise it is the
	// axis-aligned transform of sx, sy, tx and ty.
	full           bool
	sx, sy, tx, ty float32
	// m is the 2D part of the gfx::Matrix44: the columns (a, b), (c, d) and
	// (e, f).
	m affine
}

// ensureFull is Transform::EnsureFullMatrix.
func (t *gfxTransform) ensureFull() {
	if t.full {
		return
	}
	t.m = affine{a: float64(t.sx), d: float64(t.sy), e: float64(t.tx), f: float64(t.ty)}
	t.full = true
}

// translate is Transform::Translate(float x, float y).
func (t *gfxTransform) translate(x, y float32) {
	if !t.full {
		// AxisTransform2d::PreTranslate.
		t.tx += float32(x * t.sx)
		t.ty += float32(y * t.sy)
		return
	}
	// Matrix44::PreTranslate: col3 = col0 * dx + col1 * dy + col3.
	dx, dy := float64(x), float64(y)
	t.m.e = float64(t.m.a*dx) + float64(t.m.c*dy) + t.m.e
	t.m.f = float64(t.m.b*dx) + float64(t.m.d*dy) + t.m.f
}

// scale is Transform::Scale(float x, float y).
func (t *gfxTransform) scale(x, y float32) {
	if !t.full {
		// AxisTransform2d::PreScale.
		t.sx *= x
		t.sy *= y
		return
	}
	// Matrix44::PreScale.
	t.m.a *= float64(x)
	t.m.b *= float64(x)
	t.m.c *= float64(y)
	t.m.d *= float64(y)
}

// sinCosDegrees is gfx::SinCosDegrees: the sine and the cosine of an angle
// in degrees, with exact results for the multiples of 45 degrees.
func sinCosDegrees(degrees float64) (sin, cos float64) {
	n45degrees := degrees / 45
	if math.Abs(n45degrees) >= 1<<31 || math.IsNaN(n45degrees) {
		// Upstream converts to int here, which is not defined for these
		// values. Such an angle has no place in a diagram.
		rad := degToRad(degrees)
		return crmath.Sin(rad), crmath.Cos(rad)
	}
	if n45degrees == math.Trunc(n45degrees) {
		s := math.Sqrt(0.5)
		switch int64(n45degrees) & 7 {
		case 0:
			return 0, 1
		case 1:
			return s, s
		case 2:
			return 1, 0
		case 3:
			return s, -s
		case 4:
			return 0, -1
		case 5:
			return -s, -s
		case 6:
			return -1, 0
		default:
			return -s, s
		}
	}
	// The angle is reduced to the range from -45 to 45 degrees before the
	// conversion to radians, which keeps the precision for a large angle.
	quadrant := math.Round(degrees / 90)
	rad := degToRad(degrees - float64(90*quadrant))
	s, c := crmath.Sin(rad), crmath.Cos(rad)
	switch int64(quadrant) & 3 {
	case 0:
		return s, c
	case 1:
		return c, -s
	case 2:
		return -s, -c
	default:
		return -c, s
	}
}

// degToRad is base::DegToRad.
func degToRad(degrees float64) float64 {
	return degrees * (math.Pi / 180)
}

// tanDegrees is gfx::TanDegrees.
func tanDegrees(degrees float64) float64 {
	return crmath.Tan(degToRad(degrees))
}

// rotate is Transform::Rotate(double degrees): a rotation about the z axis.
func (t *gfxTransform) rotate(degrees float64) {
	sin, cos := sinCosDegrees(degrees)
	if sin == 0 && cos == 1 {
		return
	}
	t.ensureFull()
	// Matrix44::RotateAboutZAxisSinCos.
	a, b, c, d := t.m.a, t.m.b, t.m.c, t.m.d
	t.m.a = float64(a*cos) + float64(c*sin)
	t.m.b = float64(b*cos) + float64(d*sin)
	t.m.c = float64(c*cos) - float64(a*sin)
	t.m.d = float64(d*cos) - float64(b*sin)
}

// skew is Transform::Skew(double degrees_x, double degrees_y).
func (t *gfxTransform) skew(degreesX, degreesY float64) {
	if degreesX == 0 && degreesY == 0 {
		return
	}
	t.ensureFull()
	// Matrix44::Skew.
	tanX, tanY := tanDegrees(degreesX), tanDegrees(degreesY)
	a, b, c, d := t.m.a, t.m.b, t.m.c, t.m.d
	t.m.a = a + float64(c*tanY)
	t.m.b = b + float64(d*tanY)
	t.m.c = c + float64(a*tanX)
	t.m.d = d + float64(b*tanX)
}

// preConcat is Transform::PreConcat with a transform that has a full matrix,
// as the matrix function of CSS makes.
func (t *gfxTransform) preConcat(o affine) {
	if !t.full {
		// The other transform, then this axis-aligned transform after it.
		sx, sy, tx, ty := float64(t.sx), float64(t.sy), float64(t.tx), float64(t.ty)
		t.m = affine{
			a: float64(o.a * sx),
			b: float64(o.b * sy),
			c: float64(o.c * sx),
			d: float64(o.d * sy),
			e: float64(o.e*sx) + tx,
			f: float64(o.f*sy) + ty,
		}
		t.full = true
		return
	}
	// Matrix44::PreConcat.
	a, b, c, d := t.m.a, t.m.b, t.m.c, t.m.d
	t.m.a = float64(a*o.a) + float64(c*o.b)
	t.m.b = float64(b*o.a) + float64(d*o.b)
	t.m.c = float64(a*o.c) + float64(c*o.d)
	t.m.d = float64(b*o.c) + float64(d*o.d)
	t.m.e = float64(a*o.e) + float64(c*o.f) + t.m.e
	t.m.f = float64(b*o.e) + float64(d*o.f) + t.m.f
}

// affine is AffineTransform::FromTransform.
func (t *gfxTransform) affine() affine {
	t.ensureFull()
	return t.m
}

// parseTransformList parses an SVG transform attribute and returns the
// matrix that Chromium uses for layout. An invalid list gives the identity,
// as an invalid attribute does in the browser.
func parseTransformList(s string) affine {
	return parseTransformListOrigin(s, 0, 0)
}

// parseTransformListOrigin is parseTransformList with a transform-origin:
// the list is applied about the point (ox, oy). ComputedStyle::ApplyTransform
// translates to the origin, applies the functions and translates back, all
// on one gfx::Transform. A list with no function has no origin.
func parseTransformListOrigin(s string, ox, oy float32) affine {
	t := gfxTransform{sx: 1, sy: 1}
	s = skipSVGSpace(s)
	if s == "" {
		return identity
	}
	hasOrigin := ox != 0 || oy != 0
	if hasOrigin {
		t.translate(ox, oy)
	}
	for s != "" {
		open := strings.IndexByte(s, '(')
		if open < 0 {
			return identity
		}
		name := strings.TrimSpace(s[:open])
		closeIdx := strings.IndexByte(s, ')')
		if closeIdx < open {
			return identity
		}
		args := parseNumberList(s[open+1 : closeIdx])
		arg := func(i int) float32 {
			if i < len(args) {
				return args[i]
			}
			return 0
		}
		switch name {
		case "translate":
			if len(args) < 1 || len(args) > 2 {
				return identity
			}
			// The arguments are CSS lengths in Blink, which have limits.
			t.translate(clampCSSLength(arg(0)), clampCSSLength(arg(1)))
		case "scale":
			if len(args) < 1 || len(args) > 2 {
				return identity
			}
			sy := arg(0)
			if len(args) == 2 {
				sy = arg(1)
			}
			t.scale(arg(0), sy)
		case "rotate":
			switch len(args) {
			case 1:
				t.rotate(float64(arg(0)))
			case 3:
				// A rotation about a point is a translation to the point,
				// the rotation, and the translation back.
				t.translate(arg(1), arg(2))
				t.rotate(float64(arg(0)))
				t.translate(-arg(1), -arg(2))
			default:
				return identity
			}
		case "skewX":
			if len(args) != 1 {
				return identity
			}
			t.skew(float64(arg(0)), 0)
		case "skewY":
			if len(args) != 1 {
				return identity
			}
			t.skew(0, float64(arg(0)))
		case "matrix":
			if len(args) != 6 {
				return identity
			}
			t.preConcat(affine{float64(arg(0)), float64(arg(1)), float64(arg(2)), float64(arg(3)), float64(arg(4)), float64(arg(5))})
		default:
			return identity
		}
		s = skipSVGSpaceComma(s[closeIdx+1:])
	}
	if hasOrigin {
		t.translate(-ox, -oy)
	}
	return t.affine()
}
