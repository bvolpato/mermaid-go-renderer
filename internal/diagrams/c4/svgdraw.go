// Ported from mermaid 12.2.0, diagrams/c4/svgDraw.ts (MIT).

package c4

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
	"github.com/bvolpato/mermaid-go-renderer/internal/svgdraw"
)

// textAttr is one entry of the TextAttrs record of a text.
type textAttr struct {
	name  string
	value any
}

// c4Font is C4Font: the font of a text. The values are dynamic, as they
// come from the config: a number, a string or nil (undefined).
type c4Font struct {
	fontFamily any
	fontSize   any
	fontWeight any
	fontColor  any
}

// drawTextFunction is DrawTextFunction.
type drawTextFunction func(content string, g *dom.Node, x, y, width, height float64, textAttrs []textAttr, conf c4Font)

// relPoint reads the start point or the end point of a relationship.
func relPoint(rel *js.Obj[any], key string) ru.Point {
	p, _ := rel.Get(key).(ru.Point)
	return p
}

// orDefault is `value ? value : def` for a colour of a bag.
func orDefault(value any, def string) any {
	if js.Truthy(value) {
		return value
	}
	return def
}

// drawRels is drawRels(elem, rels, conf, diagramId).
func (r *renderer) svgDrawRels(elem *dom.Node, rels []*js.Obj[any], diagramID string) error {
	conf := r.rs.conf
	relsElem := elem.Append("g")
	i := 0
	for _, rel := range rels {
		textColor := orDefault(rel.Get("textColor"), "#444444")
		strokeColor := orDefault(rel.Get("lineColor"), "#444444")
		// The parseInt is load-bearing: the JISON grammar passes $key="value" attributes
		// positionally, so offsets that arrive in the textColor/lineColor parameter slots are
		// stored by the db as raw strings and must be coerced here before coordinate arithmetic.
		offsetX := 0.0
		if v := rel.Get("offsetX"); js.Truthy(v) {
			offsetX = js.ParseIntAuto(js.ToString(v))
		}
		offsetY := 0.0
		if v := rel.Get("offsetY"); js.Truthy(v) {
			offsetY = js.ParseIntAuto(js.ToString(v))
		}

		startPoint := relPoint(rel, "startPoint")
		endPoint := relPoint(rel, "endPoint")
		relType := rel.Get("type")

		const url = ""
		if i == 0 {
			line := relsElem.Append("line")
			line.Attr("x1", startPoint.X)
			line.Attr("y1", startPoint.Y)
			line.Attr("x2", endPoint.X)
			line.Attr("y2", endPoint.Y)

			line.Attr("stroke-width", "1")
			line.Attr("stroke", strokeColor)
			line.Style("fill", "none")
			if relType != "rel_b" {
				line.Attr("marker-end", "url("+url+"#"+diagramID+"-arrowhead)")
			}
			if relType == "birel" || relType == "rel_b" {
				line.Attr("marker-start", "url("+url+"#"+diagramID+"-arrowend)")
			}
			i = -1
		} else {
			line := relsElem.Append("path")
			n := js.Str
			line.
				Attr("fill", "none").
				Attr("stroke-width", "1").
				Attr("stroke", strokeColor).
				Attr(
					"d",
					// 'Mstartx,starty Qcontrolx,controly stopx,stopy ' with
					// each name replaced by its number.
					"M"+n(startPoint.X)+","+n(startPoint.Y)+
						" Q"+n(startPoint.X+
						float64((endPoint.X-startPoint.X)/2)-
						float64((endPoint.X-startPoint.X)/4))+
						","+n(startPoint.Y+float64((endPoint.Y-startPoint.Y)/2))+
						" "+n(endPoint.X)+","+n(endPoint.Y)+" ",
				)
			if relType != "rel_b" {
				line.Attr("marker-end", "url("+url+"#"+diagramID+"-arrowhead)")
			}
			if relType == "birel" || relType == "rel_b" {
				line.Attr("marker-start", "url("+url+"#"+diagramID+"-arrowend)")
			}
		}

		// The label was measured by the renderer before drawing.
		label := js.AsObj(rel.Get("label"))
		labelWidth := num(label, "width")
		messageConf := r.messageFontOf()
		r.drawTextCandidateFunc()(
			js.ToString(js.Path(label, "text")),
			relsElem,
			js.Min(startPoint.X, endPoint.X)+
				float64(math.Abs(endPoint.X-startPoint.X)/2)+
				offsetX,
			js.Min(startPoint.Y, endPoint.Y)+
				float64(math.Abs(endPoint.Y-startPoint.Y)/2)+
				offsetY,
			labelWidth,
			num(label, "height"),
			[]textAttr{{"fill", textColor}},
			messageConf,
		)

		if techn := rel.Get("techn"); js.Truthy(techn) && js.Path(techn, "text") != "" {
			technObj := js.AsObj(techn)
			messageConf = r.messageFontOf()
			r.drawTextCandidateFunc()(
				"["+js.ToString(js.Path(techn, "text"))+"]",
				relsElem,
				js.Min(startPoint.X, endPoint.X)+
					float64(math.Abs(endPoint.X-startPoint.X)/2)+
					offsetX,
				js.Min(startPoint.Y, endPoint.Y)+
					float64(math.Abs(endPoint.Y-startPoint.Y)/2)+
					js.ToNumber(conf.Get("messageFontSize"))+
					5+
					offsetY,
				js.Max(labelWidth, num(technObj, "width")),
				num(technObj, "height"),
				[]textAttr{{"fill", textColor}, {"font-style", "italic"}},
				messageConf,
			)
		}
	}
	return nil
}

