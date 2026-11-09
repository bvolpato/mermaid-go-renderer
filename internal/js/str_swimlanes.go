// This file ports no upstream file. It has String.prototype.localeCompare
// for the locale en-US, with the collation weights of str_swimlanes_table.go.

package js

// LocaleCompare is a.localeCompare(b) in Chromium with the locale en-US
// (the locale of the reference page): -1, 0 or 1. The swimlane layout
// orders node and edge ids with it.
//
// The result comes from a table of collation weights that
// tools/reference/gen-js-localecompare.mjs reads from the ICU collator of
// Chromium (str_swimlanes_table.go). The comparison is the one of the
// Unicode collation algorithm with three levels: the primary weights of the
// two strings decide first, then the secondary weights (accents), then the
// tertiary weights (case). Code points that the collator ignores are
// skipped.
//
// The model has one weight per code point. It is exact for ASCII strings.
// It is an approximation for other strings:
//   - a code point that ICU expands or contracts (for example U+00E6, ae)
//     has one weight of its own,
//   - a code point that ICU ignores on the primary level only (a combining
//     mark) is ignored on every level,
//   - a code point from U+0800 has no table entry: it sorts after every
//     code point of the table, in code point order.
func LocaleCompare(a, b string) int {
	if a == b {
		return 0
	}
	ka := localeCompareKeys(a)
	kb := localeCompareKeys(b)
	n := min(len(ka), len(kb))
	for i := 0; i < n; i++ {
		if ka[i].p != kb[i].p {
			if ka[i].p < kb[i].p {
				return -1
			}
			return 1
		}
	}
	if len(ka) != len(kb) {
		if len(ka) < len(kb) {
			return -1
		}
		return 1
	}
	for i := 0; i < n; i++ {
		if ka[i].s != kb[i].s {
			if ka[i].s < kb[i].s {
				return -1
			}
			return 1
		}
	}
	for i := 0; i < n; i++ {
		if ka[i].t != kb[i].t {
			if ka[i].t < kb[i].t {
				return -1
			}
			return 1
		}
	}
	return 0
}

// localeCompareKey has the weights of one code point.
type localeCompareKey struct {
	p    uint32
	s, t uint8
}

// localeCompareKeys returns the weights of the code points of s that the
// collator does not ignore.
func localeCompareKeys(s string) []localeCompareKey {
	keys := make([]localeCompareKey, 0, len(s))
	for _, r := range s {
		if r < localeCompareMax {
			v := localeCompareTable[r]
			if v>>16 != 0 {
				keys = append(keys, localeCompareKey{p: v >> 16, s: uint8(v >> 8), t: uint8(v)})
			}
		} else {
			keys = append(keys, localeCompareKey{p: 0x10000 + uint32(r)})
		}
	}
	return keys
}
