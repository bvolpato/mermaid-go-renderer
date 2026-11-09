// Ported from mermaid 12.2.0, mermaid/src/mermaidAPI.ts (compileCSS and its
// addNamespace middleware) (MIT).

package stylis

import "strings"

// CompileCSS compiles css so that it applies only inside namespace, for
// example "#idOfSvgElement". It removes nesting, minifies the text and drops
// the at-rules that cannot have a namespace.
func CompileCSS(namespace, css string) string {
	return Serialize(
		Compile(namespace+"{"+css+"}"),
		Middleware(AddNamespace(namespace), Stringify),
	)
}

// allowedProps are the declarations that the root selector can keep without
// a namespace in front of it.
var allowedProps = map[string]bool{
	"font-family": true,
	"font-size":   true,
	"fill":        true,
}

// allowedAtRules are the at-rules that stay in the output. The first six can
// hold nested rules. Mermaid needs keyframes for its animations.
var allowedAtRules = map[string]bool{
	MEDIA:             true,
	SUPPORTS:          true,
	LAYER:             true,
	SCOPE:             true,
	"@container":      true,
	"@starting-style": true,
	KEYFRAMES:         true,
}

// AddNamespace returns the middleware that puts namespace in front of every
// selector that does not have it. It changes an at-rule that is not allowed
// into a comment, so that Stringify drops it.
func AddNamespace(namespace string) Callback {
	return func(element *Element, _ int, _ []*Element, _ Callback) string {
		if element.Type == RULESET {
			if element.Parent != nil && element.Parent.Type == KEYFRAMES {
				// A keyframe rule has no selector.
				return ""
			}
			props := make([]string, len(element.Props))
			for i, prop := range element.Props {
				// The root selector can keep a few inherited properties without
				// the namespace.
				if prop == namespace && everyChildIsAllowed(element) {
					props[i] = prop
					continue
				}

				alreadyNamespaced := (strings.HasPrefix(prop, namespace+" ") || strings.HasPrefix(prop, namespace+">")) &&
					// The column combinator is not compressed to "namespace||".
					!strings.HasPrefix(prop, namespace+" ||")
				if !alreadyNamespaced {
					props[i] = namespace + " " + prop
					continue
				}
				props[i] = prop
			}
			element.Props = props
		} else if strings.HasPrefix(element.Type, "@") {
			if !allowedAtRules[element.Type] {
				element.Type = COMMENT
			}
		}
		return ""
	}
}

// everyChildIsAllowed reports whether each child is a declaration of an
// allowed property.
func everyChildIsAllowed(element *Element) bool {
	for _, child := range element.Children {
		if child.Type != DECLARATION {
			return false
		}
		if !allowedProps[child.Prop] {
			return false
		}
	}
	return true
}
