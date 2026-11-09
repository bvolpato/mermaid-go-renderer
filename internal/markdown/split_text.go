// Ported from mermaid 12.2.0, mermaid/src/rendering-util/splitText.ts (MIT).

package markdown

import (
	"errors"
	"slices"
	"strings"

	"github.com/go-text/typesetting/segmenter"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// SplitTextToChars is splitTextToChars. It splits a string into graphemes:
// the segments of new Intl.Segmenter(), which Chromium has. The segments
// are the extended grapheme clusters of Unicode (UAX #29).
func SplitTextToChars(text string) []string {
	ascii := true
	for i := 0; i < len(text); i++ {
		if text[i] >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii {
		// Each ASCII character is a cluster, and CR LF is one cluster.
		chars := make([]string, 0, len(text))
		for i := 0; i < len(text); i++ {
			if text[i] == '\r' && i+1 < len(text) && text[i+1] == '\n' {
				chars = append(chars, text[i:i+2])
				i++
				continue
			}
			chars = append(chars, text[i:i+1])
		}
		return chars
	}

	// The code points of the text. A surrogate without its partner stays
	// one code point.
	units := js.UTF16Units(text)
	points := make([]rune, 0, len(units))
	for i := 0; i < len(units); i++ {
		u := units[i]
		if u >= 0xd800 && u < 0xdc00 && i+1 < len(units) && units[i+1] >= 0xdc00 && units[i+1] < 0xe000 {
			points = append(points, 0x10000+(u-0xd800)<<10+(units[i+1]-0xdc00))
			i++
			continue
		}
		points = append(points, u)
	}

	var seg segmenter.Segmenter
	seg.Init(points)
	iter := seg.GraphemeIterator()
	// start is the index in points where the text of seg starts.
	start := 0
	chars := []string{}
	for iter.Next() {
		g := iter.Grapheme()
		begin := start + g.Offset
		end := begin + len(g.Text)
		// The segmenter (go-text v0.3.5) has an error in rule GB11 of UAX
		// #29, which joins a pictograph, a ZWJ and a pictograph. It does
		// not start the rule at a pictograph that follows another pictograph
		// directly. Then it puts a break after the ZWJ, for example in the
		// middle of the second emoji of two family emojis. The rule reads
		// only one cluster, so the segmenter is correct for the first
		// cluster of a text. A cluster that ends with a ZWJ is therefore
		// segmented again as the start of a text.
		if g.Offset > 0 && end < len(points) && points[end-1] == 0x200d {
			start = begin
			seg.Init(points[start:])
			iter = seg.GraphemeIterator()
			iter.Next()
			end = start + len(iter.Grapheme().Text)
		}
		chars = append(chars, pointsToString(points[begin:end]))
	}
	return chars
}

// pointsToString converts code points to a string. A surrogate keeps the
// three-byte form that internal/js defines.
func pointsToString(points []rune) string {
	units := make([]rune, 0, len(points)+2)
	for _, p := range points {
		if p >= 0x10000 {
			p -= 0x10000
			units = append(units, 0xd800+(p>>10), 0xdc00+(p&0x3ff))
			continue
		}
		units = append(units, p)
	}
	return js.FromUTF16Units(units)
}

// splitWordToFitWidth is splitWordToFitWidth with
// splitWordToFitWidthRecursion. It splits a word into two parts: the first
// part fits the width, and the second part is the rest of the word.
// Upstream is recursive: each call takes one more character. The loop
// calls checkFit with the same lines in the same order.
func splitWordToFitWidth(checkFit func(line []Word) bool, word Word) (Word, Word) {
	usedChars := []string{}
	remainingChars := SplitTextToChars(word.Content)
	for len(remainingChars) > 0 {
		nextChar := remainingChars[0]
		newWord := append(slices.Clone(usedChars), nextChar)
		if checkFit([]Word{{Content: strings.Join(newWord, ""), Type: word.Type}}) {
			usedChars, remainingChars = newWord, remainingChars[1:]
			continue
		}
		if len(usedChars) == 0 && nextChar != "" {
			// If the first character does not fit, split it anyway
			usedChars = append(usedChars, nextChar)
			remainingChars = remainingChars[1:]
		}
		break
	}
	return Word{Content: strings.Join(usedChars, ""), Type: word.Type},
		Word{Content: strings.Join(remainingChars, ""), Type: word.Type}
}

// ErrNewlineInLine is the error of SplitLineToFitWidth for a word that
// holds a line feed. Upstream throws an Error with this message.
var ErrNewlineInLine = errors.New("splitLineToFitWidth does not support newlines in the line")

// SplitLineToFitWidth is splitLineToFitWidth with
// splitLineToFitWidthRecursion. It splits a line into multiple lines that
// satisfy the checkFit function, which returns true when a line fits.
//
// Upstream consumes the line array that it gets. This function does not
// change line. Upstream is recursive: each call takes one more word. The
// loop calls checkFit with the same lines in the same order.
func SplitLineToFitWidth(line []Word, checkFit func(line []Word) bool) ([][]Word, error) {
	for _, word := range line {
		if strings.Contains(word.Content, "\n") {
			return nil, ErrNewlineInLine
		}
	}

	words := slices.Clone(line)
	lines := [][]Word{}
	newLine := []Word{}
	// Stop when there is nothing left to split
	for len(words) > 0 {
		joiner := ""
		if words[0].Content == " " {
			joiner = " "
			words = words[1:]
		}
		// const nextWord = words.shift() ?? { content: ' ', type: 'normal' };
		nextWord := Word{Content: " ", Type: "normal"}
		if len(words) > 0 {
			nextWord = words[0]
			words = words[1:]
		}
		lineWithNextWord := slices.Clone(newLine)
		if joiner != "" {
			lineWithNextWord = append(lineWithNextWord, Word{Content: joiner, Type: "normal"})
		}
		lineWithNextWord = append(lineWithNextWord, nextWord)

		if checkFit(lineWithNextWord) {
			// nextWord fits, so we can add it to the new line and continue
			newLine = lineWithNextWord
			continue
		}

		// nextWord doesn't fit, so we need to split it
		if len(newLine) > 0 {
			// There was text in newLine, so add it to lines and push nextWord back into words.
			lines = append(lines, newLine)
			words = append([]Word{nextWord}, words...)
		} else if nextWord.Content != "" {
			// There was no text in newLine, so we need to split nextWord
			first, rest := splitWordToFitWidth(checkFit, nextWord)
			lines = append(lines, []Word{first})
			if rest.Content != "" {
				words = append([]Word{rest}, words...)
			}
		}
		newLine = []Word{}
	}
	// If there is a new line, add it to the lines
	if len(newLine) > 0 {
		lines = append(lines, newLine)
	}
	return lines, nil
}