// svgDrawBoundary draws a boundary in the diagram: drawBoundary of
// svgDraw.ts.
func (r *renderer) svgDrawBoundary(elem *dom.Node, boundary *js.Obj[any]) {
	boundaryElem := elem.Append("g")

	fillColor := orDefault(boundary.Get("bgColor"), "none")
	strokeColor := orDefault(boundary.Get("borderColor"), "#444444")
	fontColor := orDefault(boundary.Get("fontColor"), "black")

	attrsValue := js.NewObj[any]()
	attrsValue.Set("stroke-width", 1.0)
	if !js.Truthy(boundary.Get("nodeType")) {
		attrsValue.Set("stroke-dasharray", "7.0,7.0")
	}
	x := num(boundary, "x")
	y := num(boundary, "y")
	width := num(boundary, "width")
	height := num(boundary, "height")
	rectData := svgdraw.RectData{
		X:      x,
		Y:      y,
		Fill:   fillColor,
		Stroke: strokeColor,
		Width:  width,
		Height: height,
		Rx:     2.5,
		Ry:     2.5,
		Attrs:  attrsValue,
	}

	svgdraw.DrawRect(boundaryElem, rectData)

	// draw label
	boundaryConf := r.boundaryFontOf()
	boundaryConf.fontWeight = "bold"
	boundaryConf.fontSize = js.Add(boundaryConf.fontSize, 2.0)
	boundaryConf.fontColor = fontColor
	label := js.AsObj(boundary.Get("label"))
	r.drawTextCandidateFunc()(
		js.ToString(js.Path(label, "text")),
		boundaryElem,
		x,
		y+num(label, "Y"),
		width,
		height,
		[]textAttr{{"fill", "#444444"}},
		boundaryConf,
	)

	// draw type
	if typ := boundary.Get("type"); js.Truthy(typ) && js.Path(typ, "text") != "" {
		boundaryConf = r.boundaryFontOf()
		boundaryConf.fontColor = fontColor
		r.drawTextCandidateFunc()(
			js.ToString(js.Path(typ, "text")),
			boundaryElem,
			x,
			y+num(js.AsObj(typ), "Y"),
			width,
			height,
			[]textAttr{{"fill", "#444444"}},
			boundaryConf,
		)
	}

	// draw descr
	if descr := boundary.Get("descr"); js.Truthy(descr) && js.Path(descr, "text") != "" {
		boundaryConf = r.boundaryFontOf()
		boundaryConf.fontSize = js.ToNumber(boundaryConf.fontSize) - 2
		boundaryConf.fontColor = fontColor
		r.drawTextCandidateFunc()(
			js.ToString(js.Path(descr, "text")),
			boundaryElem,
			x,
			y+num(js.AsObj(descr), "Y"),
			width,
			height,
			[]textAttr{{"fill", "#444444"}},
			boundaryConf,
		)
	}
}

