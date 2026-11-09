// Ported from path-data-parser/lib/parser.js, absolutize.js and normalize.js
// (path-data-parser 0.1.0).

package rough

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

const (
	tokCommand = 0
	tokNumber  = 1
	tokEOD     = 2
)

type pathToken struct {
	typ  int
	text string
}

// segment is one command of path data with its parameters.
type segment struct {
	key  byte
	data []float64
}

// paramCount is the PARAMS table: the number of parameters of a command.
func paramCount(key byte) int {
	switch key {
	case 'A', 'a':
		return 7
	case 'C', 'c':
		return 6
	case 'H', 'h', 'V', 'v':
		return 1
	case 'L', 'l', 'M', 'm', 'T', 't':
		return 2
	case 'Q', 'q', 'S', 's':
		return 4
	}
	return 0 // Z, z
}

func isPathCommand(c byte) bool {
	switch c {
	case 'a', 'A', 'c', 'C', 'h', 'H', 'l', 'L', 'm', 'M', 'q', 'Q', 's', 'S', 't', 'T', 'v', 'V', 'z', 'Z':
		return true
	}
	return false
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// matchNumber returns the length of the match of
// /^(([-+]?[0-9]+(\.[0-9]*)?|[-+]?\.[0-9]+)([eE][-+]?[0-9]+)?)/ at the start
// of d, or 0.
func matchNumber(d string) int {
	i := 0
	if i < len(d) && (d[i] == '-' || d[i] == '+') {
		i++
	}
	switch {
	case i < len(d) && isDigit(d[i]):
		// [-+]?[0-9]+(\.[0-9]*)?
		for i < len(d) && isDigit(d[i]) {
			i++
		}
		if i < len(d) && d[i] == '.' {
			i++
			for i < len(d) && isDigit(d[i]) {
				i++
			}
		}
	case i+1 < len(d) && d[i] == '.' && isDigit(d[i+1]):
		// [-+]?\.[0-9]+
		i++
		for i < len(d) && isDigit(d[i]) {
			i++
		}
	default:
		return 0
	}
	// ([eE][-+]?[0-9]+)?
	if i < len(d) && (d[i] == 'e' || d[i] == 'E') {
		j := i + 1
		if j < len(d) && (d[j] == '-' || d[j] == '+') {
			j++
		}
		if j < len(d) && isDigit(d[j]) {
			for j < len(d) && isDigit(d[j]) {
				j++
			}
			i = j
		}
	}
	return i
}

// tokenize returns false for path data with a character that is not part of
// a token. The JavaScript function returns an empty array in that case.
func tokenize(d string) ([]pathToken, bool) {
	tokens := []pathToken{}
	for d != "" {
		c := d[0]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == ',' {
			i := 1
			for i < len(d) && (d[i] == ' ' || d[i] == '\t' || d[i] == '\r' || d[i] == '\n' || d[i] == ',') {
				i++
			}
			d = d[i:]
		} else if isPathCommand(c) {
			tokens = append(tokens, pathToken{typ: tokCommand, text: d[:1]})
			d = d[1:]
		} else if n := matchNumber(d); n > 0 {
			// text: `${parseFloat(RegExp.$1)}`
			tokens = append(tokens, pathToken{typ: tokNumber, text: js.Str(js.ParseFloat(d[:n]))})
			d = d[n:]
		} else {
			return nil, false
		}
	}
	tokens = append(tokens, pathToken{typ: tokEOD, text: ""})
	return tokens, true
}

// parsePath parses path data. It raises the errors that the JavaScript
// function throws.
func parsePath(d string) []segment {
	segments := []segment{}
	tokens, ok := tokenize(d)
	if !ok {
		// tokens is [], so isType(tokens[0], EOD) reads a property of undefined.
		throw("TypeError", "Cannot read properties of undefined (reading 'type')")
	}
	mode := "BOD"
	index := 0
	token := tokens[index]
	for token.typ != tokEOD {
		paramsCount := 0
		params := []float64{}
		if mode == "BOD" {
			if token.text == "M" || token.text == "m" {
				index++
				paramsCount = paramCount(token.text[0])
				mode = token.text
			} else {
				return parsePath("M0,0" + d)
			}
		} else if token.typ == tokNumber {
			paramsCount = paramCount(mode[0])
			if paramsCount == 0 {
				// A number after Z adds a segment and does not consume
				// the number: the JavaScript loop does not end.
				hang("the path data has a number after the command " + mode)
			}
		} else {
			index++
			paramsCount = paramCount(token.text[0])
			mode = token.text
		}
		if (index + paramsCount) < len(tokens) {
			for i := index; i < index+paramsCount; i++ {
				numbeToken := tokens[i]
				if numbeToken.typ == tokNumber {
					params = append(params, js.Number(numbeToken.text))
				} else {
					throw("Error", "Param not a number: "+mode+","+numbeToken.text)
				}
			}
			segments = append(segments, segment{key: mode[0], data: params})
			index += paramsCount
			token = tokens[index]
			if mode == "M" {
				mode = "L"
			}
			if mode == "m" {
				mode = "l"
			}
		} else {
			throw("Error", "Path data ended short")
		}
	}
	return segments
}

// absolutize translates relative commands to absolute commands.
func absolutize(segments []segment) []segment {
	cx, cy := 0.0, 0.0
	subx, suby := 0.0, 0.0
	out := []segment{}
	clone := func(data []float64) []float64 { return append([]float64{}, data...) }
	// relative is data.map((d, i) => (i % 2) ? (d + cy) : (d + cx)).
	relative := func(data []float64) []float64 {
		newdata := make([]float64, len(data))
		for i, d := range data {
			if i%2 != 0 {
				newdata[i] = d + cy
			} else {
				newdata[i] = d + cx
			}
		}
		return newdata
	}
	for _, seg := range segments {
		data := seg.data
		switch seg.key {
		case 'M':
			out = append(out, segment{key: 'M', data: clone(data)})
			cx, cy = data[0], data[1]
			subx, suby = data[0], data[1]
		case 'm':
			cx += data[0]
			cy += data[1]
			out = append(out, segment{key: 'M', data: []float64{cx, cy}})
			subx = cx
			suby = cy
		case 'L':
			out = append(out, segment{key: 'L', data: clone(data)})
			cx, cy = data[0], data[1]
		case 'l':
			cx += data[0]
			cy += data[1]
			out = append(out, segment{key: 'L', data: []float64{cx, cy}})
		case 'C':
			out = append(out, segment{key: 'C', data: clone(data)})
			cx = data[4]
			cy = data[5]
		case 'c':
			newdata := relative(data)
			out = append(out, segment{key: 'C', data: newdata})
			cx = newdata[4]
			cy = newdata[5]
		case 'Q':
			out = append(out, segment{key: 'Q', data: clone(data)})
			cx = data[2]
			cy = data[3]
		case 'q':
			newdata := relative(data)
			out = append(out, segment{key: 'Q', data: newdata})
			cx = newdata[2]
			cy = newdata[3]
		case 'A':
			out = append(out, segment{key: 'A', data: clone(data)})
			cx = data[5]
			cy = data[6]
		case 'a':
			cx += data[5]
			cy += data[6]
			out = append(out, segment{key: 'A', data: []float64{data[0], data[1], data[2], data[3], data[4], cx, cy}})
		case 'H':
			out = append(out, segment{key: 'H', data: clone(data)})
			cx = data[0]
		case 'h':
			cx += data[0]
			out = append(out, segment{key: 'H', data: []float64{cx}})
		case 'V':
			out = append(out, segment{key: 'V', data: clone(data)})
			cy = data[0]
		case 'v':
			cy += data[0]
			out = append(out, segment{key: 'V', data: []float64{cy}})
		case 'S':
			out = append(out, segment{key: 'S', data: clone(data)})
			cx = data[2]
			cy = data[3]
		case 's':
			newdata := relative(data)
			out = append(out, segment{key: 'S', data: newdata})
			cx = newdata[2]
			cy = newdata[3]
		case 'T':
			out = append(out, segment{key: 'T', data: clone(data)})
			cx = data[0]
			cy = data[1]
		case 't':
			cx += data[0]
			cy += data[1]
			out = append(out, segment{key: 'T', data: []float64{cx, cy}})
		case 'Z', 'z':
			out = append(out, segment{key: 'Z', data: []float64{}})
			cx = subx
			cy = suby
		}
	}
	return out
}

// normalize rewrites the path so that it has only M, L, C and Z commands.
func normalize(g *Generator, segments []segment) []segment {
	out := []segment{}
	lastType := byte(0)
	cx, cy := 0.0, 0.0
	subx, suby := 0.0, 0.0
	lcx, lcy := 0.0, 0.0
	for _, seg := range segments {
		key, data := seg.key, seg.data
		switch key {
		case 'M':
			out = append(out, segment{key: 'M', data: append([]float64{}, data...)})
			cx, cy = data[0], data[1]
			subx, suby = data[0], data[1]
		case 'C':
			out = append(out, segment{key: 'C', data: append([]float64{}, data...)})
			cx = data[4]
			cy = data[5]
			lcx = data[2]
			lcy = data[3]
		case 'L':
			out = append(out, segment{key: 'L', data: append([]float64{}, data...)})
			cx, cy = data[0], data[1]
		case 'H':
			cx = data[0]
			out = append(out, segment{key: 'L', data: []float64{cx, cy}})
		case 'V':
			cy = data[0]
			out = append(out, segment{key: 'L', data: []float64{cx, cy}})
		case 'S':
			cx1, cy1 := 0.0, 0.0
			if lastType == 'C' || lastType == 'S' {
				cx1 = cx + (cx - lcx)
				cy1 = cy + (cy - lcy)
			} else {
				cx1 = cx
				cy1 = cy
			}
			out = append(out, segment{key: 'C', data: append([]float64{cx1, cy1}, data...)})
			lcx = data[0]
			lcy = data[1]
			cx = data[2]
			cy = data[3]
		case 'T':
			x, y := data[0], data[1]
			x1, y1 := 0.0, 0.0
			if lastType == 'Q' || lastType == 'T' {
				x1 = cx + (cx - lcx)
				y1 = cy + (cy - lcy)
			} else {
				x1 = cx
				y1 = cy
			}
			cx1 := cx + 2*(x1-cx)/3
			cy1 := cy + 2*(y1-cy)/3
			cx2 := x + 2*(x1-x)/3
			cy2 := y + 2*(y1-y)/3
			out = append(out, segment{key: 'C', data: []float64{cx1, cy1, cx2, cy2, x, y}})
			lcx = x1
			lcy = y1
			cx = x
			cy = y
		case 'Q':
			x1, y1, x, y := data[0], data[1], data[2], data[3]
			cx1 := cx + 2*(x1-cx)/3
			cy1 := cy + 2*(y1-cy)/3
			cx2 := x + 2*(x1-x)/3
			cy2 := y + 2*(y1-y)/3
			out = append(out, segment{key: 'C', data: []float64{cx1, cy1, cx2, cy2, x, y}})
			lcx = x1
			lcy = y1
			cx = x
			cy = y
		case 'A':
			r1 := math.Abs(data[0])
			r2 := math.Abs(data[1])
			angle := data[2]
			largeArcFlag := data[3]
			sweepFlag := data[4]
			x := data[5]
			y := data[6]
			if r1 == 0 || r2 == 0 {
				out = append(out, segment{key: 'C', data: []float64{cx, cy, x, y, x, y}})
				cx = x
				cy = y
			} else {
				if cx != x || cy != y {
					curves := arcToCubicCurves(g, cx, cy, x, y, r1, r2, angle, largeArcFlag, sweepFlag)
					for _, curve := range curves {
						out = append(out, segment{key: 'C', data: curve})
					}
					cx = x
					cy = y
				}
			}
		case 'Z':
			out = append(out, segment{key: 'Z', data: []float64{}})
			cx = subx
			cy = suby
		}
		lastType = key
	}
	return out
}

func degToRad(degrees float64) float64 {
	pi := math.Pi
	return (pi * degrees) / 180
}

func rotate(m *mathFuncs, x, y, angleRad float64) (float64, float64) {
	X := float64(x*m.cos(angleRad)) - float64(y*m.sin(angleRad))
	Y := float64(x*m.sin(angleRad)) + float64(y*m.cos(angleRad))
	return X, Y
}

// arcToCubicCurves is the call of the JavaScript function without the
// recursive argument. It returns the curves.
func arcToCubicCurves(g *Generator, x1, y1, x2, y2, r1, r2, angle, largeArcFlag, sweepFlag float64) [][]float64 {
	m := &g.m
	angleRad := degToRad(angle)
	params := arcToCubicParams(g, x1, y1, x2, y2, r1, r2, angle, largeArcFlag, sweepFlag, nil, 0)
	curves := [][]float64{}
	for i := 0; i+2 < len(params); i += 3 {
		r1x, r1y := rotate(m, params[i][0], params[i][1], angleRad)
		r2x, r2y := rotate(m, params[i+1][0], params[i+1][1], angleRad)
		r3x, r3y := rotate(m, params[i+2][0], params[i+2][1], angleRad)
		curves = append(curves, []float64{r1x, r1y, r2x, r2y, r3x, r3y})
	}
	return curves
}

// arcToCubicParams is the body of the JavaScript arcToCubicCurves up to the
// list [m2, m3, m4].concat(params). recursive is the last argument of the
// JavaScript function: [f1, f2, cx, cy], or nil.
func arcToCubicParams(g *Generator, x1, y1, x2, y2, r1, r2, angle, largeArcFlag, sweepFlag float64, recursive *[4]float64, depth int) []Point {
	if depth > maxCallDepth {
		throw("RangeError", "Maximum call stack size exceeded")
	}
	m := &g.m
	pi := math.Pi
	angleRad := degToRad(angle)
	params := []Point{}
	f1, f2, cx, cy := 0.0, 0.0, 0.0, 0.0
	if recursive != nil {
		f1, f2, cx, cy = recursive[0], recursive[1], recursive[2], recursive[3]
	} else {
		x1, y1 = rotate(m, x1, y1, -angleRad)
		x2, y2 = rotate(m, x2, y2, -angleRad)
		x := (x1 - x2) / 2
		y := (y1 - y2) / 2
		h := (x*x)/(r1*r1) + (y*y)/(r2*r2)
		if h > 1 {
			h = math.Sqrt(h)
			r1 = float64(h * r1)
			r2 = float64(h * r2)
		}
		sign := 1.0
		if largeArcFlag == sweepFlag {
			sign = -1
		}
		r1Pow := float64(r1 * r1)
		r2Pow := float64(r2 * r2)
		left := float64(r1Pow*r2Pow) - float64(r1Pow*y*y) - float64(r2Pow*x*x)
		right := float64(r1Pow*y*y) + float64(r2Pow*x*x)
		k := float64(sign * math.Sqrt(math.Abs(left/right)))
		cx = k*r1*y/r2 + float64((x1+x2)/2)
		cy = k*-r2*x/r1 + float64((y1+y2)/2)
		f1 = m.asin(js.ParseFloat(js.ToFixed((y1-cy)/r2, 9)))
		f2 = m.asin(js.ParseFloat(js.ToFixed((y2-cy)/r2, 9)))
		if x1 < cx {
			f1 = pi - f1
		}
		if x2 < cx {
			f2 = pi - f2
		}
		if f1 < 0 {
			f1 = float64(pi*2) + f1
		}
		if f2 < 0 {
			f2 = float64(pi*2) + f2
		}
		if truthyNum(sweepFlag) && f1 > f2 {
			f1 = f1 - float64(pi*2)
		}
		if !truthyNum(sweepFlag) && f2 > f1 {
			f2 = f2 - float64(pi*2)
		}
	}
	df := f2 - f1
	if math.Abs(df) > (pi * 120 / 180) {
		f2old := f2
		x2old := x2
		y2old := y2
		if truthyNum(sweepFlag) && f2 > f1 {
			f2 = f1 + float64((pi*120/180)*(1))
		} else {
			f2 = f1 + float64((pi*120/180)*(-1))
		}
		x2 = cx + float64(r1*m.cos(f2))
		y2 = cy + float64(r2*m.sin(f2))
		params = arcToCubicParams(g, x2, y2, x2old, y2old, r1, r2, angle, 0, sweepFlag, &[4]float64{f2, f2old, cx, cy}, depth+1)
	}
	df = f2 - f1
	c1 := m.cos(f1)
	s1 := m.sin(f1)
	c2 := m.cos(f2)
	s2 := m.sin(f2)
	t := m.tan(df / 4)
	hx := float64(4.0 / 3 * r1 * t)
	hy := float64(4.0 / 3 * r2 * t)
	m1 := Point{x1, y1}
	m2 := Point{x1 + float64(hx*s1), y1 - float64(hy*c1)}
	m3 := Point{x2 + float64(hx*s2), y2 - float64(hy*c2)}
	m4 := Point{x2, y2}
	m2[0] = float64(2*m1[0]) - m2[0]
	m2[1] = float64(2*m1[1]) - m2[1]
	return append([]Point{m2, m3, m4}, params...)
}
