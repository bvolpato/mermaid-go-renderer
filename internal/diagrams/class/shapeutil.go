// Ported from mermaid 12.2.0, diagrams/class/shapeUtil.ts (MIT).

package class

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

// The class properties of a layout node are in node.Extra: text, members,
// methods, annotations and styles. The db stores Go values there. A node
// that a test builds from JSON has dynamic values: arrays of objects and of
// strings. The functions below read both forms.

// boxMember is what the shape reads from a class member: the text, and the
// style of the classifier (parseClassifier()).
type boxMember struct {
	text    string
	hasText bool
	style   string
}

// nodeMembers is node.members or node.methods. The second result is false
// when the property is undefined.
func nodeMembers(node *ru.Node, key string) ([]boxMember, bool) {
	switch v := node.Get(key).(type) {
	case []*ClassMember:
		out := make([]boxMember, len(v))
		for i, m := range v {
			out[i] = boxMember{text: m.Text, hasText: true, style: m.ParseClassifier()}
		}
		return out, true
	case []any:
		out := make([]boxMember, len(v))
		for i, e := range v {
			o := js.AsObj(e)
			if o == nil {
				continue
			}
			if t, ok := o.Get("text").(string); ok {
				out[i].text, out[i].hasText = t, true
			}
			if c, ok := o.Get("classifier").(string); ok {
				out[i].style = classifierStyle(c)
			}
		}
		return out, true
	}
	return nil, false
}

// nodeStrings is a property of the node that is an array of strings:
// annotations or styles. The second result is false when the property is
// undefined.
func nodeStrings(node *ru.Node, key string) ([]string, bool) {
	switch v := node.Get(key).(type) {
	case []string:
		return v, true
	case []any:
		out := make([]string, len(v))
		for i, e := range v {
			out[i] = js.ToString(e)
		}
		return out, true
	}
	return nil, false
}

// textSource is the node argument of addText: a layout node, a class
// member, or the object { text } of an annotation.
type textSource struct {
	// text is node.text when the object has that property, else node.label.
	// hasText is false when the value is undefined.
	text    string
	hasText bool
	// hasUseHtmlLabels is `'useHtmlLabels' in node`.
	hasUseHtmlLabels bool
	useHtmlLabels    bool
}

// nodeTextSource is the layout node as the argument of addText.
func nodeTextSource(node *ru.Node) textSource {
	src := textSource{hasUseHtmlLabels: node.UseHtmlLabelsSet, useHtmlLabels: node.UseHtmlLabels}
	// Support regular node type (.label) and classNodes (.text)
	if t, ok := node.Extra["text"]; ok {
		if s, isString := t.(string); isString {
			src.text, src.hasText = s, true
		}
	} else if node.HasLabel {
		src.text, src.hasText = node.Label, true
	}
	return src
}

// classPadding is `config.class!.padding ?? def`.
func classPadding(config *js.Obj[any], def float64) float64 {
	if v := js.Path(config, "class", "padding"); !js.IsNullish(v) {
		return js.ToNumber(v)
	}
	return def
}

// textHelperResult is the return value of textHelper.
type textHelperResult struct {
	shapeSvg *dom.Node
	bbox     dom.Rect
}

// textHelper is textHelper(parent, node, config, useHtmlLabels, GAP): it
// creates the shapeSvg and inserts the text.
//
// textHelper and addText are async upstream, and a layout draws its nodes
// together with Promise.all. The number of awaits of a class box grows with
// the number of its texts, so a class with few members draws its rectangle,
// and takes its numbers of Math.random, before a class with many members.
// The calls of ctx.Await keep that order (render/async.go).
func textHelper(ctx *render.Context, parent *dom.Node, node *ru.Node, annotations []string, members, methods []boxMember, useHtmlLabels bool, GAP float64) (*textHelperResult, error) {
	TEXT_PADDING := 0.0
	if !useHtmlLabels {
		TEXT_PADDING = 3
	}
	id := node.DomID
	if id == "" {
		id = node.ID
	}
	shapeSvg := parent.
		Insert("g").
		Attr("class", ru.GetNodeClasses(node)).
		Attr("id", id)

	annotationGroupHeight := 0.0
	labelGroupHeight := 0.0
	membersGroupHeight := 0.0

	annotationGroup := shapeSvg.Insert("g").Attr("class", "annotation-group text")
	if len(annotations) > 0 {
		annotation := annotations[0]
		if _, err := addText(ctx, annotationGroup, textSource{text: "«" + annotation + "»", hasText: true}, 0, nil); err != nil {
			return nil, err
		}

		annotationGroupBBox := annotationGroup.BBox()
		annotationGroupHeight = annotationGroupBBox.Height
	}

	labelGroup := shapeSvg.Insert("g").Attr("class", "label-group text")
	if _, err := addText(ctx, labelGroup, nodeTextSource(node), 0, []string{"font-weight: bolder"}); err != nil {
		return nil, err
	}
	labelGroupBBox := labelGroup.BBox()
	labelGroupHeight = labelGroupBBox.Height

	membersGroup := shapeSvg.Insert("g").Attr("class", "members-group text")
	yOffset := 0.0
	for _, member := range members {
		height, err := addText(ctx, membersGroup, textSource{text: member.text, hasText: member.hasText}, yOffset, []string{member.style})
		if err != nil {
			return nil, err
		}
		yOffset += height + TEXT_PADDING
	}
	membersGroupHeight = membersGroup.BBox().Height
	if membersGroupHeight <= 0 {
		membersGroupHeight = GAP / 2
	}

	methodsGroup := shapeSvg.Insert("g").Attr("class", "methods-group text")
	methodsYOffset := 0.0
	for _, method := range methods {
		height, err := addText(ctx, methodsGroup, textSource{text: method.text, hasText: method.hasText}, methodsYOffset, []string{method.style})
		if err != nil {
			return nil, err
		}
		methodsYOffset += height + TEXT_PADDING
	}

	// Center annotation
	annotationGroupBBox := annotationGroup.BBox()
	annotationGroup.Attr("transform", "translate("+js.Str(-annotationGroupBBox.Width/2)+")")

	// Adjust label
	labelGroup.Attr("transform", "translate("+js.Str(-labelGroupBBox.Width/2)+", "+js.Str(annotationGroupHeight)+")")

	membersGroup.Attr(
		"transform",
		"translate("+js.Str(0)+", "+js.Str(annotationGroupHeight+labelGroupHeight+float64(GAP*2))+")",
	)
	methodsY := float64(GAP * 2)
	if membersGroupHeight != 0 && membersGroupHeight == membersGroupHeight {
		methodsY = membersGroupHeight + float64(GAP*4)
	}
	methodsGroup.Attr(
		"transform",
		"translate("+js.Str(0)+", "+js.Str(annotationGroupHeight+labelGroupHeight+methodsY)+")",
	)

	bbox := shapeSvg.BBox()

	// The await of the caller. See render/async.go for the convention.
	ctx.Await()
	return &textHelperResult{shapeSvg: shapeSvg, bbox: bbox}, nil
}