const databaseIconPath = "M12.258.001l.256.004.255.005.253.008.251.01.249.012.247.015.246.016.242.019.241.02.239.023.236.024.233.027.231.028.229.031.225.032.223.034.22.036.217.038.214.04.211.041.208.043.205.045.201.046.198.048.194.05.191.051.187.053.183.054.18.056.175.057.172.059.168.06.163.061.16.063.155.064.15.066.074.033.073.033.071.034.07.034.069.035.068.035.067.035.066.035.064.036.064.036.062.036.06.036.06.037.058.037.058.037.055.038.055.038.053.038.052.038.051.039.05.039.048.039.047.039.045.04.044.04.043.04.041.04.04.041.039.041.037.041.036.041.034.041.033.042.032.042.03.042.029.042.027.042.026.043.024.043.023.043.021.043.02.043.018.044.017.043.015.044.013.044.012.044.011.045.009.044.007.045.006.045.004.045.002.045.001.045v17l-.001.045-.002.045-.004.045-.006.045-.007.045-.009.044-.011.045-.012.044-.013.044-.015.044-.017.043-.018.044-.02.043-.021.043-.023.043-.024.043-.026.043-.027.042-.029.042-.03.042-.032.042-.033.042-.034.041-.036.041-.037.041-.039.041-.04.041-.041.04-.043.04-.044.04-.045.04-.047.039-.048.039-.05.039-.051.039-.052.038-.053.038-.055.038-.055.038-.058.037-.058.037-.06.037-.06.036-.062.036-.064.036-.064.036-.066.035-.067.035-.068.035-.069.035-.07.034-.071.034-.073.033-.074.033-.15.066-.155.064-.16.063-.163.061-.168.06-.172.059-.175.057-.18.056-.183.054-.187.053-.191.051-.194.05-.198.048-.201.046-.205.045-.208.043-.211.041-.214.04-.217.038-.22.036-.223.034-.225.032-.229.031-.231.028-.233.027-.236.024-.239.023-.241.02-.242.019-.246.016-.247.015-.249.012-.251.01-.253.008-.255.005-.256.004-.258.001-.258-.001-.256-.004-.255-.005-.253-.008-.251-.01-.249-.012-.247-.015-.245-.016-.243-.019-.241-.02-.238-.023-.236-.024-.234-.027-.231-.028-.228-.031-.226-.032-.223-.034-.22-.036-.217-.038-.214-.04-.211-.041-.208-.043-.204-.045-.201-.046-.198-.048-.195-.05-.19-.051-.187-.053-.184-.054-.179-.056-.176-.057-.172-.059-.167-.06-.164-.061-.159-.063-.155-.064-.151-.066-.074-.033-.072-.033-.072-.034-.07-.034-.069-.035-.068-.035-.067-.035-.066-.035-.064-.036-.063-.036-.062-.036-.061-.036-.06-.037-.058-.037-.057-.037-.056-.038-.055-.038-.053-.038-.052-.038-.051-.039-.049-.039-.049-.039-.046-.039-.046-.04-.044-.04-.043-.04-.041-.04-.04-.041-.039-.041-.037-.041-.036-.041-.034-.041-.033-.042-.032-.042-.03-.042-.029-.042-.027-.042-.026-.043-.024-.043-.023-.043-.021-.043-.02-.043-.018-.044-.017-.043-.015-.044-.013-.044-.012-.044-.011-.045-.009-.044-.007-.045-.006-.045-.004-.045-.002-.045-.001-.045v-17l.001-.045.002-.045.004-.045.006-.045.007-.045.009-.044.011-.045.012-.044.013-.044.015-.044.017-.043.018-.044.02-.043.021-.043.023-.043.024-.043.026-.043.027-.042.029-.042.03-.042.032-.042.033-.042.034-.041.036-.041.037-.041.039-.041.04-.041.041-.04.043-.04.044-.04.046-.04.046-.039.049-.039.049-.039.051-.039.052-.038.053-.038.055-.038.056-.038.057-.037.058-.037.06-.037.061-.036.062-.036.063-.036.064-.036.066-.035.067-.035.068-.035.069-.035.07-.034.072-.034.072-.033.074-.033.151-.066.155-.064.159-.063.164-.061.167-.06.172-.059.176-.057.179-.056.184-.054.187-.053.19-.051.195-.05.198-.048.201-.046.204-.045.208-.043.211-.041.214-.04.217-.038.22-.036.223-.034.226-.032.228-.031.231-.028.234-.027.236-.024.238-.023.241-.02.243-.019.245-.016.247-.015.249-.012.251-.01.253-.008.255-.005.256-.004.258-.001.258.001zm-9.258 20.499v.01l.001.021.003.021.004.022.005.021.006.022.007.022.009.023.01.022.011.023.012.023.013.023.015.023.016.024.017.023.018.024.019.024.021.024.022.025.023.024.024.025.052.049.056.05.061.051.066.051.07.051.075.051.079.052.084.052.088.052.092.052.097.052.102.051.105.052.11.052.114.051.119.051.123.051.127.05.131.05.135.05.139.048.144.049.147.047.152.047.155.047.16.045.163.045.167.043.171.043.176.041.178.041.183.039.187.039.19.037.194.035.197.035.202.033.204.031.209.03.212.029.216.027.219.025.222.024.226.021.23.02.233.018.236.016.24.015.243.012.246.01.249.008.253.005.256.004.259.001.26-.001.257-.004.254-.005.25-.008.247-.011.244-.012.241-.014.237-.016.233-.018.231-.021.226-.021.224-.024.22-.026.216-.027.212-.028.21-.031.205-.031.202-.034.198-.034.194-.036.191-.037.187-.039.183-.04.179-.04.175-.042.172-.043.168-.044.163-.045.16-.046.155-.046.152-.047.148-.048.143-.049.139-.049.136-.05.131-.05.126-.05.123-.051.118-.052.114-.051.11-.052.106-.052.101-.052.096-.052.092-.052.088-.053.083-.051.079-.052.074-.052.07-.051.065-.051.06-.051.056-.05.051-.05.023-.024.023-.025.021-.024.02-.024.019-.024.018-.024.017-.024.015-.023.014-.024.013-.023.012-.023.01-.023.01-.022.008-.022.006-.022.006-.022.004-.022.004-.021.001-.021.001-.021v-4.127l-.077.055-.08.053-.083.054-.085.053-.087.052-.09.052-.093.051-.095.05-.097.05-.1.049-.102.049-.105.048-.106.047-.109.047-.111.046-.114.045-.115.045-.118.044-.12.043-.122.042-.124.042-.126.041-.128.04-.13.04-.132.038-.134.038-.135.037-.138.037-.139.035-.142.035-.143.034-.144.033-.147.032-.148.031-.15.03-.151.03-.153.029-.154.027-.156.027-.158.026-.159.025-.161.024-.162.023-.163.022-.165.021-.166.02-.167.019-.169.018-.169.017-.171.016-.173.015-.173.014-.175.013-.175.012-.177.011-.178.01-.179.008-.179.008-.181.006-.182.005-.182.004-.184.003-.184.002h-.37l-.184-.002-.184-.003-.182-.004-.182-.005-.181-.006-.179-.008-.179-.008-.178-.01-.176-.011-.176-.012-.175-.013-.173-.014-.172-.015-.171-.016-.17-.017-.169-.018-.167-.019-.166-.02-.165-.021-.163-.022-.162-.023-.161-.024-.159-.025-.157-.026-.156-.027-.155-.027-.153-.029-.151-.03-.15-.03-.148-.031-.146-.032-.145-.033-.143-.034-.141-.035-.14-.035-.137-.037-.136-.037-.134-.038-.132-.038-.13-.04-.128-.04-.126-.041-.124-.042-.122-.042-.12-.044-.117-.043-.116-.045-.113-.045-.112-.046-.109-.047-.106-.047-.105-.048-.102-.049-.1-.049-.097-.05-.095-.05-.093-.052-.09-.051-.087-.052-.085-.053-.083-.054-.08-.054-.077-.054v4.127zm0-5.654v.011l.001.021.003.021.004.021.005.022.006.022.007.022.009.022.01.022.011.023.012.023.013.023.015.024.016.023.017.024.018.024.019.024.021.024.022.024.023.025.024.024.052.05.056.05.061.05.066.051.07.051.075.052.079.051.084.052.088.052.092.052.097.052.102.052.105.052.11.051.114.051.119.052.123.05.127.051.131.05.135.049.139.049.144.048.147.048.152.047.155.046.16.045.163.045.167.044.171.042.176.042.178.04.183.04.187.038.19.037.194.036.197.034.202.033.204.032.209.03.212.028.216.027.219.025.222.024.226.022.23.02.233.018.236.016.24.014.243.012.246.01.249.008.253.006.256.003.259.001.26-.001.257-.003.254-.006.25-.008.247-.01.244-.012.241-.015.237-.016.233-.018.231-.02.226-.022.224-.024.22-.025.216-.027.212-.029.21-.03.205-.032.202-.033.198-.035.194-.036.191-.037.187-.039.183-.039.179-.041.175-.042.172-.043.168-.044.163-.045.16-.045.155-.047.152-.047.148-.048.143-.048.139-.05.136-.049.131-.05.126-.051.123-.051.118-.051.114-.052.11-.052.106-.052.101-.052.096-.052.092-.052.088-.052.083-.052.079-.052.074-.051.07-.052.065-.051.06-.05.056-.051.051-.049.023-.025.023-.024.021-.025.02-.024.019-.024.018-.024.017-.024.015-.023.014-.023.013-.024.012-.022.01-.023.01-.023.008-.022.006-.022.006-.022.004-.021.004-.022.001-.021.001-.021v-4.139l-.077.054-.08.054-.083.054-.085.052-.087.053-.09.051-.093.051-.095.051-.097.05-.1.049-.102.049-.105.048-.106.047-.109.047-.111.046-.114.045-.115.044-.118.044-.12.044-.122.042-.124.042-.126.041-.128.04-.13.039-.132.039-.134.038-.135.037-.138.036-.139.036-.142.035-.143.033-.144.033-.147.033-.148.031-.15.03-.151.03-.153.028-.154.028-.156.027-.158.026-.159.025-.161.024-.162.023-.163.022-.165.021-.166.02-.167.019-.169.018-.169.017-.171.016-.173.015-.173.014-.175.013-.175.012-.177.011-.178.009-.179.009-.179.007-.181.007-.182.005-.182.004-.184.003-.184.002h-.37l-.184-.002-.184-.003-.182-.004-.182-.005-.181-.007-.179-.007-.179-.009-.178-.009-.176-.011-.176-.012-.175-.013-.173-.014-.172-.015-.171-.016-.17-.017-.169-.018-.167-.019-.166-.02-.165-.021-.163-.022-.162-.023-.161-.024-.159-.025-.157-.026-.156-.027-.155-.028-.153-.028-.151-.03-.15-.03-.148-.031-.146-.033-.145-.033-.143-.033-.141-.035-.14-.036-.137-.036-.136-.037-.134-.038-.132-.039-.13-.039-.128-.04-.126-.041-.124-.042-.122-.043-.12-.043-.117-.044-.116-.044-.113-.046-.112-.046-.109-.046-.106-.047-.105-.048-.102-.049-.1-.049-.097-.05-.095-.051-.093-.051-.09-.051-.087-.053-.085-.052-.083-.054-.08-.054-.077-.054v4.139zm0-5.666v.011l.001.02.003.022.004.021.005.022.006.021.007.022.009.023.01.022.011.023.012.023.013.023.015.023.016.024.017.024.018.023.019.024.021.025.022.024.023.024.024.025.052.05.056.05.061.05.066.051.07.051.075.052.079.051.084.052.088.052.092.052.097.052.102.052.105.051.11.052.114.051.119.051.123.051.127.05.131.05.135.05.139.049.144.048.147.048.152.047.155.046.16.045.163.045.167.043.171.043.176.042.178.04.183.04.187.038.19.037.194.036.197.034.202.033.204.032.209.03.212.028.216.027.219.025.222.024.226.021.23.02.233.018.236.017.24.014.243.012.246.01.249.008.253.006.256.003.259.001.26-.001.257-.003.254-.006.25-.008.247-.01.244-.013.241-.014.237-.016.233-.018.231-.02.226-.022.224-.024.22-.025.216-.027.212-.029.21-.03.205-.032.202-.033.198-.035.194-.036.191-.037.187-.039.183-.039.179-.041.175-.042.172-.043.168-.044.163-.045.16-.045.155-.047.152-.047.148-.048.143-.049.139-.049.136-.049.131-.051.126-.05.123-.051.118-.052.114-.051.11-.052.106-.052.101-.052.096-.052.092-.052.088-.052.083-.052.079-.052.074-.052.07-.051.065-.051.06-.051.056-.05.051-.049.023-.025.023-.025.021-.024.02-.024.019-.024.018-.024.017-.024.015-.023.014-.024.013-.023.012-.023.01-.022.01-.023.008-.022.006-.022.006-.022.004-.022.004-.021.001-.021.001-.021v-4.153l-.077.054-.08.054-.083.053-.085.053-.087.053-.09.051-.093.051-.095.051-.097.05-.1.049-.102.048-.105.048-.106.048-.109.046-.111.046-.114.046-.115.044-.118.044-.12.043-.122.043-.124.042-.126.041-.128.04-.13.039-.132.039-.134.038-.135.037-.138.036-.139.036-.142.034-.143.034-.144.033-.147.032-.148.032-.15.03-.151.03-.153.028-.154.028-.156.027-.158.026-.159.024-.161.024-.162.023-.163.023-.165.021-.166.02-.167.019-.169.018-.169.017-.171.016-.173.015-.173.014-.175.013-.175.012-.177.01-.178.01-.179.009-.179.007-.181.006-.182.006-.182.004-.184.003-.184.001-.185.001-.185-.001-.184-.001-.184-.003-.182-.004-.182-.006-.181-.006-.179-.007-.179-.009-.178-.01-.176-.01-.176-.012-.175-.013-.173-.014-.172-.015-.171-.016-.17-.017-.169-.018-.167-.019-.166-.02-.165-.021-.163-.023-.162-.023-.161-.024-.159-.024-.157-.026-.156-.027-.155-.028-.153-.028-.151-.03-.15-.03-.148-.032-.146-.032-.145-.033-.143-.034-.141-.034-.14-.036-.137-.036-.136-.037-.134-.038-.132-.039-.13-.039-.128-.041-.126-.041-.124-.041-.122-.043-.12-.043-.117-.044-.116-.044-.113-.046-.112-.046-.109-.046-.106-.048-.105-.048-.102-.048-.1-.05-.097-.049-.095-.051-.093-.051-.09-.052-.087-.052-.085-.053-.083-.053-.08-.054-.077-.054v4.153zm8.74-8.179l-.257.004-.254.005-.25.008-.247.011-.244.012-.241.014-.237.016-.233.018-.231.021-.226.022-.224.023-.22.026-.216.027-.212.028-.21.031-.205.032-.202.033-.198.034-.194.036-.191.038-.187.038-.183.04-.179.041-.175.042-.172.043-.168.043-.163.045-.16.046-.155.046-.152.048-.148.048-.143.048-.139.049-.136.05-.131.05-.126.051-.123.051-.118.051-.114.052-.11.052-.106.052-.101.052-.096.052-.092.052-.088.052-.083.052-.079.052-.074.051-.07.052-.065.051-.06.05-.056.05-.051.05-.023.025-.023.024-.021.024-.02.025-.019.024-.018.024-.017.023-.015.024-.014.023-.013.023-.012.023-.01.023-.01.022-.008.022-.006.023-.006.021-.004.022-.004.021-.001.021-.001.021.001.021.001.021.004.021.004.022.006.021.006.023.008.022.01.022.01.023.012.023.013.023.014.023.015.024.017.023.018.024.019.024.02.025.021.024.023.024.023.025.051.05.056.05.06.05.065.051.07.052.074.051.079.052.083.052.088.052.092.052.096.052.101.052.106.052.11.052.114.052.118.051.123.051.126.051.131.05.136.05.139.049.143.048.148.048.152.048.155.046.16.046.163.045.168.043.172.043.175.042.179.041.183.04.187.038.191.038.194.036.198.034.202.033.205.032.21.031.212.028.216.027.22.026.224.023.226.022.231.021.233.018.237.016.241.014.244.012.247.011.25.008.254.005.257.004.26.001.26-.001.257-.004.254-.005.25-.008.247-.011.244-.012.241-.014.237-.016.233-.018.231-.021.226-.022.224-.023.22-.026.216-.027.212-.028.21-.031.205-.032.202-.033.198-.034.194-.036.191-.038.187-.038.183-.04.179-.041.175-.042.172-.043.168-.043.163-.045.16-.046.155-.046.152-.048.148-.048.143-.048.139-.049.136-.05.131-.05.126-.051.123-.051.118-.051.114-.052.11-.052.106-.052.101-.052.096-.052.092-.052.088-.052.083-.052.079-.052.074-.051.07-.052.065-.051.06-.05.056-.05.051-.05.023-.025.023-.024.021-.024.02-.025.019-.024.018-.024.017-.023.015-.024.014-.023.013-.023.012-.023.01-.023.01-.022.008-.022.006-.023.006-.021.004-.022.004-.021.001-.021.001-.021-.001-.021-.001-.021-.004-.021-.004-.022-.006-.021-.006-.023-.008-.022-.01-.022-.01-.023-.012-.023-.013-.023-.014-.023-.015-.024-.017-.023-.018-.024-.019-.024-.02-.025-.021-.024-.023-.024-.023-.025-.051-.05-.056-.05-.06-.05-.065-.051-.07-.052-.074-.051-.079-.052-.083-.052-.088-.052-.092-.052-.096-.052-.101-.052-.106-.052-.11-.052-.114-.052-.118-.051-.123-.051-.126-.051-.131-.05-.136-.05-.139-.049-.143-.048-.148-.048-.152-.048-.155-.046-.16-.046-.163-.045-.168-.043-.172-.043-.175-.042-.179-.041-.183-.04-.187-.038-.191-.038-.194-.036-.198-.034-.202-.033-.205-.032-.21-.031-.212-.028-.216-.027-.22-.026-.224-.023-.226-.022-.231-.021-.233-.018-.237-.016-.241-.014-.244-.012-.247-.011-.25-.008-.254-.005-.257-.004-.26-.001-.26.001z"

