// This file ports no upstream file. It selects the platform of the reference
// browser for Math.pow and Math.tanh.

package js

import "github.com/bvolpato/mermaid-go-renderer/internal/js/crmath"

// Platform is the operating system of the reference browser. Chromium takes
// Math.pow and Math.tanh from the C library of the operating system, so Pow
// and Tanh have one result for each platform. The implementations and the
// golden tests against the two browsers are in the package crmath.
type Platform = crmath.Platform

const (
	// PlatformDarwin is Chromium on macOS on arm64.
	PlatformDarwin = crmath.PlatformDarwin
	// PlatformLinux is Chromium on Linux with glibc.
	PlatformLinux = crmath.PlatformLinux
)

// SetPlatform selects the platform of Pow and Tanh for the process. Call it
// before the renders start. The default is crmath.DefaultPlatform, or the
// value of the environment variable MMDG_MATH ("linux" or "darwin"), which
// the process reads once at the start. The engines that run JavaScript
// (internal/elk, internal/cytoscape) call Pow and Tanh for Math.pow and
// Math.tanh, so they have the same platform.
func SetPlatform(p Platform) { crmath.SetPlatform(p) }

// CurrentPlatform returns the platform of Pow and Tanh.
func CurrentPlatform() Platform { return crmath.CurrentPlatform() }

// PowLinux is Math.pow of Chromium on Linux. It does not read the current
// platform.
func PowLinux(x, y float64) float64 { return crmath.PowLinux(x, y) }

// PowDarwin is Math.pow of Chromium on macOS. It does not read the current
// platform.
func PowDarwin(x, y float64) float64 { return crmath.PowDarwin(x, y) }

// TanhLinux is Math.tanh of Chromium on Linux. It does not read the current
// platform.
func TanhLinux(x float64) float64 { return crmath.TanhLinux(x) }

// TanhDarwin is Math.tanh of Chromium on macOS. It does not read the
// current platform.
func TanhDarwin(x float64) float64 { return crmath.TanhDarwin(x) }
