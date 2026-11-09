// This file ports no upstream file. It has String.prototype.replace of
// ECMAScript for a search value that is a string.

package js

// ReplaceString is String.prototype.replace(searchValue, replaceValue) with
// a search value that is a string: it replaces the first occurrence of
// search in s. The replacement text can hold the patterns of the
// GetSubstitution operation: "$$" is a dollar sign, "$&" is the matched
// text, "$`" is the text in front of the match and "$'" is the text after
// it. A search string has no captures, so "$1" and "$<name>" stay as they
// are. The strings are sequences of UTF-16 code units.
func ReplaceString(s, search, replacement string) string {
	str := UTF16Units(s)
	pat := UTF16Units(search)
	position := -1
	for i := 0; i+len(pat) <= len(str); i++ {
		match := true
		for k, u := range pat {
			if str[i+k] != u {
				match = false
				break
			}
		}
		if match {
			position = i
			break
		}
	}
	if position < 0 {
		return s
	}
	repl := UTF16Units(replacement)
	out := make([]rune, 0, len(str)+len(repl))
	out = append(out, str[:position]...)
	tail := str[position+len(pat):]
	for i := 0; i < len(repl); i++ {
		c := repl[i]
		if c != '$' || i+1 >= len(repl) {
			out = append(out, c)
			continue
		}
		switch repl[i+1] {
		case '$':
			out = append(out, '$')
			i++
		case '&':
			out = append(out, pat...)
			i++
		case '`':
			out = append(out, str[:position]...)
			i++
		case '\'':
			out = append(out, tail...)
			i++
		default:
			out = append(out, c)
		}
	}
	out = append(out, tail...)
	return FromUTF16Units(out)
}
