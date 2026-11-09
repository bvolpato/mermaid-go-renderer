// This file ports no upstream file. It selects the float32 trigonometric
// functions of the C library of a platform.

package browser

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/font"
)

// cmath has the float32 trigonometric functions of the C library of a
// platform. Skia and Blink call sinf, cosf, tanf and atan2f for arcs and
// for a point on a path, and the results of the C libraries differ in the
// last bit.
//
// On Linux the functions are those of the C library of the reference
// environment (linux_trig.go). On the other platforms they return the
// nearest float32, which is what the C library of macOS returns in the
// probe cases of this package, with one exception for atan2f in Skia
// (skScalarATan2).
type cmath struct{ linux bool }

func cmathOf(platform font.Platform) cmath {
	return cmath{linux: platform == font.PlatformLinux}
}

func (c cmath) sin(x float32) float32 {
	if c.linux {
		return linuxSinf(x)
	}
	return float32(math.Sin(float64(x)))
}

func (c cmath) cos(x float32) float32 {
	if c.linux {
		return linuxCosf(x)
	}
	return float32(math.Cos(float64(x)))
}

func (c cmath) tan(x float32) float32 {
	if c.linux {
		return linuxTanf(x)
	}
	return float32(math.Tan(float64(x)))
}

func (c cmath) atan2(y, x float32) float32 {
	if c.linux {
		return linuxAtan2f(y, x)
	}
	return float32(math.Atan2(float64(y), float64(x)))
}
