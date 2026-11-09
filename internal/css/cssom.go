// This file names no upstream file that it ports. It reproduces how Chromium
// handles an inline style through the CSSOM. Its comments name two places
// of Blink, at Chromium 156.0.8078.4:
//
//   - MutableCSSPropertyValueSet::SetLonghandProperty:
//     core/css/css_property_value_set.cc (LGPL-2.0-or-later).
//   - The number format "%.6g": core/css/css_numeric_literal_value.cc
//     (BSD-3-Clause).
//
// The serializer of the CSSOM in Blink is
// core/css/style_property_serializer.cc and css_primitive_value.cc
// (LGPL-2.0-or-later). This file does not say if it follows those two
// files. The paths are in third_party/blink/renderer. See NOTICE in this
// directory and finding F1 of docs/LICENSE-AUDIT.md.

package css

import (
	"math"
	"strconv"
	"strings"
)

// This file reproduces how Chromium handles an inline style through the
// CSSOM. d3's selection.style() calls element.style.setProperty(), and the
// browser then rewrites the whole style attribute from its parsed
// declaration block: unknown properties and invalid values disappear, and
// each value is printed in its canonical form. The final SVG contains that
// text, so the port must print the same text.

type valueKind uint8

const (
	kindGeneric valueKind = iota // token-wise normalization only
	kindColor
	kindPaint
	kindLength     // <length> | <percentage>, units required except 0
	kindLengthAuto // kindLength | auto
	kindLengthNone // kindLength | none
	kindSVGLength  // <length> | <percentage> | <number>
	kindNumber     // <number>
	kindAlpha      // <number> | <percentage>, printed as a number
	kindInteger    // <integer> | auto
	kindLineHeight // normal | <number> | <length> | <percentage>
	kindFontSize   // keywords | <length> | <percentage>
	kindFontWeight // normal | bold | bolder | lighter | 1..1000
	kindFontFamily
	kindDashArray
	kindKeyword // one keyword out of a fixed set
	kindAnimation
)

type propInfo struct {
	kind     valueKind
	keywords string // space separated, for kindKeyword and extra keywords of other kinds
}

