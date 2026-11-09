// This file ports no upstream file. It holds the calls from utils.ts into
// diagrams/common/common.ts and @braintree/sanitize-url, which are ported in
// internal/sanitize, and the Math functions that utils.ts calls.

package utils

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js/crmath"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
)

// lineBreakRegex is common.lineBreakRegex: /<\/?br\s*\/?>/gi.
var lineBreakRegex = sanitize.LineBreakRegex

// hasBreaks is common.hasBreaks(text).
func hasBreaks(text string) bool { return sanitize.HasBreaks(text) }

// sanitizeUrl is sanitizeUrl of @braintree/sanitize-url.
func sanitizeUrl(url string) string { return sanitize.SanitizeUrl(url) }

// Math.sin, Math.cos, Math.atan and Math.atan2 of the reference browser.
// Math.atan and Math.atan2 of Chromium are not correctly rounded, so only
// the ports of internal/js/crmath give the same bits: the golden tests of
// this package fail in the last bit with the wrappers js.Sin, js.Cos,
// js.Atan and js.Atan2 as long as these call the math package of Go. Call
// the wrappers here when they forward to crmath.
func sin(x float64) float64      { return crmath.Sin(x) }
func cos(x float64) float64      { return crmath.Cos(x) }
func atan(x float64) float64     { return crmath.Atan(x) }
func atan2(y, x float64) float64 { return crmath.Atan2(y, x) }
