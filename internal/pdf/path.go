// This file implements original PDF path serialization under ISO 32000-1.
// It does not port an upstream implementation.
package pdf

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// number uses PDF decimal notation. PDF numbers do not allow exponents.
func number(v float64) string {
	if v == 0 {
		return "0"
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func matrixString(m Matrix) string {
	var out strings.Builder
	for _, v := range m {
		out.WriteString(number(v))
		out.WriteByte(' ')
	}
	out.WriteString("cm\n")
	return out.String()
}

func transformPoint(m Matrix, x, y float64) (float64, float64) {
	return float64(m[0]*x) + float64(m[2]*y) + m[4], float64(m[1]*x) + float64(m[3]*y) + m[5]
}

// canonicalPath validates the small path vocabulary accepted by this package.
// A matrix converts clip geometry to page coordinates without matrix inversion.
func canonicalPath(path string, matrix *Matrix) (string, error) {
	var out strings.Builder
	var args []float64
	hasCurrent := false
	point := func(x, y float64) error {
		if matrix != nil {
			x, y = transformPoint(*matrix, x, y)
		}
		if !finite(x) || !finite(y) {
			return fmt.Errorf("pdf: path coordinate overflow")
		}
		out.WriteString(number(x))
		out.WriteByte(' ')
		out.WriteString(number(y))
		out.WriteByte(' ')
		return nil
	}
	for _, token := range strings.Fields(path) {
		n := -1
		switch token {
		case "m", "l":
			n = 2
		case "c":
			n = 6
		case "h":
			n = 0
		case "re":
			n = 4
		}
		if n < 0 {
			v, err := strconv.ParseFloat(token, 64)
			if err != nil || !finite(v) || len(args) >= 6 {
				return "", fmt.Errorf("pdf: invalid path token %q", token)
			}
			args = append(args, v)
			continue
		}
		if len(args) != n {
			return "", fmt.Errorf("pdf: path operator %s needs %d operands", token, n)
		}
		if (token == "l" || token == "c" || token == "h") && !hasCurrent {
			return "", fmt.Errorf("pdf: path operator %s has no current point", token)
		}
		if token == "re" && matrix != nil {
			x, y, width, height := args[0], args[1], args[2], args[3]
			points := [4][2]float64{{x, y}, {x + width, y}, {x + width, y + height}, {x, y + height}}
			for i, p := range points {
				if err := point(p[0], p[1]); err != nil {
					return "", err
				}
				if i == 0 {
					out.WriteString("m\n")
				} else {
					out.WriteString("l\n")
				}
			}
			out.WriteString("h\n")
		} else {
			for i := 0; i < len(args); i += 2 {
				if err := point(args[i], args[i+1]); err != nil {
					return "", err
				}
			}
			out.WriteString(token)
			out.WriteByte('\n')
		}
		if token == "m" || token == "re" {
			hasCurrent = true
		}
		args = args[:0]
	}
	if len(args) != 0 {
		return "", fmt.Errorf("pdf: path has unused operands")
	}
	return out.String(), nil
}