var properties = map[string]propInfo{
	// colors and paint
	"color":                 {kind: kindColor},
	"background-color":      {kind: kindColor},
	"border-color":          {kind: kindGeneric},
	"border-top-color":      {kind: kindColor},
	"border-right-color":    {kind: kindColor},
	"border-bottom-color":   {kind: kindColor},
	"border-left-color":     {kind: kindColor},
	"outline-color":         {kind: kindColor},
	"stop-color":            {kind: kindColor},
	"flood-color":           {kind: kindColor},
	"lighting-color":        {kind: kindColor},
	"text-decoration-color": {kind: kindColor},
	"caret-color":           {kind: kindColor, keywords: "auto"},
	"fill":                  {kind: kindPaint},
	"stroke":                {kind: kindPaint},

	// box lengths
	"width":          {kind: kindLengthAuto, keywords: "min-content max-content fit-content"},
	"height":         {kind: kindLengthAuto, keywords: "min-content max-content fit-content"},
	"min-width":      {kind: kindLengthAuto, keywords: "min-content max-content fit-content"},
	"min-height":     {kind: kindLengthAuto, keywords: "min-content max-content fit-content"},
	"max-width":      {kind: kindLengthNone, keywords: "min-content max-content fit-content"},
	"max-height":     {kind: kindLengthNone, keywords: "min-content max-content fit-content"},
	"top":            {kind: kindLengthAuto},
	"right":          {kind: kindLengthAuto},
	"bottom":         {kind: kindLengthAuto},
	"left":           {kind: kindLengthAuto},
	"margin-top":     {kind: kindLengthAuto},
	"margin-right":   {kind: kindLengthAuto},
	"margin-bottom":  {kind: kindLengthAuto},
	"margin-left":    {kind: kindLengthAuto},
	"padding-top":    {kind: kindLength},
	"padding-right":  {kind: kindLength},
	"padding-bottom": {kind: kindLength},
	"padding-left":   {kind: kindLength},
	"text-indent":    {kind: kindLength},
	"letter-spacing": {kind: kindLength, keywords: "normal"},
	"word-spacing":   {kind: kindLength, keywords: "normal"},
	"margin":         {kind: kindGeneric},
	"padding":        {kind: kindGeneric},
	"inset":          {kind: kindGeneric},
	"gap":            {kind: kindGeneric},
	"row-gap":        {kind: kindLength, keywords: "normal"},
	"column-gap":     {kind: kindLength, keywords: "normal"},
	"border-radius":  {kind: kindGeneric},
	"border-width":   {kind: kindGeneric},
	"border-style":   {kind: kindGeneric},
	"border":         {kind: kindGeneric},
	"border-top":     {kind: kindGeneric},
	"border-right":   {kind: kindGeneric},
	"border-bottom":  {kind: kindGeneric},
	"border-left":    {kind: kindGeneric},
	"outline":        {kind: kindGeneric},
	"outline-width":  {kind: kindGeneric},
	"outline-style":  {kind: kindGeneric},
	"outline-offset": {kind: kindLength},
	"box-shadow":     {kind: kindGeneric},
	"text-shadow":    {kind: kindGeneric},

	// SVG lengths and numbers
	"stroke-width":      {kind: kindSVGLength},
	"stroke-dashoffset": {kind: kindSVGLength},
	"stroke-dasharray":  {kind: kindDashArray},
	"stroke-miterlimit": {kind: kindNumber},
	"x":                 {kind: kindSVGLength},
	"y":                 {kind: kindSVGLength},
	"cx":                {kind: kindSVGLength},
	"cy":                {kind: kindSVGLength},
	"r":                 {kind: kindSVGLength},
	"rx":                {kind: kindSVGLength, keywords: "auto"},
	"ry":                {kind: kindSVGLength, keywords: "auto"},
	"d":                 {kind: kindGeneric},
	"opacity":           {kind: kindAlpha},
	"fill-opacity":      {kind: kindAlpha},
	"stroke-opacity":    {kind: kindAlpha},
	"stop-opacity":      {kind: kindAlpha},
	"flood-opacity":     {kind: kindAlpha},
	"z-index":           {kind: kindInteger},
	"order":             {kind: kindInteger},
	"flex-grow":         {kind: kindNumber},
	"flex-shrink":       {kind: kindNumber},
	"flex-basis":        {kind: kindLengthAuto, keywords: "content"},
	"flex":              {kind: kindGeneric},
	"line-height":       {kind: kindLineHeight},
	"tab-size":          {kind: kindSVGLength},

	// fonts
	"font":                  {kind: kindGeneric},
	"font-family":           {kind: kindFontFamily},
	"font-size":             {kind: kindFontSize},
	"font-weight":           {kind: kindFontWeight},
	"font-style":            {kind: kindKeyword, keywords: "normal italic oblique"},
	"font-variant":          {kind: kindGeneric},
	"font-stretch":          {kind: kindGeneric},
	"font-kerning":          {kind: kindKeyword, keywords: "auto normal none"},
	"font-feature-settings": {kind: kindGeneric},

	// keyword properties
	"display":              {kind: kindKeyword, keywords: "none inline block inline-block flex inline-flex grid inline-grid table inline-table table-row-group table-header-group table-footer-group table-row table-cell table-column-group table-column table-caption list-item contents flow-root -webkit-box -webkit-inline-box ruby math"},
	"visibility":           {kind: kindKeyword, keywords: "visible hidden collapse"},
	"position":             {kind: kindKeyword, keywords: "static relative absolute fixed sticky"},
	"float":                {kind: kindKeyword, keywords: "none left right inline-start inline-end"},
	"clear":                {kind: kindKeyword, keywords: "none left right both inline-start inline-end"},
	"box-sizing":           {kind: kindKeyword, keywords: "content-box border-box"},
	"white-space":          {kind: kindKeyword, keywords: "normal nowrap pre pre-wrap pre-line break-spaces"},
	"text-align":           {kind: kindKeyword, keywords: "left right center justify start end match-parent -webkit-left -webkit-right -webkit-center"},
	"text-anchor":          {kind: kindKeyword, keywords: "start middle end"},
	"dominant-baseline":    {kind: kindKeyword, keywords: "auto alphabetic ideographic middle central mathematical hanging use-script no-change reset-size text-after-edge text-before-edge text-top text-bottom"},
	"alignment-baseline":   {kind: kindKeyword, keywords: "auto baseline alphabetic ideographic middle central mathematical before-edge after-edge text-before-edge text-after-edge hanging"},
	"vertical-align":       {kind: kindLength, keywords: "baseline sub super text-top text-bottom middle top bottom"},
	"word-break":           {kind: kindKeyword, keywords: "normal break-all keep-all break-word auto-phrase"},
	"overflow-wrap":        {kind: kindKeyword, keywords: "normal break-word anywhere"},
	"word-wrap":            {kind: kindKeyword, keywords: "normal break-word anywhere"},
	"text-transform":       {kind: kindKeyword, keywords: "none capitalize uppercase lowercase full-width math-auto"},
	"text-decoration":      {kind: kindGeneric},
	"text-decoration-line": {kind: kindGeneric},
	"text-overflow":        {kind: kindGeneric},
	"text-rendering":       {kind: kindKeyword, keywords: "auto optimizespeed optimizelegibility geometricprecision"},
	"shape-rendering":      {kind: kindKeyword, keywords: "auto optimizespeed crispedges geometricprecision"},
	"overflow":             {kind: kindGeneric},
	"overflow-x":           {kind: kindKeyword, keywords: "visible hidden clip scroll auto overlay"},
	"overflow-y":           {kind: kindKeyword, keywords: "visible hidden clip scroll auto overlay"},
	"pointer-events":       {kind: kindKeyword, keywords: "auto none visiblepainted visiblefill visiblestroke visible painted fill stroke all bounding-box"},
	"cursor":               {kind: kindGeneric},
	"mix-blend-mode":       {kind: kindKeyword, keywords: "normal multiply screen overlay darken lighten color-dodge color-burn hard-light soft-light difference exclusion hue saturation color luminosity plus-darker plus-lighter"},
	"stroke-linecap":       {kind: kindKeyword, keywords: "butt round square"},
	"stroke-linejoin":      {kind: kindKeyword, keywords: "miter round bevel arcs miter-clip"},
	"fill-rule":            {kind: kindKeyword, keywords: "nonzero evenodd"},
	"clip-rule":            {kind: kindKeyword, keywords: "nonzero evenodd"},
	"vector-effect":        {kind: kindKeyword, keywords: "none non-scaling-stroke"},
	"paint-order":          {kind: kindGeneric},
	"align-items":          {kind: kindGeneric},
	"align-content":        {kind: kindGeneric},
	"align-self":           {kind: kindGeneric},
	"justify-content":      {kind: kindGeneric},
	"justify-items":        {kind: kindGeneric},
	"justify-self":         {kind: kindGeneric},
	"flex-direction":       {kind: kindKeyword, keywords: "row row-reverse column column-reverse"},
	"flex-wrap":            {kind: kindKeyword, keywords: "nowrap wrap wrap-reverse"},
	"flex-flow":            {kind: kindGeneric},
	"direction":            {kind: kindKeyword, keywords: "ltr rtl"},
	"unicode-bidi":         {kind: kindGeneric},
	"writing-mode":         {kind: kindGeneric},
	"user-select":          {kind: kindKeyword, keywords: "auto none text all contain"},
	"table-layout":         {kind: kindKeyword, keywords: "auto fixed"},
	"border-collapse":      {kind: kindKeyword, keywords: "separate collapse"},
	"list-style":           {kind: kindGeneric},
	"list-style-type":      {kind: kindGeneric},
	"object-fit":           {kind: kindKeyword, keywords: "fill contain cover none scale-down"},
	"isolation":            {kind: kindKeyword, keywords: "auto isolate"},

	// complex values, normalized token by token
	"background":             {kind: kindGeneric},
	"background-image":       {kind: kindGeneric},
	"background-size":        {kind: kindGeneric},
	"background-position":    {kind: kindGeneric},
	"background-repeat":      {kind: kindGeneric},
	"background-clip":        {kind: kindGeneric},
	"transform":              {kind: kindGeneric},
	"transform-origin":       {kind: kindGeneric},
	"transform-box":          {kind: kindGeneric},
	"transition":             {kind: kindGeneric},
	"animation":              {kind: kindAnimation},
	"filter":                 {kind: kindGeneric},
	"backdrop-filter":        {kind: kindGeneric},
	"clip-path":              {kind: kindGeneric},
	"mask":                   {kind: kindGeneric},
	"marker":                 {kind: kindGeneric},
	"marker-start":           {kind: kindGeneric},
	"marker-mid":             {kind: kindGeneric},
	"marker-end":             {kind: kindGeneric},
	"content":                {kind: kindGeneric},
	"quotes":                 {kind: kindGeneric},
	"will-change":            {kind: kindGeneric},
	"aspect-ratio":           {kind: kindGeneric},
	"grid-template-columns":  {kind: kindGeneric},
	"grid-template-rows":     {kind: kindGeneric},
	"grid-column":            {kind: kindGeneric},
	"grid-row":               {kind: kindGeneric},
	"place-items":            {kind: kindGeneric},
	"place-content":          {kind: kindGeneric},
	"offset-path":            {kind: kindGeneric},
	"offset-distance":        {kind: kindGeneric},
	"offset-rotate":          {kind: kindGeneric},
	"color-scheme":           {kind: kindGeneric},
	"appearance":             {kind: kindGeneric},
	"resize":                 {kind: kindGeneric},
	"hyphens":                {kind: kindGeneric},
	"line-break":             {kind: kindGeneric},
	"text-wrap":              {kind: kindGeneric},
	"text-underline-offset":  {kind: kindGeneric},
	"-webkit-line-clamp":     {kind: kindGeneric},
	"-webkit-box-orient":     {kind: kindGeneric},
	"-webkit-text-stroke":    {kind: kindGeneric},
	"-webkit-font-smoothing": {kind: kindGeneric},
	"zoom":                   {kind: kindGeneric},
	"scale":                  {kind: kindGeneric},
	"rotate":                 {kind: kindGeneric},
	"translate":              {kind: kindGeneric},
	"all":                    {kind: kindGeneric},
}

