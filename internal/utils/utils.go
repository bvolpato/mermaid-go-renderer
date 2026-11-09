// Ported from mermaid 12.2.0, mermaid/src/utils.ts (MIT).

// Package utils is the port of the shared helpers of mermaid 12.2.0:
// utils.ts and the small modules of the utils directory. The text
// measurement functions are in text.go.
//
// Upstream keeps counters and memoize caches in module-level variables. Here
// that state lives in the *render.Context of the render, so that each render
// starts with the empty state of a new page.
package utils

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/dlclark/regexp2"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/shape"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// ZeroWidthSpace is ZERO_WIDTH_SPACE.
const ZeroWidthSpace = "​"

// Point is the Point interface of types.ts.
type Point = dom.Point

// d3CurveTypes is effectively an enum of the supported curve types,
// accessible by name.
var d3CurveTypes = map[string]shape.CurveFactory{
	"curveBasis":            shape.CurveBasis,
	"curveBasisClosed":      shape.CurveBasisClosed,
	"curveBasisOpen":        shape.CurveBasisOpen,
	"curveBumpX":            shape.CurveBumpX,
	"curveBumpY":            shape.CurveBumpY,
	"curveBundle":           shape.CurveBundle,
	"curveCardinalClosed":   shape.CurveCardinalClosed,
	"curveCardinalOpen":     shape.CurveCardinalOpen,
	"curveCardinal":         shape.CurveCardinal,
	"curveCatmullRomClosed": shape.CurveCatmullRomClosed,
	"curveCatmullRomOpen":   shape.CurveCatmullRomOpen,
	"curveCatmullRom":       shape.CurveCatmullRom,
	"curveLinear":           shape.CurveLinear,
	"curveLinearClosed":     shape.CurveLinearClosed,
	"curveMonotoneX":        shape.CurveMonotoneX,
	"curveMonotoneY":        shape.CurveMonotoneY,
	"curveNatural":          shape.CurveNatural,
	"curveStep":             shape.CurveStep,
	"curveStepAfter":        shape.CurveStepAfter,
	"curveStepBefore":       shape.CurveStepBefore,
}

// IsSubstringInArray detects whether a substring is present in a given
// array. It returns the array index that contains the substring, or -1.
//
// Upstream calls element.match(str), which reads str as a regular
// expression. The error is the SyntaxError of an invalid expression. An
// empty array gives no error, because upstream never compiles the
// expression then.
func IsSubstringInArray(str string, arr []string) (int, error) {
	var re *regexp2.Regexp
	for i, element := range arr {
		if re == nil {
			var err error
			re, err = regexp2.Compile(str, regexp2.ECMAScript)
			if err != nil {
				return -1, fmt.Errorf("SyntaxError: Invalid regular expression: /%s/: %w", str, err)
			}
		}
		ok, err := re.MatchString(element)
		if err != nil {
			return -1, err
		}
		if ok {
			return i, nil
		}
	}
	return -1, nil
}

// InterpolateToCurve returns a d3 curve given a curve name. An empty name
// (undefined upstream) or an unknown name gives defaultCurve.
func InterpolateToCurve(interpolate string, defaultCurve shape.CurveFactory) shape.CurveFactory {
	if interpolate == "" {
		return defaultCurve
	}
	curveName := "curve" + strings.ToUpper(js.CharAt(interpolate, 0)) + js.StringSliceFrom(interpolate, 1)
	if curve, ok := d3CurveTypes[curveName]; ok {
		return curve
	}
	return defaultCurve
}

// FormatUrl formats a URL string. The second result is false when upstream
// returns undefined: the trimmed string is empty.
func FormatUrl(linkStr string, config *js.Obj[any]) (string, bool) {
	url := js.TrimSpace(linkStr)
	if url == "" {
		return "", false
	}
	if !js.StrictEquals(js.Path(config, "securityLevel"), "loose") {
		return sanitizeUrl(url), true
	}
	return url, true
}

// RunFunc runs a function. functionName is a dot separated path to the
// function relative to window. A Go render has no window object, so the
// caller gives the object that stands in for it. The function at the end of
// the path must be a func(params ...any).
//
// Upstream logs an error and returns when a part of the path is missing,
// and throws a TypeError when the last part is not a function. Both cases
// are an error here.
func RunFunc(window *js.Obj[any], functionName string, params ...any) error {
	arrPaths := strings.Split(functionName, ".")

	length := len(arrPaths) - 1
	fnName := arrPaths[length]

	var obj any = window
	for i := 0; i < length; i++ {
		obj, _ = js.GetProperty(obj, arrPaths[i])
		if !js.Truthy(obj) {
			return fmt.Errorf("Function name: %s not found in window", functionName)
		}
	}

	fn, _ := js.GetProperty(obj, fnName)
	call, ok := fn.(func(params ...any))
	if !ok {
		return fmt.Errorf("TypeError: %s is not a function", functionName)
	}
	call(params...)
	return nil
}

