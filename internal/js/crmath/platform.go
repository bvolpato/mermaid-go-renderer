// This file ports no upstream file. It has the platform of the reference
// browser.
//
// Chromium takes two Math functions from the C library of the operating
// system: pow and tanh. So their results depend on the platform. The other
// functions come from V8 and from LLVM libc, and both platforms return the
// same bits for them (220000 arguments of each function).

package crmath

import (
	"os"
	"strings"
	"sync/atomic"
)

// Platform is the operating system of the reference browser.
type Platform int32

const (
	// PlatformDarwin is Chromium 156 on macOS on arm64. Its C library is
	// libsystem_m of Apple.
	PlatformDarwin Platform = iota
	// PlatformLinux is Chromium 156 on Linux on arm64 with glibc 2.39, in
	// the image mcr.microsoft.com/playwright:v1.64.1-noble.
	PlatformLinux
)

// DefaultPlatform is the platform of a process that does not select one.
// It is Linux: the default fonts of the renderer follow Chromium on Linux
// too, and that is the reference that every machine can reproduce.
//
// The module has no implementation of the pow and tanh of macOS. Those two
// come from a closed-source library, so a port of them is not part of this
// repository. With PlatformDarwin the two functions return the Linux
// results (see HasDarwinLibm). The two platforms differ in the last bit for
// 0.13% of the arguments of pow and for 8% of the arguments of tanh, and
// no diagram of the three corpora has a result that depends on it.
const DefaultPlatform = PlatformLinux

// PlatformEnv is the environment variable that selects the platform of a
// process, for tests and tools. The values are "linux" and "darwin". The
// package reads it once, when the process starts. Another value selects
// DefaultPlatform.
const PlatformEnv = "MMDG_MATH"

// platform is the current platform, a Platform.
var platform atomic.Int32

func init() {
	p := DefaultPlatform
	if v, ok := ParsePlatform(os.Getenv(PlatformEnv)); ok {
		p = v
	}
	platform.Store(int32(p))
}

// SetPlatform selects the platform of Pow and Tanh for the process. Call it
// before the renders start: a render that runs during the call can mix the
// results of two platforms. The call is safe for concurrent use.
func SetPlatform(p Platform) { platform.Store(int32(p)) }

// CurrentPlatform returns the platform of Pow and Tanh. It is safe for
// concurrent use.
func CurrentPlatform() Platform { return Platform(platform.Load()) }

// ParsePlatform returns the platform with the name "linux" or "darwin". The
// case of the letters does not matter.
func ParsePlatform(name string) (p Platform, ok bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "linux":
		return PlatformLinux, true
	case "darwin":
		return PlatformDarwin, true
	}
	return DefaultPlatform, false
}

// String returns the name of the platform: "linux" or "darwin".
func (p Platform) String() string {
	if p == PlatformLinux {
		return "linux"
	}
	return "darwin"
}

// HasDarwinLibm reports whether the package has the pow and the tanh of the
// C library of macOS. The module has no port of them, so the result is
// false, and PlatformDarwin has the functions of Linux.
func HasDarwinLibm() bool { return darwinLibmPow != nil && darwinLibmTanh != nil }
