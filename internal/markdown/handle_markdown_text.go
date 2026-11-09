// Package markdown turns the text of a mermaid label into lines of words and
// into HTML, as mermaid 12.2.0 does.
//
// Ported from mermaid 12.2.0,
// mermaid/src/rendering-util/handle-markdown-text.ts and
// mermaid/src/rendering-util/types.ts (MIT).
package markdown

import (
	"strings"
	"unicode/utf8"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/marked"
)

// Word is MarkdownWord. Type is a MarkdownWordType: "normal", "strong" or
// "em". A MarkdownLine is a []Word.
type Word struct {
	Content string
	Type    string
}

// lexer is marked.lexer(markdown).
//
// marked throws an error only when no rule matches the text, and its rules
// match each text: see marked.Lexer. So the error value is not expected,
// and the result for it is the empty token list.
func lexer(markdown string) []*marked.Token {
	tokens, err := marked.Lexer(markdown)
	if err != nil {
		return nil
	}
	return tokens
}

// preprocessMarkdown returns the processed markdown. Upstream reads
// markdownAutoWrap here, and the code that used it is a comment.
func preprocessMarkdown(markdown string) string {
	//Replace <br/>with \n
	withoutBR := strings.ReplaceAll(markdown, "<br/>", "\n")
	// Replace multiple newlines with a single newline
	//
	// This is withoutBR.replace(/\n{2,}/g, '\n').
	withoutMultipleNewlines := withoutBR
	if strings.Contains(withoutBR, "\n\n") {
		var sb strings.Builder
		sb.Grow(len(withoutBR))
		for i := 0; i < len(withoutBR); i++ {
			if withoutBR[i] == '\n' && i > 0 && withoutBR[i-1] == '\n' {
				continue
			}
			sb.WriteByte(withoutBR[i])
		}
		withoutMultipleNewlines = sb.String()
	}
	// Remove extra spaces at the beginning of each line
	withoutExtraSpaces := dedent(withoutMultipleNewlines)
	return withoutExtraSpaces
}

// brLen returns the length of the match of /<br\s*\/?>/i at the start of
// s, or 0. Without the u flag, the i flag gives b and r only their two
// ASCII cases.
func brLen(s string) int {
	if len(s) < 4 || s[0] != '<' || s[1]|0x20 != 'b' || s[2]|0x20 != 'r' {
		return 0
	}
	i := 3
	for i < len(s) {
		r, size := utf8.DecodeRuneInString(s[i:])
		if !js.IsWhiteSpace(r) {
			break
		}
		i += size
	}
	if i < len(s) && s[i] == '/' {
		i++
	}
	if i < len(s) && s[i] == '>' {
		return i + 1
	}
	return 0
}

// NonMarkdownToLines is nonMarkdownToLines. It splits text that is not
// markdown into plain-text formatted lines.
// This treats new lines, `\n`, and `<br/>` as line breaks, and splits on spaces for words.
// SVG tags are preserved as separate words to maintain proper formatting.
func NonMarkdownToLines(nonMarkdownText string) [][]Word {
	// nonMarkdownText.split(/\\n|\n|<br\s*\/?>/gi)
	var parts []string
	start := 0
	for i := 0; i < len(nonMarkdownText); {
		n := 0
		switch nonMarkdownText[i] {
		case '\\':
			// The regex has the i flag, so the n matches both cases.
			if i+1 < len(nonMarkdownText) && nonMarkdownText[i+1]|0x20 == 'n' {
				n = 2
			}
		case '\n':
			n = 1
		case '<':
			n = brLen(nonMarkdownText[i:])
		}
		if n == 0 {
			i++
			continue
		}
		parts = append(parts, nonMarkdownText[start:i])
		i += n
		start = i
	}
	parts = append(parts, nonMarkdownText[start:])

	lines := make([][]Word, len(parts))
	for i, line := range parts {
		lines[i] = svgWords(js.TrimSpace(line))
	}
	return lines
}

// svgWords is line.match(/<[^>]+>|[^\s<>]+/g), with an empty list when
// there is no match. It keeps SVG tags intact and preserves space between
// tags and text.
func svgWords(line string) []Word {
	words := []Word{}
	for i := 0; i < len(line); {
		if line[i] == '<' {
			// <[^>]+>: at least one character, then the first >.
			if end := strings.IndexByte(line[i+1:], '>'); end >= 1 {
				end += i + 2
				words = append(words, Word{Content: line[i:end], Type: "normal"})
				i = end
				continue
			}
			i++
			continue
		}
		// [^\s<>]+
		j := i
		for j < len(line) {
			r, size := utf8.DecodeRuneInString(line[j:])
			if r == '<' || r == '>' || js.IsWhiteSpace(r) {
				break
			}
			j += size
		}
		if j > i {
			words = append(words, Word{Content: line[i:j], Type: "normal"})
			i = j
			continue
		}
		_, size := utf8.DecodeRuneInString(line[i:])
		i += size
	}
	return words
}