// distance finds the distance between two points using the distance
// formula. A nil point is an undefined point.
func distance(p1, p2 *Point) float64 {
	if p1 == nil || p2 == nil {
		return 0
	}
	return math.Sqrt(js.Pow(p2.X-p1.X, 2) + js.Pow(p2.Y-p1.Y, 2))
}

// traverseEdge finds the point in the middle of the length of the points.
func traverseEdge(points []Point) (Point, error) {
	var prevPoint *Point
	totalDistance := 0.0

	for i := range points {
		point := &points[i]
		totalDistance += distance(point, prevPoint)
		prevPoint = point
	}

	// Traverse half of total distance along points
	remainingDistance := totalDistance / 2
	return CalculatePoint(points, remainingDistance)
}

// CalcLabelPosition is calcLabelPosition: the position of an edge label.
func CalcLabelPosition(points []Point) (Point, error) {
	if len(points) == 1 {
		return points[0], nil
	}
	return traverseEdge(points)
}

// RoundNumber is roundNumber(num, precision = 2).
func RoundNumber(num float64, precision ...float64) float64 {
	p := 2.0
	if len(precision) > 0 {
		p = precision[0]
	}
	factor := js.Pow(10, p)
	return js.Round(num*factor) / factor
}

// errNoSuitablePoint is the error that calculatePoint throws.
var errNoSuitablePoint = errors.New("Could not find a suitable point for the given distance")

// CalculatePoint is calculatePoint: the point at distanceToTraverse along
// the points.
func CalculatePoint(points []Point, distanceToTraverse float64) (Point, error) {
	var prevPoint *Point
	remainingDistance := distanceToTraverse
	for i := range points {
		point := &points[i]
		if prevPoint != nil {
			vectorDistance := distance(point, prevPoint)
			if vectorDistance == 0 {
				return *prevPoint, nil
			}
			if vectorDistance < remainingDistance {
				remainingDistance -= vectorDistance
			} else {
				// The point is remainingDistance from prevPoint in the vector between prevPoint and point
				// Calculate the coordinates
				distanceRatio := remainingDistance / vectorDistance
				if distanceRatio <= 0 {
					return *prevPoint, nil
				}
				if distanceRatio >= 1 {
					return Point{X: point.X, Y: point.Y}, nil
				}
				if distanceRatio > 0 && distanceRatio < 1 {
					return Point{
						X: RoundNumber(float64((1-distanceRatio)*prevPoint.X)+float64(distanceRatio*point.X), 5),
						Y: RoundNumber(float64((1-distanceRatio)*prevPoint.Y)+float64(distanceRatio*point.Y), 5),
					}, nil
				}
			}
		}
		prevPoint = point
	}
	return Point{}, errNoSuitablePoint
}

// CalcCardinalityPosition is calcCardinalityPosition.
//
// Upstream compares the first point with initialPosition by identity, and
// reverses the points in place when they differ. So initialPosition must
// point into the points slice (&points[0] or &points[len(points)-1]), and
// the caller sees the reversed order afterwards, as the caller of upstream
// does.
func CalcCardinalityPosition(isRelationTypePresent bool, points []Point, initialPosition *Point) (Point, error) {
	if len(points) == 0 || &points[0] != initialPosition {
		for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
			points[i], points[j] = points[j], points[i]
		}
	}
	// Traverse only 25 total distance along points to find cardinality point
	const distanceToCardinalityPoint = 25
	center, err := CalculatePoint(points, distanceToCardinalityPoint)
	if err != nil {
		return Point{}, err
	}
	// if relation is present (Arrows will be added), change cardinality point off-set distance (d)
	d := 5.0
	if isRelationTypePresent {
		d = 10
	}
	//Calculate Angle for x and y axis
	angle := atan2(points[0].Y-center.Y, points[0].X-center.X)
	cardinalityPosition := Point{X: 0, Y: 0}
	//Calculation cardinality position using angle, center point on the line/curve but perpendicular and with offset-distance
	cardinalityPosition.X = float64(sin(angle)*d) + float64((points[0].X+center.X)/2)
	cardinalityPosition.Y = float64(-cos(angle)*d) + float64((points[0].Y+center.Y)/2)
	return cardinalityPosition, nil
}