var cssWideKeywords = map[string]bool{
	"inherit": true, "initial": true, "unset": true, "revert": true, "revert-layer": true,
}

// KnownProperty reports whether Chromium has a CSS property with this name.
func KnownProperty(name string) bool {
	if strings.HasPrefix(name, "--") {
		return len(name) > 2
	}
	_, ok := properties[name]
	return ok
}

// ParseInlineStyle parses a style attribute as the CSSOM sees it: unknown
// properties and invalid values are dropped and values are normalized. A
// later declaration of a property replaces an earlier one. svg is true for
// an element in the SVG namespace: Blink then accepts a unitless number as
// a length, and it stores !important declarations after the normal ones.
func ParseInlineStyle(text string, svg bool) []Decl {
	var out []Decl
	for _, d := range ParseDeclarations(text) {
		v, ok := normalizeValue(d.Name, d.Value, svg)
		if !ok {
			continue
		}
		d.Raw, d.Value = d.Value, v
		replaced := false
		for i := range out {
			if out[i].Name == d.Name {
				// An !important declaration is not replaced by a normal one.
				if out[i].Important && !d.Important {
					replaced = true
					break
				}
				out = append(out[:i], out[i+1:]...)
				break
			}
		}
		if !replaced {
			out = append(out, d)
		}
	}
	if svg {
		normal := make([]Decl, 0, len(out))
		var important []Decl
		for _, d := range out {
			if d.Important {
				important = append(important, d)
			} else {
				normal = append(normal, d)
			}
		}
		out = append(normal, important...)
	}
	return out
}

