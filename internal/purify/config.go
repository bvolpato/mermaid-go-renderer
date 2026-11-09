// Ported from dompurify 3.4.12, src/config.ts and the reading of the
// configuration in _parseConfig of src/purify.ts (MPL-2.0 OR Apache-2.0).

package purify

import (
	"encoding/binary"
	"errors"
	"sort"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Flag is a boolean option whose upstream default is true. The zero value
// keeps the default. Upstream reads these options as `cfg.X !== false`.
type Flag uint8

// Values of a Flag.
const (
	Unset Flag = iota
	False
	True
)

// Profiles is the USE_PROFILES option. A non-nil pointer with no profile
// selected allows only text, as `USE_PROFILES: {}` does.
type Profiles struct {
	HTML       bool // html
	SVG        bool // svg
	SVGFilters bool // svgFilters
	MathML     bool // mathMl
}

// CustomElementHandling is the CUSTOM_ELEMENT_HANDLING option. Upstream
// accepts a RegExp or a function for the two checks. A Go caller wraps a
// regular expression in a function.
type CustomElementHandling struct {
	TagNameCheck                   func(tagName string) bool
	AttributeNameCheck             func(attributeName, tagName string) bool
	AllowCustomizedBuiltInElements bool
}

// Config is the configuration object of DOMPurify.sanitize. The zero value is
// the default configuration. Each field names the upstream option.
//
// A nil slice or map means that the option is absent. An empty, non-nil slice
// or map is an option that is present and empty.
type Config struct {
	AllowedTags       []string // ALLOWED_TAGS
	AllowedAttr       []string // ALLOWED_ATTR
	AllowedNamespaces []string // ALLOWED_NAMESPACES
	AddURISafeAttr    []string // ADD_URI_SAFE_ATTR
	AddDataURITags    []string // ADD_DATA_URI_TAGS
	ForbidContents    []string // FORBID_CONTENTS
	AddForbidContents []string // ADD_FORBID_CONTENTS
	ForbidTags        []string // FORBID_TAGS
	ForbidAttr        []string // FORBID_ATTR

	// ADD_TAGS is an array or a function upstream. The function wins when
	// both fields are set.
	AddTags     []string
	AddTagsFunc func(tagName string) bool
	// ADD_ATTR is an array or a function upstream.
	AddAttr     []string
	AddAttrFunc func(attributeName, tagName string) bool

	UseProfiles *Profiles // USE_PROFILES

	AllowAriaAttr         Flag // ALLOW_ARIA_ATTR
	AllowDataAttr         Flag // ALLOW_DATA_ATTR
	AllowUnknownProtocols bool // ALLOW_UNKNOWN_PROTOCOLS
	AllowSelfCloseInAttr  Flag // ALLOW_SELF_CLOSE_IN_ATTR
	SafeForTemplates      bool // SAFE_FOR_TEMPLATES
	SafeForXML            Flag // SAFE_FOR_XML
	WholeDocument         bool // WHOLE_DOCUMENT
	ForceBody             bool // FORCE_BODY
	SanitizeDOM           Flag // SANITIZE_DOM
	SanitizeNamedProps    bool // SANITIZE_NAMED_PROPS
	KeepContent           Flag // KEEP_CONTENT

	// ReturnDOM and ReturnDOMFragment are RETURN_DOM and RETURN_DOM_FRAGMENT.
	// Sanitize returns a string and ignores them. SanitizeDOM sets ReturnDOM.
	ReturnDOM         bool
	ReturnDOMFragment bool

	// AllowedURI replaces the test of ALLOWED_URI_REGEXP. It gets the value
	// of the attribute with the white space removed.
	AllowedURI func(value string) bool

	// Namespace is NAMESPACE. Only the HTML namespace is supported.
	Namespace string
	// ParserMediaType is PARSER_MEDIA_TYPE. Only "text/html" is supported.
	ParserMediaType string

	MathMLTextIntegrationPoints map[string]bool // MATHML_TEXT_INTEGRATION_POINTS
	HTMLIntegrationPoints       map[string]bool // HTML_INTEGRATION_POINTS

	CustomElementHandling *CustomElementHandling // CUSTOM_ELEMENT_HANDLING
}

// appendKey appends the content of the configuration to b: two
// configurations with the same bytes give the same settings. The second
// result is false for a configuration that has a function, which has no
// content to compare.
func (c *Config) appendKey(b []byte) ([]byte, bool) {
	if c.AddTagsFunc != nil || c.AddAttrFunc != nil || c.AllowedURI != nil {
		return b, false
	}
	if h := c.CustomElementHandling; h != nil && (h.TagNameCheck != nil || h.AttributeNameCheck != nil) {
		return b, false
	}
	b = keyList(b, c.AllowedTags)
	b = keyList(b, c.AllowedAttr)
	b = keyList(b, c.AllowedNamespaces)
	b = keyList(b, c.AddURISafeAttr)
	b = keyList(b, c.AddDataURITags)
	b = keyList(b, c.ForbidContents)
	b = keyList(b, c.AddForbidContents)
	b = keyList(b, c.ForbidTags)
	b = keyList(b, c.ForbidAttr)
	b = keyList(b, c.AddTags)
	b = keyList(b, c.AddAttr)
	b = keyString(b, c.Namespace)
	b = keyString(b, c.ParserMediaType)
	if u := c.UseProfiles; u != nil {
		b = append(b, 1, keyBit(u.HTML), keyBit(u.SVG), keyBit(u.SVGFilters), keyBit(u.MathML))
	} else {
		b = append(b, 0)
	}
	b = append(b,
		byte(c.AllowAriaAttr), byte(c.AllowDataAttr), keyBit(c.AllowUnknownProtocols), byte(c.AllowSelfCloseInAttr),
		keyBit(c.SafeForTemplates), byte(c.SafeForXML), keyBit(c.WholeDocument), keyBit(c.ForceBody), byte(c.SanitizeDOM),
		keyBit(c.SanitizeNamedProps), byte(c.KeepContent), keyBit(c.ReturnDOM), keyBit(c.ReturnDOMFragment))
	b = keyPoints(b, c.MathMLTextIntegrationPoints)
	b = keyPoints(b, c.HTMLIntegrationPoints)
	if h := c.CustomElementHandling; h != nil {
		b = append(b, 1, keyBit(h.AllowCustomizedBuiltInElements))
	} else {
		b = append(b, 0)
	}
	return b, true
}

func keyBit(v bool) byte {
	if v {
		return 1
	}
	return 0
}

func keyString(b []byte, v string) []byte {
	b = binary.AppendUvarint(b, uint64(len(v)))
	return append(b, v...)
}

// keyList appends a list option. A nil list is an absent option, and an
// empty list is an option that is present and empty.
func keyList(b []byte, l []string) []byte {
	if l == nil {
		return append(b, 0xff)
	}
	b = binary.AppendUvarint(append(b, 1), uint64(len(l)))
	for _, v := range l {
		b = keyString(b, v)
	}
	return b
}

// keyPoints appends the entries of a map in the order of the names.
func keyPoints(b []byte, m map[string]bool) []byte {
	if m == nil {
		return append(b, 0xff)
	}
	b = binary.AppendUvarint(append(b, 1), uint64(len(m)))
	if len(m) == 1 {
		for name, v := range m {
			b = append(keyString(b, name), keyBit(v))
		}
		return b
	}
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		b = append(keyString(b, name), keyBit(m[name]))
	}
	return b
}