// CalcTerminalLabelPosition calculates the terminal label position.
// position is "start_left", "start_right", "end_left" or "end_right". The
// points are not changed.
func CalcTerminalLabelPosition(terminalMarkerSize float64, position string, _points []Point) (Point, error) {
	points := append([]Point(nil), _points...)
	if position != "start_left" && position != "start_right" {
		for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
			points[i], points[j] = points[j], points[i]
		}
	}

	// Traverse only 25 total distance along points to find cardinality point
	distanceToCardinalityPoint := 25 + terminalMarkerSize
	center, err := CalculatePoint(points, distanceToCardinalityPoint)
	if err != nil {
		return Point{}, err
	}

	// if relation is present (Arrows will be added), change cardinality point off-set distance (d)
	d := 10 + float64(terminalMarkerSize*0.5)
	//Calculate Angle for x and y axis
	angle := atan2(points[0].Y-center.Y, points[0].X-center.X)

	cardinalityPosition := Point{X: 0, Y: 0}
	//Calculation cardinality position using angle, center point on the line/curve but perpendicular and with offset-distance

	switch position {
	case "start_left":
		cardinalityPosition.X = float64(sin(angle+math.Pi)*d) + float64((points[0].X+center.X)/2)
		cardinalityPosition.Y = float64(-cos(angle+math.Pi)*d) + float64((points[0].Y+center.Y)/2)
	case "end_right":
		cardinalityPosition.X = float64(sin(angle-math.Pi)*d) + float64((points[0].X+center.X)/2)
		cardinalityPosition.Y = float64(-cos(angle-math.Pi)*d) + float64((points[0].Y+center.Y)/2)
	case "end_left":
		cardinalityPosition.X = float64(sin(angle)*d) + float64((points[0].X+center.X)/2)
		cardinalityPosition.Y = float64(-cos(angle)*d) + float64((points[0].Y+center.Y)/2)
	default:
		cardinalityPosition.X = float64(sin(angle)*d) + float64((points[0].X+center.X)/2)
		cardinalityPosition.Y = float64(-cos(angle)*d) + float64((points[0].Y+center.Y)/2)
	}
	return cardinalityPosition, nil
}

// GetStylesFromArray gets styles from an array of declarations. It returns
// the styles grouped as strings: the text properties go to labelStyle.
func GetStylesFromArray(arr []string) (style, labelStyle string) {
	for _, element := range arr {
		// add text properties to label style definition
		if strings.HasPrefix(element, "color:") || strings.HasPrefix(element, "text-align:") {
			labelStyle = labelStyle + element + ";"
		} else {
			style = style + element + ";"
		}
	}
	return style, labelStyle
}

// GenerateId is generateId(): a random id with a counter at the end. The
// counter is the module-level cnt of upstream.
func GenerateId(ctx *render.Context) string {
	cnt := ctx.Counter("utils.generateId") + 1
	return "id-" + js.Substr(js.ToStringRadix(ctx.Random(), 36), 2, 12) + "-" + strconv.Itoa(cnt)
}

// makeRandomHex generates a random hexadecimal id of the given length.
func makeRandomHex(ctx *render.Context, length int) string {
	const characters = "0123456789abcdef"
	const charactersLength = len(characters)
	var result strings.Builder
	for i := 0; i < length; i++ {
		result.WriteByte(characters[int(math.Floor(ctx.Random()*float64(charactersLength)))])
	}
	return result.String()
}

// Random is random({ length }): a random hexadecimal string.
func Random(ctx *render.Context, length int) string {
	return makeRandomHex(ctx, length)
}

// InitIDGenerator is the class InitIDGenerator.
type InitIDGenerator struct {
	count float64
	next  func() float64
}

// NewInitIDGenerator is new InitIDGenerator(deterministic, seed). An empty
// seed is an undefined seed. The clock of ctx stands in for Date.now(); ctx
// can be nil for a deterministic generator.
func NewInitIDGenerator(ctx *render.Context, deterministic bool, seed string) *InitIDGenerator {
	g := &InitIDGenerator{}
	// TODO: Seed is only used for length?
	// v11: Use the actual value of seed string to generate an initial value for count.
	if seed != "" {
		g.count = float64(js.UTF16Len(seed))
	}
	if deterministic {
		g.next = func() float64 {
			v := g.count
			g.count++
			return v
		}
	} else {
		g.next = func() float64 { return float64(ctx.Now.UnixMilli()) }
	}
	return g
}

// Next is generator.next().
func (g *InitIDGenerator) Next() float64 { return g.next() }

// DetailedError is the DetailedError interface.
type DetailedError struct {
	Str     string
	Hash    any
	Err     any
	Message string
}

// Error makes a DetailedError usable as a Go error.
func (e *DetailedError) Error() string { return e.Str }

// IsDetailedError is isDetailedError(error): 'str' in error. It is true
// for a DetailedError and for an object that has the property str.
func IsDetailedError(err any) bool {
	switch e := err.(type) {
	case *DetailedError:
		return e != nil
	case DetailedError:
		return true
	case *js.Obj[any]:
		return e != nil && e.Has("str")
	}
	return false
}

// GetErrorMessage converts an error to an error message. A Go error is an
// Error instance of upstream.
func GetErrorMessage(err any) string {
	if e, ok := err.(error); ok {
		return e.Error()
	}
	return js.ToString(err)
}