// SetProperty is style.setProperty(name, value, priority) on a parsed block.
// The second result reports whether the block changed.
func SetProperty(block []Decl, name, value string, important bool, svg bool) ([]Decl, bool) {
	if !strings.HasPrefix(name, "--") {
		name = strings.ToLower(name)
	}
	if !KnownProperty(name) {
		return block, false
	}
	if strings.TrimSpace(value) == "" {
		return RemoveProperty(block, name)
	}
	v, ok := normalizeValue(name, value, svg)
	if !ok {
		return block, false
	}
	for i := range block {
		if block[i].Name == name {
			if block[i].Value == v && block[i].Important == important && sameColorSpace(block[i].Raw, value) &&
				ExactValue(block[i]) == ExactValue(Decl{Name: name, Value: v, Raw: value}) {
				// Blink compares the parsed values and reports no change
				// for an equal one (MutableCSSPropertyValueSet::
				// SetLonghandProperty returns kUnchanged). The style
				// attribute then keeps its text, also a text that was
				// authored in a non-canonical form.
				return block, false
			}
			block[i].Value, block[i].Important, block[i].Raw = v, important, value
			return block, true
		}
	}
	return append(block, Decl{Name: name, Value: v, Important: important, Raw: value}), true
}

// sameColorSpace reports whether two values with the same canonical text are
// also equal as parsed values. Blink keeps the color space of a color: the
// colors hsl(0, 100%, 50%) and #ff0000 both print as rgb(255, 0, 0), and
// they are not equal. The colors of hsl() and hwb() are equal only to a
// color that is written in the same way.
func sameColorSpace(a, b string) bool {
	compact := func(s string) (text string, space int) {
		s = strings.ToLower(s)
		switch {
		case strings.Contains(s, "hsl(") || strings.Contains(s, "hsla("):
			space = 1
		case strings.Contains(s, "hwb("):
			space = 2
		default:
			return "", 0
		}
		return strings.Join(strings.Fields(strings.NewReplacer(",", " , ", "(", " ( ", ")", " ) ", "/", " / ").Replace(s)), ""), space
	}
	textA, spaceA := compact(a)
	textB, spaceB := compact(b)
	return spaceA == spaceB && textA == textB
}

// RemoveProperty is style.removeProperty(name).
func RemoveProperty(block []Decl, name string) ([]Decl, bool) {
	for i := range block {
		if block[i].Name == name {
			return append(block[:i], block[i+1:]...), true
		}
	}
	return block, false
}