// insertDatabaseIcon is insertDatabaseIcon(elem, id).
func insertDatabaseIcon(elem *dom.Node, id string) {
	elem.
		Append("defs").
		Append("symbol").
		Attr("id", id+"-database").
		Attr("fill-rule", "evenodd").
		Attr("clip-rule", "evenodd").
		Append("path").
		Attr("transform", "scale(.5)").
		Attr("d", databaseIconPath)
}

// insertComputerIcon is insertComputerIcon(elem, id).
func insertComputerIcon(elem *dom.Node, id string) {
	elem.
		Append("defs").
		Append("symbol").
		Attr("id", id+"-computer").
		Attr("width", "24").
		Attr("height", "24").
		Append("path").
		Attr("transform", "scale(.5)").
		Attr(
			"d",
			"M2 2v13h20v-13h-20zm18 11h-16v-9h16v9zm-10.228 6l.466-1h3.524l.467 1h-4.457zm14.228 3h-24l2-6h2.104l-1.33 4h18.45l-1.297-4h2.073l2 6zm-5-10h-14v-7h14v7z",
		)
}

// insertClockIcon is insertClockIcon(elem, id).
func insertClockIcon(elem *dom.Node, id string) {
	elem.
		Append("defs").
		Append("symbol").
		Attr("id", id+"-clock").
		Attr("width", "24").
		Attr("height", "24").
		Append("path").
		Attr("transform", "scale(.5)").
		Attr(
			"d",
			"M12 2c5.514 0 10 4.486 10 10s-4.486 10-10 10-10-4.486-10-10 4.486-10 10-10zm0-2c-6.627 0-12 5.373-12 12s5.373 12 12 12 12-5.373 12-12-5.373-12-12-12zm5.848 12.459c.202.038.202.333.001.372-1.907.361-6.045 1.111-6.547 1.111-.719 0-1.301-.582-1.301-1.301 0-.512.77-5.447 1.125-7.445.034-.192.312-.181.343.014l.985 6.238 5.394 1.011z",
		)
}

