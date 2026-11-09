// Ported from mermaid 12.2.0, diagrams/timeline/svgDraw.js: initGraphics,
// wrap, drawNode, getVirtualNodeHeight and defaultBkg (MIT). The other
// functions of the file (drawRect, drawFace, drawCircle, drawText, drawLabel,
// drawSection, drawTask, drawBackgroundRect, getTextObj, getNoteRect) are
// copies of the user journey code that no timeline renderer calls.

package timeline

import (
	"errors"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// block is the node object that the renderers give to drawNode and
// getVirtualNodeHeight (the Block interface of the renderers).
type block struct {
	// descr is the text of the node, after the conversion to a string that
	// selection.text() does.
	descr     string
	section   float64
	width     float64
	padding   float64
	maxHeight float64
	// height is set by drawNode.
	height float64
}

// svgDraw holds the module state of svgDraw.js.
type svgDraw struct {
	nodeCount int
}

// undefinedID is the diagram id of a call that gives no id: the vertical
// renderer calls initGraphics and drawNode without the id argument.
const undefinedID = "undefined"

// initGraphics is initGraphics(graphics, id).
func (s *svgDraw) initGraphics(graphics *dom.Node, id string) {
	s.nodeCount = 0
	graphics.
		Append("defs").
		Append("marker").
		Attr("id", id+"-arrowhead").
		Attr("refX", 5).
		Attr("refY", 2).
		Attr("markerWidth", 6).
		Attr("markerHeight", 4).
		Attr("orient", "auto").
		Append("path").
		Attr("d", "M 0,0 V 4 L6,2 Z") // This is the shape of the arrowhead.
}

// splitWords is text.split(/(\s+|<br>)/): the parts between the separators
// and the separators themselves.
func splitWords(text string) []string {
	var out []string
	start := 0
	for i := 0; i < len(text); {
		end := i
		if strings.HasPrefix(text[i:], "<br>") {
			end = i + len("<br>")
		} else {
			for end < len(text) {
				// A byte that is not UTF-8 (a lone surrogate from the
				// lexer) decodes as U+FFFD with size 1: not white space.
				r, size := utf8.DecodeRuneInString(text[end:])
				if !js.IsWhiteSpace(r) {
					break
				}
				end += size
			}
		}
		if end == i {
			_, size := utf8.DecodeRuneInString(text[i:])
			i += size
			continue
		}
		out = append(out, text[start:i], text[i:end])
		start, i = end, end
	}
	return append(out, text[start:])
}

// wrap is wrap(text, width): it splits the text of a text element into
// tspan lines that are not wider than width.
func wrap(text *dom.Node, width float64) {
	words := splitWords(text.TextContent())
	// .reverse()
	for i, j := 0, len(words)-1; i < j; i, j = i+1, j-1 {
		words[i], words[j] = words[j], words[i]
	}
	var word string
	line := []string{}
	const lineHeight = 1.1 // ems
	var y any
	if v, ok := text.GetAttr("y"); ok {
		y = v
	}
	dy := math.NaN()
	if v, ok := text.GetAttr("dy"); ok {
		dy = js.ParseFloat(v)
	}
	tspan := text.
		Text("").
		Append("tspan").
		Attr("x", 0).
		Attr("y", y).
		Attr("dy", js.Str(dy)+"em")
	for j := 0; j < len(words); j++ {
		word = words[len(words)-1-j]
		line = append(line, word)
		tspan.Text(js.TrimSpace(strings.Join(line, " ")))
		if tspan.ComputedTextLength() > width || word == "<br>" {
			line = line[:len(line)-1]
			tspan.Text(js.TrimSpace(strings.Join(line, " ")))
			if word == "<br>" {
				line = []string{""}
			} else {
				line = []string{word}
			}

			tspan = text.
				Append("tspan").
				Attr("x", 0).
				Attr("y", y).
				Attr("dy", js.Str(lineHeight)+"em").
				Text(word)
		}
	}
}

// includesRedux is theme?.includes('redux').
func includesRedux(theme any) (bool, error) {
	return includes(theme, "redux")
}

// includes is value?.includes(part) for a config value.
func includes(value any, part string) (bool, error) {
	switch v := value.(type) {
	case string:
		return strings.Contains(v, part), nil
	case []any:
		for _, e := range v {
			if s, ok := e.(string); ok && s == part {
				return true, nil
			}
		}
		return false, nil
	}
	if js.IsNullish(value) {
		return false, nil
	}
	return false, errors.New("theme.includes is not a function")
}

// confFontSize is the font size that drawNode and getVirtualNodeHeight use:
// conf.fontSize?.replace ? conf.fontSize.replace('px', ”) : conf.fontSize,
// converted to a number as the multiplication does.
func confFontSize(conf *js.Obj[any]) float64 {
	fontSize := conf.Get("fontSize")
	if s, ok := fontSize.(string); ok {
		return js.Number(strings.Replace(s, "px", "", 1))
	}
	return js.ToNumber(fontSize)
}

// ownerSVGElement is node.ownerSVGElement ?? node.
func ownerSVGElement(n *dom.Node) *dom.Node {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Type == dom.ElementNode && p.NS == dom.NSSVG && p.Tag == "svg" {
			return p
		}
	}
	return n
}