// SerializeInlineStyle is the cssText of a declaration block.
func SerializeInlineStyle(block []Decl) string {
	var sb strings.Builder
	for i, d := range block {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(d.Name)
		sb.WriteString(": ")
		sb.WriteString(d.Value)
		if d.Important {
			sb.WriteString(" !important")
		}
		sb.WriteByte(';')
	}
	return sb.String()
}

// NormalizeValue validates a value for a property and returns its canonical
// CSSOM text. The second result is false when Chromium rejects the value.
func NormalizeValue(name, value string) (string, bool) {
	return normalizeValue(name, value, false)
}

func normalizeValue(name, value string, svg bool) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	if strings.HasPrefix(name, "--") {
		return value, true
	}
	info, ok := properties[name]
	if !ok {
		return "", false
	}
	lower := strings.ToLower(value)
	if cssWideKeywords[lower] {
		return lower, true
	}
	if strings.Contains(lower, "var(") {
		// A value with a variable reference is kept as written.
		return value, true
	}
	if info.keywords != "" && hasWord(info.keywords, lower) {
		return lower, true
	}
	if rejectsNegative(name, lower) {
		return "", false
	}
	switch info.kind {
	case kindKeyword:
		return "", false
	case kindColor:
		return normalizeColor(value)
	case kindPaint:
		return normalizePaint(value)
	case kindLength:
		return normalizeBoxLength(value, svg)
	case kindLengthAuto:
		if lower == "auto" {
			return lower, true
		}
		return normalizeBoxLength(value, svg)
	case kindLengthNone:
		if lower == "none" {
			return lower, true
		}
		return normalizeBoxLength(value, svg)
	case kindSVGLength:
		return normalizeLength(value, true)
	case kindNumber:
		f, unit, ok := parseDimension(lower)
		if !ok || unit != "" {
			return "", false
		}
		return FormatNumber(f), true
	case kindAlpha:
		f, unit, ok := parseDimension(lower)
		if !ok || (unit != "" && unit != "%") {
			return "", false
		}
		if unit == "%" {
			f /= 100
		}
		return FormatNumber(f), true
	case kindInteger:
		if lower == "auto" {
			return lower, true
		}
		f, unit, ok := parseDimension(lower)
		if !ok || unit != "" || f != math.Trunc(f) {
			return "", false
		}
		return FormatNumber(f), true
	case kindLineHeight:
		if lower == "normal" {
			return lower, true
		}
		return normalizeLength(value, true)
	case kindFontSize:
		if hasWord("xx-small x-small small medium large x-large xx-large xxx-large smaller larger", lower) {
			return lower, true
		}
		f, unit, ok := parseDimension(lower)
		if !ok || f < 0 {
			if strings.HasPrefix(lower, "calc(") {
				return normalizeGeneric(value), true
			}
			return "", false
		}
		if unit == "" {
			if f != 0 && !svg {
				return "", false
			}
			unit = "px"
		}
		return formatDimension(f, unit), true
	case kindFontWeight:
		if hasWord("normal bold bolder lighter", lower) {
			return lower, true
		}
		f, unit, ok := parseDimension(lower)
		if !ok || unit != "" || f < 1 || f > 1000 {
			return "", false
		}
		return FormatNumber(f), true
	case kindFontFamily:
		return normalizeFontFamily(value)
	case kindDashArray:
		return normalizeDashArray(value)
	case kindAnimation:
		if v, ok := normalizeAnimation(value); ok {
			return v, true
		}
	}
	return normalizeGeneric(value), true
}

func hasWord(list, word string) bool {
	for len(list) > 0 {
		i := strings.IndexByte(list, ' ')
		w := list
		if i >= 0 {
			w, list = list[:i], list[i+1:]
		} else {
			list = ""
		}
		if w == word {
			return true
		}
	}
	return false
}

// FormatNumber prints a CSS number as Blink does, with printf("%.6g"): at
// most 6 significant digits, no trailing zeros, and a C-style exponent.
func FormatNumber(f float64) string {
	if f == 0 || math.IsNaN(f) {
		return "0"
	}
	s := strconv.FormatFloat(f, 'g', 6, 64)
	mant, exp, hasExp := strings.Cut(s, "e")
	if strings.Contains(mant, ".") {
		mant = strings.TrimRight(strings.TrimRight(mant, "0"), ".")
	}
	if !hasExp {
		return mant
	}
	// Go prints "1e-06" and "1.23457e+07", as C does.
	return mant + "e" + exp
}

