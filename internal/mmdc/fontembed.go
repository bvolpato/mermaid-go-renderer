// Ported from mermaid-cli 12.2.0, src/browser/fontEmbedder.js (MIT).

package mmdc

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"path"
	"regexp"
	"strings"
	"sync"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/webfonts"
)

// This file embeds the fonts of the page of mermaid-cli into the SVG, as
// mmdc does by default for an .svg file (the option --no-font-embed turns
// it off). For every style sheet of the page that has a font that the
// diagram uses, or a rule that applies to an element of the diagram, mmdc
// adds one style element to the SVG. The element has the license of the
// font package and the rules of the style sheet, with the font files as
// data URLs.
//
// mmdc reads the rules from the CSS object model of Chromium:
// rule.cssText is the text that Chromium writes for a rule, not the text
// of the file. The style sheets are fixed files (internal/webfonts), so
// the rules are a record from Chromium: cssom.json.gz, which
// tools/reference/gen-mmdc-sheets.mjs writes. Native MathML labels match
// the KaTeX style rules but use no KaTeX font families, so mmdc embeds
// the stylesheet without its @font-face rules. The HTML fallback uses
// those families and embeds their WOFF2 files as well.

//go:embed cssom.json.gz
var cssomData []byte

// sheetRule is one rule of link.sheet.cssRules.
type sheetRule struct {
	// Kind is "font-face" for a CSSFontFaceRule, "style" for a
	// CSSStyleRule, and "other".
	Kind string `json:"kind"`
	// Text is rule.cssText.
	Text string `json:"text"`
	// Family is rule.style.fontFamily and Src is rule.style.src of a
	// @font-face rule.
	Family string `json:"family"`
	Src    string `json:"src"`
	// Selector is rule.selectorText of a style rule.
	Selector string `json:"selector"`

	// needClasses holds one top-level required class name per comma-separated
	// selector branch when every branch requires a class; otherwise
	// unconditional is true and ruleAppliesToSVG must run directly.
	// neverMatchesSVG is true for selectors like ":root, :host" or "body" that
	// can never match an SVG element or any element inside an SVG.
	needClasses     []string
	unconditional   bool
	neverMatchesSVG bool
}

// extractSelectorClasses extracts one top-level .class token per
// comma-separated selector branch outside (...) and [...]. It also handles
// top-level :is(.a, .b, ...) selectors. If every branch has a required class
// without backslash escapes, ok is true.
func extractSelectorClasses(sel string) (classes []string, ok bool) {
	if strings.ContainsRune(sel, '\\') {
		return nil, false
	}
	if strings.HasPrefix(sel, ":is(") {
		if closeIdx := strings.IndexByte(sel, ')'); closeIdx > 4 {
			inner := sel[4:closeIdx]
			if cls, innerOK := extractSelectorClasses(inner); innerOK {
				return cls, true
			}
		}
	}
	depth := 0
	inBracket := false
	branchStart := 0
	var out []string
	processBranch := func(part string) bool {
		d := 0
		br := false
		for i := 0; i < len(part); i++ {
			switch part[i] {
			case '(':
				d++
			case ')':
				if d > 0 {
					d--
				}
			case '[':
				br = true
			case ']':
				br = false
			case '.':
				if d == 0 && !br {
					j := i + 1
					for j < len(part) && isCSSIdentByte(part[j]) {
						j++
					}
					if j > i+1 {
						out = append(out, part[i+1:j])
						return true
					}
				}
			}
		}
		return false
	}
	for i := 0; i < len(sel); i++ {
		switch sel[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case '[':
			inBracket = true
		case ']':
			inBracket = false
		case ',':
			if depth == 0 && !inBracket {
				if !processBranch(sel[branchStart:i]) {
					return nil, false
				}
				branchStart = i + 1
			}
		}
	}
	if !processBranch(sel[branchStart:]) {
		return nil, false
	}
	return out, true
}

func isCSSIdentByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c >= 0x80
}

// pageSheet is one style sheet of the page: a link element with the class
// mermaid-cli-css.
type pageSheet struct {
	// Path is the path of the style sheet in internal/webfonts, without
	// "files/". It stands for link.href.
	Path string `json:"path"`
	// SHA256 identifies the source style sheet used to record the rules.
	SHA256 string      `json:"sha256"`
	Rules  []sheetRule `json:"rules"`
}

