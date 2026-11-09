// This file ports no upstream file. It has the atan2f that Skia calls for an
// arc, with the results of the C library of each platform that the probes
// of this package recorded.

package browser

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/font"
)

// skScalarATan2 is SkScalarATan2 of Skia: atan2f of the C library of the
// platform. On macOS atan2f of the C library (libsystem_m) does not always
// return the nearest float32.
//
// For y = 0 and a negative x the result is pi. The library of macOS
// returns 0x40490FDA, the float32 below pi, and not the nearest float32
// 0x40490FDB. SkPath::arcTo makes this call for an arc that starts or ends
// at the leftmost point of its ellipse, which is a common case. The test
// TestArcBoundsAgainstChromium shows the effect on the bounding box.
//
// For other arguments this function returns the nearest float32. The
// library of macOS differs from that in the last bit for about 6 percent of
// the arguments, and the port does not follow it there. On Linux the result
// is the result of the C library of the reference Linux environment
// (linux_trig.go), which returns the nearest float32 for pi: the arc cases
// of TestLinuxProbesAgainstChromium show it.
func skScalarATan2(y, x float32, platform font.Platform) float32 {
	if platform == font.PlatformLinux {
		return linuxAtan2f(y, x)
	}
	if platform == font.PlatformDarwin && y == 0 && x < 0 {
		const piBelow = 0x40490FDA
		if math.Signbit(float64(y)) {
			return -math.Float32frombits(piBelow)
		}
		return math.Float32frombits(piBelow)
	}
	return float32(math.Atan2(float64(y), float64(x)))
}