var lengthUnits = map[string]bool{
	"px": true, "em": true, "rem": true, "ex": true, "ch": true, "pt": true, "pc": true, "in": true,
	"cm": true, "mm": true, "q": true, "vw": true, "vh": true, "vmin": true, "vmax": true, "%": true,
	"lh": true, "rlh": true, "cap": true, "ic": true, "svw": true, "svh": true, "lvw": true, "lvh": true,
	"dvw": true, "dvh": true, "cqw": true, "cqh": true,
}

// parseDimension parses "<number><unit>". The unit is lowercase, "" or "%".
func parseDimension(s string) (float64, string, bool) {
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	digits := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
		digits++
	}
	if i < len(s) && s[i] == '.' {
		j := i + 1
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
			digits++
		}
		if j == i+1 {
			return 0, "", false // "1." is not a number in CSS
		}
		i = j
	}
	if digits == 0 {
		return 0, "", false
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		j := i + 1
		if j < len(s) && (s[j] == '+' || s[j] == '-') {
			j++
		}
		if j < len(s) && s[j] >= '0' && s[j] <= '9' {
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			i = j
		}
	}
	f, err := strconv.ParseFloat(s[:i], 64)
	if err != nil {
		return 0, "", false
	}
	unit := strings.ToLower(s[i:])
	for k := 0; k < len(unit); k++ {
		c := unit[k]
		if !(c == '%' || (c >= 'a' && c <= 'z')) {
			return 0, "", false
		}
	}
	return f, unit, true
}

func formatDimension(f float64, unit string) string {
	return FormatNumber(f) + unit
}

// normalizeBoxLength normalizes a length that needs a unit. In SVG mode a
// unitless number is a length in px.
func normalizeBoxLength(value string, svg bool) (string, bool) {
	if svg {
		if f, unit, ok := parseDimension(strings.ToLower(value)); ok && unit == "" {
			return formatDimension(f, "px"), true
		}
	}
	return normalizeLength(value, false)
}

func normalizeLength(value string, allowUnitless bool) (string, bool) {
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "calc(") || strings.HasPrefix(lower, "min(") || strings.HasPrefix(lower, "max(") || strings.HasPrefix(lower, "clamp(") {
		return normalizeGeneric(value), true
	}
	f, unit, ok := parseDimension(lower)
	if !ok {
		return "", false
	}
	if unit == "" {
		if allowUnitless {
			return FormatNumber(f), true
		}
		if f != 0 {
			return "", false
		}
		return "0px", true
	}
	if !lengthUnits[unit] {
		return "", false
	}
	return formatDimension(f, unit), true
}

func normalizeColor(value string) (string, bool) {
	lower := strings.ToLower(value)
	if lower == "currentcolor" || lower == "transparent" {
		return lower, true
	}
	if _, ok := namedColors[lower]; ok {
		return lower, true
	}
	if isSystemColor(lower) {
		return lower, true
	}
	c, ok := ParseColor(value)
	if !ok {
		return "", false
	}
	return c.CSSText(), true
}

func isSystemColor(s string) bool {
	return hasWord("canvas canvastext linktext visitedtext activetext buttonface buttontext buttonborder field fieldtext highlight highlighttext selecteditem selecteditemtext mark marktext graytext accentcolor accentcolortext", s)
}

func normalizePaint(value string) (string, bool) {
	lower := strings.ToLower(value)
	switch lower {
	case "none", "context-fill", "context-stroke":
		return lower, true
	}
	if strings.HasPrefix(lower, "url(") {
		end := strings.IndexByte(value, ')')
		if end < 0 {
			return "", false
		}
		url := normalizeURL(value[:end+1])
		rest := strings.TrimSpace(value[end+1:])
		if rest == "" {
			return url, true
		}
		if strings.EqualFold(rest, "none") {
			return url + " none", true
		}
		c, ok := normalizeColor(rest)
		if !ok {
			return "", false
		}
		return url + " " + c, true
	}
	return normalizeColor(value)
}

// normalizeURL prints url(x) as url("x").
func normalizeURL(s string) string {
	inner := strings.TrimSpace(s[4 : len(s)-1])
	if len(inner) >= 2 && (inner[0] == '"' || inner[0] == '\'') && inner[len(inner)-1] == inner[0] {
		inner = inner[1 : len(inner)-1]
	}
	return `url("` + strings.ReplaceAll(inner, `"`, `\"`) + `")`
}

var genericFamilies = map[string]bool{
	"serif": true, "sans-serif": true, "monospace": true, "cursive": true, "fantasy": true,
	"system-ui": true, "ui-serif": true, "ui-sans-serif": true, "ui-monospace": true, "ui-rounded": true,
	"math": true, "emoji": true, "fangsong": true,
}

