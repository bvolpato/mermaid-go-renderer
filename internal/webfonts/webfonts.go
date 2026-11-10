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

// Sheet is one style sheet of the page of mermaid-cli.
type Sheet struct {
	// Package is the npm package of the style sheet.
	Package string
	// Import is the module path that mermaid-cli resolves, for example
	// "@fontsource/open-sans/400.css".
	Import string
	// Path is the path of the style sheet in FS.
	Path string
}

// Sheets lists the style sheets in the order of the cssImports list of
// mermaid-cli (src/index.js, renderMermaid).
var Sheets = []Sheet{
	{"@fortawesome/fontawesome-free", "@fortawesome/fontawesome-free/css/brands.css", "files/fontawesome-free/css/brands.css"},
	{"@fortawesome/fontawesome-free", "@fortawesome/fontawesome-free/css/regular.css", "files/fontawesome-free/css/regular.css"},
	{"@fortawesome/fontawesome-free", "@fortawesome/fontawesome-free/css/solid.css", "files/fontawesome-free/css/solid.css"},
	{"@fortawesome/fontawesome-free", "@fortawesome/fontawesome-free/css/fontawesome.css", "files/fontawesome-free/css/fontawesome.css"},
	{"@fontsource-variable/recursive", "@fontsource-variable/recursive/index.css", "files/recursive/index.css"},
	{"@fontsource/open-sans", "@fontsource/open-sans/400.css", "files/open-sans/400.css"},
	{"@fontsource/open-sans", "@fontsource/open-sans/400-italic.css", "files/open-sans/400-italic.css"},
	{"katex", "katex/dist/katex.css", "files/katex/dist/katex.css"},
}
