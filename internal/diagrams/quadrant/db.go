// Ported from mermaid 12.2.0, diagrams/quadrant-chart/quadrantDb.ts (MIT).

package quadrant

import (
	"errors"
	"regexp"
	"strconv"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// DB is the db of the quadrant chart.
type DB struct {
	render.CommonDB
	ctx             *render.Context
	quadrantBuilder *QuadrantBuilder
}

func newDB(ctx *render.Context) *DB {
	db := &DB{ctx: ctx, quadrantBuilder: NewQuadrantBuilder()}
	db.Sanitize = ctx.SanitizeText
	return db
}

// textSanitizer is textSanitizer(text).
func (db *DB) textSanitizer(text string) string {
	return db.ctx.SanitizeText(js.TrimSpace(text))
}

// SetQuadrant1Text is setQuadrant1Text(textObj).
func (db *DB) SetQuadrant1Text(textObj *lexTextObj) {
	db.quadrantBuilder.data.Quadrant1Text = db.textSanitizer(textObj.text)
}

// SetQuadrant2Text is setQuadrant2Text(textObj).
func (db *DB) SetQuadrant2Text(textObj *lexTextObj) {
	db.quadrantBuilder.data.Quadrant2Text = db.textSanitizer(textObj.text)
}

// SetQuadrant3Text is setQuadrant3Text(textObj).
func (db *DB) SetQuadrant3Text(textObj *lexTextObj) {
	db.quadrantBuilder.data.Quadrant3Text = db.textSanitizer(textObj.text)
}

// SetQuadrant4Text is setQuadrant4Text(textObj).
func (db *DB) SetQuadrant4Text(textObj *lexTextObj) {
	db.quadrantBuilder.data.Quadrant4Text = db.textSanitizer(textObj.text)
}

// SetXAxisLeftText is setXAxisLeftText(textObj).
func (db *DB) SetXAxisLeftText(textObj *lexTextObj) {
	db.quadrantBuilder.data.XAxisLeftText = db.textSanitizer(textObj.text)
}

// SetXAxisRightText is setXAxisRightText(textObj).
func (db *DB) SetXAxisRightText(textObj *lexTextObj) {
	db.quadrantBuilder.data.XAxisRightText = db.textSanitizer(textObj.text)
}

// SetYAxisTopText is setYAxisTopText(textObj).
func (db *DB) SetYAxisTopText(textObj *lexTextObj) {
	db.quadrantBuilder.data.YAxisTopText = db.textSanitizer(textObj.text)
}

// SetYAxisBottomText is setYAxisBottomText(textObj).
func (db *DB) SetYAxisBottomText(textObj *lexTextObj) {
	db.quadrantBuilder.data.YAxisBottomText = db.textSanitizer(textObj.text)
}

// reStyleSeparator is /\s*:\s*/ with the white space of JavaScript.
var reStyleSeparator = regexp.MustCompile(jsSpace + `*:` + jsSpace + `*`)

// jsSpace is the class \s of a JavaScript regular expression.
const jsSpace = `[\t\n\v\f\r \x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}]`

// ParseStyles is parseStyles(styles).
func (db *DB) ParseStyles(styles []string) (StylesObject, error) {
	var stylesObject StylesObject
	for _, style := range styles {
		parts := reStyleSeparator.Split(js.TrimSpace(style), -1)
		key := parts[0]
		// value is undefined when the style has no colon. The validators
		// then test the text "undefined".
		value := "undefined"
		if len(parts) > 1 {
			value = parts[1]
		}
		switch key {
		case "radius":
			if validateNumber(value) {
				return stylesObject, invalidStyleError(key, value, "number")
			}
			// parseInt of a text of digits.
			radius, _ := strconv.ParseFloat(value, 64)
			stylesObject.Radius = &radius
		case "color":
			if validateHexCode(value) {
				return stylesObject, invalidStyleError(key, value, "hex code")
			}
			v := value
			stylesObject.Color = &v
		case "stroke-color":
			if validateHexCode(value) {
				return stylesObject, invalidStyleError(key, value, "hex code")
			}
			v := value
			stylesObject.StrokeColor = &v
		case "stroke-width":
			if validateSizeInPixels(value) {
				return stylesObject, invalidStyleError(key, value, "number of pixels (eg. 10px)")
			}
			v := value
			stylesObject.StrokeWidth = &v
		default:
			return stylesObject, errors.New("style named " + key + " is not supported.")
		}
	}
	return stylesObject, nil
}

// AddPoint is addPoint(textObj, className, x, y, styles). x and y are the
// texts of the tokens: the builder converts them to numbers.
func (db *DB) AddPoint(textObj *lexTextObj, className, x, y string, styles []string) error {
	stylesObject, err := db.ParseStyles(styles)
	if err != nil {
		return err
	}
	db.quadrantBuilder.AddPoints([]QuadrantPointInputType{{
		X:            x,
		Y:            y,
		Text:         db.textSanitizer(textObj.text),
		ClassName:    className,
		StylesObject: stylesObject,
	}})
	return nil
}

// AddClass is addClass(className, styles).
func (db *DB) AddClass(className string, styles []string) error {
	stylesObject, err := db.ParseStyles(styles)
	if err != nil {
		return err
	}
	db.quadrantBuilder.AddClass(className, stylesObject)
	return nil
}

// SetWidth is setWidth(width).
func (db *DB) SetWidth(width any) {
	db.quadrantBuilder.config.Set("chartWidth", width)
}

// SetHeight is setHeight(height).
func (db *DB) SetHeight(height any) {
	db.quadrantBuilder.config.Set("chartHeight", height)
}

// themeConfigKeys are the theme variables that getQuadrantData gives to the
// builder, in the order of upstream.
var themeConfigKeys = []string{
	"quadrant1Fill",
	"quadrant2Fill",
	"quadrant3Fill",
	"quadrant4Fill",
	"quadrant1TextFill",
	"quadrant2TextFill",
	"quadrant3TextFill",
	"quadrant4TextFill",
	"quadrantPointFill",
	"quadrantPointTextFill",
	"quadrantXAxisTextFill",
	"quadrantYAxisTextFill",
	"quadrantExternalBorderStrokeFill",
	"quadrantInternalBorderStrokeFill",
	"quadrantTitleFill",
}

// GetQuadrantData is getQuadrantData().
func (db *DB) GetQuadrantData() QuadrantBuildType {
	config := db.ctx.Config
	if quadrantChartConfig := config.Get("quadrantChart"); js.Truthy(quadrantChartConfig) {
		db.quadrantBuilder.SetConfig(js.AsObj(quadrantChartConfig))
	}
	themeConfig := js.NewObj[any]()
	for _, key := range themeConfigKeys {
		themeConfig.Set(key, js.Path(config, "themeVariables", key))
	}
	db.quadrantBuilder.SetThemeConfig(themeConfig)
	db.quadrantBuilder.data.TitleText = db.GetDiagramTitle()
	return db.quadrantBuilder.Build()
}
