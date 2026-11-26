// Ported from mermaid 12.2.0, mermaid/src/mermaidAPI.ts (createCssStyles,
// createUserStyles) (MIT).

// Package styles makes the text of the <style> element that mermaid puts at
// the start of every SVG. It ports getStyles of styles.ts, the style template
// of each diagram type and createUserStyles of mermaidAPI.ts.
//
// A config and the theme variables are dynamic values. See internal/js.
package styles

import (
	"strconv"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/memo"
	"github.com/bvolpato/mermaid-go-renderer/internal/stylis"
)

// Build returns the text of the <style> element for a diagram: the result of
// createUserStyles(config, diagramType, classDefs, svgID).
//
// cfg is the effective config: the value that getConfig() returns after
// processAndSetConfigs. svgID is the CSS selector of the SVG element, for
// example "#mermaid-0".
//
// userStyles is the text of the classDef rules, as cssStyleSheetToString
// prints them: the cssText of each rule, joined with "\n". Pass "" when the
// diagram has no classDefs. Build adds the ":root" rules of the font family
// variables in front of it, as createCssStyles does. For a config with
// themeCSS, Build uses the text as written. Call CreateCssStyles and
// CreateUserStyles to pass the text that a CSSOM gives for themeCSS.
func Build(diagramType string, userStyles string, cfg *js.Obj[any], svgID string) (string, error) {
	var classDefRules []string
	if userStyles != "" {
		classDefRules = []string{userStyles}
	}
	return CreateUserStyles(cfg, diagramType, CreateCssStyles(cfg, nil, classDefRules), svgID)
}

// userStylesCache caches the finished compiled CSS for renders whose config
// and themeVariables are unmodified copies of a frozen config template.
var userStylesCache = memo.New[string](256)

// styleReadsConfig lists the diagram types whose style functions inspect
// o.config beyond themeVariables, theme and look.
var styleReadsConfig = map[string]bool{
	"c4": true, "cynefin": true, "gitGraph": true, "radar": true,
	"railroad": true, "railroadAbnf": true, "railroadEbnf": true, "railroadPeg": true,
	"requirement": true, "sequence": true, "timeline": true, "treemap": true,
	"usecase": true, "wardley": true,
}

// configCacheKey returns a cache key when the inputs that CreateUserStyles
// reads are unmodified copies of frozen template objects.
func configCacheKey(config *js.Obj[any], graphType, userCSSstyles, svgID string) (string, bool) {
	tv := js.AsObj(config.Get("themeVariables"))
	if tv == nil || tv.SourceID() == 0 {
		return "", false
	}
	var cfgID, secID uint64
	if styleReadsConfig[graphType] {
		cfgID = config.SourceID()
		if cfgID == 0 {
			return "", false
		}
		if sec := js.AsObj(config.Get(graphType)); sec != nil {
			secID = sec.SourceID()
			if secID == 0 {
				return "", false
			}
		}
	}
	themeVal, themeOK := config.Get("theme").(string)
	if !themeOK {
		return "", false
	}
	lookVal, lookOK := config.Get("look").(string)
	if !lookOK {
		return "", false
	}
	var sb strings.Builder
	sb.Grow(64 + len(themeVal) + len(lookVal) + len(graphType) + len(svgID) + len(userCSSstyles))
	sb.WriteString(strconv.FormatUint(cfgID, 36))
	sb.WriteByte('|')
	sb.WriteString(strconv.FormatUint(tv.SourceID(), 36))
	sb.WriteByte('|')
	sb.WriteString(strconv.FormatUint(secID, 36))
	sb.WriteByte('|')
	sb.WriteString(themeVal)
	sb.WriteByte('|')
	sb.WriteString(lookVal)
	sb.WriteByte('|')
	sb.WriteString(graphType)
	sb.WriteByte('|')
	sb.WriteString(svgID)
	sb.WriteByte('|')
	sb.WriteString(userCSSstyles)
	return sb.String(), true
}

