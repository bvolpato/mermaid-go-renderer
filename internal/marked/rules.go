// Ported from marked/src/rules.ts (marked 16.3.0, the version that the
// mermaid 12.2.0 browser bundle inlines).
//
// Only the rules of the default options are here: gfm is true, breaks and
// pedantic are false. The rules that only the renderer uses are left out.
// Each rule keeps its JavaScript source, and the test compares the sources
// with the ones of the real marked.

package marked

import (
	"strconv"
	"strings"
	"sync"

	"github.com/dlclark/regexp2"
)

// jsRegex is a JavaScript regular expression: its source and its flags.
type jsRegex struct {
	source string
	flags  string
}

// bq is a backquote. A Go raw string cannot hold one.
const bq = "`"

// caret is other.caret.
var caret = sync.OnceValue(func() *regex { return mustRegex(`(^|[^\[])\^`, "g") })

// editor is the object that edit() returns.
type editor struct {
	source string
	opt    string
}

func edit(regex string, opt ...string) *editor {
	e := &editor{source: regex}
	if len(opt) > 0 {
		e.opt = opt[0]
	}
	return e
}

// valSource is valSource.replace(other.caret, '$1').
func valSource(val string) string {
	return string(replaceAll(caret(), []rune(val), func(m *regexp2.Match) []rune {
		g, _ := group(m, 1)
		return g
	}))
}

// checkReplacement stops when val has a pattern that
// String.prototype.replace expands, such as $& or $1. The rules have no
// such pattern, so a replacement is literal text.
func checkReplacement(val string) string {
	for i := 0; i+1 < len(val); i++ {
		if val[i] == '$' && strings.IndexByte("$&`'0123456789<", val[i+1]) >= 0 {
			panic("marked: a replacement pattern in a rule is not supported: " + val)
		}
	}
	return val
}

// replace is obj.replace(name, val) with a string name: the first
// occurrence of name changes.
func (e *editor) replace(name, val string) *editor {
	val = valSource(val)
	if i := strings.Index(e.source, name); i >= 0 {
		e.source = e.source[:i] + checkReplacement(val) + e.source[i+len(name):]
	}
	return e
}

// replaceG is obj.replace(/name/g, val): each occurrence of name changes.
// name is the source of a regex that matches only itself.
func (e *editor) replaceG(name, val string) *editor {
	val = checkReplacement(valSource(val))
	var sb strings.Builder
	rest := 0
	for {
		i := strings.Index(e.source[rest:], name)
		if i < 0 {
			break
		}
		i += rest
		sb.WriteString(e.source[rest:i])
		sb.WriteString(val)
		rest = i + len(name)
	}
	sb.WriteString(e.source[rest:])
	e.source = sb.String()
	return e
}

func (e *editor) getRegex() jsRegex {
	return jsRegex{source: e.source, flags: e.opt}
}

// The members of `other` that the lexer and the tokenizer use and that are
// more than a test for one character. The simple members are written as Go
// code where they are used.
var otherSources = map[string]jsRegex{
	"codeRemoveIndent":         {`^(?: {1,4}| {0,3}\t)`, "gm"},
	"outputLinkReplace":        {`\\([\[\]])`, "g"},
	"indentCodeCompensation":   {`^(\s+)(?:` + bq + bq + bq + `)`, ""},
	"beginningSpace":           {`^\s+`, ""},
	"multipleSpaceGlobal":      {`\s+`, "g"},
	"blankLine":                {`^[ \t]*$`, ""},
	"doubleBlankLine":          {`\n[ \t]*\n[ \t]*$`, ""},
	"blockquoteStart":          {`^ {0,3}>`, ""},
	"blockquoteSetextReplace":  {`\n {0,3}((?:=+|-+) *)(?=\n|$)`, "g"},
	"blockquoteSetextReplace2": {`^ {0,3}>[ \t]?`, "gm"},
	"listReplaceTabs":          {`^\t+`, ""},
	"listIsTask":               {`^\[[ xX]\] `, ""},
	"listReplaceTask":          {`^\[[ xX]\] +`, ""},
	"anyLine":                  {`\n.*\n`, ""},
	"hrefBrackets":             {`^<(.*)>$`, ""},
	"tableDelimiter":           {`[:|]`, ""},
	"tableAlignChars":          {`^\||\| *$`, "g"},
	"tableRowBlankLine":        {`\n[ \t]*$`, ""},
	"tableAlignRight":          {`^ *-+: *$`, ""},
	"tableAlignCenter":         {`^ *:-+: *$`, ""},
	"tableAlignLeft":           {`^ *:-+ *$`, ""},
	"startATag":                {`^<a `, "i"},
	"endATag":                  {`^<\/a>`, "i"},
	"startPreScriptTag":        {`^<(pre|code|kbd|script)(\s|>)`, "i"},
	"endPreScriptTag":          {`^<\/(pre|code|kbd|script)(\s|>)`, "i"},
	"unicodeAlphaNumeric":      {`[\p{L}\p{N}]`, "u"},
}

