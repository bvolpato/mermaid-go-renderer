// Ported from stylis/src/Enum.js (stylis 4.3.6).

// Package stylis is a port of the parts of stylis 4.3.6 that mermaid uses to
// compile the style sheet of a diagram: compile, serialize, stringify and
// middleware. It also has the namespace middleware of mermaid's compileCSS.
//
// 4.3.6 is the version that the mermaid 12.2.0 bundle contains. The package
// in node_modules is 4.4.0, which gives another result for some inputs (see
// parser.go). The golden file comes from the bundled source.
//
// The port works on UTF-8 bytes where stylis works on UTF-16 code units. Every
// character that stylis tests is ASCII, and no byte of a multi-byte UTF-8
// sequence is ASCII, so the output is the same.
package stylis

// Vendor prefixes.
const (
	MS     = "-ms-"
	MOZ    = "-moz-"
	WEBKIT = "-webkit-"
)

// Element types.
const (
	COMMENT     = "comm"
	RULESET     = "rule"
	DECLARATION = "decl"
)

// At-rule types.
const (
	PAGE                = "@page"
	MEDIA               = "@media"
	IMPORT              = "@import"
	CHARSET             = "@charset"
	VIEWPORT            = "@viewport"
	SUPPORTS            = "@supports"
	DOCUMENT            = "@document"
	NAMESPACE           = "@namespace"
	KEYFRAMES           = "@keyframes"
	FONT_FACE           = "@font-face"
	COUNTER_STYLE       = "@counter-style"
	FONT_FEATURE_VALUES = "@font-feature-values"
	LAYER               = "@layer"
	SCOPE               = "@scope"
)
