// Ported from mermaid 12.2.0, diagrams/c4/c4ShapeAdapter.ts (MIT).

package c4

import (
	"errors"
	"regexp"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// The adapter builds a node of the unified renderer from a legacy C4 shape,
// so that the legacy renderer can draw elements through the shared shapes.
// The layout still comes from the legacy grid.

var queueShapes = map[string]bool{
	"system_queue":             true,
	"external_system_queue":    true,
	"container_queue":          true,
	"external_container_queue": true,
	"component_queue":          true,
	"external_component_queue": true,
}

var dbShapes = map[string]bool{
	"system_db":             true,
	"external_system_db":    true,
	"container_db":          true,
	"external_container_db": true,
	"component_db":          true,
	"external_component_db": true,
}

// shapeKeywords is SHAPE_KEYWORDS: the Structurizr-style shape keywords
// accepted via $shape, $sprite or $tags.
var shapeKeywords = map[string]string{
	"person":    "person",
	"box":       "rounded",
	"rounded":   "rounded",
	"cylinder":  "cylinder",
	"database":  "cylinder",
	"db":        "cylinder",
	"queue":     "h-cyl",
	"pipe":      "h-cyl",
	"component": "fr-rect",
}

// errTypeError is the error for a property that has a type that upstream
// does not expect, where the JavaScript code throws a TypeError.
func errTypeError(message string) error {
	return errors.New("TypeError: " + message)
}

// keywordShape is keywordShape(value): the shape for a keyword, or "" when
// the value is falsy or is no keyword.
func keywordShape(value any) (string, error) {
	if !js.Truthy(value) {
		return "", nil
	}
	s, ok := value.(string)
	if !ok {
		return "", errTypeError("value.toLowerCase is not a function")
	}
	key := js.ToLowerCase(s)
	// SHAPE_KEYWORDS is a plain object, so the lookup also finds the
	// properties of Object.prototype that have a lowercase name. The value
	// is then not a shape id, and its text is the shape of the node: no
	// shape handler has that name.
	switch key {
	case "constructor":
		return "function Object() { [native code] }", nil
	case "__proto__":
		return "[object Object]", nil
	}
	return shapeKeywords[key], nil
}

// resolveNodeShape resolves the render shape for a C4 element: explicit
// $shape or $sprite first, then a recognised $tags token, then the element
// type.
func resolveNodeShape(shape *js.Obj[any]) (string, error) {
	explicit, err := keywordShape(shape.Get("shape"))
	if err != nil {
		return "", err
	}
	if explicit == "" {
		if explicit, err = keywordShape(shape.Get("sprite")); err != nil {
			return "", err
		}
	}
	if explicit != "" {
		return explicit, nil
	}
	if tags := shape.Get("tags"); js.Truthy(tags) {
		s, ok := tags.(string)
		if !ok {
			return "", errTypeError("shape.tags.split is not a function")
		}
		for _, tag := range strings.Split(s, ",") {
			tagged, err := keywordShape(js.TrimSpace(tag))
			if err != nil {
				return "", err
			}
			if tagged != "" {
				return tagged, nil
			}
		}
	}
	typeC4Shape := js.Path(shape, "typeC4Shape", "text")
	if typeC4Shape == "person" || typeC4Shape == "external_person" {
		return "person", nil
	}
	if s, ok := typeC4Shape.(string); ok {
		if dbShapes[s] {
			return "cylinder", nil
		}
		if queueShapes[s] {
			return "h-cyl", nil
		}
	}
	return "rounded", nil
}

var stereotypeNames = map[string]string{
	"person":    "Person",
	"system":    "Software System",
	"container": "Container",
	"component": "Component",
}

var (
	reExternalPrefix = regexp.MustCompile(`^external_`)
	reDbQueueSuffix  = regexp.MustCompile(`_(db|queue)$`)
)

// stereotypeLabel is the Structurizr-style stereotype, for example
// `Software System` for system, system_db and external_system.
func stereotypeLabel(typeC4Shape string) string {
	base := reExternalPrefix.ReplaceAllString(typeC4Shape, "")
	base = reDbQueueSuffix.ReplaceAllString(base, "")
	if name, ok := stereotypeNames[base]; ok {
		return name
	}
	return strings.ReplaceAll(base, "_", " ")
}

// isExternal is isExternal(typeC4Shape).
func isExternal(typeC4Shape string) bool { return strings.HasPrefix(typeC4Shape, "external_") }

// stereotypeText is the Structurizr-style type line, for example
// `[Container: Node.js]`.
func stereotypeText(shape *js.Obj[any], typeC4Shape string) string {
	stereotype := stereotypeLabel(typeC4Shape)
	if techn := js.Path(shape, "techn", "text"); js.Truthy(techn) {
		return "[" + stereotype + ": " + js.ToString(techn) + "]"
	}
	return "[" + stereotype + "]"
}

// c4ElementTypes is C4_ELEMENT_TYPES: the C4 element types, internal and
// external, that carry per-type config.
var c4ElementTypes = func() map[string]bool {
	types := map[string]bool{}
	for _, typ := range []string{
		"person",
		"system",
		"system_db",
		"system_queue",
		"container",
		"container_db",
		"container_queue",
		"component",
		"component_db",
		"component_queue",
	} {
		types[typ] = true
		types["external_"+typ] = true
	}
	return types
}()

// isC4ElementType is isC4ElementType(value).
func isC4ElementType(value string) bool { return c4ElementTypes[value] }

// elementCssStyles gives the element colours. The per-element
// `<type>_bg_color` and `<type>_border_color` config palette drives the
// fill and the border, with white text. An explicit per-element colour
// (UpdateElementStyle: $bgColor, $borderColor, $fontColor) overrides it.
func elementCssStyles(shape *js.Obj[any], elementType string, config *js.Obj[any]) []string {
	// shape.bgColor ?? (isC4ElementType(elementType) && config[`${elementType}_bg_color`])
	pick := func(own any, suffix string) any {
		if !js.IsNullish(own) {
			return own
		}
		if !isC4ElementType(elementType) {
			return false
		}
		return config.Get(elementType + suffix)
	}
	fill := pick(shape.Get("bgColor"), "_bg_color")
	stroke := pick(shape.Get("borderColor"), "_border_color")
	styles := []string{}
	if js.Truthy(fill) {
		styles = append(styles, "fill:"+js.ToString(fill))
	}
	if js.Truthy(stroke) {
		styles = append(styles, "stroke:"+js.ToString(stroke))
	}
	fontColor := shape.Get("fontColor")
	if js.IsNullish(fontColor) {
		fontColor = "#FFFFFF"
	}
	styles = append(styles, "color:"+js.ToString(fontColor))
	return styles
}

// buildC4Node converts a legacy C4 shape into a node of the unified
// renderer. config is the c4 diagram config, whose palette drives the fill
// and the border. elementWidth is the target shape width (c4.width). The
// label helper derives its own text-wrapping width from it.
func buildC4Node(shape *js.Obj[any], config *js.Obj[any], padding float64, look string, elementWidth float64) (*ru.Node, error) {
	// const typeC4Shape = shape.typeC4Shape.text;
	typeValue := js.Path(shape, "typeC4Shape", "text")
	if js.IsNullish(shape.Get("typeC4Shape")) {
		return nil, errTypeError("Cannot read properties of undefined (reading 'text')")
	}
	cssClasses := []string{"c4-shape", "c4-" + js.ToString(typeValue)}
	typeC4Shape, ok := typeValue.(string)
	if !ok {
		// isExternal calls startsWith on the value.
		return nil, errTypeError("typeC4Shape.startsWith is not a function")
	}
	if isExternal(typeC4Shape) {
		cssClasses = append(cssClasses, "c4-external")
	}
	nodeShape, err := resolveNodeShape(shape)
	if err != nil {
		return nil, err
	}
	cssStyles := elementCssStyles(shape, typeC4Shape, config)
	if nodeShape == "rounded" || nodeShape == "fr-rect" {
		// Inline so it wins over the shape's default corner radius.
		cssStyles = append(cssStyles, "rx:12px", "ry:12px")
	}

	// id: shape.alias. The alias is a string, or the object of a named
	// attribute, which prints as a string in the dom id.
	node := ru.NewNode(js.ToString(shape.Get("alias")))
	// label: shape.label.text. The label helper reads the name of the node
	// as `typeof node.label === 'string' ? node.label : (node.label?.[0] ?? '')`.
	// The text is a string, or the object of a named attribute in the
	// place of the label.
	switch label := js.Path(shape, "label", "text").(type) {
	case string:
		node.Label = label
		node.HasLabel = true
	case *js.Obj[any]:
		if first := label.Get("0"); !js.IsNullish(first) {
			name, ok := first.(string)
			if !ok {
				return nil, errTypeError("text.replace is not a function")
			}
			node.Label = name
			node.HasLabel = true
		}
	}
	node.Stereotype = stereotypeText(shape, typeC4Shape)
	node.HasStereotype = true
	// description: shape.descr?.text ? [shape.descr.text] : undefined
	if descr := js.Path(shape, "descr", "text"); js.Truthy(descr) {
		node.Description = []string{js.ToString(descr)}
	}
	node.LabelType = "string"
	node.IsGroup = false
	node.Shape = nodeShape
	node.CSSClasses = strings.Join(cssClasses, " ")
	node.CSSClassesSet = true
	node.CSSStyles = cssStyles
	node.CSSStylesSet = true
	node.Padding = padding
	node.Look = look
	node.UseHtmlLabels = false
	node.Width = elementWidth
	return node, nil
}