// minIndent is Math.min(3, indent - 1) as text.
func minIndent(indent int) string {
	return strconv.Itoa(min(3, indent-1))
}

// listItemRegex is other.listItemRegex.
func listItemRegex(bull string) jsRegex {
	return jsRegex{source: "^( {0,3}" + bull + ")((?:[\t ][^\\n]*)?(?:\\n|$))"}
}

// nextBulletRegex is other.nextBulletRegex.
func nextBulletRegex(indent int) jsRegex {
	return jsRegex{source: "^ {0," + minIndent(indent) + "}(?:[*+-]|\\d{1,9}[.)])((?:[ \t][^\\n]*)?(?:\\n|$))"}
}

// hrRegex is other.hrRegex.
func hrRegex(indent int) jsRegex {
	return jsRegex{source: "^ {0," + minIndent(indent) + "}((?:- *){3,}|(?:_ *){3,}|(?:\\* *){3,})(?:\\n+|$)"}
}

// fencesBeginRegex is other.fencesBeginRegex.
func fencesBeginRegex(indent int) jsRegex {
	return jsRegex{source: "^ {0," + minIndent(indent) + "}(?:" + bq + bq + bq + "|~~~)"}
}

// headingBeginRegex is other.headingBeginRegex.
func headingBeginRegex(indent int) jsRegex {
	return jsRegex{source: "^ {0," + minIndent(indent) + "}#"}
}

// htmlBeginRegex is other.htmlBeginRegex.
func htmlBeginRegex(indent int) jsRegex {
	return jsRegex{source: "^ {0," + minIndent(indent) + "}<(?:[a-z].*>|!--)", flags: "i"}
}

/**
 * Block-Level Grammar
 */

const (
	newline      = `^(?:[ \t]*(?:\n|$))+`
	blockCode    = `^((?: {4}| {0,3}\t)[^\n]+(?:\n(?:[ \t]*(?:\n|$))*)?)+`
	fences       = `^ {0,3}(` + bq + `{3,}(?=[^` + bq + `\n]*(?:\n|$))|~{3,})([^\n]*)(?:\n|$)(?:|([\s\S]*?)(?:\n|$))(?: {0,3}\1[~` + bq + `]* *(?=\n|$)|$)`
	hr           = `^ {0,3}((?:-[\t ]*){3,}|(?:_[ \t]*){3,}|(?:\*[ \t]*){3,})(?:\n+|$)`
	heading      = `^ {0,3}(#{1,6})(?=\s|$)(.*)(?:\n+|$)`
	bullet       = `(?:[*+-]|\d{1,9}[.)])`
	lheadingCore = `^(?!bull |blockCode|fences|blockquote|heading|html|table)((?:.|\n(?!\s*?\n|bull |blockCode|fences|blockquote|heading|html|table))+?)\n {0,3}(=+|-+) *(?:\n+|$)`
	_paragraph   = `^([^\n]+(?:\n(?!hr|heading|lheading|blockquote|fences|list|html|table| +\n)[^\n]+)*)`
	blockText    = `^[^\n]+`
	_blockLabel  = `(?!\s*\])(?:\\[\s\S]|[^\[\]\\])+`

	_tag = "address|article|aside|base|basefont|blockquote|body|caption" +
		"|center|col|colgroup|dd|details|dialog|dir|div|dl|dt|fieldset|figcaption" +
		"|figure|footer|form|frame|frameset|h[1-6]|head|header|hr|html|iframe" +
		"|legend|li|link|main|menu|menuitem|meta|nav|noframes|ol|optgroup|option" +
		"|p|param|search|section|summary|table|tbody|td|tfoot|th|thead|title" +
		"|tr|track|ul"
	_comment = `<!--(?:-?>|[\s\S]*?(?:-->|$))`
)

