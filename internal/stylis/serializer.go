// Ported from stylis/src/Serializer.js and stylis/src/Middleware.js (stylis 4.3.6).

package stylis

import "strings"

// Callback is the function that Serialize calls for each element. It returns
// the output for the element. An empty string means no output.
type Callback func(element *Element, index int, children []*Element, callback Callback) string

// Serialize calls callback for each element and joins the results.
func Serialize(children []*Element, callback Callback) string {
	var output strings.Builder
	for i := 0; i < len(children); i++ {
		output.WriteString(callback(children[i], i, children, callback))
	}
	return output.String()
}

// Stringify is the callback that prints an element as minified CSS.
func Stringify(element *Element, index int, children []*Element, callback Callback) string {
	switch element.Type {
	case LAYER:
		if len(element.Children) != 0 {
			break
		}
		return stringifyValue(element)
	case IMPORT, NAMESPACE, DECLARATION:
		return stringifyValue(element)
	case COMMENT:
		return ""
	case KEYFRAMES:
		element.Return = element.Value + "{" + Serialize(element.Children, callback) + "}"
		return element.Return
	case RULESET:
		element.Value = strings.Join(element.Props, ",")
		if len(element.Value) == 0 {
			return ""
		}
	}

	if body := Serialize(element.Children, callback); len(body) != 0 {
		element.Return = element.Value + "{" + body + "}"
		return element.Return
	}
	return ""
}

// stringifyValue is element.return = element.return || element.value.
func stringifyValue(element *Element) string {
	if element.Return == "" {
		element.Return = element.Value
	}
	return element.Return
}

// Middleware combines callbacks into one. The combined callback calls each
// one in order and joins the results.
func Middleware(collection ...Callback) Callback {
	length := len(collection)

	return func(element *Element, index int, children []*Element, callback Callback) string {
		output := ""

		for i := 0; i < length; i++ {
			output += collection[i](element, index, children, callback)
		}

		return output
	}
}
