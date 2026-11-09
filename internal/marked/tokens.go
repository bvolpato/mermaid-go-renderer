// Ported from marked/src/Tokens.ts (marked 16.3.0).

package marked

import (
	"strconv"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Token is one token of the lexer. The JavaScript token types are objects
// with different properties. This struct has the properties of all types,
// and Type tells which of them are in use:
//
//	space, hr, br    Raw
//	code             Raw, Text, and CodeBlockStyle "indented" or Lang
//	heading          Raw, Depth, Text, Tokens
//	blockquote       Raw, Tokens, Text
//	list             Raw, Ordered, Start, Loose, Items
//	list_item        Raw, Task, Checked, Loose, Text, Tokens
//	html             Raw, Text, Block, and Pre or InLink and InRawBlock
//	def              Tag, Raw, Href, Title, HasTitle
//	table            Raw, Header, Align, Rows
//	paragraph        Raw, Text, Tokens
//	text             Raw, Text, and Tokens or Escaped
//	escape, codespan Raw, Text
//	link, image      Raw, Href, Title, HasTitle, Text, Tokens
//	em, strong, del  Raw, Text, Tokens
//
// A string can hold a surrogate without its partner, because marked cuts
// strings at UTF-16 code units. Such a surrogate has the three-byte form
// that internal/js defines.
type Token struct {
	Type   string
	Raw    string
	Text   string
	Tokens []*Token

	// Depth is the level of a heading, from 1 to 6.
	Depth int

	// CodeBlockStyle is "indented" for an indented code block. Lang is the
	// info string of a fenced code block.
	CodeBlockStyle string
	Lang           string

	// Ordered and Start describe a list. Start is the number of the first
	// item of an ordered list. Loose is for a list and for a list item.
	Ordered bool
	Start   int
	Loose   bool
	Items   []*Token

	// Task and Checked describe a list item. Checked has a meaning only
	// when Task is true.
	Task    bool
	Checked bool

	// Block is true for an html block and false for an inline tag. Pre is
	// for a block. InLink and InRawBlock are for an inline tag.
	Block      bool
	Pre        bool
	InLink     bool
	InRawBlock bool

	// Tag is the normalized label of a link reference definition.
	Tag string

	// Href and Title are for a def, a link and an image. HasTitle is false
	// when the JavaScript title is undefined (def) or null (link, image).
	Href     string
	Title    string
	HasTitle bool

	// Header, Align and Rows describe a table. An Align value is "left",
	// "right", "center", or "" for null.
	Header []TableCell
	Align  []string
	Rows   [][]TableCell

	// Escaped is the escaped property of an inline text token.
	Escaped bool

	shape shape
	// n is the length of Raw in UTF-16 code units when the tokenizer
	// returned the token.
	n int
}

// TableCell is one cell of a table token.
type TableCell struct {
	Text   string
	Tokens []*Token
	Header bool
	Align  string
}

// shape is the set and the order of the properties of the JavaScript token
// object. Tokens of one type can have different shapes.
type shape uint8

const (
	shapeRaw        shape = iota // type, raw
	shapeCodeIndent              // type, raw, codeBlockStyle, text
	shapeCodeFence               // type, raw, lang, text
	shapeHeading                 // type, raw, depth, text, tokens
	shapeBlockquote              // type, raw, tokens, text
	shapeList                    // type, raw, ordered, start, loose, items
	shapeListItem                // type, raw, task, checked, loose, text, tokens
	shapeHTMLBlock               // type, block, raw, pre, text
	shapeDef                     // type, tag, raw, href, title
	shapeTable                   // type, raw, header, align, rows
	shapeTextTokens              // type, raw, text, tokens
	shapeText                    // type, raw, text
	shapeHTMLInline              // type, raw, inLink, inRawBlock, block, text
	shapeLink                    // type, raw, href, title, text, tokens
	shapeAutolink                // type, raw, text, href, tokens
	shapeInlineText              // type, raw, text, escaped
)

// JSON returns JSON.stringify(tokens) for the token list of marked.lexer:
// the same properties in the same order.
func JSON(tokens []*Token) string {
	var sb strings.Builder
	writeTokens(&sb, tokens)
	return sb.String()
}

func writeTokens(sb *strings.Builder, tokens []*Token) {
	sb.WriteByte('[')
	for i, t := range tokens {
		if i > 0 {
			sb.WriteByte(',')
		}
		writeToken(sb, t)
	}
	sb.WriteByte(']')
}

func writeString(sb *strings.Builder, key, value string) {
	sb.WriteString(`,"`)
	sb.WriteString(key)
	sb.WriteString(`":`)
	sb.WriteString(js.QuoteJSON(value))
}

func writeBool(sb *strings.Builder, key string, value bool) {
	sb.WriteString(`,"`)
	sb.WriteString(key)
	sb.WriteString(`":`)
	sb.WriteString(strconv.FormatBool(value))
}

func writeChildren(sb *strings.Builder, key string, tokens []*Token) {
	sb.WriteString(`,"`)
	sb.WriteString(key)
	sb.WriteString(`":`)
	writeTokens(sb, tokens)
}

func writeAlign(sb *strings.Builder, align string) {
	if align == "" {
		sb.WriteString("null")
	} else {
		sb.WriteString(js.QuoteJSON(align))
	}
}

func writeCells(sb *strings.Builder, cells []TableCell) {
	sb.WriteByte('[')
	for i, c := range cells {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(`{"text":`)
		sb.WriteString(js.QuoteJSON(c.Text))
		writeChildren(sb, "tokens", c.Tokens)
		writeBool(sb, "header", c.Header)
		sb.WriteString(`,"align":`)
		writeAlign(sb, c.Align)
		sb.WriteByte('}')
	}
	sb.WriteByte(']')
}

func writeToken(sb *strings.Builder, t *Token) {
	sb.WriteString(`{"type":`)
	sb.WriteString(js.QuoteJSON(t.Type))
	switch t.shape {
	case shapeRaw:
		writeString(sb, "raw", t.Raw)
	case shapeCodeIndent:
		writeString(sb, "raw", t.Raw)
		writeString(sb, "codeBlockStyle", t.CodeBlockStyle)
		writeString(sb, "text", t.Text)
	case shapeCodeFence:
		writeString(sb, "raw", t.Raw)
		writeString(sb, "lang", t.Lang)
		writeString(sb, "text", t.Text)
	case shapeHeading:
		writeString(sb, "raw", t.Raw)
		sb.WriteString(`,"depth":`)
		sb.WriteString(strconv.Itoa(t.Depth))
		writeString(sb, "text", t.Text)
		writeChildren(sb, "tokens", t.Tokens)
	case shapeBlockquote:
		writeString(sb, "raw", t.Raw)
		writeChildren(sb, "tokens", t.Tokens)
		writeString(sb, "text", t.Text)
	case shapeList:
		writeString(sb, "raw", t.Raw)
		writeBool(sb, "ordered", t.Ordered)
		sb.WriteString(`,"start":`)
		if t.Ordered {
			sb.WriteString(strconv.Itoa(t.Start))
		} else {
			sb.WriteString(`""`)
		}
		writeBool(sb, "loose", t.Loose)
		writeChildren(sb, "items", t.Items)
	case shapeListItem:
		writeString(sb, "raw", t.Raw)
		writeBool(sb, "task", t.Task)
		if t.Task {
			// checked is undefined for an item that is not a task.
			writeBool(sb, "checked", t.Checked)
		}
		writeBool(sb, "loose", t.Loose)
		writeString(sb, "text", t.Text)
		writeChildren(sb, "tokens", t.Tokens)
	case shapeHTMLBlock:
		writeBool(sb, "block", t.Block)
		writeString(sb, "raw", t.Raw)
		writeBool(sb, "pre", t.Pre)
		writeString(sb, "text", t.Text)
	case shapeDef:
		writeString(sb, "tag", t.Tag)
		writeString(sb, "raw", t.Raw)
		writeString(sb, "href", t.Href)
		if t.HasTitle {
			writeString(sb, "title", t.Title)
		}
	case shapeTable:
		writeString(sb, "raw", t.Raw)
		sb.WriteString(`,"header":`)
		writeCells(sb, t.Header)
		sb.WriteString(`,"align":[`)
		for i, a := range t.Align {
			if i > 0 {
				sb.WriteByte(',')
			}
			writeAlign(sb, a)
		}
		sb.WriteString(`],"rows":[`)
		for i, row := range t.Rows {
			if i > 0 {
				sb.WriteByte(',')
			}
			writeCells(sb, row)
		}
		sb.WriteByte(']')
	case shapeTextTokens:
		writeString(sb, "raw", t.Raw)
		writeString(sb, "text", t.Text)
		writeChildren(sb, "tokens", t.Tokens)
	case shapeText:
		writeString(sb, "raw", t.Raw)
		writeString(sb, "text", t.Text)
	case shapeHTMLInline:
		writeString(sb, "raw", t.Raw)
		writeBool(sb, "inLink", t.InLink)
		writeBool(sb, "inRawBlock", t.InRawBlock)
		writeBool(sb, "block", t.Block)
		writeString(sb, "text", t.Text)
	case shapeLink:
		writeString(sb, "raw", t.Raw)
		writeString(sb, "href", t.Href)
		if t.HasTitle {
			writeString(sb, "title", t.Title)
		} else {
			sb.WriteString(`,"title":null`)
		}
		writeString(sb, "text", t.Text)
		writeChildren(sb, "tokens", t.Tokens)
	case shapeAutolink:
		writeString(sb, "raw", t.Raw)
		writeString(sb, "text", t.Text)
		writeString(sb, "href", t.Href)
		writeChildren(sb, "tokens", t.Tokens)
	case shapeInlineText:
		writeString(sb, "raw", t.Raw)
		writeString(sb, "text", t.Text)
		writeBool(sb, "escaped", t.Escaped)
	}
	sb.WriteByte('}')
}