// htmlScriptAlternative is alternative (1) of the block html rule. It has a
// back reference under the i flag, which the translator does not support.
// compileRules expands it to one alternative for each tag name.
const htmlScriptAlternative = "<(script|pre|style|textarea)[\\s>][\\s\\S]*?(?:</\\1>[^\\n]*\\n+|$)"

// blockSources returns the sources of block.gfm.
func blockSources() map[string]jsRegex {
	lheadingGfm := edit(lheadingCore).
		replaceG("bull", bullet).                               // lists can interrupt
		replaceG("blockCode", `(?: {4}| {0,3}\t)`).             // indented code blocks can interrupt
		replaceG("fences", ` {0,3}(?:`+bq+`{3,}|~{3,})`).       // fenced code blocks can interrupt
		replaceG("blockquote", ` {0,3}>`).                      // blockquote can interrupt
		replaceG("heading", ` {0,3}#{1,6}`).                    // ATX heading can interrupt
		replaceG("html", ` {0,3}<[^\n>]+>\n`).                  // block html can interrupt
		replaceG("table", ` {0,3}\|?(?:[:\- ]*\|)+[\:\- ]*\n`). // table can interrupt
		getRegex()
	def := edit(`^ {0,3}\[(label)\]: *(?:\n[ \t]*)?([^<\s][^\s]*|<.*?>)(?:(?: +(?:\n[ \t]*)?| *\n[ \t]*)(title))? *(?:\n+|$)`).
		replace("label", _blockLabel).
		replace("title", `(?:"(?:\\"?|[^"\\])*"|'[^'\n]*(?:\n[^'\n]+)*\n?'|\([^()]*\))`).
		getRegex()

	list := edit(`^( {0,3}bull)([ \t][^\n]+?)?(?:\n|$)`).
		replaceG("bull", bullet).
		getRegex()

	html := edit(
		"^ {0,3}(?:"+ // optional indentation
			htmlScriptAlternative+ // (1)
			"|comment[^\\n]*(\\n+|$)"+ // (2)
			"|<\\?[\\s\\S]*?(?:\\?>\\n*|$)"+ // (3)
			"|<![A-Z][\\s\\S]*?(?:>\\n*|$)"+ // (4)
			"|<!\\[CDATA\\[[\\s\\S]*?(?:\\]\\]>\\n*|$)"+ // (5)
			"|</?(tag)(?: +|\\n|/?>)[\\s\\S]*?(?:(?:\\n[ \t]*)+\\n|$)"+ // (6)
			"|<(?!script|pre|style|textarea)([a-z][\\w-]*)(?:attribute)*? */?>(?=[ \\t]*(?:\\n|$))[\\s\\S]*?(?:(?:\\n[ \t]*)+\\n|$)"+ // (7) open tag
			"|</(?!script|pre|style|textarea)[a-z][\\w-]*\\s*>(?=[ \\t]*(?:\\n|$))[\\s\\S]*?(?:(?:\\n[ \t]*)+\\n|$)"+ // (7) closing tag
			")", "i").
		replace("comment", _comment).
		replace("tag", _tag).
		replace("attribute", ` +[a-zA-Z:_][\w.:-]*(?: *= *"[^"\n]*"| *= *'[^'\n]*'| *= *[^\s"'=<>`+bq+`]+)?`).
		getRegex()

	paragraph := edit(_paragraph).
		replace("hr", hr).
		replace("heading", " {0,3}#{1,6}(?:\\s|$)").
		replace("|lheading", ""). // setext headings don't interrupt commonmark paragraphs
		replace("|table", "").
		replace("blockquote", " {0,3}>").
		replace("fences", " {0,3}(?:"+bq+"{3,}(?=[^"+bq+"\\n]*\\n)|~{3,})[^\\n]*\\n").
		replace("list", " {0,3}(?:[*+-]|1[.)]) "). // only lists starting from 1 can interrupt
		replace("html", "</?(?:tag)(?: +|\\n|/?>)|<(?:script|pre|style|textarea|!--)").
		replace("tag", _tag). // pars can be interrupted by type (6) html blocks
		getRegex()

	blockquote := edit(`^( {0,3}> ?(paragraph|[^\n]*)(?:\n|$))+`).
		replace("paragraph", paragraph.source).
		getRegex()

	/**
	 * GFM Block Grammar
	 */

	gfmTable := edit(
		"^ *([^\\n ].*)\\n"+ // Header
			" {0,3}((?:\\| *)?:?-+:? *(?:\\| *:?-+:? *)*(?:\\| *)?)"+ // Align
			"(?:\\n((?:(?! *\\n|hr|heading|blockquote|code|fences|list|html).*(?:\\n|$))*)\\n*|$)"). // Cells
		replace("hr", hr).
		replace("heading", " {0,3}#{1,6}(?:\\s|$)").
		replace("blockquote", " {0,3}>").
		replace("code", "(?: {4}| {0,3}\t)[^\\n]").
		replace("fences", " {0,3}(?:"+bq+"{3,}(?=[^"+bq+"\\n]*\\n)|~{3,})[^\\n]*\\n").
		replace("list", " {0,3}(?:[*+-]|1[.)]) "). // only lists starting from 1 can interrupt
		replace("html", "</?(?:tag)(?: +|\\n|/?>)|<(?:script|pre|style|textarea|!--)").
		replace("tag", _tag). // tables can be interrupted by type (6) html blocks
		getRegex()

	return map[string]jsRegex{
		"blockquote": blockquote,
		"code":       {source: blockCode},
		"def":        def,
		"fences":     {source: fences},
		"heading":    {source: heading},
		"hr":         {source: hr},
		"html":       html,
		"lheading":   lheadingGfm,
		"list":       list,
		"newline":    {source: newline},
		"table":      gfmTable,
		"paragraph": edit(_paragraph).
			replace("hr", hr).
			replace("heading", " {0,3}#{1,6}(?:\\s|$)").
			replace("|lheading", "").          // setext headings don't interrupt commonmark paragraphs
			replace("table", gfmTable.source). // interrupt paragraphs with table
			replace("blockquote", " {0,3}>").
			replace("fences", " {0,3}(?:"+bq+"{3,}(?=[^"+bq+"\\n]*\\n)|~{3,})[^\\n]*\\n").
			replace("list", " {0,3}(?:[*+-]|1[.)]) "). // only lists starting from 1 can interrupt
			replace("html", "</?(?:tag)(?: +|\\n|/?>)|<(?:script|pre|style|textarea|!--)").
			replace("tag", _tag). // pars can be interrupted by type (6) html blocks
			getRegex(),
		"text": {source: blockText},
	}
}