// insertArrowHead sets up the arrow head and defines the marker. The result
// is appended to the svg.
func insertArrowHead(elem *dom.Node, id string) {
	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"-arrowhead").
		Attr("refX", 9).
		Attr("refY", 5).
		Attr("markerUnits", "userSpaceOnUse").
		Attr("markerWidth", 12).
		Attr("markerHeight", 12).
		Attr("orient", "auto").
		Append("path").
		Attr("d", "M 0 0 L 10 5 L 0 10 z") // this is actual shape for arrowhead
}

// insertArrowEnd is insertArrowEnd(elem, id).
func insertArrowEnd(elem *dom.Node, id string) {
	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"-arrowend").
		Attr("refX", 1).
		Attr("refY", 5).
		Attr("markerUnits", "userSpaceOnUse").
		Attr("markerWidth", 12).
		Attr("markerHeight", 12).
		Attr("orient", "auto").
		Append("path").
		Attr("d", "M 10 0 L 0 5 L 10 10 z") // this is actual shape for arrowhead
}

// insertArrowFilledHead sets up the filled arrow head and defines the
// marker. The result is appended to the svg.
func insertArrowFilledHead(elem *dom.Node, id string) {
	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"-filled-head").
		Attr("refX", 18).
		Attr("refY", 7).
		Attr("markerWidth", 20).
		Attr("markerHeight", 28).
		Attr("orient", "auto").
		Append("path").
		Attr("d", "M 18,7 L9,13 L14,7 L9,1 Z")
}