// CreateUserStyles is createUserStyles of mermaidAPI.ts. userCSSstyles is the
// result of createCssStyles.
func CreateUserStyles(config *js.Obj[any], graphType string, userCSSstyles string, svgID string) (string, error) {
	if config == nil {
		config = js.NewObj[any]()
	}
	if key, ok := configCacheKey(config, graphType, userCSSstyles, svgID); ok {
		if cached, hit := userStylesCache.Get(key); hit {
			return cached, nil
		}
		opts := &options{base: obj(config.Get("themeVariables")), config: config}
		opts.Set("theme", config.Get("theme"))
		opts.Set("look", config.Get("look"))
		baseStyles, err := getBaseStyles(graphType, opts, svgID)
		if err != nil {
			return "", err
		}
		css := compileSplitCSS(svgID, baseStyles, userCSSstyles)
		userStylesCache.Put(key, css)
		return css, nil
	}
	// { ...config.themeVariables, theme: config.theme, look: config.look }
	opts := &options{base: obj(config.Get("themeVariables")), config: config}
	opts.Set("theme", config.Get("theme"))
	opts.Set("look", config.Get("look"))

	baseStyles, err := getBaseStyles(graphType, opts, svgID)
	if err != nil {
		return "", err
	}
	return compileSplitCSS(svgID, baseStyles, userCSSstyles), nil
}

// compiled is a compiled style sheet and the selector of the SVG element
// that it was compiled for.
type compiled struct{ svgID, css string }

// compiledSheets has the result of stylis.CompileCSS by the text of the
// style sheet. The compiled text is a function of the two arguments of
// CompileCSS alone, and renders of one diagram type with one theme have the
// same style sheet. An entry holds the result for one selector: the most
// usual case is that all renders have the same id.
var compiledSheets = memo.New[compiled](512)

// skipStylisDelimiter advances past a quote, parenthesized, or bracketed
// sequence using the same nesting rules as stylis tokenizer.delimiter.
func skipStylisDelimiter(s string, i int, closeByte byte, depth int) (int, bool) {
	if depth > 64 {
		return 0, false
	}
	for i++; i < len(s); i++ {
		c := s[i]
		if c == 0 {
			return 0, false
		}
		if c == closeByte {
			return i, true
		}
		switch c {
		case '"', '\'':
			if closeByte != '"' && closeByte != '\'' {
				end, ok := skipStylisDelimiter(s, i, c, depth+1)
				if !ok {
					return 0, false
				}
				i = end
			}
		case '(':
			if closeByte == ')' {
				end, ok := skipStylisDelimiter(s, i, ')', depth+1)
				if !ok {
					return 0, false
				}
				i = end
			}
		case '\\':
			i++
			if i >= len(s) || s[i] == 0 {
				return 0, false
			}
		}
	}
	return 0, false
}

// cssIsTopLevelBalanced reports whether s consists solely of balanced top-level
// rule/at-rule blocks (plus whitespace and closed comments) with no top-level
// declarations or dangling selectors, so stylis compiles it independently of
// adjacent fragments.
func cssIsTopLevelBalanced(s string) bool {
	depth := 0
	hasPending := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == 0 {
			return false
		}
		switch c {
		case ' ', '\t', '\n', '\r':
			continue
		case '"', '\'':
			end, ok := skipStylisDelimiter(s, i, c, 0)
			if !ok {
				return false
			}
			i = end
			if depth == 0 {
				hasPending = true
			}
		case '(':
			end, ok := skipStylisDelimiter(s, i, ')', 0)
			if !ok {
				return false
			}
			i = end
			if depth == 0 {
				hasPending = true
			}
		case '[':
			end, ok := skipStylisDelimiter(s, i, ']', 0)
			if !ok {
				return false
			}
			i = end
			if depth == 0 {
				hasPending = true
			}
		case '\\':
			return false
		case '/':
			if i+1 < len(s) {
				if s[i+1] == '*' {
					i += 2
					closed := false
					for i < len(s) {
						if s[i] == 0 {
							return false
						}
						if s[i] == '*' && i+1 < len(s) && s[i+1] == '/' {
							i++
							closed = true
							break
						}
						i++
					}
					if !closed {
						return false
					}
					continue
				}
				if s[i+1] == '/' {
					// Require a terminating newline so a trailing line comment
					// in baseStyles cannot comment out userStyles.
					i += 2
					closed := false
					for i < len(s) {
						if s[i] == 0 {
							return false
						}
						if s[i] == '\n' {
							closed = true
							break
						}
						i++
					}
					if !closed {
						return false
					}
					continue
				}
			}
			if depth == 0 {
				hasPending = true
			}
		case ';':
			if depth == 0 {
				return false
			}
		case '@':
			if i+1 < len(s) && s[i+1] == '-' {
				return false
			}
			if depth == 0 {
				hasPending = true
			}
		case '-':
			if depth == 0 && !hasPending && i+1 < len(s) && s[i+1] == '-' {
				return false
			}
			if depth == 0 {
				hasPending = true
			}
		case '{':
			depth++
			if depth == 1 {
				hasPending = false
			}
		case '}':
			depth--
			if depth < 0 {
				return false
			}
		default:
			if depth == 0 {
				hasPending = true
			}
		}
	}
	return depth == 0 && !hasPending
}

