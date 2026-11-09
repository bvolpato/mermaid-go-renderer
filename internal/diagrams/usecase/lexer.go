// Ported from the lexer of chevrotain 13.2.0 (scan/lexer_public.ts,
// tokenizeInternal) (Apache-2.0), with the configuration of
// diagrams/usecase/parser/usecase.lexer.ts of mermaid 12.2.0 (MIT): the
// default lexer config, which tracks the full position of each token.
//
// chevrotain has the Apache License 2.0. Its files have no copyright line,
// and the package has no NOTICE file. This file is a Go translation of the
// lexer. It is a changed version of scan/lexer_public.ts.
// LICENSE-chevrotain.txt in this directory has the license.

package usecase

import (
	"strconv"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// token is IToken of chevrotain. The offsets count UTF-16 code units, as in
// JavaScript. endOffset is the offset of the last code unit of the image.
type token struct {
	typ         tokenType
	image       string
	startOffset int
	endOffset   int
	startLine   int
	startColumn int
}

// endOfFile is END_OF_FILE of chevrotain: the token that the parser sees
// after the last token. Its location values are -1.
var endOfFile = &token{typ: tokEof, image: "", startOffset: -1, endOffset: -1, startLine: -1, startColumn: -1}

// lexError is ILexingError of chevrotain.
type lexError struct {
	offset  int
	line    int
	column  int
	length  int
	message string
}

// findPattern returns the pattern of a token type in a mode.
func findPattern(patterns []patternConfig, typ tokenType) *patternConfig {
	for i := range patterns {
		if patterns[i].typ == typ {
			return &patterns[i]
		}
	}
	return nil
}

// tokenize is Lexer.tokenize(text). It returns the tokens, or the first
// lexer error. The real lexer continues after an error, but the caller
// rejects a text with an error, so the tokens after it are not needed.
func tokenize(text []rune) ([]*token, *lexError) {
	orgLength := len(text)
	offset := 0
	var matchedTokens []*token
	line := 1
	column := 1

	modeStack := []string{defaultLexerMode}
	patternIdxToConfig := lexerModes[defaultLexerMode]

	for offset < orgLength {
		imageLength := -1
		var currConfig *patternConfig

		for i := range patternIdxToConfig {
			currConfig = &patternIdxToConfig[i]
			imageLength = matchers[currConfig.typ](text, offset)

			// longer alts handling
			if imageLength != -1 {
				// even though this pattern matched we must try a another longer alternative.
				// this can be used to prioritize keywords over identifiers
				for _, alt := range currConfig.longerAlt {
					longerAltConfig := findPattern(patternIdxToConfig, alt)
					altLength := matchers[alt](text, offset)
					if altLength > 0 && altLength > imageLength {
						imageLength = altLength
						currConfig = longerAltConfig
						// Exit the loop early after matching one of the longer alternatives
						// The first matched alternative takes precedence
						break
					}
				}
				break
			}
		}

		// successful match
		if imageLength != -1 {
			var newToken *token
			if !currConfig.skipped {
				newToken = &token{
					typ:         currConfig.typ,
					image:       js.FromUTF16Units(text[offset : offset+imageLength]),
					startOffset: offset,
					endOffset:   offset + imageLength - 1,
					startLine:   line,
					startColumn: column,
				}
				matchedTokens = append(matchedTokens, newToken)
			}

			// line terminator handling
			if currConfig.canLineTerminator {
				numOfLTsInMatch := 0
				lastLTEndOffset := 0
				// LineTerminatorOptimizedTester: /\n|\r\n?/g
				for i := 0; i < imageLength; i++ {
					c := text[offset+i]
					if c == '\n' {
						lastLTEndOffset = i
						numOfLTsInMatch++
					} else if c == '\r' {
						if i+1 < imageLength && text[offset+i+1] == '\n' {
							i++
						}
						lastLTEndOffset = i
						numOfLTsInMatch++
					}
				}
				if numOfLTsInMatch != 0 {
					line = line + numOfLTsInMatch
					column = imageLength - lastLTEndOffset
				} else {
					column = column + imageLength
				}
			} else {
				column = column + imageLength
			}

			offset = offset + imageLength

			// handleModes
			if currConfig.pop {
				// A pop token exists only in a mode that a push token entered,
				// so the mode stack has more than one mode here.
				if len(modeStack) > 1 {
					modeStack = modeStack[:len(modeStack)-1]
					patternIdxToConfig = lexerModes[modeStack[len(modeStack)-1]]
				}
				if currConfig.push != "" {
					modeStack = append(modeStack, currConfig.push)
					patternIdxToConfig = lexerModes[currConfig.push]
				}
			} else if currConfig.push != "" {
				modeStack = append(modeStack, currConfig.push)
				patternIdxToConfig = lexerModes[currConfig.push]
			}
		} else {
			// error recovery, drop characters until we identify a valid token's start point
			errorStartOffset := offset
			errorLine := line
			errorColumn := column
			foundResyncPoint := false

			for !foundResyncPoint && offset < orgLength {
				offset++
				for j := range patternIdxToConfig {
					foundResyncPoint = matchers[patternIdxToConfig[j].typ](text, offset) != -1
					if foundResyncPoint {
						break
					}
				}
			}

			errLength := offset - errorStartOffset
			// buildUnexpectedCharactersMessage
			msg := "unexpected character: ->" + js.FromUTF16Units(text[errorStartOffset:errorStartOffset+1]) + "<- at offset: " +
				strconv.Itoa(errorStartOffset) + ", skipped " + strconv.Itoa(errLength) + " characters."
			return nil, &lexError{
				offset:  errorStartOffset,
				line:    errorLine,
				column:  errorColumn,
				length:  errLength,
				message: msg,
			}
		}
	}

	return matchedTokens, nil
}
