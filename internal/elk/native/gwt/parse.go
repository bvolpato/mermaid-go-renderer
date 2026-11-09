// Ported from java.lang.Double.parseDouble, java.lang.Integer.parseInt and
// java.lang.String.trim of the GWT runtime in elk-worker.js of elkjs 0.9.3
// (__parseAndValidateDouble, __parseAndValidateInt, $trim) (Apache-2.0).
//
// GWT has the Apache License 2.0. elk-worker.js has no notice for this
// code. The Java sources of GWT for it are Number.java and String.java in
// user/super/com/google/gwt/emul/java/lang. Their copyright lines, read
// from the main branch of https://github.com/gwtproject/gwt on 2026-11-09:
//
//	Copyright 2007 Google Inc. (Number.java)
//	Copyright 2008 Google Inc. (String.java)
//
// This file is a Go translation of that code. It is a changed version of
// it. LICENSE-GWT.txt in this directory has the license, and NOTICE in this
// directory has the license notice of the GWT files.

package gwt

import (
	"regexp"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// floatRegex is the check of __parseAndValidateDouble. \s of JavaScript also
// matches white space outside ASCII. ParseDouble rejects such input, so the
// caller falls back.
var floatRegex = regexp.MustCompile(`^[ \t\n\v\f\r]*[+-]?(NaN|Infinity|(([0-9]+\.?[0-9]*)|(\.[0-9]+))([eE][+-]?[0-9]+)?[dDfF]?)[ \t\n\v\f\r]*$`)

// ParseDouble is Double.parseDouble. ok is false where Java throws a
// NumberFormatException.
func ParseDouble(s string) (v float64, ok bool) {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return 0, false
		}
	}
	if !floatRegex.MatchString(s) {
		return 0, false
	}
	return js.ParseFloat(s), true
}

// ParseInt is Integer.parseInt. ok is false where Java throws a
// NumberFormatException.
func ParseInt(s string) (v int, ok bool) {
	start := 0
	if len(s) > 0 && (s[0] == '-' || s[0] == '+') {
		start = 1
	}
	for i := start; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
	}
	f := js.ParseInt(s, 10)
	if f != f || f < -2147483648 || f > 2147483647 {
		return 0, false
	}
	return int(f), true
}

// Trim is String.trim: it removes the code units up to U+0020 at both ends.
func Trim(s string) string {
	return strings.TrimFunc(s, func(r rune) bool { return r <= 32 })
}
