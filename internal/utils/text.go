// Ported from mermaid 12.2.0, mermaid/src/utils.ts: getTextObj,
// drawSimpleText, wrapLabel, breakString, calculateTextHeight,
// calculateTextWidth and calculateTextDimensions (MIT).

package utils

import (
	"errors"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// TextObj is the object that getTextObj of utils.ts returns, and the
// textData argument of drawSimpleText.
//
// The fields of type any are dynamic values: nil is undefined. FontSize
// and FontWeight are a number (float64 or int), a string or nil.
type TextObj struct {
	X, Y          float64
	Fill          any
	Anchor        string
	Style         string
	Width, Height float64
	TextMargin    float64
	Rx, Ry        float64
	Valign        any
	Text          string

	// getTextObj does not set the fields below. drawSimpleText reads them.
	FontFamily any
	FontSize   any
	FontWeight any
	// Class is the class attribute. Nil (undefined) writes no attribute.
	Class any
}

// GetTextObj is getTextObj() of utils.ts. The function with the same name
// in svgDrawCommon.ts is svgdraw.GetTextObj.
func GetTextObj() TextObj {
	return TextObj{
		X:          0,
		Y:          0,
		Fill:       nil,
		Anchor:     "start",
		Style:      "#666",
		Width:      100,
		Height:     100,
		TextMargin: 0,
		Rx:         0,
		Ry:         0,
		Valign:     nil,
		Text:       "",
	}
}

// nullish maps undefined and null to nil, which removes an attribute or a
// style property, as d3 does for a value == null.
func nullish(v any) any {
	if js.IsNullish(v) {
		return nil
	}
	return v
}

// DrawSimpleText adds text to an element. It returns the text element with
// the given styling and content.
func DrawSimpleText(elem *dom.Node, textData TextObj) *dom.Node {
	// Remove and ignore br:s
	nText := lineBreakRegex.ReplaceAllLiteralString(textData.Text, " ")

	var fontSizePx any
	if _, px, ok := ParseFontSize(textData.FontSize); ok {
		fontSizePx = px
	}

	textElem := elem.Append("text")
	textElem.Attr("x", textData.X)
	textElem.Attr("y", textData.Y)
	textElem.Style("text-anchor", textData.Anchor)
	textElem.Style("font-family", nullish(textData.FontFamily))
	textElem.Style("font-size", fontSizePx)
	textElem.Style("font-weight", nullish(textData.FontWeight))
	textElem.Attr("fill", nullish(textData.Fill))

	if textData.Class != nil {
		textElem.Attr("class", nullish(textData.Class))
	}

	span := textElem.Append("tspan")
	span.Attr("x", textData.X+float64(textData.TextMargin*2))
	span.Attr("fill", nullish(textData.Fill))
	span.Text(nText)

	return textElem
}

// TextDimensionConfig is the TextDimensionConfig interface: the font of a
// text to measure. The values are dynamic, as they come from a config
// object: FontSize and FontWeight are a number (float64 or int), a string
// or nil (undefined), and FontFamily is a string or nil. The defaults of
// upstream for undefined are 12, 400 and "Arial".
type TextDimensionConfig struct {
	FontSize   any
	FontWeight any
	FontFamily any
}

// TextDimensionConfigOf reads the font of a text from an object that has
// the properties fontSize, fontWeight and fontFamily. Upstream passes such
// an object, for example the whole config, where a TextDimensionConfig is
// expected.
func TextDimensionConfigOf(obj *js.Obj[any]) TextDimensionConfig {
	if obj == nil {
		return TextDimensionConfig{}
	}
	return TextDimensionConfig{
		FontSize:   obj.Get("fontSize"),
		FontWeight: obj.Get("fontWeight"),
		FontFamily: obj.Get("fontFamily"),
	}
}

// WrapLabelConfig is the WrapLabelConfig interface. The font fields are as
// in TextDimensionConfig. JoinWith is the text between the lines; nil is an
// object without the property, which gives the default "<br/>".
type WrapLabelConfig struct {
	FontSize   any
	FontWeight any
	FontFamily any
	JoinWith   any
}

// WrapLabelConfigOf reads a WrapLabelConfig from an object that has the
// properties fontSize, fontWeight, fontFamily and joinWith.
func WrapLabelConfigOf(obj *js.Obj[any]) WrapLabelConfig {
	if obj == nil {
		return WrapLabelConfig{}
	}
	return WrapLabelConfig{
		FontSize:   obj.Get("fontSize"),
		FontWeight: obj.Get("fontWeight"),
		FontFamily: obj.Get("fontFamily"),
		JoinWith:   obj.Get("joinWith"),
	}
}

// TextDimensionConfig returns the font fields of the config.
func (c WrapLabelConfig) TextDimensionConfig() TextDimensionConfig {
	return TextDimensionConfig{FontSize: c.FontSize, FontWeight: c.FontWeight, FontFamily: c.FontFamily}
}

// The memoize caches of upstream are module-level, so they live as long as
// the page. Each render has its own caches in the context. A cache key is
// the text that the resolver function of upstream makes, so two calls with
// different arguments and the same key text share a result, as upstream.
const (
	wrapLabelCache               = "utils.wrapLabel.cache"
	breakStringCache             = "utils.breakString.cache"
	calculateTextDimensionsCache = "utils.calculateTextDimensions.cache"
)

func memoCache[V any](ctx *render.Context, name string) map[string]V {
	return ctx.Value(name, func() any { return map[string]V{} }).(map[string]V)
}

// fontKey is `${config.fontSize}${config.fontWeight}${config.fontFamily}`.
func fontKey(fontSize, fontWeight, fontFamily any) string {
	return js.ToString(fontSize) + js.ToString(fontWeight) + js.ToString(fontFamily)
}

// WrapLabel is wrapLabel(label, maxWidth, config): it breaks the label into
// lines that are not wider than maxWidth, and joins them with
// config.JoinWith. The result is memoized for the render, as upstream.
func WrapLabel(ctx *render.Context, label string, maxWidth float64, config WrapLabelConfig) (string, error) {
	cache := memoCache[string](ctx, wrapLabelCache)
	key := label + js.Str(maxWidth) + fontKey(config.FontSize, config.FontWeight, config.FontFamily) + js.ToString(config.JoinWith)
	if result, ok := cache[key]; ok {
		return result, nil
	}
	result, err := wrapLabel(ctx, label, maxWidth, config)
	if err != nil {
		return "", err
	}
	cache[key] = result
	return result, nil
}

func wrapLabel(ctx *render.Context, label string, maxWidth float64, config WrapLabelConfig) (string, error) {
	if label == "" {
		return label, nil
	}
	// Object.assign({ fontSize: 12, fontWeight: 400, fontFamily: 'Arial',
	// joinWith: '<br/>' }, config): the font fields of a config are own
	// properties, so the font defaults never apply here. They apply in
	// calculateTextDimensions.
	joinWith := "<br/>"
	if config.JoinWith != nil {
		joinWith = js.ToString(config.JoinWith)
	}
	textConfig := config.TextDimensionConfig()
	if hasBreaks(label) {
		return label, nil
	}
	var words []string
	for _, word := range strings.Split(label, " ") {
		if word != "" {
			words = append(words, word)
		}
	}
	var completedLines []string
	nextLine := ""
	for index, word := range words {
		wordLength, err := CalculateTextWidth(ctx, word+" ", textConfig)
		if err != nil {
			return "", err
		}
		nextLineLength, err := CalculateTextWidth(ctx, nextLine, textConfig)
		if err != nil {
			return "", err
		}
		if wordLength > maxWidth {
			broken, err := BreakString(ctx, word, maxWidth, "-", config)
			if err != nil {
				return "", err
			}
			completedLines = append(completedLines, nextLine)
			completedLines = append(completedLines, broken.HyphenatedStrings...)
			nextLine = broken.RemainingWord
		} else if nextLineLength+wordLength >= maxWidth {
			completedLines = append(completedLines, nextLine)
			nextLine = word
		} else {
			if nextLine != "" {
				nextLine = nextLine + " " + word
			} else {
				nextLine = word
			}
		}
		currentWord := index + 1
		isLastWord := currentWord == len(words)
		if isLastWord {
			completedLines = append(completedLines, nextLine)
		}
	}
	lines := completedLines[:0:0]
	for _, line := range completedLines {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, joinWith), nil
}

// BreakStringOutput is the BreakStringOutput interface.
type BreakStringOutput struct {
	HyphenatedStrings []string
	RemainingWord     string
}

// BreakString is breakString(word, maxWidth, hyphenCharacter, config): it
// breaks a word into lines that are narrower than maxWidth. Upstream does
// not export it, and the default of hyphenCharacter is "-". The result is
// memoized for the render, as upstream. Do not change the slice of the
// result.
func BreakString(ctx *render.Context, word string, maxWidth float64, hyphenCharacter string, config WrapLabelConfig) (BreakStringOutput, error) {
	cache := memoCache[BreakStringOutput](ctx, breakStringCache)
	key := word + js.Str(maxWidth) + hyphenCharacter + fontKey(config.FontSize, config.FontWeight, config.FontFamily)
	if result, ok := cache[key]; ok {
		return result, nil
	}
	result, err := breakString(ctx, word, maxWidth, hyphenCharacter, config)
	if err != nil {
		return BreakStringOutput{}, err
	}
	cache[key] = result
	return result, nil
}

func breakString(ctx *render.Context, word string, maxWidth float64, hyphenCharacter string, config WrapLabelConfig) (BreakStringOutput, error) {
	textConfig := config.TextDimensionConfig()
	characters := codePoints(word)
	lines := []string{}
	currentLine := ""
	for index, character := range characters {
		nextLine := currentLine + character
		lineWidth, err := CalculateTextWidth(ctx, nextLine, textConfig)
		if err != nil {
			return BreakStringOutput{}, err
		}
		if lineWidth >= maxWidth {
			currentCharacter := index + 1
			isLastLine := len(characters) == currentCharacter
			hyphenatedNextLine := nextLine + hyphenCharacter
			if isLastLine {
				lines = append(lines, nextLine)
			} else {
				lines = append(lines, hyphenatedNextLine)
			}
			currentLine = ""
		} else {
			currentLine = nextLine
		}
	}
	return BreakStringOutput{HyphenatedStrings: lines, RemainingWord: currentLine}, nil
}

// codePoints is [...s]: the string split into code points. A lone surrogate
// in the three-byte form of internal/js is one element.
func codePoints(s string) []string {
	out := make([]string, 0, len(s))
	for i := 0; i < len(s); {
		_, size := utf8.DecodeRuneInString(s[i:])
		if size == 1 && s[i] == 0xed && i+2 < len(s) && s[i+1] >= 0xa0 && s[i+1] <= 0xbf && s[i+2]&0xc0 == 0x80 {
			size = 3
		}
		out = append(out, s[i:i+size])
		i += size
	}
	return out
}

// CalculateTextHeight calculates the height of the given text. It takes the
// line breaks into account.
func CalculateTextHeight(ctx *render.Context, text string, config TextDimensionConfig) (float64, error) {
	dim, err := CalculateTextDimensions(ctx, text, config)
	return dim.Height, err
}

// CalculateTextWidth calculates the width of the given text, font size and
// family.
func CalculateTextWidth(ctx *render.Context, text string, config TextDimensionConfig) (float64, error) {
	dim, err := CalculateTextDimensions(ctx, text, config)
	return dim.Width, err
}

// TextDimensions is the TextDimensions interface. HasLineHeight is false
// for the result of an empty text, which has no lineHeight upstream.
type TextDimensions struct {
	Width         float64
	Height        float64
	LineHeight    float64
	HasLineHeight bool
}

// errNotInRenderTree is the error that calculateTextDimensions throws.
var errNotInRenderTree = errors.New("svg element not in render tree")

// CalculateTextDimensions calculates the dimensions of the given text, font
// size, font family and font weight. It measures in a temporary svg element
// that it appends to the body, so ctx.Body must be in a document with an
// engine. The result is memoized for the render, as upstream.
func CalculateTextDimensions(ctx *render.Context, text string, config TextDimensionConfig) (TextDimensions, error) {
	cache := memoCache[TextDimensions](ctx, calculateTextDimensionsCache)
	key := text + fontKey(config.FontSize, config.FontWeight, config.FontFamily)
	if result, ok := cache[key]; ok {
		return result, nil
	}
	result, err := calculateTextDimensions(ctx, text, config)
	if err != nil {
		return TextDimensions{}, err
	}
	cache[key] = result
	return result, nil
}

func calculateTextDimensions(ctx *render.Context, text string, config TextDimensionConfig) (TextDimensions, error) {
	// const { fontSize = 12, fontFamily = 'Arial', fontWeight = 400 } = config;
	var fontSize, fontFamily, fontWeight any = config.FontSize, config.FontFamily, config.FontWeight
	if fontSize == nil {
		fontSize = 12.0
	}
	if fontFamily == nil {
		fontFamily = "Arial"
	}
	if fontWeight == nil {
		fontWeight = 400.0
	}
	if text == "" {
		return TextDimensions{Width: 0, Height: 0}, nil
	}

	var fontSizePx any
	if _, px, ok := ParseFontSize(fontSize); ok {
		fontSizePx = px
	}

	// We can't really know if the user supplied font family will render on the user agent;
	// thus, we'll take the max width between the user supplied font family, and a default
	// of sans-serif.
	fontFamilies := [2]any{"sans-serif", fontFamily}
	lines := lineBreakRegex.Split(text, -1)
	var dims [2]TextDimensions

	body := ctx.Body
	// We don't want to leak DOM elements - if a removal operation isn't available
	// for any reason, do not continue.
	if body == nil {
		return TextDimensions{Width: 0, Height: 0, LineHeight: 0, HasLineHeight: true}, nil
	}

	g := body.Append("svg")

	for i, fontFamily := range fontFamilies {
		cHeight := 0.0
		dim := TextDimensions{Width: 0, Height: 0, LineHeight: 0, HasLineHeight: true}
		for _, line := range lines {
			textObj := GetTextObj()
			if line != "" {
				textObj.Text = line
			} else {
				textObj.Text = ZeroWidthSpace
			}
			textElem := DrawSimpleText(g, textObj).
				Style("font-size", fontSizePx).
				Style("font-weight", nullish(fontWeight)).
				Style("font-family", nullish(fontFamily))

			bBox := textElem.BBox()
			if bBox.Width == 0 && bBox.Height == 0 {
				// Upstream throws here, and the temporary svg stays in the body.
				return TextDimensions{}, errNotInRenderTree
			}
			dim.Width = js.Round(js.Max(dim.Width, bBox.Width))
			cHeight = js.Round(bBox.Height)
			dim.Height += cHeight
			dim.LineHeight = js.Round(js.Max(dim.LineHeight, cHeight))
		}
		dims[i] = dim
	}

	g.Remove()

	index := 1
	if math.IsNaN(dims[1].Height) ||
		math.IsNaN(dims[1].Width) ||
		math.IsNaN(dims[1].LineHeight) ||
		(dims[0].Height > dims[1].Height &&
			dims[0].Width > dims[1].Width &&
			dims[0].LineHeight > dims[1].LineHeight) {
		index = 0
	}
	return dims[index], nil
}