// pageSheets returns the style sheets of the page in the order of the link
// elements in the head: the order of the cssImports list of mmdc.
var pageSheets = sync.OnceValues(func() ([]pageSheet, error) {
	zr, err := gzip.NewReader(bytes.NewReader(cssomData))
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		return nil, err
	}
	var record struct {
		Sheets []pageSheet `json:"sheets"`
	}
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, err
	}
	for i := range record.Sheets {
		rules := record.Sheets[i].Rules
		for j := range rules {
			if rules[j].Kind == "style" {
				if rules[j].Selector == ":root, :host" || rules[j].Selector == "body" {
					rules[j].neverMatchesSVG = true
				} else if cls, ok := extractSelectorClasses(rules[j].Selector); ok {
					rules[j].needClasses = cls
				} else {
					rules[j].unconditional = true
				}
			}
		}
	}
	return record.Sheets, nil
})

// fontAwesomeFontClasses is fontAwesomeFontClasses.
var fontAwesomeFontClasses = []string{"fa", "fab", "fak", "fal", "far", "fas"}

// pseudoElements is the regular expression of ruleAppliesToSVG.
var pseudoElements = regexp.MustCompile(`::[\w-]+|:(before|after|first-letter|first-line)\b`)

// errFontEmbed is the message of mmdc when the font embedder fails.
const errFontEmbed = "Failed to embed fonts into SVG. Please report this error to https://github.com/mermaid-js/mermaid-cli/issues, and/or run using `--no-font-embed` if you don't need font embedding."

// EmbedFonts is fontEmbedder({ document, svg }): it embeds the fonts that
// the diagram uses, from the style sheets of the page, into the svg
// element. It does nothing in a page without the style sheets of
// mermaid-cli (Options.PageSheets).
func (p *Page) EmbedFonts() error {
	if len(p.opts.PageSheets) == 0 {
		return nil
	}
	if err := p.embedFonts(); err != nil {
		return fmt.Errorf("%s (%w)", errFontEmbed, err)
	}
	return nil
}

func (p *Page) embedFonts() error {
	svg := p.SVG

	// The first family of the computed font-family of every element that
	// has a text node as a child or a class of Font Awesome. Also detect
	// whether any Font Awesome or KaTeX classes appear in the SVG subtree so
	// normal diagrams can skip decompressing/parsing cssom.json.gz and scanning
	// 2,900+ Font Awesome and KaTeX rules.
	usedFontFamilies := map[string]bool{}
	var classBuf []string
	hasFA := false
	hasKatex := false
	scanClasses := func(el *dom.Node) (elHasFAFont bool) {
		classBuf = el.AppendClasses(classBuf[:0])
		for _, cl := range classBuf {
			if !hasFA && isFAClassToken(cl) {
				hasFA = true
			}
			if !hasKatex && strings.Contains(cl, "katex") {
				hasKatex = true
			}
			if !elHasFAFont && isFAFontClass(cl) {
				elHasFAFont = true
			}
		}
		return elHasFAFont
	}
	scanClasses(svg)
	var walk func(n *dom.Node)
	walk = func(n *dom.Node) {
		for el := n.FirstChild; el != nil; el = el.Next {
			if el.Type != dom.ElementNode {
				continue
			}
			elHasFAFont := scanClasses(el)
			if hasTextChild(el) || elHasFAFont {
				// window.getComputedStyle(el).fontFamily. The engine gives
				// the list of the families, which parseFontFamily would
				// make from the text of the computed value.
				if families := p.engine.RasterTextStyleOf(el).Families; len(families) > 0 {
					usedFontFamilies[families[0]] = true
				}
			}
			walk(el)
		}
	}
	walk(svg)

	needFullSheets := hasFA || hasKatex
	if !needFullSheets {
		for fam := range usedFontFamilies {
			if strings.Contains(fam, "Font Awesome") || strings.Contains(fam, "KaTeX") {
				needFullSheets = true
				break
			}
		}
	}

	var newCSSStyleSheets []string
	if !needFullSheets {
		useRecursive := usedFontFamilies["Recursive Variable"]
		useOpenSans := usedFontFamilies["Open Sans"]
		if !useRecursive && !useOpenSans {
			return nil
		}
		for i := range baseFontSheets {
			sheet := &baseFontSheets[i]
			switch sheet.Path {
			case "recursive/index.css":
				if !useRecursive {
					continue
				}
			case "open-sans/400.css", "open-sans/400-italic.css":
				if !useOpenSans {
					continue
				}
			default:
				continue
			}
			css, err := cachedBaseSheetCSS(sheet)
			if err != nil {
				return err
			}
			newCSSStyleSheets = append(newCSSStyleSheets, css)
		}
	} else {
		sheets, err := pageSheets()
		if err != nil {
			return err
		}
		presentClasses := map[string]struct{}{}
		recordClasses := func(el *dom.Node) {
			classBuf = el.AppendClasses(classBuf[:0])
			for _, cl := range classBuf {
				presentClasses[cl] = struct{}{}
			}
		}
		recordClasses(svg)
		var collectClasses func(n *dom.Node)
		collectClasses = func(n *dom.Node) {
			for el := n.FirstChild; el != nil; el = el.Next {
				if el.Type != dom.ElementNode {
					continue
				}
				recordClasses(el)
				collectClasses(el)
			}
		}
		collectClasses(svg)

		// ruleAppliesToSVG checks whether a style rule applies to an element
		// inside the SVG. Pseudo-elements are removed: the rule still applies
		// to the element that they belong to.
		ruleAppliesToSVG := func(rule *sheetRule) bool {
			if rule.neverMatchesSVG {
				return false
			}
			if !rule.unconditional {
				found := false
				for _, cl := range rule.needClasses {
					if _, ok := presentClasses[cl]; ok {
						found = true
						break
					}
				}
				if !found {
					return false
				}
			}
			selector := rule.Selector
			if strings.IndexByte(selector, ':') >= 0 {
				selector = pseudoElements.ReplaceAllString(selector, "")
			}
			return svg.Matches(selector) || svg.Select(selector) != nil
		}

		for i := range sheets {
			sheet := &sheets[i]
			// srcRules is the set of the src values of the used font families.
			var srcRules []string
			// hasMatchingRule is true when a style rule of the style sheet
			// applies to the SVG.
			hasMatchingRule := false
			var cssRulesText []string
			for ri := range sheet.Rules {
				rule := &sheet.Rules[ri]
				if rule.Kind == "font-face" {
					families, err := parseFontFamily(rule.Family)
					if err != nil {
						return err
					}
					if len(families) == 0 || !usedFontFamilies[families[0]] {
						continue
					}
					if !contains(srcRules, rule.Src) {
						srcRules = append(srcRules, rule.Src)
					}
				} else if !hasMatchingRule && rule.Kind == "style" {
					hasMatchingRule = ruleAppliesToSVG(rule)
				}
				// Push everything, except unused @font-face rules
				cssRulesText = append(cssRulesText, rule.Text)
			}
			if len(srcRules) == 0 && !hasMatchingRule {
				continue
			}

			css, err := formatEmbeddedSheet(sheet.Path, cssRulesText, srcRules)
			if err != nil {
				return err
			}
			newCSSStyleSheets = append(newCSSStyleSheets, css)
		}
	}

	for _, newCSS := range newCSSStyleSheets {
		originalStyleElement := svg.Select("style")
		if originalStyleElement == nil {
			return errors.New("Failed to find original style element in SVG")
		}
		// originalStyleElement.insertAdjacentElement("afterend", ...)
		newStyleElement := svg.Document().CreateElementNS(dom.NSSVG, "style")
		newStyleElement.Text(newCSS)
		originalStyleElement.Parent.InsertBefore(newStyleElement, originalStyleElement.Next)
	}
	return nil
}

