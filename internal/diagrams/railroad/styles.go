// Ported from mermaid 12.2.0, diagrams/railroad/styles.ts
// (buildRailroadStyleOptions) and from DEFAULT_RAILROAD_CONFIG in
// railroadTypes.ts (MIT).

package railroad

import (
	"math"
	"regexp"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/theme"
)

// Local copy of buildRailroadStyleOptions; replace with internal/styles.
//
// The style package has the complete function, for the style sheet, and
// does not export it. This copy has the options that the renderer reads:
// the spacing, the font and the radius of the markers. The colors go only
// into the style sheet.

// styleOptions is the part of Required<RailroadStyleOptions> that the
// renderer reads.
type styleOptions struct {
	padding              float64
	verticalSeparation   float64
	horizontalSeparation float64
	arcRadius            float64
	fontSize             float64
	fontFamily           string
	markerRadius         float64
}

// defaultRailroadConfig is DEFAULT_RAILROAD_CONFIG.
var defaultRailroadConfig = styleOptions{
	padding:              10,
	verticalSeparation:   8,
	horizontalSeparation: 10,
	arcRadius:            10,
	fontSize:             14,
	fontFamily:           "monospace",
	markerRadius:         5,
}

// fontFamilyPattern is FONT_FAMILY_PATTERN: /^[\w "',.-]+$/.
var fontFamilyPattern = regexp.MustCompile(`^[0-9A-Za-z_ "',.-]+$`)

// sanitizeFontFamilyValue is sanitizeFontFamilyValue(value, fallback).
func sanitizeFontFamilyValue(value any, fallback string) string {
	str, ok := value.(string)
	if !ok {
		return fallback
	}

	normalized := js.TrimSpace(str)
	if fontFamilyPattern.MatchString(normalized) {
		return normalized
	}
	return fallback
}

// parseNumber is the number that sanitizeNumberValue and parseThemeFontSize
// read from a value: a number as it is, a string through parseFloat, and
// NaN for all other values.
func parseNumber(value any) float64 {
	switch x := value.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case string:
		return js.ParseFloat(x)
	}
	return math.NaN()
}

// sanitizeNumberValue is sanitizeNumberValue(value, fallback).
func sanitizeNumberValue(value any, fallback float64) float64 {
	parsed := parseNumber(value)

	if !math.IsNaN(parsed) && !math.IsInf(parsed, 0) && parsed >= 0 {
		return parsed
	}
	return fallback
}

// parseThemeFontSize is parseThemeFontSize(value). The second result is
// false when the value is not a positive number: undefined upstream.
func parseThemeFontSize(value any) (float64, bool) {
	parsed := parseNumber(value)

	if !math.IsNaN(parsed) && !math.IsInf(parsed, 0) && parsed > 0 {
		return parsed, true
	}
	return 0, false
}

// buildRailroadStyleOptions is buildRailroadStyleOptions() with no options,
// as the renderer calls it.
func buildRailroadStyleOptions(ctx *render.Context) (styleOptions, error) {
	currentConfig := ctx.Config
	// const themeVariables = {
	//   ...getThemeVariables(),
	//   ...(currentConfig.themeVariables ?? {}),
	// };
	// A key of the config wins, also when its value is undefined or null.
	// The variables of the default theme are made only when a key is not in
	// the config.
	configVariables := js.AsObj(currentConfig.Get("themeVariables"))
	var defaultVariables *js.Obj[any]
	var themeErr error
	themeVariable := func(key string) any {
		if configVariables != nil && configVariables.Has(key) {
			return configVariables.Get(key)
		}
		if defaultVariables == nil && themeErr == nil {
			defaultVariables, themeErr = theme.GetThemeVariables("default", nil)
		}
		if defaultVariables == nil {
			return nil
		}
		return defaultVariables.Get(key)
	}

	// buildThemeDefaults(themeVariables)
	themeDefaults := defaultRailroadConfig
	themeDefaults.fontFamily = sanitizeFontFamilyValue(themeVariable("fontFamily"), defaultRailroadConfig.fontFamily)
	if fontSize, ok := parseThemeFontSize(themeVariable("fontSize")); ok {
		themeDefaults.fontSize = fontSize
	}
	if themeErr != nil {
		return styleOptions{}, themeErr
	}

	// const railroadOptions = { ...(currentConfig.railroad ?? {}) };
	railroadOptions := js.AsObj(currentConfig.Get("railroad"))
	option := func(key string) any {
		if railroadOptions == nil {
			return nil
		}
		return railroadOptions.Get(key)
	}

	return styleOptions{
		padding:              sanitizeNumberValue(option("padding"), themeDefaults.padding),
		verticalSeparation:   sanitizeNumberValue(option("verticalSeparation"), themeDefaults.verticalSeparation),
		horizontalSeparation: sanitizeNumberValue(option("horizontalSeparation"), themeDefaults.horizontalSeparation),
		arcRadius:            sanitizeNumberValue(option("arcRadius"), themeDefaults.arcRadius),
		fontSize:             sanitizeNumberValue(option("fontSize"), themeDefaults.fontSize),
		fontFamily:           sanitizeFontFamilyValue(option("fontFamily"), themeDefaults.fontFamily),
		markerRadius:         sanitizeNumberValue(option("markerRadius"), themeDefaults.markerRadius),
	}, nil
}