/**
 * Inline-Level Grammar
 */

const (
	escape     = `^\\([!"#$%&'()*+,\-./:;<=>?@\[\]\\^_` + bq + `{|}~])`
	inlineCode = `^(` + bq + `+)([^` + bq + `]|[^` + bq + `][\s\S]*?[^` + bq + `])\1(?!` + bq + `)`
	br         = `^( {2,}|\\)\n(?!\s*$)`

	// list of unicode punctuation marks, plus any missing characters from CommonMark spec
	_punctuation           = `[\p{P}\p{S}]`
	_punctuationOrSpace    = `[\s\p{P}\p{S}]`
	_notPunctuationOrSpace = `[^\s\p{P}\p{S}]`

	// GFM allows ~ inside strong and em for strikethrough
	_punctuationGfmStrongEm           = `(?!~)[\p{P}\p{S}]`
	_punctuationOrSpaceGfmStrongEm    = `(?!~)[\s\p{P}\p{S}]`
	_notPunctuationOrSpaceGfmStrongEm = `(?:[^\s\p{P}\p{S}]|~)`

	// sequences em should skip over [title](link), `code`, <html>
	blockSkip = `\[[^\[\]]*?\]\((?:\\[\s\S]|[^\\\(\)]|\((?:\\[\s\S]|[^\\\(\)])*\))*\)|` + bq + `[^` + bq + `]*?` + bq + `|<(?! )[^<>]*?>`

	emStrongLDelimCore = `^(?:\*+(?:((?!\*)punct)|[^\s*]))|^_+(?:((?!_)punct)|([^\s_]))`

	emStrongRDelimAstCore = "^[^_*]*?__[^_*]*?\\*[^_*]*?(?=__)" + // Skip orphan inside strong
		"|[^*]+(?=[^*])" + // Consume to delim
		"|(?!\\*)punct(\\*+)(?=[\\s]|$)" + // (1) #*** can only be a Right Delimiter
		"|notPunctSpace(\\*+)(?!\\*)(?=punctSpace|$)" + // (2) a***#, a*** can only be a Right Delimiter
		"|(?!\\*)punctSpace(\\*+)(?=notPunctSpace)" + // (3) #***a, ***a can only be Left Delimiter
		"|[\\s](\\*+)(?!\\*)(?=punct)" + // (4) ***# can only be Left Delimiter
		"|(?!\\*)punct(\\*+)(?!\\*)(?=punct)" + // (5) #***# can be either Left or Right Delimiter
		"|notPunctSpace(\\*+)(?=notPunctSpace)" // (6) a***a can be either Left or Right Delimiter

	_inlineLabel = `(?:\[(?:\\[\s\S]|[^\[\]\\])*\]|\\[\s\S]|` + bq + `[^` + bq + `]*` + bq + `|[^\[\]\\` + bq + `])*?`
)