// insertArrowCrossHead sets up the cross arrow head and defines the marker.
// The result is appended to the svg.
func insertArrowCrossHead(elem *dom.Node, id string) {
	defs := elem.Append("defs")
	marker := defs.
		Append("marker").
		Attr("id", id+"-crosshead").
		Attr("markerWidth", 15).
		Attr("markerHeight", 8).
		Attr("orient", "auto").
		Attr("refX", 16).
		Attr("refY", 4)

	// The arrow
	marker.
		Append("path").
		Attr("fill", "black").
		Attr("stroke", "#000000").
		Style("stroke-dasharray", "0, 0").
		Attr("stroke-width", "1px").
		Attr("d", "M 9,2 V 6 L16,4 Z")

	// The cross
	marker.
		Append("path").
		Attr("fill", "none").
		Attr("stroke", "#000000").
		Style("stroke-dasharray", "0, 0").
		Attr("stroke-width", "1px").
		Attr("d", "M 0,1 L 6,7 M 6,1 L 0,7")
	// this is actual shape for arrowhead
}

// setTextAttrs is _setTextAttrs(toText, fromTextAttrsDict).
func setTextAttrs(toText *dom.Node, fromTextAttrsDict []textAttr) {
	for _, attr := range fromTextAttrsDict {
		toText.Attr(attr.name, utilsNullish(attr.value))
	}
}

