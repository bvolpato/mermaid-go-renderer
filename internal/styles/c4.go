// Ported from mermaid 12.2.0, mermaid/src/diagrams/c4/styles.js and from
// C4_ELEMENT_TYPES in mermaid/src/diagrams/c4/c4ShapeAdapter.ts (MIT).

package styles

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/css"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// c4ElementTypes is C4_ELEMENT_TYPES: each element type, then its external form.
var c4ElementTypes = func() []string {
	var types []string
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
		types = append(types, typ, "external_"+typ)
	}
	return types
}()

// c4ElementFontStyles makes the font rules of each element type from the c4
// config (personFontFamily and the like).
//
// Mermaid builds the rules through the CSSOM: it inserts an empty rule in a
// CSSStyleSheet, calls rule.style.setProperty for each value and reads
// rule.cssText. So the browser parses each value, drops a value that does not
// fit the property, and prints the others in its own form. The port takes
// that form from internal/css.
func c4ElementFontStyles(o *options) string {
	c4 := obj(o.config.Get("c4"))
	var rules []string
	for _, typ := range c4ElementTypes {
		var declarations []string
		setProperty := func(name, value string) {
			if text, ok := css.NormalizeValue(name, value); ok {
				declarations = append(declarations, name+": "+text+";")
			}
		}
		fontFamily := c4.Get(typ + "FontFamily")
		fontSize := c4.Get(typ + "FontSize")
		fontWeight := c4.Get(typ + "FontWeight")
		if js.Truthy(fontFamily) {
			setProperty("font-family", s(fontFamily))
		}
		if js.Truthy(fontSize) {
			if js.TypeOf(fontSize) == "number" {
				setProperty("font-size", s(fontSize)+"px")
			} else {
				setProperty("font-size", s(fontSize))
			}
		}
		if js.Truthy(fontWeight) {
			setProperty("font-weight", s(fontWeight))
		}
		// A rule with no declaration is left out.
		if len(declarations) > 0 {
			rules = append(rules, "  .c4-shape.c4-"+typ+" .label { "+strings.Join(declarations, " ")+" }")
		}
	}
	return strings.Join(rules, "\n")
}

// c4Styles is getStyles of the C4 diagram: the CSS of the diagram type.
func c4Styles(o *options) string {
	return `.person {
    stroke: ` + o.s("personBorder") + `;
    fill: ` + o.s("personBkg") + `;
  }
` + c4ElementFontStyles(o) + `

  /* The element font colour is set inline per element (default white); the
     label text takes it via currentColor. */
  .c4-shape .label,
  .c4-shape .label text {
    color: inherit;
    fill: currentColor;
  }
  /* Structurizr typography: bold name, smaller stereotype/type and description lines. */
  .c4-shape .label .c4-name {
    font-weight: bold;
  }
  .c4-shape .label .c4-type {
    font-size: 0.75em;
  }
  .c4-shape .label .c4-descr {
    font-size: 0.82em;
  }
  .c4-shape .basic,
  .c4-shape rect,
  .c4-shape path,
  .c4-shape circle,
  .c4-shape ellipse,
  .c4-shape line {
    stroke-width: 2px;
  }
`
}