// MarkdownToLines is markdownToLines. It splits markdown into lines.
// config is the mermaid config. It can be nil. Upstream reads its key
// markdownAutoWrap and does not use the value.
func MarkdownToLines(markdown string, config *js.Obj[any]) [][]Word {
	preprocessedMarkdown := preprocessMarkdown(markdown)
	nodes := lexer(preprocessedMarkdown)
	lines := [][]Word{{}}
	currentLine := 0

	var processNode func(node *marked.Token, parentType string)
	processNode = func(node *marked.Token, parentType string) {
		if node.Type == "text" {
			textLines := strings.Split(node.Text, "\n")
			for index, textLine := range textLines {
				if index != 0 {
					currentLine++
					lines = append(lines, []Word{})
				}
				for _, word := range strings.Split(textLine, " ") {
					word = strings.ReplaceAll(word, "&#39;", "'")
					if word != "" {
						lines[currentLine] = append(lines[currentLine], Word{Content: word, Type: parentType})
					}
				}
			}
		} else if node.Type == "strong" || node.Type == "em" {
			for _, contentNode := range node.Tokens {
				processNode(contentNode, node.Type)
			}
		} else if node.Type == "html" {
			lines[currentLine] = append(lines[currentLine], Word{Content: node.Text, Type: "normal"})
		}
	}

	for _, treeNode := range nodes {
		if treeNode.Type == "paragraph" {
			for _, contentNode := range treeNode.Tokens {
				processNode(contentNode, "normal")
			}
		} else if treeNode.Type == "html" {
			lines[currentLine] = append(lines[currentLine], Word{Content: treeNode.Text, Type: "normal"})
		} else {
			lines[currentLine] = append(lines[currentLine], Word{Content: treeNode.Raw, Type: "normal"})
		}
	}

	return lines
}

// NonMarkdownToHTML is nonMarkdownToHTML: the counterpart to MarkdownToHTML
// for non-markdown text.
//
// Non-markdown text is not wrapped normally, and users can use an explicit `\n`
// sequence to add a line break.
func NonMarkdownToHTML(text string) string {
	if text == "" {
		return ""
	}
	/*
	 * Edge labels may have double backgrounds if `addBackground` is `true`.
	 * This `<p>` wrapper aligns with how MarkdownToHTML wraps its output, and
	 * ensures both backgrounds are the same size.
	 *
	 * We can't set it for empty labels, otherwise it causes rendering changes.
	 */
	var sb strings.Builder
	sb.Grow(len(text) + 7)
	sb.WriteString("<p>")
	// Replace new lines with <br /> tags: text.replace(/\\n|\n/g, '<br />').
	//
	// Unlike in markdown text, `\n` sequences are treated as line breaks here.
	for i := 0; i < len(text); i++ {
		switch {
		case text[i] == '\\' && i+1 < len(text) && text[i+1] == 'n':
			sb.WriteString("<br />")
			i++
		case text[i] == '\n':
			sb.WriteString("<br />")
		default:
			sb.WriteByte(text[i])
		}
	}
	sb.WriteString("</p>")
	return sb.String()
}

// MarkdownToHTML is markdownToHTML. config is the mermaid config. It can be
// nil. The function reads its key markdownAutoWrap.
func MarkdownToHTML(markdown string, config *js.Obj[any]) string {
	// markdownAutoWrap === false
	noAutoWrap := false
	if config != nil {
		if v, ok := config.Get("markdownAutoWrap").(bool); ok && !v {
			noAutoWrap = true
		}
	}
	nodes := lexer(markdown)

	var sb strings.Builder
	sb.Grow(len(markdown) + 16)
	var output func(node *marked.Token)
	children := func(node *marked.Token) {
		for _, child := range node.Tokens {
			output(child)
		}
	}
	output = func(node *marked.Token) {
		switch node.Type {
		case "text":
			// node.text.replace(/\n */g, '<br/>'), and then
			// .replace(/ /g, '&nbsp;') when markdownAutoWrap is false.
			text := node.Text
			for i := 0; i < len(text); i++ {
				switch {
				case text[i] == '\n':
					sb.WriteString("<br/>")
					for i+1 < len(text) && text[i+1] == ' ' {
						i++
					}
				case text[i] == ' ' && noAutoWrap:
					sb.WriteString("&nbsp;")
				default:
					sb.WriteByte(text[i])
				}
			}
		case "strong":
			sb.WriteString("<strong>")
			children(node)
			sb.WriteString("</strong>")
		case "em":
			sb.WriteString("<em>")
			children(node)
			sb.WriteString("</em>")
		case "paragraph":
			sb.WriteString("<p>")
			children(node)
			sb.WriteString("</p>")
		case "space":
		case "html":
			sb.WriteString(node.Text)
		case "escape":
			sb.WriteString(node.Text)
		default:
			// Upstream logs "Unsupported markdown" here.
			sb.WriteString(node.Raw)
		}
	}

	for _, node := range nodes {
		output(node)
	}
	return sb.String()
}
