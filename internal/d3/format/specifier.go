// Ported from d3-format 3.1.0, d3-format/src/formatSpecifier.js (ISC).

// Package format is a port of d3-format 3.1.0, the version that the mermaid
// 12.2.0 bundle contains. The package in node_modules is 3.1.2, which gives
// another result for some inputs (see locale.go). The golden file comes from
// the bundled source.
package format

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf16"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Specifier is a parsed format specifier:
// [[fill]align][sign][symbol][0][width][,][.precision][~][type].
type Specifier struct {
	Fill   string
	Align  string
	Sign   string
	Symbol string
	Zero   bool
	// Width is NaN when the specifier has no width (undefined in d3).
	Width float64
	Comma bool
	// Precision is NaN when the specifier has no precision (undefined in d3).
	Precision float64
	Trim      bool
	Type      string
}

func isLineTerminator(c uint16) bool {
	return c == '\n' || c == '\r' || c == 0x2028 || c == 0x2029
}

func isAlign(c uint16) bool {
	return c == '<' || c == '>' || c == '=' || c == '^'
}

func isDigit(c uint16) bool {
	return c >= '0' && c <= '9'
}

// ParseSpecifier is d3.formatSpecifier(specifier). It returns an error where
// d3 throws "invalid format".
//
// The function matches the regular expression of d3 by hand:
//
//	/^(?:(.)?([<>=^]))?([+\-( ])?([$#])?(0)?(\d+)?(,)?(\.\d+)?(~)?([a-z%])?$/i
//
// It works on UTF-16 code units, as the expression does. A fill character
// outside of the Basic Multilingual Plane is therefore invalid.
func ParseSpecifier(specifier string) (*Specifier, error) {
	u := utf16.Encode([]rune(specifier))
	s := &Specifier{Fill: " ", Align: ">", Sign: "-", Width: math.NaN(), Precision: math.NaN()}
	i, n := 0, len(u)
	if n >= 2 && isAlign(u[1]) && !isLineTerminator(u[0]) {
		s.Fill = string(utf16.Decode(u[0:1]))
		s.Align = string(rune(u[1]))
		i = 2
	} else if n >= 1 && isAlign(u[0]) {
		s.Align = string(rune(u[0]))
		i = 1
	}
	if i < n && (u[i] == '+' || u[i] == '-' || u[i] == '(' || u[i] == ' ') {
		s.Sign = string(rune(u[i]))
		i++
	}
	if i < n && (u[i] == '$' || u[i] == '#') {
		s.Symbol = string(rune(u[i]))
		i++
	}
	if i < n && u[i] == '0' {
		s.Zero = true
		i++
	}
	if i < n && isDigit(u[i]) {
		j := i
		for j < n && isDigit(u[j]) {
			j++
		}
		s.Width = js.Number(string(utf16.Decode(u[i:j])))
		i = j
	}
	if i < n && u[i] == ',' {
		s.Comma = true
		i++
	}
	if i+1 < n && u[i] == '.' && isDigit(u[i+1]) {
		j := i + 1
		for j < n && isDigit(u[j]) {
			j++
		}
		s.Precision = js.Number(string(utf16.Decode(u[i+1 : j])))
		i = j
	}
	if i < n && u[i] == '~' {
		s.Trim = true
		i++
	}
	if i < n && (u[i] == '%' || (u[i] >= 'a' && u[i] <= 'z') || (u[i] >= 'A' && u[i] <= 'Z')) {
		s.Type = string(rune(u[i]))
		i++
	}
	if i != n {
		return nil, fmt.Errorf("invalid format: %s", specifier)
	}
	return s, nil
}

// String is FormatSpecifier.prototype.toString.
func (s *Specifier) String() string {
	var sb strings.Builder
	sb.WriteString(s.Fill)
	sb.WriteString(s.Align)
	sb.WriteString(s.Sign)
	sb.WriteString(s.Symbol)
	if s.Zero {
		sb.WriteByte('0')
	}
	if !math.IsNaN(s.Width) {
		sb.WriteString(js.Str(js.Max(1, float64(js.ToInt32(s.Width)))))
	}
	if s.Comma {
		sb.WriteByte(',')
	}
	if !math.IsNaN(s.Precision) {
		sb.WriteByte('.')
		sb.WriteString(js.Str(js.Max(0, float64(js.ToInt32(s.Precision)))))
	}
	if s.Trim {
		sb.WriteByte('~')
	}
	sb.WriteString(s.Type)
	return sb.String()
}