// compileSplitCSS compiles baseStyles + userStyles + "\n", reusing the cached
// compilation of baseStyles whenever both fragments are top-level balanced.
func compileSplitCSS(svgID, baseStyles, userStyles string) string {
	if userStyles == "" {
		return compileCSS(svgID, baseStyles+"\n")
	}
	if cssIsTopLevelBalanced(baseStyles) && cssIsTopLevelBalanced(userStyles) {
		return compileCSS(svgID, baseStyles) + compileCSS(svgID, userStyles+"\n")
	}
	return compileCSS(svgID, baseStyles+userStyles+"\n")
}

// compileCSS is stylis.CompileCSS with the cache.
func compileCSS(svgID, allStyles string) string {
	if c, ok := compiledSheets.Get(allStyles); ok && c.svgID == svgID {
		return c.css
	}
	css := stylis.CompileCSS(svgID, allStyles)
	compiledSheets.Put(allStyles, compiled{svgID, css})
	return css
}

// CreateCssStyles is createCssStyles of mermaidAPI.ts without the CSSOM. It
// joins the user styles in the order of mermaid: the themeCSS rules, the font
// family variables, then the classDef rules.
//
// Mermaid passes each part through a CSSStyleSheet and reads the cssText of
// the rules, so the browser prints each rule in its own form. The caller must
// supply that form for the two parts that need a CSS parser:
//
//   - classDefRules is the cssText of each rule that mermaid inserts for the
//     classDefs of the diagram, in order.
//   - themeCSS is cssStyleSheetToString of the sheet that replaceSync makes
//     from config.themeCSS: the cssText of each rule, joined with "\n". When
//     it is nil, the function uses config.themeCSS as written, which is what
//     mermaid does on a host that has no replaceSync.
//
// The function makes the ":root" rules of config.fontFamily and
// config.altFontFamily itself.
func CreateCssStyles(config *js.Obj[any], themeCSS *string, classDefRules []string) string {
	if config == nil {
		config = js.NewObj[any]()
	}
	var cssRules []string

	// user provided theme CSS info
	if fontFamily := config.Get("fontFamily"); fontFamily != nil {
		cssRules = append(cssRules, rootVariableRule("--mermaid-font-family", fontFamily))
	}
	if altFontFamily := config.Get("altFontFamily"); altFontFamily != nil {
		cssRules = append(cssRules, rootVariableRule("--mermaid-alt-font-family", altFontFamily))
	}

	// classDefs defined in the diagram text
	cssRules = append(cssRules, classDefRules...)

	cssString := ""
	if configThemeCSS := config.Get("themeCSS"); configThemeCSS != nil {
		if themeCSS != nil {
			cssString = *themeCSS + "\n"
		} else {
			cssString += s(configThemeCSS) + "\n"
		}
	}

	return cssString + strings.Join(cssRules, "\n")
}

// rootVariableRule is the cssText of the rule that mermaid inserts as
// `:root { name: ${value}}`. A custom property keeps its value as written.
// The browser removes the white space around the value and prints one ";"
// after it.
//
// The result is exact for a value that has no ";", "{", "}" or "!" inside,
// which a font family list does not have. Other values need a CSS parser.
func rootVariableRule(name string, value any) string {
	v := trimCSSSpace(s(value))
	if strings.HasSuffix(v, ";") {
		v = trimCSSSpace(v[:len(v)-1])
	}
	return ":root { " + name + ": " + v + "; }"
}

// trimCSSSpace removes CSS white space from both ends.
func trimCSSSpace(v string) string {
	return strings.Trim(v, " \t\n\r\f")
}