// ErrTrustedTypesPolicy is the TypeError that upstream throws for a
// TRUSTED_TYPES_POLICY value that has no createHTML function. A policy cannot
// come from JSON, so every truthy value from a config object is invalid.
var ErrTrustedTypesPolicy = errors.New(`TRUSTED_TYPES_POLICY configuration option must provide a "createHTML" hook.`)

// hasUpperASCII reports whether s has an ASCII capital letter.
func hasUpperASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			return true
		}
	}
	return false
}

// setOption reads an option that upstream passes to addToSet. The second
// result is false when the option is absent or is not an array. lower is
// false for an option whose transform is stringToString.
func setOption(cfg *js.Obj[any], key string, lower bool) ([]string, bool) {
	if !cfg.Has(key) {
		return nil, false
	}
	arr, ok := cfg.Get(key).([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, isString := e.(string); isString {
			out = append(out, s)
			continue
		}
		// addToSet does not transform a value that is not a string. The value
		// becomes a property key as it is. Every lookup uses a lowercase
		// name, so a key with a capital letter can never match.
		s := js.ToString(e)
		if lower && hasUpperASCII(s) {
			continue
		}
		out = append(out, s)
	}
	return out, true
}

// objectOption reads an option that upstream clones and then uses as a set
// with truthy values.
func objectOption(cfg *js.Obj[any], key string) map[string]bool {
	if !cfg.Has(key) {
		return nil
	}
	v := cfg.Get(key)
	if !js.Truthy(v) {
		return nil
	}
	out := map[string]bool{}
	switch x := v.(type) {
	case *js.Obj[any]:
		x.Each(func(k string, e any) { out[k] = js.Truthy(e) })
	case []any:
		for i, e := range x {
			out[js.Str(float64(i))] = js.Truthy(e)
		}
	default:
		// A primitive is not an object: the default applies.
		return nil
	}
	return out
}

func notFalse(v any) Flag {
	if b, ok := v.(bool); ok && !b {
		return False
	}
	return Unset
}

// ConfigFromJS converts a dynamic configuration object, for example the
// dompurifyConfig of a mermaid config, to a Config. It reads each option as
// _parseConfig does. A value that is not an object gives the default
// configuration. Options that only a JavaScript caller can express (functions
// and regular expressions) are ignored, as upstream ignores a value of the
// wrong type.
func ConfigFromJS(v any) (Config, error) {
	var c Config
	cfg, ok := v.(*js.Obj[any])
	if !ok || cfg == nil {
		return c, nil
	}
	if s, isString := cfg.Get("PARSER_MEDIA_TYPE").(string); isString {
		c.ParserMediaType = s
	}
	lower := c.ParserMediaType != "application/xhtml+xml"
	if l, ok := setOption(cfg, "ALLOWED_TAGS", lower); ok {
		c.AllowedTags = l
	}
	if l, ok := setOption(cfg, "ALLOWED_ATTR", lower); ok {
		c.AllowedAttr = l
	}
	if l, ok := setOption(cfg, "ALLOWED_NAMESPACES", false); ok {
		c.AllowedNamespaces = l
	}
	if l, ok := setOption(cfg, "ADD_URI_SAFE_ATTR", lower); ok {
		c.AddURISafeAttr = l
	}
	if l, ok := setOption(cfg, "ADD_DATA_URI_TAGS", lower); ok {
		c.AddDataURITags = l
	}
	if l, ok := setOption(cfg, "FORBID_CONTENTS", lower); ok {
		c.ForbidContents = l
	}
	if l, ok := setOption(cfg, "ADD_FORBID_CONTENTS", lower); ok {
		c.AddForbidContents = l
	}
	if l, ok := setOption(cfg, "FORBID_TAGS", lower); ok {
		c.ForbidTags = l
	}
	if l, ok := setOption(cfg, "FORBID_ATTR", lower); ok {
		c.ForbidAttr = l
	}
	if l, ok := setOption(cfg, "ADD_TAGS", lower); ok {
		c.AddTags = l
	}
	if l, ok := setOption(cfg, "ADD_ATTR", lower); ok {
		c.AddAttr = l
	}
	if cfg.Has("USE_PROFILES") {
		if p := cfg.Get("USE_PROFILES"); js.Truthy(p) {
			c.UseProfiles = &Profiles{}
			if o, isObj := p.(*js.Obj[any]); isObj {
				isTrue := func(key string) bool {
					b, isBool := o.Get(key).(bool)
					return isBool && b
				}
				c.UseProfiles.HTML = isTrue("html")
				c.UseProfiles.SVG = isTrue("svg")
				c.UseProfiles.SVGFilters = isTrue("svgFilters")
				c.UseProfiles.MathML = isTrue("mathMl")
			}
		}
	}
	c.AllowAriaAttr = notFalse(cfg.Get("ALLOW_ARIA_ATTR"))
	c.AllowDataAttr = notFalse(cfg.Get("ALLOW_DATA_ATTR"))
	c.AllowUnknownProtocols = js.Truthy(cfg.Get("ALLOW_UNKNOWN_PROTOCOLS"))
	c.AllowSelfCloseInAttr = notFalse(cfg.Get("ALLOW_SELF_CLOSE_IN_ATTR"))
	c.SafeForTemplates = js.Truthy(cfg.Get("SAFE_FOR_TEMPLATES"))
	c.SafeForXML = notFalse(cfg.Get("SAFE_FOR_XML"))
	c.WholeDocument = js.Truthy(cfg.Get("WHOLE_DOCUMENT"))
	c.ReturnDOM = js.Truthy(cfg.Get("RETURN_DOM"))
	c.ReturnDOMFragment = js.Truthy(cfg.Get("RETURN_DOM_FRAGMENT"))
	c.ForceBody = js.Truthy(cfg.Get("FORCE_BODY"))
	c.SanitizeDOM = notFalse(cfg.Get("SANITIZE_DOM"))
	c.SanitizeNamedProps = js.Truthy(cfg.Get("SANITIZE_NAMED_PROPS"))
	c.KeepContent = notFalse(cfg.Get("KEEP_CONTENT"))
	if s, isString := cfg.Get("NAMESPACE").(string); isString {
		c.Namespace = s
	}
	c.MathMLTextIntegrationPoints = objectOption(cfg, "MATHML_TEXT_INTEGRATION_POINTS")
	c.HTMLIntegrationPoints = objectOption(cfg, "HTML_INTEGRATION_POINTS")
	if cfg.Has("CUSTOM_ELEMENT_HANDLING") {
		if o, isObj := cfg.Get("CUSTOM_ELEMENT_HANDLING").(*js.Obj[any]); isObj && o != nil {
			if b, isBool := o.Get("allowCustomizedBuiltInElements").(bool); isBool {
				c.CustomElementHandling = &CustomElementHandling{AllowCustomizedBuiltInElements: b}
			}
		}
	}
	if js.Truthy(cfg.Get("TRUSTED_TYPES_POLICY")) {
		return c, ErrTrustedTypesPolicy
	}
	return c, nil
}

// lowerAll is the transform of addToSet for the text/html parser.
func lowerAll(values []string) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = toLowerCase(v)
	}
	return out
}