// inlineSources returns the sources of inline.gfm.
func inlineSources() map[string]jsRegex {
	punctuation := edit(`^((?![*_])punctSpace)`, "u").
		replaceG("punctSpace", _punctuationOrSpace).getRegex()

	emStrongLDelimGfm := edit(emStrongLDelimCore, "u").
		replaceG("punct", _punctuationGfmStrongEm).
		getRegex()

	emStrongRDelimAstGfm := edit(emStrongRDelimAstCore, "gu").
		replaceG("notPunctSpace", _notPunctuationOrSpaceGfmStrongEm).
		replaceG("punctSpace", _punctuationOrSpaceGfmStrongEm).
		replaceG("punct", _punctuationGfmStrongEm).
		getRegex()

	// (6) Not allowed for _
	emStrongRDelimUnd := edit(
		"^[^_*]*?\\*\\*[^_*]*?_[^_*]*?(?=\\*\\*)"+ // Skip orphan inside strong
			"|[^_]+(?=[^_])"+ // Consume to delim
			"|(?!_)punct(_+)(?=[\\s]|$)"+ // (1) #___ can only be a Right Delimiter
			"|notPunctSpace(_+)(?!_)(?=punctSpace|$)"+ // (2) a___#, a___ can only be a Right Delimiter
			"|(?!_)punctSpace(_+)(?=notPunctSpace)"+ // (3) #___a, ___a can only be Left Delimiter
			"|[\\s](_+)(?!_)(?=punct)"+ // (4) ___# can only be Left Delimiter
			"|(?!_)punct(_+)(?!_)(?=punct)", "gu"). // (5) #___# can be either Left or Right Delimiter
		replaceG("notPunctSpace", _notPunctuationOrSpace).
		replaceG("punctSpace", _punctuationOrSpace).
		replaceG("punct", _punctuation).
		getRegex()

	anyPunctuation := edit(`\\(punct)`, "gu").
		replaceG("punct", _punctuation).
		getRegex()

	autolink := edit(`^<(scheme:[^\s\x00-\x1f<>]*|email)>`).
		replace("scheme", `[a-zA-Z][a-zA-Z0-9+.-]{1,31}`).
		replace("email", `[a-zA-Z0-9.!#$%&'*+/=?^_`+bq+`{|}~-]+(@)[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)+(?![-_])`).
		getRegex()

	_inlineComment := edit(_comment).replace("(?:-->|$)", "-->").getRegex()
	tag := edit(
		"^comment"+
			"|^</[a-zA-Z][\\w:-]*\\s*>"+ // self-closing tag
			"|^<[a-zA-Z][\\w-]*(?:attribute)*?\\s*/?>"+ // open tag
			"|^<\\?[\\s\\S]*?\\?>"+ // processing instruction, e.g. <?php ?>
			"|^<![a-zA-Z]+\\s[\\s\\S]*?>"+ // declaration, e.g. <!DOCTYPE html>
			"|^<!\\[CDATA\\[[\\s\\S]*?\\]\\]>"). // CDATA section
		replace("comment", _inlineComment.source).
		replace("attribute", `\s+[a-zA-Z:_][\w.:-]*(?:\s*=\s*"[^"]*"|\s*=\s*'[^']*'|\s*=\s*[^\s"'=<>`+bq+`]+)?`).
		getRegex()

	link := edit(`^!?\[(label)\]\(\s*(href)(?:(?:[ \t]*(?:\n[ \t]*)?)(title))?\s*\)`).
		replace("label", _inlineLabel).
		replace("href", `<(?:\\.|[^\n<>\\])+>|[^ \t\n\x00-\x1f]*`).
		replace("title", `"(?:\\"?|[^"\\])*"|'(?:\\'?|[^'\\])*'|\((?:\\\)?|[^)\\])*\)`).
		getRegex()

	reflink := edit(`^!?\[(label)\]\[(ref)\]`).
		replace("label", _inlineLabel).
		replace("ref", _blockLabel).
		getRegex()

	nolink := edit(`^!?\[(ref)\](?:\[\])?`).
		replace("ref", _blockLabel).
		getRegex()

	reflinkSearch := edit("reflink|nolink(?!\\()", "g").
		replace("reflink", reflink.source).
		replace("nolink", nolink.source).
		getRegex()

	/**
	 * GFM Inline Grammar
	 */

	return map[string]jsRegex{
		"anyPunctuation":    anyPunctuation,
		"autolink":          autolink,
		"blockSkip":         {blockSkip, "g"},
		"br":                {source: br},
		"code":              {source: inlineCode},
		"emStrongLDelim":    emStrongLDelimGfm,
		"emStrongRDelimAst": emStrongRDelimAstGfm,
		"emStrongRDelimUnd": emStrongRDelimUnd,
		"escape":            {source: escape},
		"link":              link,
		"nolink":            nolink,
		"punctuation":       punctuation,
		"reflink":           reflink,
		"reflinkSearch":     reflinkSearch,
		"tag":               tag,
		"url": edit(`^((?:ftp|https?):\/\/|www\.)(?:[a-zA-Z0-9\-]+\.?)+[^\s<]*|^email`, "i").
			replace("email", `[A-Za-z0-9._+-]+(@)[a-zA-Z0-9-_]+(?:\.[a-zA-Z0-9-_]*[a-zA-Z0-9])+(?![-_])`).
			getRegex(),
		"_backpedal": {source: `(?:[^?!.,:;*_'"~()&]+|\([^)]*\)|&(?![a-zA-Z0-9]+;$)|[?!.,:;*_'"~)]+(?!$))+`},
		"del":        {source: `^(~~?)(?=[^\s~])((?:\\[\s\S]|[^\\])*?(?:\\[\s\S]|[^\s~\\]))\1(?=[^~]|$)`},
		"text":       {source: `^([` + bq + `~]+|[^` + bq + `~])(?:(?= {2,}\n)|(?=[a-zA-Z0-9.!#$%&'*+\/=?_` + bq + `{\|}~-]+@)|[\s\S]*?(?:(?=[\\<!\[` + bq + `*~_]|\b_|https?:\/\/|ftp:\/\/|www\.|$)|[^ ](?= {2,}\n)|[^a-zA-Z0-9.!#$%&'*+\/=?_` + bq + `{\|}~-](?=[a-zA-Z0-9.!#$%&'*+\/=?_` + bq + `{\|}~-]+@)))`},
	}
}