// InsertTitle appends a <text> element with the given title and css class.
// It forwards to render.InsertTitle.
func InsertTitle(parent *dom.Node, cssClass string, titleTopMargin float64, title string) {
	render.InsertTitle(parent, cssClass, titleTopMargin, title)
}

// ParseFontSize parses a raw fontSize configuration value into a number and
// a string value. fontSize is a number, a string or nil (undefined). The
// last result is false when upstream returns [undefined, undefined]: no
// number could be parsed from the input.
func ParseFontSize(fontSize any) (float64, string, bool) {
	// if the font size is a number, assume a px string representation
	switch v := fontSize.(type) {
	case float64:
		return v, js.Str(v) + "px", true
	case int:
		return float64(v), js.Str(float64(v)) + "px", true
	}

	text := ""
	if !js.IsNullish(fontSize) {
		text = js.ToString(fontSize)
	}
	fontSizeNumber := js.ParseInt(text, 10)
	if math.IsNaN(fontSizeNumber) {
		// if a number value can't be parsed, return null for both values
		return 0, "", false
	} else if s, ok := fontSize.(string); ok && s == js.Str(fontSizeNumber) {
		// if a string input doesn't contain any units, assume px units
		return fontSizeNumber, s + "px", true
	}
	return fontSizeNumber, text, true
}

// CleanAndMerge is cleanAndMerge(defaultData, data): merge({}, defaultData,
// data) of es-toolkit. The result is a new object, and the nested objects
// and arrays of defaultData are copied.
//
// Upstream throws a RangeError when a merge sets an invalid length on an
// array, for example the data { a: { length: -1 } } on the default
// { a: [1] }. This function ignores such an assignment. Use
// CleanAndMergeErr to get the error.
func CleanAndMerge(defaultData, data any) *js.Obj[any] {
	result, _ := merger{}.merge(js.NewObj[any](), defaultData, data)
	return result.(*js.Obj[any])
}

// CleanAndMergeErr is CleanAndMerge with the error that upstream throws: a
// *MergeError. It also fails when an array would pass 2^20 elements, where
// upstream makes a sparse array.
func CleanAndMergeErr(defaultData, data any) (*js.Obj[any], error) {
	result, err := merger{strict: true}.merge(js.NewObj[any](), defaultData, data)
	if err != nil {
		return nil, err
	}
	return result.(*js.Obj[any]), nil
}

// EncodeEntities is encodeEntities(text). It forwards to render.
func EncodeEntities(text string) string { return render.EncodeEntities(text) }

// DecodeEntities is decodeEntities(text). It forwards to render.
func DecodeEntities(text string) string { return render.DecodeEntities(text) }

// IsString is isString(value).
func IsString(value any) bool {
	_, ok := value.(string)
	return ok
}

// EdgeIdOptions is the options argument of getEdgeId.
type EdgeIdOptions struct {
	// Counter is the counter of the edge. The default of upstream is 0.
	Counter int
	Prefix  string
	Suffix  string
}

// GetEdgeId is getEdgeId(from, to, { counter, prefix, suffix }, id). A
// non-empty id wins.
func GetEdgeId(from, to string, opts EdgeIdOptions, id string) string {
	if id != "" {
		return id
	}
	var sb strings.Builder
	if opts.Prefix != "" {
		sb.WriteString(opts.Prefix)
		sb.WriteByte('_')
	}
	sb.WriteString(from)
	sb.WriteByte('_')
	sb.WriteString(to)
	sb.WriteByte('_')
	sb.WriteString(strconv.Itoa(opts.Counter))
	if opts.Suffix != "" {
		sb.WriteByte('_')
		sb.WriteString(opts.Suffix)
	}
	return sb.String()
}

// HandleUndefinedAttr is handleUndefinedAttr(attrValue): attrValue ?? null.
// The result is nil for undefined and for null, and (*dom.Node).Attr removes
// the attribute for nil, as d3 does for null.
func HandleUndefinedAttr(attrValue any) any {
	if js.IsNullish(attrValue) {
		return nil
	}
	return attrValue
}

var reDecimalNumber = regexp.MustCompile(`([0-9]+\.[0-9]+)`)

// IsLabelCoordinateInPath checks if the x or y coordinate of the edge label
// appears in the given SVG path data string (the d attribute of a path).
func IsLabelCoordinateInPath(point Point, dAttr string) bool {
	roundedX := js.Round(point.X)
	roundedY := js.Round(point.Y)

	sanitizedD := reDecimalNumber.ReplaceAllStringFunc(dAttr, func(match string) string {
		return js.Str(js.Round(js.ParseFloat(match)))
	})

	return strings.Contains(sanitizedD, js.Str(roundedX)) || strings.Contains(sanitizedD, js.Str(roundedY))
}