// SplitFontFamily returns the family names of a font-family value, without
// quotes. Generic families are returned in lowercase. A quoted name that is
// the keyword of a generic family, such as "serif", is a family name and
// not the generic family: it keeps double quotes, so that the caller can
// tell the two apart.
func SplitFontFamily(value string) []string {
	var out []string
	for _, part := range splitTopLevel(value, ',') {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if len(part) >= 2 && (part[0] == '"' || part[0] == '\'') && part[len(part)-1] == part[0] {
			name := part[1 : len(part)-1]
			if genericFamilies[strings.ToLower(name)] {
				name = `"` + name + `"`
			}
			out = append(out, name)
			continue
		}
		name := strings.Join(strings.Fields(part), " ")
		if genericFamilies[strings.ToLower(name)] {
			name = strings.ToLower(name)
		}
		out = append(out, name)
	}
	return out
}

func normalizeFontFamily(value string) (string, bool) {
	var parts []string
	for _, part := range splitTopLevel(value, ',') {
		part = strings.TrimSpace(part)
		if part == "" {
			return "", false
		}
		if len(part) >= 2 && (part[0] == '"' || part[0] == '\'') && part[len(part)-1] == part[0] {
			parts = append(parts, `"`+strings.ReplaceAll(part[1:len(part)-1], `"`, `\"`)+`"`)
			continue
		}
		words := strings.Fields(part)
		for _, w := range words {
			if !isIdent(w) || (w[0] >= '0' && w[0] <= '9') {
				return "", false
			}
		}
		name := strings.Join(words, " ")
		if genericFamilies[strings.ToLower(name)] {
			parts = append(parts, strings.ToLower(name))
		} else if len(words) > 1 {
			// Blink quotes a family name of several identifiers.
			parts = append(parts, `"`+name+`"`)
		} else {
			parts = append(parts, name)
		}
	}
	if len(parts) == 0 {
		return "", false
	}
	return strings.Join(parts, ", "), true
}

func normalizeDashArray(value string) (string, bool) {
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "none" {
		return lower, true
	}
	fields := strings.FieldsFunc(lower, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' })
	if len(fields) == 0 {
		return "", false
	}
	out := make([]string, len(fields))
	for i, f := range fields {
		v, unit, ok := parseDimension(f)
		if !ok || v < 0 || (unit != "" && !lengthUnits[unit]) {
			return "", false
		}
		out[i] = formatDimension(v, unit)
	}
	return strings.Join(out, ", "), true
}

// normalizeGeneric rewrites a value token by token: one space between
// tokens, ", " after a comma, lowercase keywords and units, canonical
// numbers, colors as rgb(), and quoted url().
func normalizeGeneric(value string) string {
	var out []string
	for _, tok := range tokenizeValue(value) {
		out = append(out, normalizeToken(tok))
	}
	var sb strings.Builder
	for i, t := range out {
		if i > 0 && t != "," && out[i-1] != "/" && t != "/" {
			sb.WriteByte(' ')
		} else if i > 0 && (t == "/" || out[i-1] == "/") {
			sb.WriteByte(' ')
		}
		sb.WriteString(t)
	}
	return sb.String()
}

func tokenizeValue(s string) []string {
	var toks []string
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f':
			i++
		case c == ',' || c == '/':
			toks = append(toks, string(c))
			i++
		case c == '"' || c == '\'':
			j := i + 1
			for j < len(s) && s[j] != c {
				if s[j] == '\\' {
					j++
				}
				j++
			}
			if j < len(s) {
				j++
			}
			toks = append(toks, s[i:min(j, len(s))])
			i = j
		default:
			j := i
			depth := 0
			for j < len(s) {
				ch := s[j]
				if ch == '(' {
					depth++
				} else if ch == ')' {
					depth--
					if depth == 0 {
						j++
						break
					}
				} else if depth == 0 && (ch == ' ' || ch == '\t' || ch == '\n' || ch == ',' || ch == '/') {
					break
				}
				j++
			}
			toks = append(toks, s[i:min(j, len(s))])
			i = j
		}
	}
	return toks
}

func normalizeToken(tok string) string {
	if tok == "," || tok == "/" {
		return tok
	}
	if tok[0] == '"' {
		return tok
	}
	if tok[0] == '\'' {
		return `"` + strings.ReplaceAll(tok[1:len(tok)-1], `"`, `\"`) + `"`
	}
	lower := strings.ToLower(tok)
	if strings.HasPrefix(lower, "url(") && strings.HasSuffix(tok, ")") {
		return normalizeURL(tok)
	}
	if tok[0] == '#' || strings.HasPrefix(lower, "rgb") || strings.HasPrefix(lower, "hsl") {
		if c, ok := ParseColor(tok); ok {
			return c.CSSText()
		}
	}
	if f, unit, ok := parseDimension(lower); ok {
		return formatDimension(f, unit)
	}
	if open := strings.IndexByte(tok, '('); open > 0 && strings.HasSuffix(tok, ")") {
		// A function: normalize the arguments.
		inner := tok[open+1 : len(tok)-1]
		return lower[:open] + "(" + normalizeGeneric(inner) + ")"
	}
	return lower
}