// blockRules is rules.block for the default options.
type blockRules struct {
	blockquote, code, def, fences, heading, hr, html, lheading, list,
	newline, paragraph, table, text *regex
}

// inlineRules is rules.inline for the default options.
type inlineRules struct {
	backpedal, anyPunctuation, autolink, blockSkip, br, code, del,
	emStrongLDelim, emStrongRDelimAst, emStrongRDelimUnd, escape, link,
	nolink, punctuation, reflink, reflinkSearch, tag, text, url *regex
}

// otherRules is rules.other.
type otherRules struct {
	codeRemoveIndent, outputLinkReplace, indentCodeCompensation,
	beginningSpace, multipleSpaceGlobal, blankLine, doubleBlankLine,
	blockquoteStart, blockquoteSetextReplace, blockquoteSetextReplace2,
	listReplaceTabs, listIsTask, listReplaceTask, anyLine, hrefBrackets,
	tableDelimiter, tableAlignChars, tableRowBlankLine, tableAlignRight,
	tableAlignCenter, tableAlignLeft, startATag, endATag, startPreScriptTag,
	endPreScriptTag, unicodeAlphaNumeric *regex

	// The regexes that marked builds for each list item. The key of
	// listItemRegex is the last character of the bullet. The index of the
	// others is Math.min(3, indent - 1).
	listItemRegex     map[rune]*regex
	nextBulletRegex   [4]*regex
	hrRegex           [4]*regex
	fencesBeginRegex  [4]*regex
	headingBeginRegex [4]*regex
	htmlBeginRegex    [4]*regex
}

