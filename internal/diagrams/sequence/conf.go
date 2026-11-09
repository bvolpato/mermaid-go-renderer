// Ported from mermaid 12.2.0, diagrams/sequence/sequenceRenderer.ts: the
// module variable `conf`, setConf and the font helpers (MIT).

package sequence

import (
	"errors"
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

// conf is the module variable `conf` of sequenceRenderer.ts: the sequence
// section of the config, with the whole config merged into it by setConf.
// The fields are the properties that the renderer and svgDraw.js read. obj
// is the object itself.
//
// A numeric property that is undefined is NaN, as in the arithmetic of
// upstream. A numeric property that is a string is converted to a number:
// upstream would concatenate it in some expressions.
type conf struct {
	obj *js.Obj[any]

	boxMargin, boxTextMargin, noteMargin float64
	width, height, actorMargin           float64
	activationWidth                      float64
	diagramMarginX, diagramMarginY       float64
	wrapPadding                          float64
	labelBoxWidth, labelBoxHeight        float64
	// hasLabelBoxWidth is false when labelBoxWidth is null or undefined.
	hasLabelBoxWidth bool
	bottomMarginAdj  float64

	mirrorActors, rightAngles, showSequenceNumbers bool
	forceMenus, useMaxWidth, arrowMarkerAbsolute   bool
	// hideUnusedParticipants is `conf.hideUnusedParticipants === true`.
	hideUnusedParticipants bool

	// messageAlign and noteAlign are strings, or nil for undefined.
	messageAlign, noteAlign any
	textPlacement           any

	actorFontFamily, actorFontSize, actorFontWeight       any
	noteFontFamily, noteFontSize, noteFontWeight          any
	messageFontFamily, messageFontSize, messageFontWeight any

	// look and theme are the properties of conf. setConf copies them from
	// the whole config.
	look, theme    any
	themeVariables *js.Obj[any]
	// sequenceLabelBoxHeight is conf?.sequence?.labelBoxHeight ?? 0.
	sequenceLabelBoxHeight float64
}

// num reads a numeric property.
func num(obj *js.Obj[any], key string) float64 {
	v := obj.Get(key)
	if v == nil {
		return math.NaN()
	}
	return js.ToNumber(v)
}

// undef maps null to nil: both are `== null` upstream. It is for the values
// that upstream tests with `!== undefined`, where null behaves as a value
// that d3 ignores.
func undef(v any) any {
	if js.IsNullish(v) {
		return nil
	}
	return v
}

// setConf is setConf(cnf), followed by the read of the properties.
func setConf(c *conf, cnf *js.Obj[any]) error {
	if c.obj == nil {
		return errors.New("TypeError: Cannot set properties of undefined (setting 'actorFontFamily')")
	}
	config.AssignWithDepth(c.obj, cnf, 2)

	if v := cnf.Get("fontFamily"); js.Truthy(v) {
		c.obj.Set("actorFontFamily", v)
		c.obj.Set("noteFontFamily", v)
		c.obj.Set("messageFontFamily", v)
	}
	if v := cnf.Get("fontSize"); js.Truthy(v) {
		c.obj.Set("actorFontSize", v)
		c.obj.Set("noteFontSize", v)
		c.obj.Set("messageFontSize", v)
	}
	if v := cnf.Get("fontWeight"); js.Truthy(v) {
		c.obj.Set("actorFontWeight", v)
		c.obj.Set("noteFontWeight", v)
		c.obj.Set("messageFontWeight", v)
	}
	c.read()
	return nil
}

// read copies the properties of the object into the fields.
func (c *conf) read() {
	o := c.obj
	c.boxMargin = num(o, "boxMargin")
	c.boxTextMargin = num(o, "boxTextMargin")
	c.noteMargin = num(o, "noteMargin")
	c.width = num(o, "width")
	c.height = num(o, "height")
	c.actorMargin = num(o, "actorMargin")
	c.activationWidth = num(o, "activationWidth")
	c.diagramMarginX = num(o, "diagramMarginX")
	c.diagramMarginY = num(o, "diagramMarginY")
	c.wrapPadding = num(o, "wrapPadding")
	c.labelBoxWidth = num(o, "labelBoxWidth")
	c.hasLabelBoxWidth = !js.IsNullish(o.Get("labelBoxWidth"))
	c.labelBoxHeight = num(o, "labelBoxHeight")
	c.bottomMarginAdj = num(o, "bottomMarginAdj")

	c.mirrorActors = js.Truthy(o.Get("mirrorActors"))
	c.rightAngles = js.Truthy(o.Get("rightAngles"))
	c.showSequenceNumbers = js.Truthy(o.Get("showSequenceNumbers"))
	c.forceMenus = js.Truthy(o.Get("forceMenus"))
	c.useMaxWidth = js.Truthy(o.Get("useMaxWidth"))
	c.arrowMarkerAbsolute = js.Truthy(o.Get("arrowMarkerAbsolute"))
	c.hideUnusedParticipants = o.Get("hideUnusedParticipants") == true

	c.messageAlign = o.Get("messageAlign")
	c.noteAlign = o.Get("noteAlign")
	c.textPlacement = o.Get("textPlacement")

	c.actorFontFamily = o.Get("actorFontFamily")
	c.actorFontSize = o.Get("actorFontSize")
	c.actorFontWeight = o.Get("actorFontWeight")
	c.noteFontFamily = o.Get("noteFontFamily")
	c.noteFontSize = o.Get("noteFontSize")
	c.noteFontWeight = o.Get("noteFontWeight")
	c.messageFontFamily = o.Get("messageFontFamily")
	c.messageFontSize = o.Get("messageFontSize")
	c.messageFontWeight = o.Get("messageFontWeight")

	c.look = o.Get("look")
	c.theme = o.Get("theme")
	c.themeVariables = js.AsObj(o.Get("themeVariables"))
	c.sequenceLabelBoxHeight = 0
	if v := js.Path(o, "sequence", "labelBoxHeight"); !js.IsNullish(v) {
		c.sequenceLabelBoxHeight = js.ToNumber(v)
	}
}

// isNeo is `conf.look === 'neo'`.
func (c *conf) isNeo() bool { return c.look == "neo" }

// themeVar reads a theme variable of conf.themeVariables.
func (c *conf) themeVar(key string) any {
	if c.themeVariables == nil {
		return nil
	}
	return c.themeVariables.Get(key)
}

// messageFont is messageFont(cnf).
func (c *conf) messageFont() utils.TextDimensionConfig {
	return utils.TextDimensionConfig{
		FontFamily: c.messageFontFamily,
		FontSize:   c.messageFontSize,
		FontWeight: c.messageFontWeight,
	}
}

// noteFont is noteFont(cnf).
func (c *conf) noteFont() utils.TextDimensionConfig {
	return utils.TextDimensionConfig{
		FontFamily: c.noteFontFamily,
		FontSize:   c.noteFontSize,
		FontWeight: c.noteFontWeight,
	}
}

// actorFont is actorFont(cnf).
func (c *conf) actorFont() utils.TextDimensionConfig {
	return utils.TextDimensionConfig{
		FontFamily: c.actorFontFamily,
		FontSize:   c.actorFontSize,
		FontWeight: c.actorFontWeight,
	}
}
