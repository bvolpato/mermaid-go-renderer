// This file ports no upstream file. It has Math.tanh.
//
// V8 15.6 calls std::tanh of the C library for Math.tanh
// (src/base/ieee754.cc). So the last bits of a result depend on the platform
// of the browser:
//
//   - Linux: glibc, which has the tanh of fdlibm (linux_libm_tanh.go).
//   - macOS: libsystem_m of Apple, which is closed source. The module has no
//     port of it. TanhDarwin returns the results of Linux (platform.go).
//
// The golden files of the two platforms differ for 18602 of 220000 arguments
// of tanh, by at most two units in the last place.

package crmath

// darwinLibmTanh is the place of a tanh of the C library of macOS. No file
// of the module sets it. It is nil, and the platform Darwin has the tanh of
// Linux.
var darwinLibmTanh func(x float64) float64

// Tanh is Math.tanh of the current platform.
func Tanh(x float64) float64 {
	if CurrentPlatform() == PlatformLinux {
		return TanhLinux(x)
	}
	return TanhDarwin(x)
}

// TanhLinux is Math.tanh of Chromium on Linux. It does not read the current
// platform.
func TanhLinux(x float64) float64 { return linuxTanh(x) }

// TanhDarwin is Math.tanh of Chromium on macOS. It does not read the
// current platform. It is TanhLinux when the package is built without the
// tanh of macOS (see HasDarwinLibm).
func TanhDarwin(x float64) float64 {
	if libm := darwinLibmTanh; libm != nil {
		return libm(x)
	}
	return linuxTanh(x)
}
