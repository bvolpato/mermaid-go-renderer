// Ported from mermaid-cli 12.0.0, src/browser/fontEmbedder.js.

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
// tools/reference/gen-mmdc-sheets.mjs writes. The page of mmdc also has
// the style sheet of KaTeX. It is not in the record, because this module
// does not render math.

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
}

// pageSheet is one style sheet of the page: a link element with the class
// mermaid-cli-css.
type pageSheet struct {
	// Path is the path of the style sheet in internal/webfonts, without
	// "files/". It stands for link.href.
	Path  string      `json:"path"`
	Rules []sheetRule `json:"rules"`
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
	sheets, err := pageSheets()
	if err != nil {
		return err
	}

	// The first family of the computed font-family of every element that
	// has a text node as a child or a class of Font Awesome.
	usedFontFamilies := map[string]bool{}
	var walk func(n *dom.Node) error
	walk = func(n *dom.Node) error {
		for el := n.FirstChild; el != nil; el = el.Next {
			if el.Type != dom.ElementNode {
				continue
			}
			if hasTextChild(el) || hasAnyClass(el, fontAwesomeFontClasses) {
				// window.getComputedStyle(el).fontFamily. The engine gives
				// the list of the families, which parseFontFamily would
				// make from the text of the computed value.
				if families := p.engine.RasterTextStyleOf(el).Families; len(families) > 0 {
					usedFontFamilies[families[0]] = true
				}
			}
			if err := walk(el); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(svg); err != nil {
		return err
	}

	// ruleAppliesToSVG checks whether a style rule applies to an element
	// inside the SVG. Pseudo-elements are removed: the rule still applies
	// to the element that they belong to.
	ruleAppliesToSVG := func(selectorText string) bool {
		selector := pseudoElements.ReplaceAllString(selectorText, "")
		return svg.Matches(selector) || svg.Select(selector) != nil
	}

	var newCSSStyleSheets []string
	for _, sheet := range sheets {
		// srcRules is the set of the src values of the used font families.
		var srcRules []string
		// hasMatchingRule is true when a style rule of the style sheet
		// applies to the SVG.
		hasMatchingRule := false
		var cssRulesText []string
		for _, rule := range sheet.Rules {
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
				hasMatchingRule = ruleAppliesToSVG(rule.Selector)
			}
			// Push everything, except unused @font-face rules
			cssRulesText = append(cssRulesText, rule.Text)
		}
		if len(srcRules) == 0 && !hasMatchingRule {
			continue
		}

		license, err := loadLicense(sheet.Path)
		if err != nil {
			return err
		}
		css := strings.Join(cssRulesText, "\n")
		for _, src := range srcRules {
			srcTokens, err := tokenizeCSSSrc(src)
			if err != nil {
				return err
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
				return errors.New("No valid font src token found for used font family")
			}
			fontAsDataURL, err := fontDataURL(sheet.Path, fontSrcToken.url)
			if err != nil {
				return err
			}
			newToken := *fontSrcToken
			newToken.url = fontAsDataURL
			newSrc := stringifyCSSSrc([]cssSourceToken{newToken})
			// css.replace(oldSrc, newSrc) replaces the first match. The new
			// text has no "$", which would be a pattern there.
			newCSS := strings.Replace(css, src, newSrc, 1)
			if newCSS == css {
				// This might happen if the old src had some different escaping
				return errors.New("Failed to replace old src with new src for " + src)
			}
			css = newCSS
		}
		// Make sure that the license comment doesn't break by adding a
		// zero-width space.
		newCSSStyleSheets = append(newCSSStyleSheets, "/* "+strings.ReplaceAll(license, "*/", "*\u200b/")+" */\n\n"+css)
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

// loadLicense is loadLicense(new URL(link.href)): the text of the license
// of the package of a style sheet. mmdc asks for six file names at the
// same time and takes the first answer. Each package has one of them.
func loadLicense(sheetPath string) (string, error) {
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
			return strings.TrimPrefix(string(data), "\ufeff"), nil
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
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}
