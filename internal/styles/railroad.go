// Ported from mermaid 12.2.0, mermaid/src/diagrams/railroad/styles.ts and from
// DEFAULT_RAILROAD_CONFIG in mermaid/src/diagrams/railroad/railroadTypes.ts
// (MIT).

package styles

import (
	"math"
	"regexp"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// railroadSpace is the class that \s matches in a JavaScript regular
// expression. The Go class \s is ASCII only.
const railroadSpace = `\t\n\v\f\r \x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}`

// The patterns spell out both letter cases. The i flag of JavaScript does not
// fold a character outside ASCII to an ASCII letter. The i flag of Go does.
var (
	// /^#(?:[\da-f]{3,4}|[\da-f]{6}|[\da-f]{8})$|^(?:rgb|rgba|hsl|hsla|hwb|lab|lch|oklab|oklch)\([\d\s%+,./-]+\)$|^[a-z]+$/i
	railroadColorValuePattern = regexp.MustCompile(
		`^#(?:[0-9a-fA-F]{3,4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$` +
			`|^(?:[rR][gG][bB]|[rR][gG][bB][aA]|[hH][sS][lL]|[hH][sS][lL][aA]|[hH][wW][bB]|[lL][aA][bB]|[lL][cC][hH]|[oO][kK][lL][aA][bB]|[oO][kK][lL][cC][hH])\([0-9` + railroadSpace + `%+,./-]+\)$` +
			`|^[a-zA-Z]+$`)
	// /^[\w "',.-]+$/
	railroadFontFamilyPattern = regexp.MustCompile(`^[0-9A-Za-z_ "',.-]+$`)
)

// railroadStyleOptionKeys is RAILROAD_STYLE_OPTION_KEYS.
var railroadStyleOptionKeys = map[string]bool{
	"compactMode":          true,
	"padding":              true,
	"verticalSeparation":   true,
	"horizontalSeparation": true,
	"arcRadius":            true,
	"fontSize":             true,
	"fontFamily":           true,
	"terminalFill":         true,
	"terminalStroke":       true,
	"terminalTextColor":    true,
	"nonTerminalFill":      true,
	"nonTerminalStroke":    true,
	"nonTerminalTextColor": true,
	"lineColor":            true,
	"strokeWidth":          true,
	"markerFill":           true,
	"commentFill":          true,
	"commentStroke":        true,
	"commentTextColor":     true,
	"specialFill":          true,
	"specialStroke":        true,
	"ruleNameColor":        true,
	"showMarkers":          true,
	"markerRadius":         true,
}

// railroadStyleOptions is Required<RailroadStyleOptions>.
type railroadStyleOptions struct {
	compactMode          any
	padding              float64
	verticalSeparation   float64
	horizontalSeparation float64
	arcRadius            float64
	fontSize             float64
	fontFamily           string
	terminalFill         string
	terminalStroke       string
	terminalTextColor    string
	nonTerminalFill      string
	nonTerminalStroke    string
	nonTerminalTextColor string
	lineColor            string
	strokeWidth          float64
	markerFill           string
	commentFill          string
	commentStroke        string
	commentTextColor     string
	specialFill          string
	specialStroke        string
	ruleNameColor        string
	showMarkers          any
	markerRadius         float64
}

// defaultRailroadConfig is DEFAULT_RAILROAD_CONFIG.
var defaultRailroadConfig = railroadStyleOptions{
	// Layout
	compactMode: false,

	// Spacing
	padding:              10,
	verticalSeparation:   8,
	horizontalSeparation: 10,
	arcRadius:            10,

	// Typography
	fontSize:   14,
	fontFamily: "monospace",

	// Colors - Terminal
	terminalFill:      "#FFFFC0",
	terminalStroke:    "#000000",
	terminalTextColor: "#000000",

	// Colors - Non-terminal
	nonTerminalFill:      "#FFFFFF",
	nonTerminalStroke:    "#000000",
	nonTerminalTextColor: "#000000",

	// Colors - Lines
	lineColor:   "#000000",
	strokeWidth: 2,

	// Colors - Markers
	markerFill: "#000000",

	// Colors - Comment
	commentFill:      "#E8E8E8",
	commentStroke:    "#888888",
	commentTextColor: "#666666",

	// Colors - Special
	specialFill:   "#F0E0FF",
	specialStroke: "#8800CC",

	// Other
	ruleNameColor: "#000066",
	showMarkers:   true,
	markerRadius:  5,
}

// railroadIsStyleOptions reports whether every key of options is a railroad
// style option, or the key "railroad".
func railroadIsStyleOptions(options *js.Obj[any]) bool {
	if options == nil {
		return false
	}

	for _, key := range options.Keys() {
		if key != "railroad" && !railroadStyleOptionKeys[key] {
			return false
		}
	}
	return true
}

func railroadExtractOverrides(options *js.Obj[any]) *js.Obj[any] {
	if options == nil {
		return js.NewObj[any]()
	}

	if options.Has("railroad") && js.Truthy(options.Get("railroad")) {
		return obj(options.Get("railroad"))
	}

	if railroadIsStyleOptions(options) {
		return options
	}
	return js.NewObj[any]()
}

func railroadExtractThemeOverrides(options *js.Obj[any]) *js.Obj[any] {
	themeOverrides := js.NewObj[any]()
	if options == nil || railroadIsStyleOptions(options) {
		return themeOverrides
	}

	options.Each(func(key string, v any) {
		switch key {
		case "railroad", "svgId", "theme", "look":
		default:
			themeOverrides.Set(key, v)
		}
	})
	return themeOverrides
}

func railroadSanitizeColorValue(value any, fallback string) string {
	str, ok := value.(string)
	if !ok {
		return fallback
	}

	normalized := js.TrimSpace(str)
	if railroadColorValuePattern.MatchString(normalized) {
		return normalized
	}
	return fallback
}

func railroadSanitizeFontFamilyValue(value any, fallback string) string {
	str, ok := value.(string)
	if !ok {
		return fallback
	}

	normalized := js.TrimSpace(str)
	if railroadFontFamilyPattern.MatchString(normalized) {
		return normalized
	}
	return fallback
}

// railroadParseNumber is the number that sanitizeNumberValue and
// parseThemeFontSize read from a value: a number as it is, a string through
// parseFloat, and NaN for all other values.
func railroadParseNumber(value any) float64 {
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

func railroadSanitizeNumberValue(value any, fallback float64) float64 {
	parsed := railroadParseNumber(value)

	if !math.IsNaN(parsed) && !math.IsInf(parsed, 0) && parsed >= 0 {
		return parsed
	}
	return fallback
}

// railroadParseThemeFontSize gives the font size and true, or false when the
// value is not a positive number.
func railroadParseThemeFontSize(value any) (float64, bool) {
	parsed := railroadParseNumber(value)

	if !math.IsNaN(parsed) && !math.IsInf(parsed, 0) && parsed > 0 {
		return parsed, true
	}
	return 0, false
}

func railroadBuildThemeDefaults(themeVariables *js.Obj[any]) railroadStyleOptions {
	tv := themeVariables.Get
	fontFamily := railroadSanitizeFontFamilyValue(tv("fontFamily"), defaultRailroadConfig.fontFamily)
	fontSize, ok := railroadParseThemeFontSize(tv("fontSize"))
	if !ok {
		fontSize = defaultRailroadConfig.fontSize
	}

	d := defaultRailroadConfig
	d.fontFamily = fontFamily
	d.fontSize = fontSize
	d.terminalFill = railroadSanitizeColorValue(nn(tv("secondBkg"), tv("secondaryColor")), defaultRailroadConfig.terminalFill)
	d.terminalStroke = railroadSanitizeColorValue(nn(tv("secondaryBorderColor"), tv("lineColor")), defaultRailroadConfig.terminalStroke)
	d.terminalTextColor = railroadSanitizeColorValue(nn(tv("secondaryTextColor"), tv("textColor")), defaultRailroadConfig.terminalTextColor)
	d.nonTerminalFill = railroadSanitizeColorValue(nn(tv("mainBkg"), tv("background")), defaultRailroadConfig.nonTerminalFill)
	d.nonTerminalStroke = railroadSanitizeColorValue(nn(tv("primaryBorderColor"), tv("lineColor")), defaultRailroadConfig.nonTerminalStroke)
	d.nonTerminalTextColor = railroadSanitizeColorValue(nn(tv("primaryTextColor"), tv("textColor")), defaultRailroadConfig.nonTerminalTextColor)
	d.lineColor = railroadSanitizeColorValue(tv("lineColor"), defaultRailroadConfig.lineColor)
	d.markerFill = railroadSanitizeColorValue(tv("lineColor"), defaultRailroadConfig.markerFill)
	d.commentFill = railroadSanitizeColorValue(nn(tv("labelBackground"), tv("tertiaryColor")), defaultRailroadConfig.commentFill)
	d.commentStroke = railroadSanitizeColorValue(nn(tv("tertiaryBorderColor"), tv("lineColor")), defaultRailroadConfig.commentStroke)
	d.commentTextColor = railroadSanitizeColorValue(nn(tv("tertiaryTextColor"), tv("textColor")), defaultRailroadConfig.commentTextColor)
	d.specialFill = railroadSanitizeColorValue(nn(tv("tertiaryColor"), tv("secondaryColor")), defaultRailroadConfig.specialFill)
	d.specialStroke = railroadSanitizeColorValue(nn(tv("tertiaryBorderColor"), tv("secondaryBorderColor")), defaultRailroadConfig.specialStroke)
	d.ruleNameColor = railroadSanitizeColorValue(nn(tv("titleColor"), tv("textColor")), defaultRailroadConfig.ruleNameColor)
	return d
}

// railroadBuildStyleOptions is buildRailroadStyleOptions. The theme values
// come from the default theme, then the theme variables of the config, then
// the options. The railroad values come from the railroad section of the
// config, then the railroad theme variable.
func railroadBuildStyleOptions(in *options) railroadStyleOptions {
	currentConfig := in.config
	themeVariables := js.NewObj[any]()
	spread := func(key string, v any) { themeVariables.Set(key, v) }
	getThemeVariables().Each(spread)
	obj(currentConfig.Get("themeVariables")).Each(spread)
	// The functions below enumerate the options.
	inOptions := in.object()
	railroadExtractThemeOverrides(inOptions).Each(spread)
	themeDefaults := railroadBuildThemeDefaults(themeVariables)
	railroadOptions := js.NewObj[any]()
	spreadRailroad := func(key string, v any) { railroadOptions.Set(key, v) }
	obj(currentConfig.Get("railroad")).Each(spreadRailroad)
	railroadExtractOverrides(inOptions).Each(spreadRailroad)
	ro := railroadOptions.Get

	return railroadStyleOptions{
		compactMode:          nn(ro("compactMode"), themeDefaults.compactMode),
		padding:              railroadSanitizeNumberValue(ro("padding"), themeDefaults.padding),
		verticalSeparation:   railroadSanitizeNumberValue(ro("verticalSeparation"), themeDefaults.verticalSeparation),
		horizontalSeparation: railroadSanitizeNumberValue(ro("horizontalSeparation"), themeDefaults.horizontalSeparation),
		arcRadius:            railroadSanitizeNumberValue(ro("arcRadius"), themeDefaults.arcRadius),
		fontSize:             railroadSanitizeNumberValue(ro("fontSize"), themeDefaults.fontSize),
		fontFamily:           railroadSanitizeFontFamilyValue(ro("fontFamily"), themeDefaults.fontFamily),
		terminalFill:         railroadSanitizeColorValue(ro("terminalFill"), themeDefaults.terminalFill),
		terminalStroke:       railroadSanitizeColorValue(ro("terminalStroke"), themeDefaults.terminalStroke),
		terminalTextColor:    railroadSanitizeColorValue(ro("terminalTextColor"), themeDefaults.terminalTextColor),
		nonTerminalFill:      railroadSanitizeColorValue(ro("nonTerminalFill"), themeDefaults.nonTerminalFill),
		nonTerminalStroke:    railroadSanitizeColorValue(ro("nonTerminalStroke"), themeDefaults.nonTerminalStroke),
		nonTerminalTextColor: railroadSanitizeColorValue(ro("nonTerminalTextColor"), themeDefaults.nonTerminalTextColor),
		lineColor:            railroadSanitizeColorValue(ro("lineColor"), themeDefaults.lineColor),
		strokeWidth:          railroadSanitizeNumberValue(ro("strokeWidth"), themeDefaults.strokeWidth),
		markerFill:           railroadSanitizeColorValue(ro("markerFill"), themeDefaults.markerFill),
		commentFill:          railroadSanitizeColorValue(ro("commentFill"), themeDefaults.commentFill),
		commentStroke:        railroadSanitizeColorValue(ro("commentStroke"), themeDefaults.commentStroke),
		commentTextColor:     railroadSanitizeColorValue(ro("commentTextColor"), themeDefaults.commentTextColor),
		specialFill:          railroadSanitizeColorValue(ro("specialFill"), themeDefaults.specialFill),
		specialStroke:        railroadSanitizeColorValue(ro("specialStroke"), themeDefaults.specialStroke),
		ruleNameColor:        railroadSanitizeColorValue(ro("ruleNameColor"), themeDefaults.ruleNameColor),
		showMarkers:          nn(ro("showMarkers"), themeDefaults.showMarkers),
		markerRadius:         railroadSanitizeNumberValue(ro("markerRadius"), themeDefaults.markerRadius),
	}
}

// railroadStyles makes the CSS of the railroad diagrams.
func railroadStyles(in *options) string {
	r := railroadBuildStyleOptions(in)
	fontFamily := r.fontFamily
	fontSize := js.Str(r.fontSize)
	strokeWidth := js.Str(r.strokeWidth)
	terminalFill, terminalStroke, terminalTextColor := r.terminalFill, r.terminalStroke, r.terminalTextColor
	nonTerminalFill, nonTerminalStroke, nonTerminalTextColor := r.nonTerminalFill, r.nonTerminalStroke, r.nonTerminalTextColor
	lineColor, markerFill := r.lineColor, r.markerFill
	commentFill, commentStroke, commentTextColor := r.commentFill, r.commentStroke, r.commentTextColor
	specialFill, specialStroke, ruleNameColor := r.specialFill, r.specialStroke, r.ruleNameColor

	return `
  .railroad-diagram {
    font-family: ` + fontFamily + `;
    font-size: ` + fontSize + `px;
  }

  .railroad-terminal rect {
    fill: ` + terminalFill + `;
    stroke: ` + terminalStroke + `;
    stroke-width: ` + strokeWidth + `px;
  }

  .railroad-terminal text {
    fill: ` + terminalTextColor + `;
    font-family: ` + fontFamily + `;
    font-size: ` + fontSize + `px;
    text-anchor: middle;
    dominant-baseline: middle;
  }

  .railroad-nonterminal rect {
    fill: ` + nonTerminalFill + `;
    stroke: ` + nonTerminalStroke + `;
    stroke-width: ` + strokeWidth + `px;
  }

  .railroad-nonterminal text {
    fill: ` + nonTerminalTextColor + `;
    font-family: ` + fontFamily + `;
    font-size: ` + fontSize + `px;
    text-anchor: middle;
    dominant-baseline: middle;
  }

  .railroad-line {
    stroke: ` + lineColor + `;
    stroke-width: ` + strokeWidth + `px;
    fill: none;
  }

  .railroad-start circle,
  .railroad-end circle {
    fill: ` + markerFill + `;
  }

  .railroad-comment ellipse {
    fill: ` + commentFill + `;
    stroke: ` + commentStroke + `;
    stroke-width: ` + strokeWidth + `px;
  }

  .railroad-comment text {
    fill: ` + commentTextColor + `;
    font-style: italic;
    font-family: ` + fontFamily + `;
    font-size: ` + fontSize + `px;
    text-anchor: middle;
    dominant-baseline: middle;
  }

  .railroad-special rect {
    fill: ` + specialFill + `;
    stroke: ` + specialStroke + `;
    stroke-width: ` + strokeWidth + `px;
    stroke-dasharray: 5,3;
  }

  .railroad-special text {
    fill: ` + nonTerminalTextColor + `;
    font-family: ` + fontFamily + `;
    font-size: ` + fontSize + `px;
    text-anchor: middle;
    dominant-baseline: middle;
  }

  .railroad-rule-name {
    font-weight: bold;
    fill: ` + ruleNameColor + `;
    font-family: ` + fontFamily + `;
    font-size: ` + fontSize + `px;
  }

  .railroad-group {
    /* Grouping container, no specific styles */
  }
`
}
