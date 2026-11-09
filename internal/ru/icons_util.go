// Ported from @iconify/utils 3.0.2. That is the version that the build of
// mermaid 12.2.0 contains (dist/chunks/mermaid.esm/chunk-5QBFSAUC.mjs), and
// it is not the version in tools/reference/node_modules/@iconify/utils. The
// files are lib/icon/defaults.js, lib/customisations/defaults.js,
// lib/icon/name.js, lib/icon/transformations.js, lib/icon/merge.js,
// lib/icon-set/tree.js, lib/icon-set/get-icon.js, lib/svg/size.js,
// lib/svg/defs.js, lib/svg/build.js, lib/svg/id.js and lib/svg/html.js.
//
// An icon set is JSON. Its values keep the dynamic model of internal/js, so
// that a value of an unexpected type behaves as it does in JavaScript.

package ru

import (
	"errors"
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// errIconType stands for the TypeError that upstream throws when a value
// of an icon set has a type that the code cannot use.
var errIconType = errors.New("TypeError: invalid value in icon data")

// defaultIconTransformations is defaultIconTransformations of defaults.js.
func defaultIconTransformations() *js.Obj[any] {
	o := js.NewObj[any]()
	o.Set("rotate", 0.0)
	o.Set("vFlip", false)
	o.Set("hFlip", false)
	return o
}

// defaultIconProps is defaultIconProps of defaults.js.
func defaultIconProps() *js.Obj[any] {
	o := js.NewObj[any]()
	o.Set("left", 0.0)
	o.Set("top", 0.0)
	o.Set("width", 16.0)
	o.Set("height", 16.0)
	o.Set("rotate", 0.0)
	o.Set("vFlip", false)
	o.Set("hFlip", false)
	return o
}

// defaultExtendedIconPropKeys are the keys of defaultExtendedIconProps, in
// their order.
var defaultExtendedIconPropKeys = []string{"left", "top", "width", "height", "rotate", "vFlip", "hFlip", "body", "hidden"}

// defaultIconCustomisations is defaultIconCustomisations of
// customisations/defaults.js.
func defaultIconCustomisations() *js.Obj[any] {
	o := js.NewObj[any]()
	o.Set("width", js.Null)
	o.Set("height", js.Null)
	o.Set("rotate", 0.0)
	o.Set("vFlip", false)
	o.Set("hFlip", false)
	return o
}

// iconifyIconName is IconifyIconName.
type iconifyIconName struct {
	provider string
	prefix   string
	name     string
}

// stringToIcon is stringToIcon(value, validate, allowSimpleName). The
// second result is false where upstream returns null.
func stringToIcon(value string, validate, allowSimpleName bool) (iconifyIconName, bool) {
	provider := ""
	colonSeparated := strings.Split(value, ":")
	if strings.HasPrefix(value, "@") {
		if len(colonSeparated) < 2 || len(colonSeparated) > 3 {
			return iconifyIconName{}, false
		}
		provider = colonSeparated[0][1:]
		colonSeparated = colonSeparated[1:]
	}
	if len(colonSeparated) > 3 || len(colonSeparated) == 0 {
		return iconifyIconName{}, false
	}
	if len(colonSeparated) > 1 {
		name := colonSeparated[len(colonSeparated)-1]
		prefix := colonSeparated[len(colonSeparated)-2]
		colonSeparated = colonSeparated[:len(colonSeparated)-2]
		result := iconifyIconName{provider: provider, prefix: prefix, name: name}
		if len(colonSeparated) > 0 {
			result.provider = colonSeparated[0]
		}
		if validate && !validateIconName(result, false) {
			return iconifyIconName{}, false
		}
		return result, true
	}
	name := colonSeparated[0]
	dashSeparated := strings.Split(name, "-")
	if len(dashSeparated) > 1 {
		result := iconifyIconName{provider: provider, prefix: dashSeparated[0], name: strings.Join(dashSeparated[1:], "-")}
		if validate && !validateIconName(result, false) {
			return iconifyIconName{}, false
		}
		return result, true
	}
	if allowSimpleName && provider == "" {
		result := iconifyIconName{provider: provider, prefix: "", name: name}
		if validate && !validateIconName(result, allowSimpleName) {
			return iconifyIconName{}, false
		}
		return result, true
	}
	return iconifyIconName{}, false
}

// validateIconName is validateIconName(icon, allowSimpleName).
func validateIconName(icon iconifyIconName, allowSimpleName bool) bool {
	return ((allowSimpleName && icon.prefix == "") || icon.prefix != "") && icon.name != ""
}

// objectPrototypeMembers are the properties that a plain JavaScript object
// inherits. Upstream reads icons[name] and aliases[name] from plain
// objects, so these names give a function, which is truthy.
var objectPrototypeMembers = map[string]bool{
	"constructor": true, "__defineGetter__": true, "__defineSetter__": true, "hasOwnProperty": true,
	"__lookupGetter__": true, "__lookupSetter__": true, "isPrototypeOf": true, "propertyIsEnumerable": true,
	"toString": true, "valueOf": true, "__proto__": true, "toLocaleString": true,
}

// inheritedMember stands for a function or an object that a plain object
// inherits from Object.prototype. It has none of the properties of an icon.
type inheritedMember struct{}

// iconSetMember is obj[name] for a plain object. With inherited set, the
// object has Object.prototype, as an object from JSON has.
func iconSetMember(obj *js.Obj[any], name string, inherited bool) any {
	if obj != nil {
		if v, ok := obj.Lookup(name); ok {
			return v
		}
	}
	if inherited && objectPrototypeMembers[name] {
		return inheritedMember{}
	}
	return nil
}

// iconObject returns a value as the object that mergeIconData reads. An
// array and an inherited member have no icon properties. A primitive value
// is an error: upstream throws a TypeError for `key in value`.
func iconObject(v any) (*js.Obj[any], error) {
	switch x := v.(type) {
	case *js.Obj[any]:
		if x == nil {
			return nil, errIconType
		}
		return x, nil
	case []any, inheritedMember:
		return js.NewObj[any](), nil
	}
	return nil, errIconType
}

// jsAdd is the + operator of JavaScript for dynamic values.
func jsAdd(a, b any) any {
	prim := func(v any) any {
		switch v.(type) {
		case []any, *js.Obj[any], inheritedMember:
			return js.ToString(v)
		}
		return v
	}
	a, b = prim(a), prim(b)
	_, aStr := a.(string)
	_, bStr := b.(string)
	if aStr || bStr {
		return js.ToString(a) + js.ToString(b)
	}
	return js.ToNumber(a) + js.ToNumber(b)
}

// jsStrictEquals is the === operator for dynamic values. Two arrays or two
// objects are equal only when they are the same object.
func jsStrictEquals(a, b any) bool {
	switch x := a.(type) {
	case nil:
		return b == nil
	case bool:
		y, ok := b.(bool)
		return ok && x == y
	case float64:
		y, ok := b.(float64)
		return ok && x == y
	case string:
		y, ok := b.(string)
		return ok && x == y
	case *js.Obj[any]:
		y, ok := b.(*js.Obj[any])
		return ok && x == y
	case []any:
		return false
	}
	if js.IsNullish(a) && a != nil {
		return js.IsNullish(b) && b != nil
	}
	return false
}

// jsOr0 is `v || 0`.
func jsOr0(v any) any {
	if js.Truthy(v) {
		return v
	}
	return 0.0
}

// jsToStringMethod is value.toString(). Upstream throws a TypeError for
// undefined and null.
func jsToStringMethod(v any) (string, error) {
	if js.IsNullish(v) {
		return "", errIconType
	}
	if _, ok := v.(inheritedMember); ok {
		return "", errIconType
	}
	return js.ToString(v), nil
}

// mergeIconTransformations is mergeIconTransformations(obj1, obj2).
func mergeIconTransformations(obj1, obj2 *js.Obj[any]) *js.Obj[any] {
	result := js.NewObj[any]()
	if !js.Truthy(obj1.Get("hFlip")) != !js.Truthy(obj2.Get("hFlip")) {
		result.Set("hFlip", true)
	}
	if !js.Truthy(obj1.Get("vFlip")) != !js.Truthy(obj2.Get("vFlip")) {
		result.Set("vFlip", true)
	}
	rotate := math.Mod(js.ToNumber(jsAdd(jsOr0(obj1.Get("rotate")), jsOr0(obj2.Get("rotate")))), 4)
	if js.Truthy(rotate) {
		result.Set("rotate", rotate)
	}
	return result
}

// mergeIconData is mergeIconData(parent, child): it merges an icon and an
// alias, or default values and an icon.
func mergeIconData(parent any, child *js.Obj[any]) (*js.Obj[any], error) {
	p, err := iconObject(parent)
	if err != nil {
		return nil, err
	}
	result := mergeIconTransformations(p, child)
	transformations := defaultIconTransformations()
	for _, key := range defaultExtendedIconPropKeys {
		if transformations.Has(key) {
			if p.Has(key) && !result.Has(key) {
				result.Set(key, transformations.Get(key))
			}
		} else if child.Has(key) {
			result.Set(key, child.Get(key))
		} else if p.Has(key) {
			result.Set(key, p.Get(key))
		}
	}
	return result, nil
}

// iconSetParts returns data.icons and data.aliases. hasAliases is false
// when upstream uses Object.create(null) in place of data.aliases.
func iconSetParts(data *js.Obj[any]) (icons, aliases *js.Obj[any], hasAliases bool) {
	icons = js.AsObj(data.Get("icons"))
	if v := data.Get("aliases"); js.Truthy(v) {
		return icons, js.AsObj(v), true
	}
	return icons, nil, false
}

// getIconsTree is getIconsTree(data, [name])[name]: the parents of an
// icon. The second result is false where upstream gives null.
func getIconsTree(data *js.Obj[any], name string) ([]string, bool) {
	icons, aliases, hasAliases := iconSetParts(data)
	type entry struct {
		tree []string
		ok   bool
	}
	resolved := map[string]entry{}
	var resolve func(name string) ([]string, bool)
	resolve = func(name string) ([]string, bool) {
		if js.Truthy(iconSetMember(icons, name, true)) {
			resolved[name] = entry{tree: []string{}, ok: true}
			return []string{}, true
		}
		if _, seen := resolved[name]; !seen {
			resolved[name] = entry{}
			// const parent = aliases[name] && aliases[name].parent
			parent := iconSetMember(aliases, name, hasAliases)
			if js.Truthy(parent) {
				// A value that is not an object has no parent property.
				if o := js.AsObj(parent); o != nil {
					parent = o.Get("parent")
				} else {
					parent = nil
				}
			}
			if js.Truthy(parent) {
				parentName := js.ToString(parent)
				if value, ok := resolve(parentName); ok {
					resolved[name] = entry{tree: append([]string{parentName}, value...), ok: true}
				}
			}
		}
		e := resolved[name]
		return e.tree, e.ok
	}
	return resolve(name)
}

// internalGetIconData is internalGetIconData(data, name, tree).
func internalGetIconData(data *js.Obj[any], name string, tree []string) (*js.Obj[any], error) {
	icons, aliases, hasAliases := iconSetParts(data)
	currentProps := js.NewObj[any]()
	parse := func(name string) error {
		// icons[name] || aliases[name]
		v := iconSetMember(icons, name, true)
		if !js.Truthy(v) {
			v = iconSetMember(aliases, name, hasAliases)
		}
		merged, err := mergeIconData(v, currentProps)
		if err != nil {
			return err
		}
		currentProps = merged
		return nil
	}
	if err := parse(name); err != nil {
		return nil, err
	}
	for _, parent := range tree {
		if err := parse(parent); err != nil {
			return nil, err
		}
	}
	return mergeIconData(data, currentProps)
}

// getIconData is getIconData(data, name). The result is nil where upstream
// returns null.
func getIconData(data *js.Obj[any], name string) (*js.Obj[any], error) {
	icons, _, _ := iconSetParts(data)
	if icons == nil {
		// data.icons[name] of a missing icons object is a TypeError.
		return nil, errIconType
	}
	if js.Truthy(iconSetMember(icons, name, true)) {
		return internalGetIconData(data, name, nil)
	}
	tree, ok := getIconsTree(data, name)
	if !ok {
		return nil, nil
	}
	return internalGetIconData(data, name, tree)
}

// isUnitsChar reports whether c is in the class [0-9.].
func isUnitsChar(c byte) bool { return c == '.' || (c >= '0' && c <= '9') }

// unitsSplit is size.split(/(-?[0-9.]*[0-9]+[0-9.]*)/g): texts and numbers
// in turn. The first and the last part are texts, which can be empty.
func unitsSplit(size string) []string {
	var parts []string
	last := 0
	for i := 0; i < len(size); {
		start := i
		j := i
		if size[j] == '-' {
			j++
		}
		// The expression matches a run of [0-9.] that has a digit.
		end := j
		hasDigit := false
		for end < len(size) && isUnitsChar(size[end]) {
			if size[end] != '.' {
				hasDigit = true
			}
			end++
		}
		if !hasDigit {
			i++
			continue
		}
		parts = append(parts, size[last:start], size[start:end])
		last = end
		i = end
	}
	return append(parts, size[last:])
}

// calculateSize is calculateSize(size, ratio) with the default precision.
func calculateSize(size any, ratio float64) any {
	if ratio == 1 {
		return size
	}
	const precision = 100.0
	switch s := size.(type) {
	case float64:
		return math.Ceil(s*ratio*precision) / precision
	case string:
		oldParts := unitsSplit(s)
		var newParts strings.Builder
		// The first part is never a number: a number is always a match of
		// the split expression.
		isNumber := false
		for _, code := range oldParts {
			if isNumber {
				num := js.ParseFloat(code)
				if num != num {
					newParts.WriteString(code)
				} else {
					newParts.WriteString(js.Str(math.Ceil(num*ratio*precision) / precision))
				}
			} else {
				newParts.WriteString(code)
			}
			isNumber = !isNumber
		}
		return newParts.String()
	}
	return size
}

// errIconDefs reports content for which splitSVGDefs of upstream does not
// end: its loop then runs for ever.
var errIconDefs = errors.New("icon body has a </defs> before its <defs>: upstream does not end")

// splitSVGDefs is splitSVGDefs(content) for the tag "defs". The indexes
// count UTF-16 code units, as in JavaScript: upstream finds the index of
// the first tag once and uses it again after it changed the content, so
// the unit of the index matters.
func splitSVGDefs(text string) (defs, rest string, err error) {
	const tag = "defs"
	content := js.UTF16Units(text)
	// indexOf is content.indexOf(sub, from) for an ASCII sub.
	indexOf := func(sub string, from int) int {
		for i := max(from, 0); i+len(sub) <= len(content); i++ {
			match := true
			for k := 0; k < len(sub); k++ {
				if content[i+k] != rune(sub[k]) {
					match = false
					break
				}
			}
			if match {
				return i
			}
		}
		return -1
	}
	// slice is content.slice(from, to) as a string.
	slice := func(from, to int) string {
		from, to = min(from, len(content)), min(to, len(content))
		if from >= to {
			return ""
		}
		return js.FromUTF16Units(content[from:to])
	}
	index := indexOf("<"+tag, 0)
	// A pass that removes the tags makes the content shorter. A pass for
	// a closing tag that is before the first opening tag keeps the tags,
	// and then the loop of upstream can go on for ever: the content comes
	// back to an earlier value, or it grows without limit.
	limit := 64*len(content) + 4096
	var seen map[string]bool
	for index >= 0 {
		start := indexOf(">", index)
		end := indexOf("</"+tag, 0)
		if start == -1 || end == -1 {
			break
		}
		endEnd := indexOf(">", end)
		if endEnd == -1 {
			break
		}
		if endEnd < index {
			key := js.FromUTF16Units(content)
			if seen[key] || len(content) > limit {
				return "", "", errIconDefs
			}
			if seen == nil {
				seen = map[string]bool{}
			}
			seen[key] = true
		}
		defs += js.TrimSpace(slice(start+1, end))
		content = js.UTF16Units(js.TrimSpace(slice(0, index)) + slice(endEnd+1, len(content)))
	}
	return defs, js.FromUTF16Units(content), nil
}

// mergeDefsAndContent is mergeDefsAndContent(defs, content).
func mergeDefsAndContent(defs, content string) string {
	if defs != "" {
		return "<defs>" + defs + "</defs>" + content
	}
	return content
}

// wrapSVGContent is wrapSVGContent(body, start, end): it wraps the content
// and keeps the definitions outside.
func wrapSVGContent(body, start, end string) (string, error) {
	defs, content, err := splitSVGDefs(body)
	if err != nil {
		return "", err
	}
	return mergeDefsAndContent(defs, start+content+end), nil
}

// isUnsetKeyword is isUnsetKeyword(value).
func isUnsetKeyword(value any) bool {
	s, ok := value.(string)
	return ok && (s == "unset" || s == "undefined" || s == "none")
}

// iconBuildResult is IconifyIconBuildResult.
type iconBuildResult struct {
	attributes *js.Obj[string]
	viewBox    []any
	// body is a string for a valid icon. Upstream passes on a body of
	// another type when no transformation wraps it.
	body any
}

// iconToSVG is iconToSVG(icon, customisations): the attributes and the
// content of the svg element of an icon.
func iconToSVG(icon, customisations *js.Obj[any]) (*iconBuildResult, error) {
	fullIcon := defaultIconProps()
	icon.Each(func(k string, v any) { fullIcon.Set(k, v) })
	fullCustomisations := defaultIconCustomisations()
	if customisations != nil {
		customisations.Each(func(k string, v any) { fullCustomisations.Set(k, v) })
	}

	boxLeft, boxTop := fullIcon.Get("left"), fullIcon.Get("top")
	boxWidth, boxHeight := fullIcon.Get("width"), fullIcon.Get("height")
	body := fullIcon.Get("body")

	half := func(v any) float64 { return js.ToNumber(v) / 2 }
	for _, props := range []*js.Obj[any]{fullIcon, fullCustomisations} {
		var transformations []string
		hFlip := props.Get("hFlip")
		vFlip := props.Get("vFlip")
		rotation := props.Get("rotate")
		if js.Truthy(hFlip) {
			if js.Truthy(vFlip) {
				rotation = jsAdd(rotation, 2.0)
			} else {
				x, err := jsToStringMethod(jsAdd(boxWidth, boxLeft))
				if err != nil {
					return nil, err
				}
				transformations = append(transformations, "translate("+x+" "+js.Str(0-js.ToNumber(boxTop))+")")
				transformations = append(transformations, "scale(-1 1)")
				boxTop, boxLeft = 0.0, 0.0
			}
		} else if js.Truthy(vFlip) {
			y, err := jsToStringMethod(jsAdd(boxHeight, boxTop))
			if err != nil {
				return nil, err
			}
			transformations = append(transformations, "translate("+js.Str(0-js.ToNumber(boxLeft))+" "+y+")")
			transformations = append(transformations, "scale(1 -1)")
			boxTop, boxLeft = 0.0, 0.0
		}
		if js.ToNumber(rotation) < 0 {
			r := js.ToNumber(rotation)
			rotation = r - float64(math.Floor(r/4)*4)
		}
		rot := math.Mod(js.ToNumber(rotation), 4)
		unshift := func(s string) { transformations = append([]string{s}, transformations...) }
		switch rot {
		case 1:
			tempValue, err := jsToStringMethod(jsAdd(half(boxHeight), boxTop))
			if err != nil {
				return nil, err
			}
			unshift("rotate(90 " + tempValue + " " + tempValue + ")")
		case 2:
			x, err := jsToStringMethod(jsAdd(half(boxWidth), boxLeft))
			if err != nil {
				return nil, err
			}
			y, err := jsToStringMethod(jsAdd(half(boxHeight), boxTop))
			if err != nil {
				return nil, err
			}
			unshift("rotate(180 " + x + " " + y + ")")
		case 3:
			tempValue, err := jsToStringMethod(jsAdd(half(boxWidth), boxLeft))
			if err != nil {
				return nil, err
			}
			unshift("rotate(-90 " + tempValue + " " + tempValue + ")")
		}
		if math.Mod(rot, 2) == 1 {
			if !jsStrictEquals(boxLeft, boxTop) {
				boxLeft, boxTop = boxTop, boxLeft
			}
			if !jsStrictEquals(boxWidth, boxHeight) {
				boxWidth, boxHeight = boxHeight, boxWidth
			}
		}
		if len(transformations) > 0 {
			// wrapSVGContent calls string methods of the body. A body that
			// is not a string is a TypeError upstream. An array is the
			// exception: it has some of these methods, so upstream can
			// wrap it. This port gives an error for an array too.
			content, ok := body.(string)
			if !ok {
				return nil, errIconType
			}
			wrapped, err := wrapSVGContent(content, `<g transform="`+strings.Join(transformations, " ")+`">`, "</g>")
			if err != nil {
				return nil, err
			}
			body = wrapped
		}
	}

	customisationsWidth := fullCustomisations.Get("width")
	customisationsHeight := fullCustomisations.Get("height")
	isNull := func(v any) bool { return v != nil && js.IsNullish(v) }
	isAuto := func(v any) bool { s, ok := v.(string); return ok && s == "auto" }
	var width, height any
	if isNull(customisationsWidth) {
		switch {
		case isNull(customisationsHeight):
			height = "1em"
		case isAuto(customisationsHeight):
			height = boxHeight
		default:
			height = customisationsHeight
		}
		width = calculateSize(height, js.ToNumber(boxWidth)/js.ToNumber(boxHeight))
	} else {
		if isAuto(customisationsWidth) {
			width = boxWidth
		} else {
			width = customisationsWidth
		}
		switch {
		case isNull(customisationsHeight):
			height = calculateSize(width, js.ToNumber(boxHeight)/js.ToNumber(boxWidth))
		case isAuto(customisationsHeight):
			height = boxHeight
		default:
			height = customisationsHeight
		}
	}

	attributes := js.NewObj[string]()
	setAttr := func(prop string, value any) error {
		if isUnsetKeyword(value) {
			return nil
		}
		s, err := jsToStringMethod(value)
		if err != nil {
			return err
		}
		attributes.Set(prop, s)
		return nil
	}
	if err := setAttr("width", width); err != nil {
		return nil, err
	}
	if err := setAttr("height", height); err != nil {
		return nil, err
	}
	viewBox := []any{boxLeft, boxTop, boxWidth, boxHeight}
	attributes.Set("viewBox", jsArrayJoin(viewBox, " "))
	return &iconBuildResult{attributes: attributes, viewBox: viewBox, body: body}, nil
}

// jsArrayJoin is Array.prototype.join: undefined and null give "".
func jsArrayJoin(values []any, sep string) string {
	parts := make([]string, len(values))
	for i, v := range values {
		if !js.IsNullish(v) {
			parts[i] = js.ToString(v)
		}
	}
	return strings.Join(parts, sep)
}

// isRegExpSpace reports whether r matches \s of a JavaScript expression.
func isRegExpSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

// findIDs returns the first group of every match of /\sid="(\S+)"/g.
func findIDs(body string) []string {
	var ids []string
	r := []rune(body)
	prefix := []rune(`id="`)
	for i := 0; i < len(r); i++ {
		if !isRegExpSpace(r[i]) || i+1+len(prefix) > len(r) || string(r[i+1:i+1+len(prefix)]) != string(prefix) {
			continue
		}
		start := i + 1 + len(prefix)
		end := start
		for end < len(r) && !isRegExpSpace(r[end]) {
			end++
		}
		// \S+ is greedy: the id ends at the last quote of the run.
		quote := -1
		for q := end - 1; q > start; q-- {
			if r[q] == '"' {
				quote = q
				break
			}
		}
		if quote < 0 {
			continue
		}
		ids = append(ids, string(r[start:quote]))
		i = quote
	}
	return ids
}

// replaceIDReferences is body.replace(new RegExp('([#;"])(' + escapedID +
// ')([")]|\\.[a-z])', 'g'), '$1' + newID + '$3').
func replaceIDReferences(body, id, newID string) string {
	var out strings.Builder
	for i := 0; i < len(body); {
		c := body[i]
		if (c == '#' || c == ';' || c == '"') && strings.HasPrefix(body[i+1:], id) {
			j := i + 1 + len(id)
			if j < len(body) && (body[j] == '"' || body[j] == ')') {
				out.WriteByte(c)
				out.WriteString(newID)
				out.WriteByte(body[j])
				i = j + 1
				continue
			}
			if j+1 < len(body) && body[j] == '.' && body[j+1] >= 'a' && body[j+1] <= 'z' {
				out.WriteByte(c)
				out.WriteString(newID)
				out.WriteString(body[j : j+2])
				i = j + 2
				continue
			}
		}
		out.WriteByte(c)
		i++
	}
	return out.String()
}

// firstRandomDraw is the first number of the generator that stands for
// Math.random, for the seed of the reference oracle.
func firstRandomDraw() float64 {
	rng := render.DefaultRandomSeed + 0x6d2b79f5
	t := rng
	t = (t ^ (t >> 15)) * (t | 1)
	t ^= t + (t^(t>>7))*(t|61)
	return float64(t^(t>>14)) / 4294967296
}

// iconIDPrefix is randomPrefix of lib/svg/id.js. Upstream computes it once,
// when the bundle loads: 'IconifyId' + Date.now().toString(16) +
// (Math.random() * 16777216 | 0).toString(16). That call of Math.random is
// the one that the page makes before a render starts, so it is the first
// draw of the generator.
func iconIDPrefix(ctx *render.Context) string {
	return ctx.Value("ru.iconify.randomPrefix", func() any {
		now := js.ToStringRadix(float64(ctx.Now.UnixMilli()), 16)
		random := js.ToStringRadix(float64(js.ToInt32(firstRandomDraw()*16777216)), 16)
		return "IconifyId" + now + random
	}).(string)
}

// SetIconIDPrefix sets the prefix of the ids that replaceIDs makes. Use it
// when the page computed randomPrefix from another clock or another random
// number than the reference oracle does. Call it before the first icon.
func SetIconIDPrefix(ctx *render.Context, prefix string) {
	ctx.Value("ru.iconify.randomPrefix", func() any { return prefix })
}

// iconIDCounter is the name of the counter of replaceIDs in the context.
const iconIDCounter = "ru.iconify.counter"

// SkipIconIDs advances the counter of replaceIDs by n, as if n ids had
// been replaced before. The counter of upstream belongs to the page: a
// program that puts several diagrams in one page can keep the ids of their
// icons apart with it.
func SkipIconIDs(ctx *render.Context, n int) {
	for i := 0; i < n; i++ {
		ctx.Counter(iconIDCounter)
	}
}

// replaceIDs is replaceIDs(body): it replaces the ids of an icon with
// unique ids. The counter of upstream is a module variable. Here it belongs
// to the render.
func replaceIDs(ctx *render.Context, body string) string {
	ids := findIDs(body)
	if len(ids) == 0 {
		return body
	}
	prefix := iconIDPrefix(ctx)
	// The suffix marks the new ids until all ids are replaced, so that a
	// new id is not replaced again.
	random := js.ToInt32(ctx.Random() * 16777216)
	now := js.ToInt32(float64(ctx.Now.UnixMilli()))
	suffix := "suffix" + js.ToStringRadix(float64(random|now), 16)
	for _, id := range ids {
		newID := prefix + js.Str(float64(ctx.Counter(iconIDCounter)))
		body = replaceIDReferences(body, id, newID+suffix)
	}
	return strings.ReplaceAll(body, suffix, "")
}

// iconToHTML is iconToHTML(body, attributes): the svg element of an icon.
func iconToHTML(body string, attributes *js.Obj[string]) string {
	renderAttribsHTML := ""
	if strings.Contains(body, "xlink:") {
		renderAttribsHTML = ` xmlns:xlink="http://www.w3.org/1999/xlink"`
	}
	attributes.Each(func(attr, value string) {
		renderAttribsHTML += " " + attr + `="` + value + `"`
	})
	return `<svg xmlns="http://www.w3.org/2000/svg"` + renderAttribsHTML + ">" + body + "</svg>"
}