// utilsNullish maps undefined and null to nil, which removes an attribute
// or a style property, as d3 does for a value == null.
func utilsNullish(v any) any {
	if js.IsNullish(v) {
		return nil
	}
	return v
}

// byText is byText of _drawTextCandidateFunc.
func byText(content string, g *dom.Node, x, y, width, height float64, textAttrs []textAttr, _ c4Font) {
	text := g.
		Append("text").
		Attr("x", x+float64(width/2)).
		Attr("y", y+float64(height/2)+5).
		Style("text-anchor", "middle").
		Text(content)
	setTextAttrs(text, textAttrs)
}

// byTspan is byTspan of _drawTextCandidateFunc.
func byTspan(content string, g *dom.Node, x, y, width, _ float64, textAttrs []textAttr, conf c4Font) {
	fontSize, fontFamily, fontWeight := conf.fontSize, conf.fontFamily, conf.fontWeight

	lines := sanitize.LineBreakRegex.Split(content, -1)
	size := js.ToNumber(fontSize)
	for i := 0; i < len(lines); i++ {
		dy := float64(float64(i)*size) - float64(float64(size*float64(len(lines)-1))/2)
		text := g.
			Append("text").
			Attr("x", x+float64(width/2)).
			Attr("y", y).
			Style("text-anchor", "middle").
			Attr("dominant-baseline", "middle").
			Style("font-size", utilsNullish(fontSize)).
			Style("font-weight", utilsNullish(fontWeight)).
			Style("font-family", utilsNullish(fontFamily))
		text.
			Append("tspan").
			// .attr('x', x + width / 2)
			Attr("dy", dy).
			Text(lines[i]).
			// .attr('y', y + height / 2)
			Attr("alignment-baseline", "mathematical")

		setTextAttrs(text, textAttrs)
	}
}

// byFo is byFo of _drawTextCandidateFunc.
func byFo(content string, g *dom.Node, x, y, width, height float64, textAttrs []textAttr, conf c4Font) {
	s := g.Append("switch")
	f := s.
		Append("foreignObject").
		Attr("x", x).
		Attr("y", y).
		Attr("width", width).
		Attr("height", height)

	text := f.
		Append("xhtml:div").
		Style("display", "table").
		Style("height", "100%").
		Style("width", "100%")

	text.
		Append("div").
		Style("display", "table-cell").
		Style("text-align", "center").
		Style("vertical-align", "middle").
		Text(content)

	byTspan(content, s, x, y, width, height, textAttrs, conf)
	setTextAttrs(text, textAttrs)
}

// drawTextCandidateFunc is _drawTextCandidateFunc(conf): the text function
// for the textPlacement of the config.
func (r *renderer) drawTextCandidateFunc() drawTextFunction {
	switch r.rs.conf.Get("textPlacement") {
	case "fo":
		return byFo
	case "old":
		return byText
	}
	return byTspan
}
