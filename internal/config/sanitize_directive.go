// Ported from packages/mermaid/src/utils/sanitizeDirective.ts (mermaid 12.2.0).

package config

import (
	"regexp"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// jsSpaceChars is the content of the character class \s of a JavaScript
// regular expression. The Go class \s is ASCII only.
const jsSpaceChars = `\t\n\v\f\r \x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}`

// Dictionary-style configs have arbitrary user-defined keys, so instead of
// checking the keys against configKeys, their values are validated against a
// pattern (and suspicious keys are dropped).
//
// The patterns spell out both letter cases instead of using (?i): the Go flag
// folds 's' with U+017F and 'k' with U+212A, and JavaScript does not.
var dictionaryConfigPatterns = map[string]*regexp.Regexp{
	// CSS colors (sankey):
	// /^#[\da-f]{3,8}$|^rgb\([\d\s%,.]+\)$|^hsl\([\d\s%,.]+\)$|^[a-z]+$/i
	"nodeColors": regexp.MustCompile(`^#[\dA-Fa-f]{3,8}$|^[rR][gG][bB]\([\d` + jsSpaceChars + `%,.]+\)$|^[hH][sS][lL]\([\d` + jsSpaceChars + `%,.]+\)$|^[A-Za-z]+$`),
	// iconify icon references (treeView filenameIcons/extensionIcons):
	// /^[\w-]+(?::[\w-]+)?$/
	"filenameIcons":  regexp.MustCompile(`^[\w-]+(?::[\w-]+)?$`),
	"extensionIcons": regexp.MustCompile(`^[\w-]+(?::[\w-]+)?$`),
}

// themeVariableRe is /^[\d "#%(),.;A-Za-z]+$/.
var themeVariableRe = regexp.MustCompile(`^[\d "#%(),.;A-Za-z]+$`)

// deleteProperty is delete container[key]. Deleting an element of an array
// leaves a hole, which reads as undefined.
func deleteProperty(container any, key string) {
	switch c := container.(type) {
	case *js.Obj[any]:
		c.Delete(key)
	case []any:
		if i, ok := js.IsArrayIndex(key); ok && int(i) < len(c) {
			c[i] = nil
		}
	}
}

// setProperty is container[key] = value for an object or an array element.
func setProperty(container any, key string, value any) {
	switch c := container.(type) {
	case *js.Obj[any]:
		c.Set(key, value)
	case []any:
		if i, ok := js.IsArrayIndex(key); ok && int(i) < len(c) {
			c[i] = value
		}
	}
}

func sanitizeDictionaryConfig(dict any, valuePattern *regexp.Regexp) {
	keys, _ := js.OwnKeys(dict)
	for _, key := range keys {
		value, _ := js.GetProperty(dict, key)
		s, isString := value.(string)
		if strings.HasPrefix(key, "__") ||
			strings.Contains(key, "proto") ||
			strings.Contains(key, "constr") ||
			!isString ||
			!valuePattern.MatchString(s) {
			deleteProperty(dict, key)
		}
	}
}

// SanitizeDirective sanitizes directive objects (sanitizeDirective). It
// modifies args in place. The error is a *Error when the JavaScript code
// throws a TypeError, for example for a fontFamily that is a number.
func SanitizeDirective(args any) error {
	// Return if not an object
	if !js.IsObject(args) {
		return nil
	}

	// Sanitize each element if an array
	if arr, ok := args.([]any); ok {
		for _, arg := range arr {
			if err := SanitizeDirective(arg); err != nil {
				return err
			}
		}
		return nil
	}

	// Sanitize each key if an object
	obj := args.(*js.Obj[any])
	for _, key := range obj.Keys() {
		value := obj.Get(key)
		if strings.HasPrefix(key, "__") ||
			strings.Contains(key, "proto") ||
			strings.Contains(key, "constr") ||
			!IsConfigKey(key) ||
			js.IsNullish(value) {
			obj.Delete(key)
			continue
		}

		// Recurse if an object, but handle dictionary-style configs specially
		// (like nodeColors or filenameIcons) by validating their values instead
		if js.IsObject(value) {
			if valuePattern, ok := dictionaryConfigPatterns[key]; ok {
				sanitizeDictionaryConfig(value, valuePattern)
			} else if err := SanitizeDirective(value); err != nil {
				return err
			}
			continue
		}

		for _, cssKey := range []string{"themeCSS", "fontFamily", "altFontFamily"} {
			if strings.Contains(key, cssKey) {
				s, isString := obj.Get(key).(string)
				if !isString {
					// sanitizeCss iterates its argument.
					return typeError("str is not iterable")
				}
				obj.Set(key, SanitizeCss(s))
			}
		}
	}

	if themeVariables := obj.Get("themeVariables"); js.Truthy(themeVariables) {
		keys, _ := js.OwnKeys(themeVariables)
		for _, k := range keys {
			val, _ := js.GetProperty(themeVariables, k)
			if s, ok := val.(string); ok && !themeVariableRe.MatchString(s) {
				if _, isString := themeVariables.(string); isString {
					// A character of a string cannot be assigned in strict mode.
					return typeError("Cannot assign to read only property '%s' of string", k)
				}
				setProperty(themeVariables, k, "")
			}
		}
	}
	return nil
}

// SanitizeCss is sanitizeCss: it rejects CSS with unbalanced braces.
func SanitizeCss(str string) string {
	startCnt := 0
	endCnt := 0

	for _, element := range str {
		if startCnt < endCnt {
			return "{ /* ERROR: Unbalanced CSS */ }"
		}
		if element == '{' {
			startCnt++
		} else if element == '}' {
			endCnt++
		}
	}
	if startCnt != endCnt {
		return "{ /* ERROR: Unbalanced CSS */ }"
	}
	// Todo add more checks here
	return str
}
