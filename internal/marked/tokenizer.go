// Ported from marked/src/Tokenizer.ts (marked 16.3.0), for the default
// options: gfm is true, breaks and pedantic are false.

package marked

import (
	"slices"

	"github.com/dlclark/regexp2"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// link is the value of a link reference definition: { href, title }.
type link struct {
	href     string
	title    string
	hasTitle bool
}

// outputLink makes a link or an image token. cap0 and cap1 are cap[0] and
// cap[1] of the caller.
func outputLink(cap0, cap1 []rune, link link, raw []rune, lexer *lexer, rules *rules) *Token {
	href := link.href
	// const title = link.title || null;
	hasTitle := link.hasTitle && link.title != ""
	text := replaceAll(rules.other.outputLinkReplace, cap1, func(m *regexp2.Match) []rune {
		g, _ := group(m, 1)
		return g
	})

	lexer.state.inLink = true
	token := &Token{
		Type:     "link",
		Raw:      str(raw),
		Href:     href,
		HasTitle: hasTitle,
		Text:     str(text),
		shape:    shapeLink,
		n:        len(raw),
	}
	if len(cap0) > 0 && cap0[0] == '!' {
		token.Type = "image"
	}
	if hasTitle {
		token.Title = link.title
	}
	token.Tokens = lexer.inlineTokens(text, []*Token{})
	lexer.state.inLink = false
	return token
}

func indentCodeCompensation(raw, text []rune, rules *rules) []rune {
	matchIndentToCode := rules.other.indentCodeCompensation.exec(raw)

	if matchIndentToCode == nil {
		return text
	}

	indentToCode, _ := group(matchIndentToCode, 1)

	nodes := split(text, '\n')
	for i, node := range nodes {
		matchIndentInNode := rules.other.beginningSpace.exec(node)
		if matchIndentInNode == nil {
			continue
		}

		indentInNode := matchIndentInNode.Length

		if indentInNode >= len(indentToCode) {
			nodes[i] = sliceFrom(node, len(indentToCode))
		}
	}
	return join(nodes, '\n')
}

// tokenizer is the Tokenizer class.
type tokenizer struct {
	rules *rules // set by the lexer
	lexer *lexer // set by the lexer
}

func (t *tokenizer) space(src []rune) *Token {
	cap := t.rules.block.newline.exec(src)
	if cap != nil && cap.Length > 0 {
		return &Token{
			Type:  "space",
			Raw:   str(cap.Runes()),
			shape: shapeRaw,
			n:     cap.Length,
		}
	}
	return nil
}

func (t *tokenizer) code(src []rune) *Token {
	cap := t.rules.block.code.exec(src)
	if cap != nil {
		text := replaceAll(t.rules.other.codeRemoveIndent, cap.Runes(), func(*regexp2.Match) []rune { return nil })
		return &Token{
			Type:           "code",
			Raw:            str(cap.Runes()),
			CodeBlockStyle: "indented",
			Text:           str(rtrim(text, '\n')),
			shape:          shapeCodeIndent,
			n:              cap.Length,
		}
	}
	return nil
}

// unescapePunctuation is s.replace(this.rules.inline.anyPunctuation, '$1').
func (t *tokenizer) unescapePunctuation(s []rune) []rune {
	if indexOf(s, '\\') < 0 {
		// The rule needs a backslash.
		return s
	}
	u := newUString(s)
	var out []rune
	last := 0
	for pos := 0; ; {
		m, ok := t.rules.inline.anyPunctuation.execU(&u, pos)
		if !ok {
			break
		}
		g, _ := m.group(1)
		out = append(out, s[last:m.index()]...)
		out = append(out, g...)
		last = m.end()
		pos = last
	}
	if last == 0 {
		return s
	}
	return append(out, s[last:]...)
}

func (t *tokenizer) fences(src []rune) *Token {
	cap := t.rules.block.fences.exec(src)
	if cap != nil {
		raw := cap.Runes()
		cap3, _ := group(cap, 3)
		text := indentCodeCompensation(raw, cap3, t.rules)

		// cap[2] takes part in each match, so lang is a string.
		cap2, _ := group(cap, 2)
		lang := cap2
		if len(cap2) > 0 {
			lang = t.unescapePunctuation(trim(cap2))
		}
		return &Token{
			Type:  "code",
			Raw:   str(raw),
			Lang:  str(lang),
			Text:  str(text),
			shape: shapeCodeFence,
			n:     len(raw),
		}
	}
	return nil
}

func (t *tokenizer) heading(src []rune) *Token {
	cap := t.rules.block.heading.exec(src)
	if cap != nil {
		cap1, _ := group(cap, 1)
		cap2, _ := group(cap, 2)
		text := trim(cap2)

		// remove trailing #s
		//
		// The tests are other.endingHash, /#$/, and other.endingSpaceChar, / $/.
		if len(text) > 0 && text[len(text)-1] == '#' {
			trimmed := rtrim(text, '#')
			if len(trimmed) == 0 || trimmed[len(trimmed)-1] == ' ' {
				// CommonMark requires space before trailing #s
				text = trim(trimmed)
			}
		}

		token := &Token{
			Type:   "heading",
			Raw:    str(cap.Runes()),
			Depth:  len(cap1),
			Text:   str(text),
			Tokens: []*Token{},
			shape:  shapeHeading,
			n:      cap.Length,
		}
		t.lexer.inline(token.Text, &token.Tokens)
		return token
	}
	return nil
}

func (t *tokenizer) hr(src []rune) *Token {
	cap := t.rules.block.hr.exec(src)
	if cap != nil {
		raw := rtrim(cap.Runes(), '\n')
		return &Token{
			Type:  "hr",
			Raw:   str(raw),
			shape: shapeRaw,
			n:     len(raw),
		}
	}
	return nil
}

func (t *tokenizer) blockquote(src []rune) *Token {
	// Only cap[0] of this rule is in use. Its capture groups differ from
	// the ones of JavaScript: see wholeMatches in the test.
	cap := t.rules.block.blockquote.exec(src)
	if cap != nil {
		lines := split(rtrim(cap.Runes(), '\n'), '\n')
		var raw, text []rune
		tokens := []*Token{}

		for len(lines) > 0 {
			inBlockquote := false
			var currentLines [][]rune

			var i int
			for i = 0; i < len(lines); i++ {
				// get lines up to a continuation
				if t.rules.other.blockquoteStart.test(lines[i]) {
					currentLines = append(currentLines, lines[i])
					inBlockquote = true
				} else if !inBlockquote {
					currentLines = append(currentLines, lines[i])
				} else {
					break
				}
			}
			lines = lines[i:]

			currentRaw := join(currentLines, '\n')
			// precede setext continuation with 4 spaces so it isn't a setext
			currentText := replaceAll(t.rules.other.blockquoteSetextReplace, currentRaw, func(m *regexp2.Match) []rune {
				g, _ := group(m, 1)
				return concat([]rune("\n    "), g)
			})
			currentText = replaceAll(t.rules.other.blockquoteSetextReplace2, currentText, func(*regexp2.Match) []rune { return nil })
			if len(raw) > 0 {
				raw = concat(raw, []rune{'\n'}, currentRaw)
			} else {
				raw = currentRaw
			}
			if len(text) > 0 {
				text = concat(text, []rune{'\n'}, currentText)
			} else {
				text = currentText
			}

			// parse blockquote lines as top level tokens
			// merge paragraphs if this is a continuation
			top := t.lexer.state.top
			t.lexer.state.top = true
			tokens = t.lexer.blockTokens(currentText, tokens, true)
			t.lexer.state.top = top

			// if there is no continuation then we are done
			if len(lines) == 0 {
				break
			}

			var lastToken *Token
			if len(tokens) > 0 {
				lastToken = tokens[len(tokens)-1]
			}

			if lastToken != nil && lastToken.Type == "code" {
				// blockquote continuation cannot be preceded by a code block
				break
			} else if lastToken != nil && lastToken.Type == "blockquote" {
				// include continuation in nested blockquote
				oldToken := lastToken
				newText := concat(units(oldToken.Raw), []rune{'\n'}, join(lines, '\n'))
				newToken := t.blockquote(newText)
				if newToken == nil {
					// marked reads a property of undefined here.
					t.lexer.throw("TypeError: Cannot read properties of undefined (reading 'raw')")
				}
				tokens[len(tokens)-1] = newToken

				raw = concat(substring(raw, 0, len(raw)-js.UTF16Len(oldToken.Raw)), units(newToken.Raw))
				text = concat(substring(text, 0, len(text)-js.UTF16Len(oldToken.Text)), units(newToken.Text))
				break
			} else if lastToken != nil && lastToken.Type == "list" {
				// include continuation in nested list
				oldToken := lastToken
				newText := concat(units(oldToken.Raw), []rune{'\n'}, join(lines, '\n'))
				newToken := t.list(newText)
				if newToken == nil {
					// marked reads a property of undefined here.
					t.lexer.throw("TypeError: Cannot read properties of undefined (reading 'raw')")
				}
				tokens[len(tokens)-1] = newToken

				raw = concat(substring(raw, 0, len(raw)-js.UTF16Len(lastToken.Raw)), units(newToken.Raw))
				text = concat(substring(text, 0, len(text)-js.UTF16Len(oldToken.Raw)), units(newToken.Raw))
				lines = split(substringFrom(newText, js.UTF16Len(tokens[len(tokens)-1].Raw)), '\n')
				continue
			}
		}

		return &Token{
			Type:   "blockquote",
			Raw:    str(raw),
			Tokens: tokens,
			Text:   str(text),
			shape:  shapeBlockquote,
			n:      len(raw),
		}
	}
	return nil
}

func (t *tokenizer) list(src []rune) *Token {
	cap := t.rules.block.list.exec(src)
	if cap != nil {
		cap1, _ := group(cap, 1)
		bull := trim(cap1)
		isordered := len(bull) > 1

		list := &Token{
			Type:    "list",
			Ordered: isordered,
			Loose:   false,
			Items:   []*Token{},
			shape:   shapeList,
		}
		if isordered {
			// start: +bull.slice(0, -1). The rule allows one to nine digits.
			for _, d := range bull[:len(bull)-1] {
				list.Start = list.Start*10 + int(d-'0')
			}
		}
		var listRaw []rune

		// bull = isordered ? `\\d{1,9}\\${bull.slice(-1)}` : `\\${bull}`;
		// The regex of a bullet depends only on its last character.
		itemRegex := t.rules.other.listItemRegex[bull[len(bull)-1]]

		endsWithBlankLine := false
		// Check if current bullet point can start a new List Item
		for len(src) > 0 {
			endEarly := false
			var raw, itemContents []rune
			cap = itemRegex.exec(src)
			if cap == nil {
				break
			}

			if t.rules.block.hr.test(src) { // End list if bullet was actually HR (possibly move into itemRegex?)
				break
			}

			raw = cap.Runes()
			src = substringFrom(src, len(raw))

			cap1, _ = group(cap, 1)
			cap2, _ := group(cap, 2)
			line := replaceFirst(t.rules.other.listReplaceTabs, firstLine(cap2), func(m *regexp2.Match) []rune {
				return repeat(' ', 3*m.Length)
			})
			nextLine := firstLine(src)
			blankLine := len(trim(line)) == 0

			indent := 0
			if blankLine {
				indent = len(cap1) + 1
			} else {
				indent = searchNonSpace(cap2) // Find first non-space char
				if indent > 4 {               // Treat indented code blocks (> 4 spaces) as having only 1 indent
					indent = 1
				}
				itemContents = sliceFrom(line, indent)
				indent += len(cap1)
			}

			if blankLine && t.rules.other.blankLine.test(nextLine) { // Items begin with at most one blank line
				raw = concat(raw, nextLine, []rune{'\n'})
				src = substringFrom(src, len(nextLine)+1)
				endEarly = true
			}

			if !endEarly {
				// The index is Math.min(3, indent - 1). indent is 1 or more.
				n := min(3, indent-1)
				nextBulletRegex := t.rules.other.nextBulletRegex[n]
				hrRegex := t.rules.other.hrRegex[n]
				fencesBeginRegex := t.rules.other.fencesBeginRegex[n]
				headingBeginRegex := t.rules.other.headingBeginRegex[n]
				htmlBeginRegex := t.rules.other.htmlBeginRegex[n]

				// Check if following lines should be included in List Item
				for len(src) > 0 {
					rawLine := firstLine(src)
					nextLine = rawLine

					// Re-align to follow commonmark nesting rules
					nextLineWithoutTabs := replaceTabs(nextLine)

					// End list item if found code fences
					if fencesBeginRegex.test(nextLine) {
						break
					}

					// End list item if found start of new heading
					if headingBeginRegex.test(nextLine) {
						break
					}

					// End list item if found start of html block
					if htmlBeginRegex.test(nextLine) {
						break
					}

					// End list item if found start of new bullet
					if nextBulletRegex.test(nextLine) {
						break
					}

					// Horizontal rule found
					if hrRegex.test(nextLine) {
						break
					}

					if searchNonSpace(nextLineWithoutTabs) >= indent || len(trim(nextLine)) == 0 { // Dedent if possible
						itemContents = concat(itemContents, []rune{'\n'}, sliceFrom(nextLineWithoutTabs, indent))
					} else {
						// not enough indentation
						if blankLine {
							break
						}

						// paragraph continuation unless last line was a different block level element
						if searchNonSpace(replaceTabs(line)) >= 4 { // indented code block
							break
						}
						if fencesBeginRegex.test(line) {
							break
						}
						if headingBeginRegex.test(line) {
							break
						}
						if hrRegex.test(line) {
							break
						}

						itemContents = concat(itemContents, []rune{'\n'}, nextLine)
					}

					if !blankLine && len(trim(nextLine)) == 0 { // Check if current line is blank
						blankLine = true
					}

					raw = concat(raw, rawLine, []rune{'\n'})
					src = substringFrom(src, len(rawLine)+1)
					line = sliceFrom(nextLineWithoutTabs, indent)
				}
			}

			if !list.Loose {
				// If the previous item ended with a blank line, the list is loose
				if endsWithBlankLine {
					list.Loose = true
				} else if t.rules.other.doubleBlankLine.test(raw) {
					endsWithBlankLine = true
				}
			}

			istask := false
			ischecked := false
			// Check for task list items
			if m := t.rules.other.listIsTask.exec(itemContents); m != nil {
				istask = true
				ischecked = !equal(m.Runes(), "[ ] ")
				itemContents = replaceFirst(t.rules.other.listReplaceTask, itemContents, func(*regexp2.Match) []rune { return nil })
			}

			list.Items = append(list.Items, &Token{
				Type:    "list_item",
				Raw:     str(raw),
				Task:    istask,
				Checked: ischecked,
				Loose:   false,
				Text:    str(itemContents),
				Tokens:  []*Token{},
				shape:   shapeListItem,
			})

			listRaw = concat(listRaw, raw)
		}

		// Do not consume newlines at end of final item. Alternatively, make itemRegex *start* with any newlines to simplify/speed up endsWithBlankLine logic
		if len(list.Items) > 0 {
			lastItem := list.Items[len(list.Items)-1]
			lastItem.Raw = js.TrimEnd(lastItem.Raw)
			lastItem.Text = js.TrimEnd(lastItem.Text)
		} else {
			// not a list since there were no items
			return nil
		}
		listRaw = trimEnd(listRaw)
		list.Raw = str(listRaw)
		list.n = len(listRaw)

		// Item child tokens handled here at end because we needed to have the final item to trim it first
		for _, item := range list.Items {
			t.lexer.state.top = false
			item.Tokens = t.lexer.blockTokens(units(item.Text), []*Token{}, false)

			if !list.Loose {
				// Check if list should be loose
				hasMultipleLineBreaks := false
				for _, tok := range item.Tokens {
					if tok.Type == "space" && t.rules.other.anyLine.test(units(tok.Raw)) {
						hasMultipleLineBreaks = true
						break
					}
				}

				list.Loose = hasMultipleLineBreaks
			}
		}

		// Set all items to loose if list is loose
		if list.Loose {
			for _, item := range list.Items {
				item.Loose = true
			}
		}

		return list
	}
	return nil
}

func (t *tokenizer) html(src []rune) *Token {
	cap := t.rules.block.html.exec(src)
	if cap != nil {
		// cap[1] of marked is one of the first groups here: see
		// expandHTMLScript.
		pre := false
		for i := 1; i <= htmlScriptGroups; i++ {
			if cap1, ok := group(cap, i); ok {
				pre = equal(cap1, "pre") || equal(cap1, "script") || equal(cap1, "style")
				break
			}
		}
		raw := str(cap.Runes())
		return &Token{
			Type:  "html",
			Block: true,
			Raw:   raw,
			Pre:   pre,
			Text:  raw,
			shape: shapeHTMLBlock,
			n:     cap.Length,
		}
	}
	return nil
}

// collapseSpaces is s.replace(this.rules.other.multipleSpaceGlobal, ' ').
func (t *tokenizer) collapseSpaces(s []rune) []rune {
	return replaceAll(t.rules.other.multipleSpaceGlobal, s, func(*regexp2.Match) []rune { return []rune{' '} })
}

func (t *tokenizer) def(src []rune) *Token {
	cap := t.rules.block.def.exec(src)
	if cap != nil {
		cap1, _ := group(cap, 1)
		cap2, _ := group(cap, 2)
		cap3, hasTitle := group(cap, 3)
		tag := t.collapseSpaces(units(js.ToLowerCase(str(cap1))))
		var href []rune
		if len(cap2) > 0 {
			href = replaceFirst(t.rules.other.hrefBrackets, cap2, func(m *regexp2.Match) []rune {
				g, _ := group(m, 1)
				return g
			})
			href = t.unescapePunctuation(href)
		}
		title := cap3
		if len(cap3) > 0 {
			title = t.unescapePunctuation(substring(cap3, 1, len(cap3)-1))
		}
		return &Token{
			Type:     "def",
			Tag:      str(tag),
			Raw:      str(cap.Runes()),
			Href:     str(href),
			Title:    str(title),
			HasTitle: hasTitle,
			shape:    shapeDef,
			n:        cap.Length,
		}
	}
	return nil
}

func (t *tokenizer) table(src []rune) *Token {
	cap := t.rules.block.table.exec(src)
	if cap == nil {
		return nil
	}

	cap1, _ := group(cap, 1)
	cap2, _ := group(cap, 2)
	cap3, _ := group(cap, 3)
	if !t.rules.other.tableDelimiter.test(cap2) {
		// delimiter row must have a pipe (|) or colon (:) otherwise it is a setext heading
		return nil
	}

	headers := splitCells(cap1, 0)
	aligns := split(replaceAll(t.rules.other.tableAlignChars, cap2, func(*regexp2.Match) []rune { return nil }), '|')
	var rows [][]rune
	if len(trim(cap3)) > 0 {
		rows = split(replaceFirst(t.rules.other.tableRowBlankLine, cap3, func(*regexp2.Match) []rune { return nil }), '\n')
	}

	item := &Token{
		Type:   "table",
		Raw:    str(cap.Runes()),
		Header: []TableCell{},
		Align:  []string{},
		Rows:   [][]TableCell{},
		shape:  shapeTable,
		n:      cap.Length,
	}

	if len(headers) != len(aligns) {
		// header and align columns must be equal, rows can be different.
		return nil
	}

	for _, align := range aligns {
		if t.rules.other.tableAlignRight.test(align) {
			item.Align = append(item.Align, "right")
		} else if t.rules.other.tableAlignCenter.test(align) {
			item.Align = append(item.Align, "center")
		} else if t.rules.other.tableAlignLeft.test(align) {
			item.Align = append(item.Align, "left")
		} else {
			item.Align = append(item.Align, "")
		}
	}

	// The cells are made before the inline queue takes the address of
	// their tokens.
	item.Header = make([]TableCell, len(headers))
	for i := range headers {
		item.Header[i] = TableCell{
			Text:   str(headers[i]),
			Tokens: []*Token{},
			Header: true,
			Align:  item.Align[i],
		}
		t.lexer.inline(item.Header[i].Text, &item.Header[i].Tokens)
	}

	for _, row := range rows {
		cells := splitCells(row, len(item.Header))
		out := make([]TableCell, len(cells))
		for i, cell := range cells {
			out[i] = TableCell{
				Text:   str(cell),
				Tokens: []*Token{},
				Header: false,
				Align:  item.Align[i],
			}
			t.lexer.inline(out[i].Text, &out[i].Tokens)
		}
		item.Rows = append(item.Rows, out)
	}

	return item
}

// setextUnderline reports whether the lheading rule can match src. The
// rule ends with \n {0,3}(=+|-+) *(?:\n+|$), so a match has a line feed,
// then zero to three spaces, then = or -. The text of the heading cannot
// go over a line feed that another line feed follows, so the search stops
// there.
func setextUnderline(src []rune) bool {
	for i := 0; i < len(src); i++ {
		if src[i] != '\n' {
			continue
		}
		j := i + 1
		for j < len(src) && j < i+4 && src[j] == ' ' {
			j++
		}
		if j == len(src) {
			return false
		}
		switch src[j] {
		case '=', '-':
			return true
		case '\n':
			if j == i+1 {
				return false
			}
		}
	}
	return false
}

func (t *tokenizer) lheading(src []rune) *Token {
	if !setextUnderline(src) {
		return nil
	}
	cap := t.rules.block.lheading.exec(src)
	if cap != nil {
		cap1, _ := group(cap, 1)
		cap2, _ := group(cap, 2)
		depth := 2
		if len(cap2) > 0 && cap2[0] == '=' {
			depth = 1
		}
		token := &Token{
			Type:   "heading",
			Raw:    str(cap.Runes()),
			Depth:  depth,
			Text:   str(cap1),
			Tokens: []*Token{},
			shape:  shapeHeading,
			n:      cap.Length,
		}
		t.lexer.inline(token.Text, &token.Tokens)
		return token
	}
	return nil
}

func (t *tokenizer) paragraph(src []rune) *Token {
	cap := t.rules.block.paragraph.exec(src)
	if cap != nil {
		// Group 1 of the rule holds the whole rule, so cap[1] is cap[0].
		cap1 := cap.Runes()
		text := cap1
		if len(cap1) > 0 && cap1[len(cap1)-1] == '\n' {
			text = slice(cap1, 0, -1)
		}
		token := &Token{
			Type:   "paragraph",
			Raw:    str(cap.Runes()),
			Text:   str(text),
			Tokens: []*Token{},
			shape:  shapeTextTokens,
			n:      cap.Length,
		}
		t.lexer.inline(token.Text, &token.Tokens)
		return token
	}
	return nil
}

func (t *tokenizer) text(src []rune) *Token {
	cap := t.rules.block.text.exec(src)
	if cap != nil {
		raw := str(cap.Runes())
		token := &Token{
			Type:   "text",
			Raw:    raw,
			Text:   raw,
			Tokens: []*Token{},
			shape:  shapeTextTokens,
			n:      cap.Length,
		}
		t.lexer.inline(token.Text, &token.Tokens)
		return token
	}
	return nil
}

func (t *tokenizer) escape(src []rune) *Token {
	cap := t.rules.inline.escape.exec(src)
	if cap != nil {
		cap1, _ := group(cap, 1)
		return &Token{
			Type:  "escape",
			Raw:   str(cap.Runes()),
			Text:  str(cap1),
			shape: shapeText,
			n:     cap.Length,
		}
	}
	return nil
}

func (t *tokenizer) tag(src []rune) *Token {
	cap := t.rules.inline.tag.exec(src)
	if cap != nil {
		cap0 := cap.Runes()
		if !t.lexer.state.inLink && t.rules.other.startATag.test(cap0) {
			t.lexer.state.inLink = true
		} else if t.lexer.state.inLink && t.rules.other.endATag.test(cap0) {
			t.lexer.state.inLink = false
		}
		if !t.lexer.state.inRawBlock && t.rules.other.startPreScriptTag.test(cap0) {
			t.lexer.state.inRawBlock = true
		} else if t.lexer.state.inRawBlock && t.rules.other.endPreScriptTag.test(cap0) {
			t.lexer.state.inRawBlock = false
		}

		raw := str(cap0)
		return &Token{
			Type:       "html",
			Raw:        raw,
			InLink:     t.lexer.state.inLink,
			InRawBlock: t.lexer.state.inRawBlock,
			Block:      false,
			Text:       raw,
			shape:      shapeHTMLInline,
			n:          cap.Length,
		}
	}
	return nil
}

func (t *tokenizer) link(src []rune) *Token {
	cap := t.rules.inline.link.exec(src)
	if cap != nil {
		cap0 := cap.Runes()
		cap1, _ := group(cap, 1)
		cap2, _ := group(cap, 2)
		cap3, _ := group(cap, 3)
		trimmedUrl := trim(cap2)
		// The tests are other.startAngleBracket, /^</, and
		// other.endAngleBracket, />$/.
		if len(trimmedUrl) > 0 && trimmedUrl[0] == '<' {
			// commonmark requires matching angle brackets
			if trimmedUrl[len(trimmedUrl)-1] != '>' {
				return nil
			}

			// ending angle bracket cannot be escaped
			rtrimSlash := rtrim(slice(trimmedUrl, 0, -1), '\\')
			if (len(trimmedUrl)-len(rtrimSlash))%2 == 0 {
				return nil
			}
		} else {
			// find closing parenthesis
			lastParenIndex := findClosingBracket(cap2, '(', ')')
			if lastParenIndex == -2 {
				// more open parens than closed
				return nil
			}

			if lastParenIndex > -1 {
				start := 4
				if cap0[0] == '!' {
					start = 5
				}
				linkLen := start + len(cap1) + lastParenIndex
				cap2 = substring(cap2, 0, lastParenIndex)
				cap0 = trim(substring(cap0, 0, linkLen))
				cap3 = nil
			}
		}
		href := cap2
		var title []rune
		if len(cap3) > 0 {
			title = slice(cap3, 1, -1)
		}

		href = trim(href)
		if len(href) > 0 && href[0] == '<' {
			href = slice(href, 1, -1)
		}
		if len(href) > 0 {
			href = t.unescapePunctuation(href)
		}
		if len(title) > 0 {
			title = t.unescapePunctuation(title)
		}
		return outputLink(cap0, cap1, link{
			href:     str(href),
			title:    str(title),
			hasTitle: true,
		}, cap0, t.lexer, t.rules)
	}
	return nil
}

func (t *tokenizer) reflink(src []rune, links map[string]link) *Token {
	cap := t.rules.inline.reflink.exec(src)
	if cap == nil {
		cap = t.rules.inline.nolink.exec(src)
	}
	if cap != nil {
		cap0 := cap.Runes()
		cap1, _ := group(cap, 1)
		cap2, _ := group(cap, 2)
		// (cap[2] || cap[1])
		linkString := cap2
		if len(linkString) == 0 {
			linkString = cap1
		}
		linkString = t.collapseSpaces(linkString)
		link, ok := links[js.ToLowerCase(str(linkString))]
		if !ok {
			text := str(cap0[:1])
			return &Token{
				Type:  "text",
				Raw:   text,
				Text:  text,
				shape: shapeText,
				n:     1,
			}
		}
		return outputLink(cap0, cap1, link, cap0, t.lexer, t.rules)
	}
	return nil
}

func (t *tokenizer) emStrong(src, maskedSrc []rune, prevChar []rune) *Token {
	// emStrongLDelim has ^ at its start, and then a run of * or of _.
	if src[0] != '*' && src[0] != '_' {
		return nil
	}

	// The rule matches a run of one delimiter and then one code point. So
	// it reads at most the run and two more code units.
	run := 0
	for run < len(src) && src[run] == src[0] {
		run++
	}
	usrc := newUString(src[:min(len(src), run+2)])
	match, ok := t.rules.inline.emStrongLDelim.execU(&usrc, 0)
	if !ok {
		return nil
	}

	// _ can't be between two alphanumerics. \p{L}\p{N} includes non-english alphabet/numbers as well
	match3, _ := match.group(3)
	if len(match3) > 0 && len(prevChar) > 0 {
		uprev := newUString(prevChar)
		if _, ok := t.rules.other.unicodeAlphaNumeric.execU(&uprev, 0); ok {
			return nil
		}
	}

	nextChar, _ := match.group(1)
	if len(nextChar) == 0 {
		nextChar, _ = match.group(2)
	}

	punctuation := false
	if len(nextChar) > 0 && len(prevChar) > 0 {
		uprev := newUString(prevChar)
		_, punctuation = t.rules.inline.punctuation.execU(&uprev, 0)
	}
	if len(nextChar) == 0 || len(prevChar) == 0 || punctuation {
		// unicode Regex counts emoji as 1 char; spread into array for proper count (used multiple times below)
		lLength := match.m.Length - 1
		delimTotal, midDelimTotal := lLength, 0

		endReg := t.rules.inline.emStrongRDelimUnd
		if src[0] == '*' {
			endReg = t.rules.inline.emStrongRDelimAst
		}
		lastIndex := 0

		// Clip maskedSrc to same section of string as src (move to lexer?)
		maskedSrc = sliceFrom(maskedSrc, -1*len(src)+lLength)
		umasked := newUString(maskedSrc)

		for {
			match, ok = endReg.execU(&umasked, lastIndex)
			if !ok {
				break
			}
			lastIndex = match.end()

			var rDelim []rune
			for i := 1; i <= 6 && len(rDelim) == 0; i++ {
				rDelim, _ = match.group(i)
			}

			if len(rDelim) == 0 {
				continue // skip single * in __abc*abc__
			}

			// The delimiter holds only * or _, so its code points are its
			// code units.
			rLength := len(rDelim)

			g3, _ := match.group(3)
			g4, _ := match.group(4)
			g5, _ := match.group(5)
			g6, _ := match.group(6)
			if len(g3) > 0 || len(g4) > 0 { // found another Left Delim
				delimTotal += rLength
				continue
			} else if len(g5) > 0 || len(g6) > 0 { // either Left or Right Delim
				if lLength%3 != 0 && (lLength+rLength)%3 == 0 {
					midDelimTotal += rLength
					continue // CommonMark Emphasis Rules 9-10
				}
			}

			delimTotal -= rLength

			if delimTotal > 0 {
				continue // Haven't found enough closing delimiters
			}

			// Remove extra characters. *a*** -> *a*
			rLength = min(rLength, rLength+delimTotal+midDelimTotal)
			// char length can be >1 for unicode characters;
			lastCharLength := match.firstLen()
			raw := slice(src, 0, lLength+match.index()+lastCharLength+rLength)

			// Create `em` if smallest delimiter has odd char count. *a***
			if min(lLength, rLength)%2 != 0 {
				text := slice(raw, 1, -1)
				return &Token{
					Type:   "em",
					Raw:    str(raw),
					Text:   str(text),
					Tokens: t.lexer.inlineTokens(text, []*Token{}),
					shape:  shapeTextTokens,
					n:      len(raw),
				}
			}

			// Create 'strong' if smallest delimiter has even char count. **a***
			text := slice(raw, 2, -2)
			return &Token{
				Type:   "strong",
				Raw:    str(raw),
				Text:   str(text),
				Tokens: t.lexer.inlineTokens(text, []*Token{}),
				shape:  shapeTextTokens,
				n:      len(raw),
			}
		}
	}
	return nil
}

func (t *tokenizer) codespan(src []rune) *Token {
	cap := t.rules.inline.code.exec(src)
	if cap != nil {
		cap2, _ := group(cap, 2)
		// The replace is .replace(other.newLineCharGlobal, ' '), where
		// newLineCharGlobal is /\n/g.
		text := make([]rune, len(cap2))
		for i, r := range cap2 {
			if r == '\n' {
				r = ' '
			}
			text[i] = r
		}
		// The tests are other.nonSpaceChar, /[^ ]/, other.startingSpaceChar,
		// /^ /, and other.endingSpaceChar, / $/.
		hasNonSpaceChars := searchNonSpace(text) >= 0
		hasSpaceCharsOnBothEnds := len(text) > 0 && text[0] == ' ' && text[len(text)-1] == ' '
		if hasNonSpaceChars && hasSpaceCharsOnBothEnds {
			text = substring(text, 1, len(text)-1)
		}
		return &Token{
			Type:  "codespan",
			Raw:   str(cap.Runes()),
			Text:  str(text),
			shape: shapeText,
			n:     cap.Length,
		}
	}
	return nil
}

func (t *tokenizer) br(src []rune) *Token {
	cap := t.rules.inline.br.exec(src)
	if cap != nil {
		return &Token{
			Type:  "br",
			Raw:   str(cap.Runes()),
			shape: shapeRaw,
			n:     cap.Length,
		}
	}
	return nil
}

func (t *tokenizer) del(src []rune) *Token {
	cap := t.rules.inline.del.exec(src)
	if cap != nil {
		cap2, _ := group(cap, 2)
		return &Token{
			Type:   "del",
			Raw:    str(cap.Runes()),
			Text:   str(cap2),
			Tokens: t.lexer.inlineTokens(cap2, []*Token{}),
			shape:  shapeTextTokens,
			n:      cap.Length,
		}
	}
	return nil
}

// autolinkToken makes the link token of autolink and url.
func autolinkToken(raw, text []rune, href string) *Token {
	textStr := str(text)
	return &Token{
		Type: "link",
		Raw:  str(raw),
		Text: textStr,
		Href: href,
		Tokens: []*Token{
			{
				Type:  "text",
				Raw:   textStr,
				Text:  textStr,
				shape: shapeText,
				n:     len(text),
			},
		},
		shape: shapeAutolink,
		n:     len(raw),
	}
}

func (t *tokenizer) autolink(src []rune) *Token {
	cap := t.rules.inline.autolink.exec(src)
	if cap != nil {
		cap1, _ := group(cap, 1)
		cap2, _ := group(cap, 2)
		text := cap1
		href := str(text)
		if equal(cap2, "@") {
			href = "mailto:" + href
		}

		return autolinkToken(cap.Runes(), text, href)
	}
	return nil
}

// urlStart reports whether the url rule can match src. The rule has two
// alternatives. The first one starts with "ftp://", "http://", "https://"
// or "www." in any case. The second one starts with a run of the class
// [A-Za-z0-9._+-] and then an @, which is not in the class.
func urlStart(src []rune) bool {
	i := 0
	for i < len(src) && (isASCIILetter(src[i]) || isASCIIDigit(src[i]) || src[i] == '.' || src[i] == '_' || src[i] == '+' || src[i] == '-') {
		i++
	}
	if i > 0 && i < len(src) && src[i] == '@' {
		return true
	}
	lower := make([]rune, 0, 8)
	for _, c := range src[:min(len(src), 8)] {
		if c >= 'A' && c <= 'Z' {
			c |= 0x20
		}
		lower = append(lower, c)
	}
	return hasPrefix(lower, "ftp://") || hasPrefix(lower, "http://") || hasPrefix(lower, "https://") || hasPrefix(lower, "www.")
}

func (t *tokenizer) url(src []rune) *Token {
	if !urlStart(src) {
		return nil
	}
	cap := t.rules.inline.url.exec(src)
	if cap != nil {
		cap0 := cap.Runes()
		cap1, _ := group(cap, 1)
		cap2, _ := group(cap, 2)
		var text []rune
		var href string
		if equal(cap2, "@") {
			text = cap0
			href = "mailto:" + str(text)
		} else {
			// do extended autolink path validation
			for {
				prevCapZero := cap0
				// cap[0] = this.rules.inline._backpedal.exec(cap[0])?.[0] ?? '';
				if m := t.rules.inline.backpedal.exec(cap0); m != nil {
					cap0 = m.Runes()
				} else {
					cap0 = nil
				}
				if slices.Equal(prevCapZero, cap0) {
					break
				}
			}
			text = cap0
			if equal(cap1, "www.") {
				href = "http://" + str(cap0)
			} else {
				href = str(cap0)
			}
		}
		return autolinkToken(cap0, text, href)
	}
	return nil
}

func (t *tokenizer) inlineText(src []rune) *Token {
	// The match is this.rules.inline.text.exec(src). The rule matches each
	// input that is not empty, and the token needs only the length of the
	// match: see inlineTextLen.
	if len(src) > 0 {
		n := inlineTextLen(src)
		escaped := t.lexer.state.inRawBlock
		raw := str(src[:n])
		return &Token{
			Type:    "text",
			Raw:     raw,
			Text:    raw,
			Escaped: escaped,
			shape:   shapeInlineText,
			n:       n,
		}
	}
	return nil
}

// emailChars is the class [a-zA-Z0-9.!#$%&'*+\/=?_`{\|}~-] of the inline
// text rule.
var emailChars = func() (t [128]bool) {
	for c := range rune(128) {
		t[c] = isASCIILetter(c) || isASCIIDigit(c)
	}
	for _, c := range []byte(".!#$%&'*+/=?_`{|}~-") {
		t[c] = true
	}
	return t
}()

func isEmailChar(c rune) bool { return c < 128 && emailChars[c] }

// emailAhead is the lookahead (?=[a-zA-Z0-9.!#$%&'*+\/=?_`{\|}~-]+@) at
// s[i:]. The @ is not in the class, so the run of class characters is the
// longest one.
func emailAhead(s []rune, i int) bool {
	j := i
	for j < len(s) && isEmailChar(s[j]) {
		j++
	}
	return j > i && j < len(s) && s[j] == '@'
}

// hardBreakAhead is the lookahead (?= {2,}\n) at s[i:].
func hardBreakAhead(s []rune, i int) bool {
	j := i
	for j < len(s) && s[j] == ' ' {
		j++
	}
	return j-i >= 2 && j < len(s) && s[j] == '\n'
}

// hasPrefix reports whether s starts with the ASCII text.
func hasPrefix(s []rune, ascii string) bool {
	return len(s) >= len(ascii) && equal(s[:len(ascii)], ascii)
}

// inlineTextLen returns the length of the match of the inline text rule of
// the GFM grammar on s, which is not empty:
//
//	^([`~]+|[^`~])(?:(?= {2,}\n)|(?=EMAIL+@)|[\s\S]*?(?:(?=[\\<!\[`*~_]|\b_|https?:\/\/|ftp:\/\/|www\.|$)|[^ ](?= {2,}\n)|[^EMAIL](?=EMAIL+@)))
//
// EMAIL is the class of emailChars. This rule runs on each character of
// each label, and the regex engine needs about one microsecond for each
// character. So this function reads the rule by hand. The test compares it
// with the compiled rule on all strings of a small alphabet.
//
// The first group takes a run of backquotes and tildes, or one other
// character. The rest of the rule matches after each such start, because
// its last alternative matches at the end of the input. So the engine
// never gives back a part of the run.
func inlineTextLen(s []rune) int {
	q := 1
	if s[0] == '`' || s[0] == '~' {
		for q < len(s) && (s[q] == '`' || s[q] == '~') {
			q++
		}
	}
	// (?= {2,}\n) and (?=EMAIL+@): the match ends after the first group.
	if hardBreakAhead(s, q) || emailAhead(s, q) {
		return q
	}
	// [\s\S]*? is lazy: p is the first position where one of the three
	// alternatives matches.
	for p := q; ; p++ {
		// (?=...|$): the match ends at p.
		if p == len(s) {
			return p
		}
		c := s[p]
		switch c {
		case '\\', '<', '!', '[', '`', '*', '~', '_':
			// [\\<!\[`*~_]. The alternative \b_ adds nothing to it.
			return p
		case 'h':
			if hasPrefix(s[p:], "http://") || hasPrefix(s[p:], "https://") {
				return p
			}
		case 'f':
			if hasPrefix(s[p:], "ftp://") {
				return p
			}
		case 'w':
			if hasPrefix(s[p:], "www.") {
				return p
			}
		}
		// [^ ](?= {2,}\n): the match ends after the character at p.
		if c != ' ' && hardBreakAhead(s, p+1) {
			return p + 1
		}
		// [^EMAIL](?=EMAIL+@): the match ends after the character at p.
		if !isEmailChar(c) && emailAhead(s, p+1) {
			return p + 1
		}
	}
}
