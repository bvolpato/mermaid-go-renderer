// Ported from mermaid 12.2.0, mermaidAPI.ts (createCssStyles,
// cssImportantStyles) (MIT).

package render

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/css"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/styles"
)

// cssImportantStyles builds the rule text for one classDef and one element.
func cssImportantStyles(cssClass, element string, cssClasses []string) string {
	declarationBlock := config.SanitizeCss("{ " + strings.Join(cssClasses, " !important; ") + " !important; }")
	return "." + cssClass + " " + element + " " + declarationBlock
}

// cssRuleText is the cssText of a style rule after CSSStyleSheet.insertRule:
// the browser parses the rule and prints it in canonical form.
func cssRuleText(rule string) (string, bool) {
	open := strings.IndexByte(rule, '{')
	closeIdx := strings.LastIndexByte(rule, '}')
	if open < 0 || closeIdx < open {
		return "", false
	}
	selector := strings.Join(strings.Fields(rule[:open]), " ")
	decls := css.ParseInlineStyle(rule[open+1:closeIdx], false)
	body := css.SerializeInlineStyle(decls)
	if body == "" {
		return selector + " { }", true
	}
	return selector + " { " + body + " }", true
}

// createCssStyles is createCssStyles(config, classDefs): the CSS text for
// the classDefs of the diagram, in the form that the CSSOM prints.
func createCssStyles(cfg *js.Obj[any], classDefs *js.Map[string, *ClassDef]) string {
	var rules []string
	if classDefs != nil {
		cssElements := []string{"rect", "polygon", "ellipse", "circle", "path"}
		if config.GetEffectiveHtmlLabels(cfg) {
			cssElements = []string{"> *", "span"}
		}
		classDefs.Each(func(_ string, def *ClassDef) {
			if len(def.Styles) > 0 {
				for _, el := range cssElements {
					if text, ok := cssRuleText(cssImportantStyles(def.ID, el, def.Styles)); ok {
						rules = append(rules, text)
					}
				}
			}
			if len(def.TextStyles) > 0 {
				textStyles := make([]string, len(def.TextStyles))
				for i, s := range def.TextStyles {
					textStyles[i] = strings.Replace(s, "color", "fill", 1)
				}
				if text, ok := cssRuleText(cssImportantStyles(def.ID, "tspan", textStyles)); ok {
					rules = append(rules, text)
				}
			}
		})
	}
	// themeCSS goes through CSSStyleSheet.replaceSync, and the rules print in
	// canonical form.
	var themeCSS *string
	if v := cfg.Get("themeCSS"); v != nil {
		text := strings.Join(styleSheetRules(js.ToString(v)), "\n")
		themeCSS = &text
	}
	return styles.CreateCssStyles(cfg, themeCSS, rules)
}