var (
	baseSheetCacheMu sync.RWMutex
	baseSheetCache   = map[string]string{}
)

func cachedBaseSheetCSS(sheet *pageSheet) (string, error) {
	baseSheetCacheMu.RLock()
	if css, ok := baseSheetCache[sheet.Path]; ok {
		baseSheetCacheMu.RUnlock()
		return css, nil
	}
	baseSheetCacheMu.RUnlock()

	cssRulesText := make([]string, 0, len(sheet.Rules))
	srcRules := make([]string, 0, len(sheet.Rules))
	for ri := range sheet.Rules {
		rule := &sheet.Rules[ri]
		if !contains(srcRules, rule.Src) {
			srcRules = append(srcRules, rule.Src)
		}
		cssRulesText = append(cssRulesText, rule.Text)
	}
	css, err := formatEmbeddedSheet(sheet.Path, cssRulesText, srcRules)
	if err != nil {
		return "", err
	}
	baseSheetCacheMu.Lock()
	baseSheetCache[sheet.Path] = css
	baseSheetCacheMu.Unlock()
	return css, nil
}

func formatEmbeddedSheet(sheetPath string, cssRulesText, srcRules []string) (string, error) {
	license, err := loadLicense(sheetPath)
	if err != nil {
		return "", err
	}
	css := strings.Join(cssRulesText, "\n")
	for _, src := range srcRules {
		srcTokens, err := tokenizeCSSSrc(src)
		if err != nil {
			return "", err
		}
		var fontSrcToken *cssSourceToken
		for i := range srcTokens {
			if srcTokens[i].format == "woff2" || strings.Contains(srcTokens[i].url, "woff2") {
				fontSrcToken = &srcTokens[i]
				break
			}
		}
		if fontSrcToken == nil && len(srcTokens) > 0 {
			fontSrcToken = &srcTokens[0]
		}
		if fontSrcToken == nil || fontSrcToken.url == "" {
			return "", errors.New("No valid font src token found for used font family")
		}
		fontAsDataURL, err := fontDataURL(sheetPath, fontSrcToken.url)
		if err != nil {
			return "", err
		}
		newToken := *fontSrcToken
		newToken.url = fontAsDataURL
		newSrc := stringifyCSSSrc([]cssSourceToken{newToken})
		// css.replace(oldSrc, newSrc) replaces the first match. The new
		// text has no "$", which would be a pattern there.
		newCSS := strings.Replace(css, src, newSrc, 1)
		if newCSS == css {
			// This might happen if the old src had some different escaping
			return "", errors.New("Failed to replace old src with new src for " + src)
		}
		css = newCSS
	}
	// Make sure that the license comment doesn't break by adding a
	// zero-width space.
	return "/* " + strings.ReplaceAll(license, "*/", "*\u200b/") + " */\n\n" + css, nil
}