// rules is the Rules object that the lexer gives to the tokenizer.
type rules struct {
	other  otherRules
	block  blockRules
	inline inlineRules
}

// htmlScriptGroups is the number of capture groups that replace group 1 of
// the block html rule: see expandHTMLScript.
const htmlScriptGroups = 4

// expandHTMLScript writes alternative (1) of the block html rule without
// the back reference: one alternative for each tag name. Group 1 of the
// rule becomes the groups 1 to 4, and exactly one of them takes part in a
// match of the alternative.
func expandHTMLScript(source string) string {
	if !strings.Contains(source, htmlScriptAlternative) {
		panic("marked: the block html rule has changed")
	}
	var parts []string
	for _, name := range []string{"script", "pre", "style", "textarea"} {
		part := strings.Replace(htmlScriptAlternative, "(script|pre|style|textarea)", "("+name+")", 1)
		parts = append(parts, strings.Replace(part, "\\1", name, 1))
	}
	return strings.Replace(source, htmlScriptAlternative, "(?:"+strings.Join(parts, "|")+")", 1)
}

func compileRules() *rules {
	compile := func(r jsRegex) *regex { return mustRegex(r.source, r.flags) }
	r := &rules{}

	o := otherSources
	r.other = otherRules{
		codeRemoveIndent:         compile(o["codeRemoveIndent"]),
		outputLinkReplace:        compile(o["outputLinkReplace"]),
		indentCodeCompensation:   compile(o["indentCodeCompensation"]),
		beginningSpace:           compile(o["beginningSpace"]),
		multipleSpaceGlobal:      compile(o["multipleSpaceGlobal"]),
		blankLine:                compile(o["blankLine"]),
		doubleBlankLine:          compile(o["doubleBlankLine"]),
		blockquoteStart:          compile(o["blockquoteStart"]),
		blockquoteSetextReplace:  compile(o["blockquoteSetextReplace"]),
		blockquoteSetextReplace2: compile(o["blockquoteSetextReplace2"]),
		listReplaceTabs:          compile(o["listReplaceTabs"]),
		listIsTask:               compile(o["listIsTask"]),
		listReplaceTask:          compile(o["listReplaceTask"]),
		anyLine:                  compile(o["anyLine"]),
		hrefBrackets:             compile(o["hrefBrackets"]),
		tableDelimiter:           compile(o["tableDelimiter"]),
		tableAlignChars:          compile(o["tableAlignChars"]),
		tableRowBlankLine:        compile(o["tableRowBlankLine"]),
		tableAlignRight:          compile(o["tableAlignRight"]),
		tableAlignCenter:         compile(o["tableAlignCenter"]),
		tableAlignLeft:           compile(o["tableAlignLeft"]),
		startATag:                compile(o["startATag"]),
		endATag:                  compile(o["endATag"]),
		startPreScriptTag:        compile(o["startPreScriptTag"]),
		endPreScriptTag:          compile(o["endPreScriptTag"]),
		unicodeAlphaNumeric:      compile(o["unicodeAlphaNumeric"]),
		listItemRegex:            map[rune]*regex{},
	}
	for _, bull := range listBullets {
		r.other.listItemRegex[rune(bull[len(bull)-1])] = compile(listItemRegex(bull))
	}
	for i := range 4 {
		indent := i + 1
		r.other.nextBulletRegex[i] = compile(nextBulletRegex(indent))
		r.other.hrRegex[i] = compile(hrRegex(indent))
		r.other.fencesBeginRegex[i] = compile(fencesBeginRegex(indent))
		r.other.headingBeginRegex[i] = compile(headingBeginRegex(indent))
		r.other.htmlBeginRegex[i] = compile(htmlBeginRegex(indent))
	}

	b := blockSources()
	html := b["html"]
	html.source = expandHTMLScript(html.source)
	r.block = blockRules{
		blockquote: compile(b["blockquote"]),
		code:       compile(b["code"]),
		def:        compile(b["def"]),
		fences:     compile(b["fences"]),
		heading:    compile(b["heading"]),
		hr:         compile(b["hr"]),
		html:       compile(html),
		lheading:   compile(b["lheading"]),
		list:       compile(b["list"]),
		newline:    compile(b["newline"]),
		paragraph:  compile(b["paragraph"]),
		table:      compile(b["table"]),
		text:       compile(b["text"]),
	}

	in := inlineSources()
	r.inline = inlineRules{
		backpedal:         compile(in["_backpedal"]),
		anyPunctuation:    compile(in["anyPunctuation"]),
		autolink:          compile(in["autolink"]),
		blockSkip:         compile(in["blockSkip"]),
		br:                compile(in["br"]),
		code:              compile(in["code"]),
		del:               compile(in["del"]),
		emStrongLDelim:    compile(in["emStrongLDelim"]),
		emStrongRDelimAst: compile(in["emStrongRDelimAst"]),
		emStrongRDelimUnd: compile(in["emStrongRDelimUnd"]),
		escape:            compile(in["escape"]),
		link:              compile(in["link"]),
		nolink:            compile(in["nolink"]),
		punctuation:       compile(in["punctuation"]),
		reflink:           compile(in["reflink"]),
		reflinkSearch:     compile(in["reflinkSearch"]),
		tag:               compile(in["tag"]),
		text:              compile(in["text"]),
		url:               compile(in["url"]),
	}
	setPrefilters(r)
	return r
}