// ExactValue returns the value of a declaration with the full number that
// the author wrote. Blink keeps a number as a double in the parsed value
// and rounds it to 6 significant digits only when it prints the value, so
// layout sees 91.828125px where the style attribute shows 91.8281px. The
// result differs from d.Value only for a value that is one number or one
// dimension and that lost digits when it was printed.
func ExactValue(d Decl) string {
	if d.Raw == "" || d.Raw == d.Value {
		return d.Value
	}
	vn, vunit, ok := parseDimension(d.Value)
	if !ok {
		return d.Value
	}
	rn, runit, ok := parseDimension(strings.TrimSpace(d.Raw))
	if !ok || (runit != vunit && runit != "") {
		return d.Value
	}
	// The printed value must be the rounding of the written one.
	if rn == vn || FormatNumber(rn) != FormatNumber(vn) {
		return d.Value
	}
	return strconv.FormatFloat(rn, 'f', -1, 64) + vunit
}

// SerializeInlineStyleExact is SerializeInlineStyle with the exact value of
// each declaration. It is the form that layout reads.
func SerializeInlineStyleExact(block []Decl) string {
	exact := make([]Decl, len(block))
	for i, d := range block {
		d.Value = ExactValue(d)
		exact[i] = d
	}
	return SerializeInlineStyle(exact)
}

// normalizeAnimation prints the animation shorthand as Blink does: every
// animation with its eight longhand values in a fixed order, with the
// defaults for the values that the author left out. So "dash 25s linear
// infinite" prints as "25s linear 0s infinite normal none running dash".
// It reports false for a value that it cannot read.
func normalizeAnimation(value string) (string, bool) {
	var animations []string
	var current []string
	flush := func() bool {
		if len(current) == 0 {
			return false
		}
		a, ok := normalizeSingleAnimation(current)
		if !ok {
			return false
		}
		animations = append(animations, a)
		current = nil
		return true
	}
	for _, tok := range tokenizeValue(value) {
		if tok == "," {
			if !flush() {
				return "", false
			}
			continue
		}
		current = append(current, tok)
	}
	if !flush() {
		return "", false
	}
	return strings.Join(animations, ", "), true
}

var (
	animationEasings    = map[string]bool{"linear": true, "ease": true, "ease-in": true, "ease-out": true, "ease-in-out": true, "step-start": true, "step-end": true}
	animationDirections = map[string]bool{"normal": true, "reverse": true, "alternate": true, "alternate-reverse": true}
	animationFillModes  = map[string]bool{"none": true, "forwards": true, "backwards": true, "both": true}
	animationPlayStates = map[string]bool{"running": true, "paused": true}
)

func normalizeSingleAnimation(tokens []string) (string, bool) {
	var duration, easing, delay, count, direction, fill, state, name string
	for _, tok := range tokens {
		lower := strings.ToLower(tok)
		if f, unit, ok := parseDimension(lower); ok && (unit == "s" || unit == "ms") {
			t := formatDimension(f, unit)
			switch {
			case duration == "":
				duration = t
			case delay == "":
				delay = t
			default:
				return "", false
			}
			continue
		}
		if f, unit, ok := parseDimension(lower); ok && unit == "" && count == "" {
			count = FormatNumber(f)
			continue
		}
		isEasingFunction := strings.HasPrefix(lower, "cubic-bezier(") || strings.HasPrefix(lower, "steps(") || strings.HasPrefix(lower, "linear(")
		// A keyword goes to the first longhand that accepts it and has no
		// value yet. What is left is the name.
		switch {
		case easing == "" && (animationEasings[lower] || isEasingFunction):
			easing = normalizeToken(tok)
		case count == "" && lower == "infinite":
			count = lower
		case direction == "" && animationDirections[lower]:
			direction = lower
		case fill == "" && animationFillModes[lower]:
			fill = lower
		case state == "" && animationPlayStates[lower]:
			state = lower
		case name == "":
			name = tok
		default:
			return "", false
		}
	}
	or := func(v, def string) string {
		if v == "" {
			return def
		}
		return v
	}
	return strings.Join([]string{
		or(duration, "0s"), or(easing, "ease"), or(delay, "0s"), or(count, "1"),
		or(direction, "normal"), or(fill, "none"), or(state, "running"), or(name, "none"),
	}, " "), true
}