// drawNode is drawNode(elem, node, fullSection, conf, diagramId, isEvent).
func (s *svgDraw) drawNode(elem *dom.Node, node *block, fullSection float64, conf *js.Obj[any], diagramID string, isEvent bool) (*block, error) {
	theme, look := conf.Get("theme"), conf.Get("look")
	isReduxTheme, err := includesRedux(theme)
	if err != nil {
		return nil, err
	}
	maxSections := 12.0
	if v := js.Path(conf, "themeVariables", "THEME_COLOR_LIMIT"); !js.IsNullish(v) {
		maxSections = js.ToNumber(v)
	}
	section := math.Mod(fullSection, maxSections) - 1
	nodeElem := elem.Append("g")
	node.section = section
	nodeElem.Attr("class", "timeline-node "+("section-"+js.Str(section)))
	bkgElem := nodeElem.Append("g")

	// Create the wrapped text element.
	textElem := nodeElem.Append("g")

	txt := textElem.
		Append("text").
		Text(node.descr).
		Attr("dy", "1em").
		Attr("alignment-baseline", "middle").
		Attr("dominant-baseline", "middle").
		Attr("text-anchor", "middle")
	wrap(txt, node.width)
	bbox := txt.BBox()
	fontSize := confFontSize(conf)
	node.height = bbox.Height + float64(fontSize*1.1*0.5) + node.padding
	node.height = js.Max(node.height, node.maxHeight)
	node.width = node.width + float64(2*node.padding)

	textElem.Attr("transform", "translate("+js.Str(node.width/2)+", "+js.Str(node.padding/2)+")")
	if isReduxTheme {
		ty := node.padding
		if isEvent {
			ty = float64(node.padding/2) + 3
		}
		textElem.Attr("transform", "translate("+js.Str(node.width/2)+", "+js.Str(ty)+")")
	}

	// Create the background element.
	s.defaultBkg(bkgElem, node, section, diagramID, isReduxTheme)

	if look == "neo" {
		nodeElem.Attr("data-look", "neo")
		if isReduxTheme {
			isDark, err := includes(theme, "dark")
			if err != nil {
				return nil, err
			}
			rootSvg := ownerSVGElement(elem)
			svgID := rootSvg.AttrOr("id", "")
			dropShadowID := "drop-shadow"
			if svgID != "" {
				dropShadowID = svgID + "-drop-shadow"
			}

			// Only add the filter once per SVG to avoid duplicate definitions.
			if rootSvg.Select("#"+dropShadowID) == nil {
				defsEl := rootSvg.Select("defs")
				if defsEl == nil {
					defsEl = rootSvg.Append("defs")
				}
				floodOpacity, floodColor := "0.06", "#000000"
				if isDark {
					floodOpacity, floodColor = "0.2", "#FFFFFF"
				}
				defsEl.
					Append("filter").
					Attr("id", dropShadowID).
					Attr("height", "130%").
					Attr("width", "130%").
					Append("feDropShadow").
					Attr("dx", "4").
					Attr("dy", "4").
					Attr("stdDeviation", 0).
					Attr("flood-opacity", floodOpacity).
					Attr("flood-color", floodColor)
			}
		}
	}

	return node, nil
}

// getVirtualNodeHeight is getVirtualNodeHeight(elem, node, conf).
func (s *svgDraw) getVirtualNodeHeight(elem *dom.Node, node *block, conf *js.Obj[any]) float64 {
	textElem := elem.Append("g")
	txt := textElem.
		Append("text").
		Text(node.descr).
		Attr("dy", "1em").
		Attr("alignment-baseline", "middle").
		Attr("dominant-baseline", "middle").
		Attr("text-anchor", "middle")
	wrap(txt, node.width)
	bbox := txt.BBox()
	fontSize := confFontSize(conf)
	textElem.Remove()
	return bbox.Height + float64(fontSize*1.1*0.5) + node.padding
}

// defaultBkg is defaultBkg(elem, node, section, diagramId, config). The
// node has no type, so the class of the path is "node-undefined".
func (s *svgDraw) defaultBkg(elem *dom.Node, node *block, section float64, diagramID string, isReduxTheme bool) {
	r := 5.0
	if isReduxTheme {
		r = 0
	}
	const rd = 5.0
	// When r=0 (redux themes), use straight line segments for sharp corners
	// instead of degenerate quadratic bezier curves.
	var d string
	if r > 0 {
		d = "M0 " + js.Str(node.height-rd) +
			" v" + js.Str(-node.height+float64(2*rd)) +
			" q0,-" + js.Str(r) + "," + js.Str(r) + ",-" + js.Str(r) +
			" h" + js.Str(node.width-float64(2*rd)) +
			" q" + js.Str(r) + ",0," + js.Str(r) + "," + js.Str(r) +
			" v" + js.Str(node.height-rd) +
			" H0 Z"
	} else {
		d = "M0 " + js.Str(node.height-rd) +
			" v" + js.Str(-(node.height - rd)) +
			" h" + js.Str(node.width) +
			" v" + js.Str(node.height) +
			" H0 Z"
	}
	elem.
		Append("path").
		Attr("id", diagramID+"-node-"+js.Str(float64(s.nodeCount))).
		Attr("class", "node-bkg node-undefined").
		Attr("d", d)
	s.nodeCount++
	if !isReduxTheme {
		elem.
			Append("line").
			Attr("class", "node-line-"+js.Str(section)).
			Attr("x1", 0).
			Attr("y1", node.height).
			Attr("x2", node.width).
			Attr("y2", node.height)
	}
}