// addText is addText(parentGroup, node, yOffset, styles): a modified
// version of labelHelper() that creates and places the text of a class.
func addText(ctx *render.Context, parentGroup *dom.Node, node textSource, yOffset float64, styles []string) (float64, error) {
	textEl := parentGroup.Insert("g").Attr("class", "label").Attr("style", strings.Join(styles, "; "))
	config := ctx.Config
	// 'useHtmlLabels' in node ? node.useHtmlLabels : (evaluate(config.htmlLabels) ?? true)
	useHtmlLabels := sanitize.Evaluate(config.Get("htmlLabels"))
	if node.hasUseHtmlLabels {
		useHtmlLabels = node.useHtmlLabels
	}

	textContent := node.text

	// createText() will cause unwanted behavior because of classDiagram syntax so workarounds are needed

	if !node.hasText {
		// textContent is undefined.
		if !useHtmlLabels {
			return 0, undefinedRead("startsWith")
		}
		return 0, undefinedRead("match")
	}
	if !useHtmlLabels && strings.HasPrefix(textContent, `\`) {
		textContent = textContent[1:]
	}

	if sanitize.HasKatex(textContent) {
		useHtmlLabels = true
	}

	textWidth, err := utils.CalculateTextWidth(ctx, textContent, utils.TextDimensionConfigOf(config))
	if err != nil {
		return 0, err
	}
	options := ru.DefaultTextOptions()
	options.Width = textWidth + 50 // Add room for error when splitting text into multiple lines
	options.Classes = "markdown-node-label"
	options.UseHtmlLabels = useHtmlLabels
	text, err := ru.CreateText(ctx, textEl, ctx.SanitizeText(render.DecodeEntities(textContent)), options)
	if err != nil {
		return 0, err
	}
	var bbox dom.Rect
	numberOfLines := 1

	if !useHtmlLabels {
		// Undo font-weight normal
		for _, s := range styles {
			if s == "font-weight: bolder" {
				text.SelectAll("tspan").Attr("font-weight", "")
				break
			}
		}

		children := text.Children()
		numberOfLines = len(children)

		if len(children) == 0 {
			// textChild is undefined.
			if tc := text.TextContent(); tc == "" || strings.Contains(tc, "&gt") {
				return 0, undefinedSet("textContent")
			}
			return 0, undefinedRead("textContent")
		}
		textChild := children[0]
		if tc := text.TextContent(); tc == "" || strings.Contains(tc, "&gt") {
			// textContent[0] + textContent.substring(1)...trim(): the first
			// UTF-16 code unit, or the text "undefined" for an empty string.
			first := "undefined"
			if textContent != "" {
				first = js.CharAt(textContent, 0)
			}
			rest := js.StringSliceFrom(textContent, 1)
			rest = strings.ReplaceAll(strings.ReplaceAll(rest, "&gt;", ">"), "&lt;", "<")
			textChild.Text(js.Concat(first, js.TrimSpace(rest)))

			// Text was improperly removed due to spaces (preserve one space if present)
			preserveSpace := js.CharAt(textContent, 1) == " "
			if preserveSpace {
				current := textChild.TextContent()
				head := "undefined"
				if current != "" {
					head = js.CharAt(current, 0)
				}
				textChild.Text(js.Concat(head, " ", js.StringSliceFrom(current, 1)))
			}
		}

		// To support empty boxes
		if textChild.TextContent() == "undefined" {
			textChild.Text("")
		}

		// Get the bounding box after the text update
		bbox = text.BBox()
	} else {
		div := text.FirstElementChild()
		dv := text

		innerHTML := div.InnerHTML()
		numberOfLines = strings.Count(innerHTML, "<br>") + 1
		// Katex math support
		if strings.Contains(innerHTML, "</math>") {
			numberOfLines += strings.Count(innerHTML, "<mrow>")
		}

		ru.ConfigureLabelImages(ctx, div)

		bbox = div.BoundingClientRect()
		dv.Attr("width", bbox.Width)
		dv.Attr("height", bbox.Height)
	}

	// Center text and offset by yOffset
	textEl.Attr("transform", "translate(0,"+js.Str(float64(-bbox.Height/float64(2*numberOfLines))+yOffset)+")")
	// The await of the caller. See render/async.go for the convention.
	ctx.Await()
	return bbox.Height, nil
}