// setPrefilters sets the fields first and needs of the rules: see regex.
// They only save time.
func setPrefilters(r *rules) {
	// Each of these rules has ^ at its start. Each alternative of the rule
	// then needs one of the characters of the list, after zero to three
	// spaces where the rule starts with " {0,3}". So a match starts with a
	// character of the list.
	r.block.newline.startsWith(" \t\n")
	r.block.code.startsWith(" \t")
	r.block.fences.startsWith(" `~")
	r.block.heading.startsWith(" #")
	r.block.hr.startsWith(" -_*")
	r.block.blockquote.startsWith(" >")
	r.block.list.startsWith(" *+-0123456789")
	r.block.html.startsWith(" <")
	r.block.def.startsWith(" [")
	r.other.blockquoteStart.startsWith(" >")
	r.other.listIsTask.startsWith("[")
	for _, re := range r.other.listItemRegex {
		re.startsWith(" *+-0123456789")
	}
	for i := range 4 {
		r.other.nextBulletRegex[i].startsWith(" *+-0123456789")
		r.other.hrRegex[i].startsWith(" -_*")
		r.other.fencesBeginRegex[i].startsWith(" `~")
		r.other.headingBeginRegex[i].startsWith(" #")
		r.other.htmlBeginRegex[i].startsWith(" <")
	}
	r.inline.escape.startsWith("\\")
	r.inline.tag.startsWith("<")
	r.inline.link.startsWith("![")
	r.inline.reflink.startsWith("![")
	r.inline.nolink.startsWith("![")
	r.inline.code.startsWith("`")
	r.inline.br.startsWith(" \\")
	r.inline.del.startsWith("~")
	r.inline.autolink.startsWith("<")
	// The first alternative of url starts with a letter of ftp, http or
	// www. The second starts with a character of the class below.
	r.inline.url.startsWith("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789._+-")

	// Each of these rules has a line feed that is not optional: after the
	// header row of a table, before the underline of a setext heading, and
	// at the end of a hard line break.
	r.block.table.needs = '\n'
	r.block.lheading.needs = '\n'
	r.inline.br.needs = '\n'
}

// listBullets holds each value that Tokenizer.list gives to listItemRegex:
// an ordered bullet with a dot or a parenthesis, and the three characters
// of an unordered bullet.
var listBullets = []string{`\d{1,9}\.`, `\d{1,9}\)`, `\*`, `\+`, `\-`}

// getRules compiles the rules on the first use. A compiled rule has no
// state, so all lexers share the rules.
var getRules = sync.OnceValue(compileRules)