// toLowerCase is String.prototype.toLowerCase.
func toLowerCase(s string) string {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 0x80 || (c >= 'A' && c <= 'Z') {
			return js.ToLowerCase(s)
		}
	}
	return s
}

// set is an object that upstream uses as a set of names.
type set map[string]bool

func newSet(lists ...[]string) set {
	n := 0
	for _, l := range lists {
		n += len(l)
	}
	s := make(set, n)
	for _, l := range lists {
		for _, v := range l {
			s[v] = true
		}
	}
	return s
}

func (s set) clone() set {
	out := make(set, len(s))
	for k, v := range s {
		out[k] = v
	}
	return out
}

func (s set) add(values []string) {
	for _, v := range values {
		s[v] = true
	}
}

// The default sets of createDOMPurify. They are shared by every Purifier and
// are never changed: a Purifier clones a default set before it adds to it.
var (
	defaultAllowedTags    = newSet(lowerAll(htmlTags), lowerAll(svgTags), lowerAll(svgFilterTags), lowerAll(mathMLTags), textTags)
	defaultAllowedAttr    = newSet(lowerAll(htmlAttrs), lowerAll(svgAttrs), lowerAll(mathMLAttrs), lowerAll(xmlAttrs))
	defaultForbidContents = newSet([]string{
		"annotation-xml", "audio", "colgroup", "desc", "foreignobject", "head", "iframe", "math",
		"mi", "mn", "mo", "ms", "mtext", "noembed", "noframes", "noscript", "plaintext", "script",
		"selectedcontent", "style", "svg", "template", "thead", "title", "video", "xmp",
	})
	defaultDataURITags       = newSet([]string{"audio", "video", "img", "source", "image", "track"})
	defaultURISafeAttributes = newSet([]string{
		"alt", "class", "for", "id", "label", "name", "pattern", "placeholder", "role", "summary",
		"title", "value", "style", "xmlns",
	})
	defaultAllowedNamespaces           = newSet([]string{mathMLNamespace, svgNamespace, htmlNamespace})
	defaultMathMLTextIntegrationPoints = newSet([]string{"mi", "mo", "mn", "ms", "mtext"})
	defaultHTMLIntegrationPoints       = newSet([]string{"annotation-xml"})
	commonSVGAndHTMLElements           = newSet([]string{"title", "style", "font", "a", "script"})
	allSVGTags                         = newSet(lowerAll(svgTags), lowerAll(svgFilterTags), lowerAll(svgDisallowedTags))
	allMathMLTags                      = newSet(lowerAll(mathMLTags), lowerAll(mathMLDisallowedTags))
	reservedCustomElementNames         = newSet([]string{
		"annotation-xml", "color-profile", "font-face", "font-face-format", "font-face-name",
		"font-face-src", "font-face-uri", "missing-glyph",
	})
)

const (
	mathMLNamespace = "http://www.w3.org/1998/Math/MathML"
	svgNamespace    = "http://www.w3.org/2000/svg"
	htmlNamespace   = "http://www.w3.org/1999/xhtml"
)

// trimSpace is String.prototype.trim.
func trimSpace(s string) string {
	if s == "" {
		return s
	}
	// The common case has no white space at either end.
	if first, last := s[0], s[len(s)-1]; first > ' ' && first < 0x80 && last > ' ' && last < 0x80 {
		return s
	}
	return js.TrimSpace(s)
}
