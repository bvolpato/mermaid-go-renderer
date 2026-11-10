// Ported from the directive functions of packages/mermaid/src/utils.ts:
// detectInit, detectDirective and removeDirectives (mermaid 12.2.0).

package config

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"github.com/dlclark/regexp2"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// commentWithoutDirectives is the regular expression that detectDirective
// builds:
//
//	new RegExp(`[%]{2}(?![{]${directiveWithoutOpen.source})(?=[}][%]{2}).*\n`, 'ig')
//
// where directiveWithoutOpen is
// /\s*(?:(\w+)(?=:):|(\w+))\s*(?:(\w+)|((?:(?!}%{2}).|\r?\n)*))?\s*(?:}%{2})?/gi.
var commentWithoutDirectives = mustCompile(
	`[%]{2}(?![{]\s*(?:(\w+)(?=:):|(\w+))\s*(?:(\w+)|((?:(?!\}%{2})<dot>|\r?\n)*))?\s*(?:\}%{2})?)(?=[}][%]{2})<dot>*\n`)

// initDirectiveType is /(?:init\b)|(?:initialize\b)/, the type that detectInit
// looks for. The match is a search, so "reinit" matches too.
var initDirectiveType = regexp.MustCompile(`(?:init\b)|(?:initialize\b)`).MatchString

// wrapDirectiveType is the type 'wrap'. String.prototype.match turns the
// string into a regular expression, so the match is a search.
func wrapDirectiveType(s string) bool { return strings.Contains(s, "wrap") }

// directive is the Directive interface of utils.ts.
type directive struct {
	// typ is undefined when hasType is false.
	typ     string
	hasType bool
	args    any
}

// directiveScan holds directiveRegex.lastIndex. Upstream shares one global
// regular expression between the calls of detectDirective. An exception in
// the middle of a scan leaves lastIndex where it is, and the next scan starts
// there. The index counts runes.
type directiveScan struct {
	lastIndex  int
	includeZen bool
}

// group returns capture group n of a match and whether it took part.
func group(m *regexp2.Match, n int) (string, bool) {
	g := m.GroupByNumber(n)
	if g == nil || len(g.Captures) == 0 {
		return "", false
	}
	return g.String(), true
}

// detectDirective detects the directives of the given type in the text
// (detectDirective). The text can be single line or multiline.
//
// The second result is true when upstream returns an array: more than one
// directive matched. Otherwise the slice has one element: the directive that
// matched, or { type: text, args: null } when none did, or
// { type: undefined, args: null } when the arguments are not valid JSON.
func (scan *directiveScan) detectDirective(text string, typeMatches func(string) bool) ([]directive, bool) {
	text = strings.ReplaceAll(replaceAll(commentWithoutDirectives, commentMarker, js.TrimSpace(text)), "'", `"`)
	if !strings.Contains(text, directiveMarker) {
		// directiveRegex has no match: exec returns null and sets lastIndex
		// to 0.
		scan.lastIndex = 0
		return []directive{{typ: text, hasType: true, args: js.Null}}, false
	}
	runes := []rune(text)
	var result []directive
	for {
		// RegExp.prototype.exec with the g flag.
		if scan.lastIndex > len(runes) {
			scan.lastIndex = 0
			break
		}
		m, err := directiveRegex.FindRunesMatchStartingAt(runes, scan.lastIndex)
		if err != nil || m == nil {
			scan.lastIndex = 0
			break
		}
		scan.lastIndex = m.Index + m.Length
		// This is necessary to avoid infinite loops with zero-width matches
		if m.Length == 0 {
			scan.lastIndex++
		}
		g1, _ := group(m, 1)
		g2, _ := group(m, 2)
		if (g1 != "" && typeMatches(g1)) || (g2 != "" && typeMatches(g2)) {
			typ := g2
			if g1 != "" {
				typ = g1
			}
			var args any = js.Null
			if g3, _ := group(m, 3); g3 != "" {
				args = js.TrimSpace(g3)
			} else if g4, _ := group(m, 4); g4 != "" {
				parsed, err := parseJSON(js.TrimSpace(g4))
				if err != nil {
					// JSON.parse throws. The catch block returns this value
					// and leaves lastIndex as it is.
					return []directive{{args: js.Null}}, false
				}
				args = parsed
			}
			result = append(result, directive{typ: typ, hasType: true, args: args})
		}
	}
	if len(result) == 0 {
		return []directive{{typ: text, hasType: true, args: js.Null}}, false
	}
	return result, len(result) > 1
}

// parseJSON is JSON.parse. js.ParseJSON accepts text after the first value
// when that text is not JSON, so the whole input is validated first.
func parseJSON(text string) (any, error) {
	if !json.Valid([]byte(text)) {
		return nil, errors.New("JSON.parse: the text is not valid JSON")
	}
	return js.ParseJSON([]byte(text))
}

// detectInit detects the init config object from the text (detectInit).
// The result is the json object representing the init passed to
// mermaid.initialize(), or nil for undefined. It is a dynamic value: the
// argument of a directive can be any JSON value or a bare word.
//
// The error is an UnknownDiagramError when the text has an init directive and
// no diagram detector matches it.
func (scan *directiveScan) detectInit(text string) (any, error) {
	inits, isArray := scan.detectDirective(text, initDirectiveType)
	var results any

	if isArray {
		args := make([]any, len(inits))
		for i, init := range inits {
			args[i] = init.args
		}
		if err := SanitizeDirective(args); err != nil {
			return nil, err
		}
		results = assignWithDepth(js.NewObj[any](), args)
	} else {
		results = inits[0].args
	}

	if !js.Truthy(results) {
		return nil, nil
	}

	typ, err := detect(text, nil, scan.includeZen)
	if err != nil {
		return nil, err
	}

	// Move the `config` value to appropriate diagram type value
	const prop = "config"
	if value, _ := js.GetProperty(results, prop); value != nil {
		if typ == "flowchart-v2" {
			typ = "flowchart"
		}
		if obj, ok := results.(*js.Obj[any]); ok {
			obj.Set(typ, value)
			obj.Delete(prop)
		}
	}

	return results, nil
}

// removeDirectives is removeDirectives. It resets directiveRegex.lastIndex,
// as String.prototype.replace does for a global regular expression.
func (scan *directiveScan) removeDirectives(text string) string {
	scan.lastIndex = 0
	return replaceAll(directiveRegex, directiveMarker, text)
}
