// Ported from roughjs 4.6.6, roughjs/bin/generator.js (defaultOptions and _o)
// and roughjs/bin/core.d.ts (Options and ResolvedOptions) (MIT).

package rough

import (
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// ResolvedOptions are the options of one drawable: the default options of the
// generator with the options of the call assigned on top of them.
//
// A numeric option is stored as a number. An option of another type is
// converted with the JavaScript Number conversion when it is assigned. The
// options that roughjs writes to attributes without a conversion (stroke,
// strokeWidth, fill, fillWeight, the dash options and seed) keep their
// dynamic value.
type ResolvedOptions struct {
	maxRandomnessOffset     float64
	roughness               float64
	bowing                  float64
	stroke                  any
	strokeWidth             any
	curveFitting            float64
	curveTightness          float64
	curveStepCount          float64
	fill                    any
	fillStyle               any
	fillWeight              any
	hachureAngle            float64
	hachureGap              float64
	simplification          float64
	dashOffset              float64
	dashGap                 float64
	zigzagOffset            float64
	seed                    any
	strokeLineDash          any
	strokeLineDashOffset    any
	fillLineDash            any
	fillLineDashOffset      any
	disableMultiStroke      bool
	disableMultiStrokeFill  bool
	preserveVertices        bool
	fixedDecimalPlaceDigits any
	fillShapeRoughnessGain  float64

	// randomizer is the lazily created Random of roughjs. A copy of the
	// options shares it, as a copy made with Object.assign does.
	randomizer *random
	gen        *Generator
}

// newDefaultOptions returns the defaultOptions of a new RoughGenerator.
func newDefaultOptions(gen *Generator) *ResolvedOptions {
	return &ResolvedOptions{
		maxRandomnessOffset:    2,
		roughness:              1,
		bowing:                 1,
		stroke:                 "#000",
		strokeWidth:            1.0,
		curveTightness:         0,
		curveFitting:           0.95,
		curveStepCount:         9,
		fillStyle:              "hachure",
		fillWeight:             -1.0,
		hachureAngle:           -41,
		hachureGap:             -1,
		dashOffset:             -1,
		dashGap:                -1,
		zigzagOffset:           -1,
		seed:                   0.0,
		disableMultiStroke:     false,
		disableMultiStrokeFill: false,
		preserveVertices:       false,
		fillShapeRoughnessGain: 0.8,
		// These options have no default: they are undefined.
		simplification: math.NaN(),
		gen:            gen,
	}
}

// clone is Object.assign({}, o).
func (o *ResolvedOptions) clone() *ResolvedOptions {
	c := *o
	return &c
}

// assign sets one property, as Object.assign does for each key of the source.
// A key that roughjs does not read has no effect.
func (o *ResolvedOptions) assign(key string, v any) {
	switch key {
	case "maxRandomnessOffset":
		o.maxRandomnessOffset = toNumber(v)
	case "roughness":
		o.roughness = toNumber(v)
	case "bowing":
		o.bowing = toNumber(v)
	case "stroke":
		o.stroke = v
	case "strokeWidth":
		o.strokeWidth = v
	case "curveFitting":
		o.curveFitting = toNumber(v)
	case "curveTightness":
		o.curveTightness = toNumber(v)
	case "curveStepCount":
		o.curveStepCount = toNumber(v)
	case "fill":
		o.fill = v
	case "fillStyle":
		o.fillStyle = v
	case "fillWeight":
		o.fillWeight = v
	case "hachureAngle":
		o.hachureAngle = toNumber(v)
	case "hachureGap":
		o.hachureGap = toNumber(v)
	case "simplification":
		o.simplification = toNumber(v)
	case "dashOffset":
		o.dashOffset = toNumber(v)
	case "dashGap":
		o.dashGap = toNumber(v)
	case "zigzagOffset":
		o.zigzagOffset = toNumber(v)
	case "seed":
		o.seed = v
	case "strokeLineDash":
		o.strokeLineDash = v
	case "strokeLineDashOffset":
		o.strokeLineDashOffset = v
	case "fillLineDash":
		o.fillLineDash = v
	case "fillLineDashOffset":
		o.fillLineDashOffset = v
	case "disableMultiStroke":
		o.disableMultiStroke = js.Truthy(v)
	case "disableMultiStrokeFill":
		o.disableMultiStrokeFill = js.Truthy(v)
	case "preserveVertices":
		o.preserveVertices = js.Truthy(v)
	case "fixedDecimalPlaceDigits":
		o.fixedDecimalPlaceDigits = v
	case "fillShapeRoughnessGain":
		o.fillShapeRoughnessGain = toNumber(v)
	}
}

// strokeWidthNum is o.strokeWidth in a numeric expression.
func (o *ResolvedOptions) strokeWidthNum() float64 { return toNumber(o.strokeWidth) }

// fillWeightNum is o.fillWeight in a numeric expression.
func (o *ResolvedOptions) fillWeightNum() float64 { return toNumber(o.fillWeight) }

// hasStroke is o.stroke !== 'none'.
func (o *ResolvedOptions) hasStroke() bool { return !isString(o.stroke, "none") }

// toNumber is Number(v). It also accepts the Go integer and float types that
// are not part of the js value model.
func toNumber(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int32:
		return float64(x)
	case int64:
		return float64(x)
	case uint32:
		return float64(x)
	case float32:
		return float64(x)
	}
	return js.ToNumber(v)
}

// truthyNum is the ToBoolean conversion of a number.
func truthyNum(x float64) bool { return x != 0 && !math.IsNaN(x) }

// orNum is x || y for a number x.
func orNum(x, y float64) float64 {
	if truthyNum(x) {
		return x
	}
	return y
}

// isString is v === s.
func isString(v any, s string) bool {
	x, ok := v.(string)
	return ok && x == s
}

// truthy is the ToBoolean conversion of a dynamic value.
func truthy(v any) bool {
	switch x := v.(type) {
	case int32, int64, uint32, float32:
		return truthyNum(toNumber(x))
	case []float64:
		return true
	}
	return js.Truthy(v)
}

// toString is String(v) for a dynamic value.
func toString(v any) string {
	switch x := v.(type) {
	case int32, int64, uint32, float32:
		return js.Str(toNumber(x))
	case []float64:
		s, _ := joinArray(x, ",")
		return s
	}
	return js.ToString(v)
}

// joinArray is v.join(sep) for an array option. ok is false when v is not an
// array.
func joinArray(v any, sep string) (joined string, ok bool) {
	var parts []string
	switch x := v.(type) {
	case []any:
		parts = make([]string, len(x))
		for i, e := range x {
			// join writes an empty string for undefined and null.
			if !js.IsNullish(e) {
				parts[i] = toString(e)
			}
		}
	case []float64:
		parts = make([]string, len(x))
		for i, e := range x {
			parts[i] = js.Str(e)
		}
	default:
		return "", false
	}
	return strings.Join(parts, sep), true
}
