// This file ports no upstream file. It has the length and the order of
// strings in UTF-16 code units, as ECMAScript defines them.

package js

import (
	"sort"
	"unicode/utf16"
)

// UTF16Len is String.prototype.length: the number of UTF-16 code units.
func UTF16Len(s string) int {
	n := 0
	for _, r := range s {
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// CompareStrings orders strings as the JavaScript < operator does: by UTF-16
// code units. It differs from a byte comparison for characters above U+FFFF.
func CompareStrings(a, b string) int {
	if a == b {
		return 0
	}
	ascii := true
	for i := 0; i < len(a) && ascii; i++ {
		ascii = a[i] < 0x80
	}
	for i := 0; i < len(b) && ascii; i++ {
		ascii = b[i] < 0x80
	}
	if ascii {
		if a < b {
			return -1
		}
		return 1
	}
	ua, ub := utf16.Encode([]rune(a)), utf16.Encode([]rune(b))
	for i := 0; i < len(ua) && i < len(ub); i++ {
		if ua[i] != ub[i] {
			if ua[i] < ub[i] {
				return -1
			}
			return 1
		}
	}
	switch {
	case len(ua) < len(ub):
		return -1
	case len(ua) > len(ub):
		return 1
	}
	return 0
}

// SortStrings is Array.prototype.sort with no comparator, for an array of strings.
func SortStrings(s []string) {
	sort.SliceStable(s, func(i, j int) bool { return CompareStrings(s[i], s[j]) < 0 })
}

// SortFunc is Array.prototype.sort with a comparator. The sort is stable, as
// it is in V8. The comparator returns a negative, zero or positive number.
func SortFunc[T any](s []T, cmp func(a, b T) float64) {
	sort.SliceStable(s, func(i, j int) bool { return cmp(s[i], s[j]) < 0 })
}
