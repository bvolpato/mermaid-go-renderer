// The asset list follows mermaid-cli 12.2.0, src/index.js (MIT).

// Package webfonts has the web fonts and the style sheets that mermaid-cli
// 12 loads into its page before it renders a diagram. A plain web page has
// none of them.
//
// The files are copies from the npm packages that mermaid-cli 12.2.0
// depends on, with the directory layout of each package, so that a relative
// url() of a style sheet finds its font file:
//
//	files/recursive          @fontsource-variable/recursive 5.3.0
//	files/open-sans          @fontsource/open-sans 5.3.0
//	files/fontawesome-free   @fortawesome/fontawesome-free 7.3.1
//	files/katex              katex 0.18.10 (the dependency of mermaid-cli)
//
// Only the files that the page of mermaid-cli reads are here: the style
// sheets of the Sheets list and the WOFF2 files that they name. The style
// sheets of Open Sans also name WOFF files, which Chromium does not load
// when it can read WOFF2. KaTeX also names WOFF and TTF alternatives.
//
// Each directory has the license file of its package. The fonts are under
// the SIL Open Font License 1.1. The style sheets of Font Awesome are under
// the MIT license. KaTeX has an MIT package license and a separate font
// license. Its font notice is files/katex/fonts-OFL.txt.
package webfonts

import "embed"

// FS has the directory "files".
//
//go:embed files
var FS embed.FS

// Direct string embeds for the 8 page stylesheets so callers read them from
// .rodata with zero heap allocations on cold startup.
//
//go:embed files/fontawesome-free/css/brands.css
var cssFABrands string

//go:embed files/fontawesome-free/css/regular.css
var cssFARegular string

//go:embed files/fontawesome-free/css/solid.css
var cssFASolid string

//go:embed files/fontawesome-free/css/fontawesome.css
var cssFAAll string

//go:embed files/recursive/index.css
var cssRecursive string

//go:embed files/open-sans/400.css
var cssOpenSans400 string

//go:embed files/open-sans/400-italic.css
var cssOpenSans400Italic string

//go:embed files/katex/dist/katex.css
var cssKatex string

// Sheet is one style sheet of the page of mermaid-cli.
type Sheet struct {
	// Package is the npm package of the style sheet.
	Package string
	// Import is the module path that mermaid-cli resolves, for example
	// "@fontsource/open-sans/400.css".
	Import string
	// Path is the path of the style sheet in FS.
	Path string
	// Text is the raw CSS text of the style sheet, backed directly by read-only
	// binary data.
	Text string
}

// Sheets lists the style sheets in the order of the cssImports list of
// mermaid-cli (src/index.js, renderMermaid).
var Sheets = []Sheet{
	{"@fortawesome/fontawesome-free", "@fortawesome/fontawesome-free/css/brands.css", "files/fontawesome-free/css/brands.css", cssFABrands},
	{"@fortawesome/fontawesome-free", "@fortawesome/fontawesome-free/css/regular.css", "files/fontawesome-free/css/regular.css", cssFARegular},
	{"@fortawesome/fontawesome-free", "@fortawesome/fontawesome-free/css/solid.css", "files/fontawesome-free/css/solid.css", cssFASolid},
	{"@fortawesome/fontawesome-free", "@fortawesome/fontawesome-free/css/fontawesome.css", "files/fontawesome-free/css/fontawesome.css", cssFAAll},
	{"@fontsource-variable/recursive", "@fontsource-variable/recursive/index.css", "files/recursive/index.css", cssRecursive},
	{"@fontsource/open-sans", "@fontsource/open-sans/400.css", "files/open-sans/400.css", cssOpenSans400},
	{"@fontsource/open-sans", "@fontsource/open-sans/400-italic.css", "files/open-sans/400-italic.css", cssOpenSans400Italic},
	{"katex", "katex/dist/katex.css", "files/katex/dist/katex.css", cssKatex},
}

// SheetText returns the embedded CSS text for path without allocating.
func SheetText(path string) (string, bool) {
	for i := range Sheets {
		if Sheets[i].Path == path {
			return Sheets[i].Text, true
		}
	}
	return "", false
}
