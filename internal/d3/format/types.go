// Ported from d3-format 3.1.0, d3-format/src: formatTypes.js,
// formatDecimal.js, formatRounded.js, formatPrefixAuto.js, formatTrim.js,
// formatGroup.js, formatNumerals.js and exponent.js (ISC).

package format

import (
	"math"
	"math/big"
	"strings"
	"unicode/utf16"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// integerToString is Math.round(x).toString(radix) for x >= 0 and a radix of
// 2, 8 or 16. V8 prints every digit of an integer in these radixes.
func integerToString(x float64, radix int) string {
	x = js.Round(x)
	if math.IsInf(x, 1) {
		return "Infinity"
	}
	if x == 0 {
		return "0"
	}
	i, _ := new(big.Float).SetFloat64(x).Int(nil)
	return i.Text(radix)
}

// formatDecimal ports formatDecimal.js.
func formatDecimal(x float64) string {
	x = js.Round(x)
	if math.Abs(x) >= 1e21 {
		// x.toLocaleString("en").replace(/,/g, "")
		if math.IsInf(x, 0) {
			if x < 0 {
				return "-∞"
			}
			return "∞"
		}
		// The locale string of a large number has the shortest digits that
		// identify the number, followed by zeros.
		coefficient, exponent, _ := formatDecimalParts(math.Abs(x), 0)
		s := coefficient + strings.Repeat("0", exponent+1-len(coefficient))
		if x < 0 {
			return "-" + s
		}
		return s
	}
	return js.Str(x)
}

// formatDecimalParts computes the decimal coefficient and exponent of x with
// p significant digits. A p of zero means as many digits as necessary. The
// result is false for NaN and an infinity, where d3 returns null: the text of
// toExponential has no "e" for them. Zero has the exponent 0 and a
// coefficient of zeros. (d3-format 3.1.2 returns null for zero too.)
func formatDecimalParts(x float64, p int) (coefficient string, exponent int, ok bool) {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return "", 0, false
	}
	var s string
	if p != 0 {
		s = js.ToExponential(x, p-1)
	} else {
		s = js.ToExponential(x, -1)
	}
	i := strings.IndexByte(s, 'e')
	coefficient = s[:i]
	// The string has the form d.ddde±x or de±x.
	if len(coefficient) > 1 {
		coefficient = coefficient[:1] + coefficient[2:]
	}
	return coefficient, int(js.Number(s[i+1:])), true
}

// exponent ports exponent.js. The result is NaN for NaN and an infinity.
func exponent(x float64) float64 {
	_, e, ok := formatDecimalParts(math.Abs(x), 0)
	if !ok {
		return math.NaN()
	}
	return float64(e)
}

// zeros is new Array(n).join("0"): n - 1 zeros.
func zeros(n int) string {
	if n <= 1 {
		return ""
	}
	return strings.Repeat("0", n-1)
}

// formatRounded ports formatRounded.js.
func formatRounded(x float64, p int) string {
	coefficient, exponent, ok := formatDecimalParts(x, p)
	if !ok {
		return js.Str(x)
	}
	switch {
	case exponent < 0:
		return "0." + zeros(-exponent) + coefficient
	case len(coefficient) > exponent+1:
		return coefficient[:exponent+1] + "." + coefficient[exponent+1:]
	}
	return coefficient + zeros(exponent-len(coefficient)+2)
}

// formatPrefixAuto ports formatPrefixAuto.js. It writes the module variable
// prefixExponent, which is in state. For an infinity it returns x + "" and
// leaves the variable as it is.
func formatPrefixAuto(state *State, x float64, p int) string {
	coefficient, exponent, ok := formatDecimalParts(x, p)
	if !ok {
		return js.Str(x)
	}
	prefixExponent := int(js.Max(-8, js.Min(8, math.Floor(float64(exponent)/3)))) * 3
	state.prefixExponent, state.defined = prefixExponent, true
	i := exponent - prefixExponent + 1
	n := len(coefficient)
	switch {
	case i == n:
		return coefficient
	case i > n:
		return coefficient + zeros(i-n+1)
	case i > 0:
		return coefficient[:i] + "." + coefficient[i:]
	}
	// Less than 1y.
	q := p + i - 1
	if q < 0 {
		q = 0
	}
	c, _, _ := formatDecimalParts(x, q)
	return "0." + zeros(1-i) + c
}

// formatTrim ports formatTrim.js: it removes insignificant zeros, for
// example 1.2000k becomes 1.2k.
func formatTrim(s string) string {
	n := len(s)
	i0, i1 := -1, 0
loop:
	for i := 1; i < n; i++ {
		switch c := s[i]; {
		case c == '.':
			i0, i1 = i, i
		case c == '0':
			if i0 == 0 {
				i0 = i
			}
			i1 = i
		default:
			if c < '1' || c > '9' {
				break loop
			}
			if i0 > 0 {
				i0 = 0
			}
		}
	}
	if i0 > 0 {
		return s[:i0] + s[i1+1:]
	}
	return s
}

// formatGroup ports formatGroup.js. The width is the number of UTF-16 code
// units that the result may take, or +Inf.
func formatGroup(grouping []int, thousands string) func(value string, width float64) string {
	return func(value string, width float64) string {
		u := utf16.Encode([]rune(value))
		i := len(u)
		var t []string
		j := 0
		g := grouping[0]
		length := 0
		for i > 0 && g > 0 {
			if float64(length+g+1) > width {
				g = int(js.Max(1, width-float64(length)))
			}
			// value.substring(i -= g, i + g)
			i -= g
			start, end := i, i+g
			if start < 0 {
				start = 0
			}
			t = append(t, string(utf16.Decode(u[start:end])))
			length += g + 1
			if float64(length) > width {
				break
			}
			j = (j + 1) % len(grouping)
			g = grouping[j]
		}
		for a, b := 0, len(t)-1; a < b; a, b = a+1, b-1 {
			t[a], t[b] = t[b], t[a]
		}
		return strings.Join(t, thousands)
	}
}

// formatNumerals ports formatNumerals.js.
func formatNumerals(numerals []string) func(string) string {
	return func(value string) string {
		var sb strings.Builder
		for _, r := range value {
			if r >= '0' && r <= '9' {
				sb.WriteString(numerals[r-'0'])
			} else {
				sb.WriteRune(r)
			}
		}
		return sb.String()
	}
}

// formatType applies the format function of a type to a number that is not
// negative. The type "s" writes the SI prefix exponent to state.
func formatType(state *State, typ byte, x float64, p int) string {
	switch typ {
	case '%':
		return js.ToFixed(float64(x*100), p)
	case 'b':
		return integerToString(x, 2)
	case 'c':
		return js.Str(x)
	case 'd':
		return formatDecimal(x)
	case 'e':
		return js.ToExponential(x, p)
	case 'f':
		return js.ToFixed(x, p)
	case 'g':
		return js.ToPrecision(x, p)
	case 'o':
		return integerToString(x, 8)
	case 'p':
		return formatRounded(float64(x*100), p)
	case 'r':
		return formatRounded(x, p)
	case 's':
		return formatPrefixAuto(state, x, p)
	case 'X':
		return strings.ToUpper(integerToString(x, 16))
	case 'x':
		return integerToString(x, 16)
	}
	return ""
}

func isFormatType(typ string) bool {
	return len(typ) == 1 && strings.Contains("%bcdefgoprsXx", typ)
}