func isFAClassToken(cl string) bool {
	if len(cl) < 2 || cl[0] != 'f' || cl[1] != 'a' {
		return false
	}
	return len(cl) == 2 || cl[2] == '-' || len(cl) == 3
}

func isFAFontClass(cl string) bool {
	switch cl {
	case "fa", "fab", "fak", "fal", "far", "fas":
		return true
	}
	return false
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// hasTextChild is [...el.childNodes].some((n) => n.nodeType === Node.TEXT_NODE).
func hasTextChild(el *dom.Node) bool {
	for c := el.FirstChild; c != nil; c = c.Next {
		if c.Type == dom.TextNode {
			return true
		}
	}
	return false
}

func hasAnyClass(el *dom.Node, names []string) bool {
	for _, name := range names {
		if el.HasClass(name) {
			return true
		}
	}
	return false
}

// resolve is new URL(ref, link.href) for a style sheet of the page: the
// path of a file in internal/webfonts.
func resolve(sheetPath, ref string) (string, error) {
	base := &url.URL{Scheme: "https", Host: "mermaid-cli-intercept.invalid", Path: "/" + sheetPath}
	u, err := base.Parse(ref)
	if err != nil {
		return "", err
	}
	if u.Host != base.Host {
		return "", fmt.Errorf("%s is not a file of the page", ref)
	}
	return path.Join("files", u.Path), nil
}

var (
	licenseCacheMu sync.RWMutex
	licenseCache   = map[string]string{}
	fontDataMu     sync.RWMutex
	fontDataCache  = map[string]string{}
)

// loadLicense is loadLicense(new URL(link.href)): the text of the license
// of the package of a style sheet. mmdc asks for six file names at the
// same time and takes the first answer. Each package has one of them.
func loadLicense(sheetPath string) (string, error) {
	licenseCacheMu.RLock()
	if s, ok := licenseCache[sheetPath]; ok {
		licenseCacheMu.RUnlock()
		return s, nil
	}
	licenseCacheMu.RUnlock()
	for _, name := range []string{
		"./LICENSE",
		"./LICENSE.txt",
		"./LICENSE.md",
		"../LICENSE",
		"../LICENSE.txt",
		"../LICENSE.md",
	} {
		file, err := resolve(sheetPath, name)
		if err != nil {
			continue
		}
		if data, err := fs.ReadFile(webfonts.FS, file); err == nil {
			// response.text() decodes UTF-8 and drops a byte order mark.
			out := strings.TrimPrefix(string(data), "\ufeff")
			licenseCacheMu.Lock()
			licenseCache[sheetPath] = out
			licenseCacheMu.Unlock()
			return out, nil
		}
	}
	return "", errors.New("All promises were rejected")
}

// fontDataURL is blobToDataURL of the font file that a src names: a data
// URL with the content type that mmdc serves the file with.
func fontDataURL(sheetPath, ref string) (string, error) {
	file, err := resolve(sheetPath, ref)
	if err != nil {
		return "", errors.New("Failed to fetch font from " + ref)
	}
	fontDataMu.RLock()
	if s, ok := fontDataCache[file]; ok {
		fontDataMu.RUnlock()
		return s, nil
	}
	fontDataMu.RUnlock()
	data, err := fs.ReadFile(webfonts.FS, file)
	if err != nil {
		return "", errors.New("Failed to fetch font from " + ref)
	}
	contentType := ""
	switch strings.ToLower(path.Ext(file)) {
	case ".woff2":
		contentType = "font/woff2"
	case ".woff":
		contentType = "font/woff"
	default:
		return "", errors.New("Failed to fetch font from " + ref)
	}
	out := "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data)
	fontDataMu.Lock()
	fontDataCache[file] = out
	fontDataMu.Unlock()
	return out, nil
}
