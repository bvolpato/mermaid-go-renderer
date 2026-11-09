// Ported from d3-format 3.1.0, d3-format/src: locale.js, defaultLocale.js,
// precisionFixed.js, precisionPrefix.js and precisionRound.js (ISC).
//
// 3.1.0 is the version inside the mermaid 12.2.0 bundle. d3-format 3.1.2, the
// version in node_modules, changed formatDecimal.js, formatPrefixAuto.js and
// locale.js: it formats zero, NaN and the infinities in another way, and it
// puts the SI prefix of formatPrefix inside the padding. Do not port those
// changes.

package format

import (
	"fmt"
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/internal/jsm"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

var prefixes = []string{"y", "z", "a", "f", "p", "n", "µ", "m", "", "k", "M", "G", "T", "P", "E", "Z", "Y"}

// maxWidth bounds the width of a specifier. V8 throws a RangeError when the
// padding is longer than the largest string, 2**29-24 code units. The port
// returns the error when it makes the format function, not when the function
// runs, so that the function cannot fail.
const maxWidth = 1<<29 - 24

// LocaleDefinition is the argument of d3.formatLocale. An empty string and a
// nil slice mean that the property is undefined.
type LocaleDefinition struct {
	Decimal   string    // default "."
	Thousands string    // the group separator; needs Grouping
	Grouping  []int     // the group sizes; needs Thousands
	Currency  [2]string // the currency prefix and suffix
	Numerals  []string  // ten strings that replace the digits 0 to 9
	Percent   string    // default "%"
	Minus     string    // default "−" (U+2212)
	Nan       string    // default "NaN"
}

// Locale is the result of d3.formatLocale.
type Locale struct {
	group          func(value string, width float64) string
	currencyPrefix string
	currencySuffix string
	decimal        string
	numerals       func(string) string
	percent        string
	minus          string
	nan            string
}

// Formatter formats a number as text.
type Formatter func(value float64) string

// State holds the module variable prefixExponent of formatPrefixAuto.js.
//
// A format of the type "s" writes the exponent of the SI prefix to that
// variable when it formats a finite number, and then reads the variable to
// choose the prefix. For NaN and for an infinity it does not write the
// variable. So the prefix of such a value is the prefix of the last finite
// number that a format of the type "s" formatted in the page. Before the
// first such number the variable is undefined, and the text "undefined" is in
// the place of the prefix: format("s")(Infinity) gives "Infinityundefined".
//
// A State is the variable of one page. The zero value is the state of a new
// page. Formatters that share a State must not run in two goroutines at the
// same time.
type State struct {
	prefixExponent int
	defined        bool
}

// NewState returns the state of a new page.
func NewState() *State {
	return &State{}
}

// prefix is prefixes[8 + prefixExponent / 3] in a string concatenation. The
// element is undefined when prefixExponent is undefined.
func (s *State) prefix() string {
	if !s.defined {
		return "undefined"
	}
	return prefixes[8+s.prefixExponent/3]
}

// Format is d3.format(specifier) with the default locale, in the page of s.
func (s *State) Format(specifier string) (Formatter, error) {
	return defaultLocale.FormatState(s, specifier)
}

// NewLocale is d3.formatLocale(definition).
func NewLocale(def LocaleDefinition) *Locale {
	l := &Locale{
		currencyPrefix: def.Currency[0],
		currencySuffix: def.Currency[1],
		decimal:        ".",
		percent:        "%",
		minus:          "−",
		nan:            "NaN",
	}
	if len(def.Grouping) > 0 && def.Thousands != "" {
		l.group = formatGroup(def.Grouping, def.Thousands)
	}
	if def.Decimal != "" {
		l.decimal = def.Decimal
	}
	if len(def.Numerals) == 10 {
		l.numerals = formatNumerals(def.Numerals)
	}
	if def.Percent != "" {
		l.percent = def.Percent
	}
	if def.Minus != "" {
		l.minus = def.Minus
	}
	if def.Nan != "" {
		l.nan = def.Nan
	}
	return l
}

// defaultLocale is the en-US locale of d3-format.
var defaultLocale = NewLocale(LocaleDefinition{
	Thousands: ",",
	Grouping:  []int{3},
	Currency:  [2]string{"$", ""},
})

// Format is d3.format(specifier) with the default locale. The formatter has
// a State of its own. Use State.Format for formatters that work in one page.
func Format(specifier string) (Formatter, error) {
	return defaultLocale.Format(specifier)
}

// FormatSpecifier is d3.format(specifier) for a specifier object.
func FormatSpecifier(specifier *Specifier) (Formatter, error) {
	return defaultLocale.FormatSpecifier(specifier)
}

// FormatPrefix is d3.formatPrefix(specifier, value) with the default locale.
func FormatPrefix(specifier string, value float64) (Formatter, error) {
	return defaultLocale.FormatPrefix(specifier, value)
}

// Format is locale.format(specifier). The formatter has a State of its own.
func (l *Locale) Format(specifier string) (Formatter, error) {
	return l.newFormat(NewState(), specifier)
}

// FormatState is locale.format(specifier) in the page of state.
func (l *Locale) FormatState(state *State, specifier string) (Formatter, error) {
	return l.newFormat(state, specifier)
}

// FormatSpecifier is locale.format(specifier) for a specifier object. d3
// converts the object to a string and parses the string again.
func (l *Locale) FormatSpecifier(specifier *Specifier) (Formatter, error) {
	return l.newFormat(NewState(), specifier.String())
}

func (l *Locale) newFormat(state *State, specifierText string) (Formatter, error) {
	specifier, err := ParseSpecifier(specifierText)
	if err != nil {
		return nil, err
	}
	fill := specifier.Fill
	align := specifier.Align
	sign := specifier.Sign
	symbol := specifier.Symbol
	zero := specifier.Zero
	width := specifier.Width
	comma := specifier.Comma
	precision := specifier.Precision
	trim := specifier.Trim
	typ := specifier.Type

	if width > maxWidth {
		return nil, fmt.Errorf("invalid format: %s: the width is too large", specifierText)
	}

	if typ == "n" {
		// The "n" type is an alias for ",g".
		comma, typ = true, "g"
	} else if !isFormatType(typ) {
		// The "" type, and any invalid type, is an alias for ".12~g".
		if math.IsNaN(precision) {
			precision = 12
		}
		trim, typ = true, "g"
	}

	// If zero fill is specified, padding goes after sign and before digits.
	if zero || (fill == "0" && align == "=") {
		zero, fill, align = true, "0", "="
	}

	// Compute the prefix and suffix.
	// For SI-prefix, the suffix is lazily computed.
	prefix := ""
	if symbol == "$" {
		prefix = l.currencyPrefix
	} else if symbol == "#" && strings.Contains("boxX", typ) {
		prefix = "0" + strings.ToLower(typ)
	}
	suffix := ""
	if symbol == "$" {
		suffix = l.currencySuffix
	} else if typ == "%" || typ == "p" {
		suffix = l.percent
	}

	// Is this an integer type?
	// Can this type generate exponential notation?
	typeByte := typ[0]
	maybeSuffix := strings.Contains("defgprs%", typ)

	// Set the default precision if not specified,
	// or clamp the specified precision to the supported range.
	// For significant precision, it must be in [1, 21].
	// For fixed precision, it must be in [0, 20].
	switch {
	case math.IsNaN(precision):
		precision = 6
	case strings.Contains("gprs", typ):
		precision = js.Max(1, js.Min(21, precision))
	default:
		precision = js.Max(0, js.Min(20, precision))
	}
	p := int(precision)
	fillLen := js.UTF16Len(fill)

	format := func(value float64) string {
		valuePrefix := prefix
		valueSuffix := suffix
		var text string

		if typ == "c" {
			valueSuffix = formatType(state, typeByte, value, p) + valueSuffix
			text = ""
		} else {
			// Determine the sign. -0 is not less than 0, but 1 / -0 is!
			valueNegative := value < 0 || 1/value < 0

			// Perform the initial formatting.
			if math.IsNaN(value) {
				text = l.nan
			} else {
				text = formatType(state, typeByte, math.Abs(value), p)
			}

			// Trim insignificant zeros.
			if trim {
				text = formatTrim(text)
			}

			// If a negative value rounds to zero after formatting, and no explicit positive sign is requested, hide the sign.
			if valueNegative && js.Number(text) == 0 && sign != "+" {
				valueNegative = false
			}

			// Compute the prefix and suffix.
			switch {
			case valueNegative && sign == "(":
				valuePrefix = sign + valuePrefix
			case valueNegative:
				valuePrefix = l.minus + valuePrefix
			case sign == "-" || sign == "(":
			default:
				valuePrefix = sign + valuePrefix
			}
			if typ == "s" {
				// prefixExponent is the module variable. See State.
				valueSuffix = state.prefix() + valueSuffix
			}
			if valueNegative && sign == "(" {
				valueSuffix += ")"
			}

			// Break the formatted value into the integer "value" part that can be
			// grouped, and fractional or exponential "suffix" part that is not.
			if maybeSuffix {
				for i := 0; i < len(text); i++ {
					if c := text[i]; c < '0' || c > '9' {
						if c == '.' {
							valueSuffix = l.decimal + text[i+1:] + valueSuffix
						} else {
							valueSuffix = text[i:] + valueSuffix
						}
						text = text[:i]
						break
					}
				}
			}
		}

		// If the fill character is not "0", grouping is applied before padding.
		if comma && !zero && l.group != nil {
			text = l.group(text, math.Inf(1))
		}

		// Compute the padding.
		length := js.UTF16Len(valuePrefix) + js.UTF16Len(text) + js.UTF16Len(valueSuffix)
		padCount := 0
		if float64(length) < width {
			// new Array(width - length + 1).join(fill)
			padCount = int(width) - length
		}
		padding := strings.Repeat(fill, padCount)

		// If the fill character is "0", grouping is applied after padding.
		if comma && zero {
			if l.group != nil {
				groupWidth := math.Inf(1)
				if padCount > 0 {
					groupWidth = width - float64(js.UTF16Len(valueSuffix))
				}
				text = l.group(padding+text, groupWidth)
			} else {
				text = padding + text
			}
			padding, padCount = "", 0
		}

		// Reconstruct the final output based on the desired alignment.
		switch align {
		case "<":
			text = valuePrefix + text + valueSuffix + padding
		case "=":
			text = valuePrefix + padding + text + valueSuffix
		case "^":
			half := (padCount * fillLen) >> 1
			text = sliceUnits(padding, 0, half) + valuePrefix + text + valueSuffix + sliceUnits(padding, half, padCount*fillLen)
		default:
			text = padding + valuePrefix + text + valueSuffix
		}

		if l.numerals != nil {
			return l.numerals(text)
		}
		return text
	}
	return format, nil
}

// sliceUnits is s.slice(start, end) for a string of characters that take
// one UTF-16 code unit each.
func sliceUnits(s string, start, end int) string {
	if start == 0 && end == 0 {
		return ""
	}
	r := []rune(s)
	if end > len(r) {
		end = len(r)
	}
	return string(r[start:end])
}

// FormatPrefix is locale.formatPrefix(specifier, value).
func (l *Locale) FormatPrefix(specifier string, value float64) (Formatter, error) {
	spec, err := ParseSpecifier(specifier)
	if err != nil {
		return nil, err
	}
	return l.formatPrefix(spec, value)
}

// FormatPrefixSpecifier is locale.formatPrefix(specifier, value) for a
// specifier object.
func (l *Locale) FormatPrefixSpecifier(specifier *Specifier, value float64) (Formatter, error) {
	spec, err := ParseSpecifier(specifier.String())
	if err != nil {
		return nil, err
	}
	return l.formatPrefix(spec, value)
}

// FormatPrefixSpecifier is d3.formatPrefix(specifier, value) for a specifier
// object, with the default locale.
func FormatPrefixSpecifier(specifier *Specifier, value float64) (Formatter, error) {
	return defaultLocale.FormatPrefixSpecifier(specifier, value)
}

func (l *Locale) formatPrefix(spec *Specifier, value float64) (Formatter, error) {
	spec.Type = "f"
	// The type is "f", so the format does not use the state.
	f, err := l.newFormat(NewState(), spec.String())
	if err != nil {
		return nil, err
	}
	e := float64(js.Max(-8, js.Min(8, math.Floor(exponent(value)/3))) * 3)
	k := jsm.Pow(10, -e)
	// prefixes[8 + e / 3] is undefined when e is NaN, which it is for a value
	// that is NaN or an infinity. The concatenation gives the text "undefined".
	prefix := "undefined"
	if !math.IsNaN(e) {
		prefix = prefixes[8+int(e)/3]
	}
	return func(value float64) string {
		return f(float64(k*value)) + prefix
	}, nil
}

// PrecisionFixed is d3.precisionFixed(step).
func PrecisionFixed(step float64) float64 {
	return js.Max(0, -exponent(math.Abs(step)))
}

// PrecisionPrefix is d3.precisionPrefix(step, value).
func PrecisionPrefix(step, value float64) float64 {
	return js.Max(0, float64(js.Max(-8, js.Min(8, math.Floor(exponent(value)/3)))*3)-exponent(math.Abs(step)))
}

// PrecisionRound is d3.precisionRound(step, max).
func PrecisionRound(step, max float64) float64 {
	step = math.Abs(step)
	max = math.Abs(max) - step
	return js.Max(0, exponent(max)-exponent(step)) + 1
}
