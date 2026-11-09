// Ported from stylis/src/Parser.js (stylis 4.3.6).
//
// 4.3.6 is the version inside the mermaid 12.2.0 bundle. Stylis 4.4.0 parses
// a pseudo-class function in another way (it counts the parentheses), and it
// keeps a custom property that has no value. Do not port those changes.

package stylis

import (
	"slices"
	"unicode/utf16"
)

// Compile parses CSS text into a tree of elements.
func Compile(value string) []*Element {
	for i := 0; i < len(value); i++ {
		if value[i] >= 0x80 {
			return compile(utf16.Encode([]rune(value)))
		}
	}
	return compile([]byte(value))
}

func compile[T unit](value []T) []*Element {
	t := &tokenizer[T]{}
	t.alloc(value)
	rulesets := []*Element{}
	points := []int{0}
	t.parse(nil, nil, nil, nil, [][]T{{}}, &rulesets, 0, &points, &rulesets)
	t.dealloc()
	return rulesets
}

// setPoint is points[index] = value for an array that grows on assignment.
func setPoint(points *[]int, index, value int) {
	for len(*points) <= index {
		*points = append(*points, 0)
	}
	(*points)[index] = value
}

// parse reads one block. rulesets and declarations point to the arrays that
// receive the elements. Both can point to the same array.
func (t *tokenizer[T]) parse(value []T, root, parent, rule *Element, rules [][]T, rulesets *[]*Element, pseudo int, points *[]int, declarations *[]*Element) {
	index := 0
	offset := 0
	length := pseudo
	atrule := 0
	property := 0
	previous := 0
	variable := 1
	scanning := 1
	ampersand := 1
	character := 0
	typ := ""
	var reference *Element
	var characters []T

	for scanning != 0 {
		previous = character
		character = t.next()
		switch {
		// (
		case character == 40:
			if previous != 108 && charat(characters, length-1) == 58 {
				// characters += replace(delimit(character), '&', '&\f')
				characters = appendAmpersand(characters, t.delimit(character))
				position := 0
				if index != 0 && index-1 < len(*points) {
					position = abs((*points)[index-1])
				}
				if indexof(characters, position) != -1 {
					ampersand = -1
				}
				break
			}
			characters = append(characters, t.delimit(character)...)
		// " ' [
		case character == 34, character == 39, character == 91:
			characters = append(characters, t.delimit(character)...)
		// \t \n \r \s
		case character == 9, character == 10, character == 13, character == 32:
			if t.whitespace(previous) {
				characters = append(characters, ' ')
			}
		// \
		case character == 92:
			characters = append(characters, t.escaping(t.caret()-1, 7)...)
			continue
		// /
		case character == 47:
			switch t.peek() {
			case 42, 47:
				commentType := t.next()
				*declarations = append(*declarations, t.comment(t.commenter(commentType, t.caret()), root, parent))
				if (token(orOne(previous)) == 5 || token(orOne(t.peek())) == 5) && len(characters) > 0 && characters[len(characters)-1] != ' ' {
					characters = append(characters, ' ')
				}
			default:
				characters = append(characters, '/')
			}
		// {
		case character == 123*variable:
			setPoint(points, index, len(characters)*ampersand)
			index++
			fallthrough
		// } ; \0
		case character == 125*variable, character == 59, character == 0:
			switch {
			// \0 }
			case character == 0, character == 125:
				scanning = 0
				fallthrough
			// ;
			case character == 59+offset:
				if ampersand == -1 {
					characters = remove(characters, '\f', true)
				}
				if property > 0 && (len(characters)-length != 0 || (variable == 0 && previous == 47)) {
					if property > 32 {
						*declarations = append(*declarations, t.declaration(withSemicolon(characters), rule, parent, length-1))
					} else {
						*declarations = append(*declarations, t.declaration(withSemicolon(remove(characters, ' ', false)), rule, parent, length-2))
					}
				}
			// @ ;
			case character == 59:
				characters = append(characters, ';')
				fallthrough
			// { rule/at-rule
			default:
				block := slices.Clone(characters)
				var props [][]T
				reference, props = t.ruleset(block, root, parent, index, offset, rules, *points, typ, nil, length)
				*rulesets = append(*rulesets, reference)

				if character == 123 {
					if offset == 0 {
						t.parse(block, root, reference, reference, props, rulesets, length, points, &reference.Children)
					} else {
						switch atrule {
						// c(ontainer)
						case 99:
							if charat(block, 3) == 110 {
								break
							}
							if charat(block, 2) == 97 {
								break
							}
							offset = 0
						// l(ayer)
						case 108:
							if charat(block, 2) == 97 {
								break
							}
							offset = 0
						// d(ocument) m(edia) s(upports)
						case 100, 109, 115:
						default:
							offset = 0
						}
						if offset != 0 {
							var nested *Element
							nestedDeclarations := &reference.Children
							if rule != nil {
								nested, _ = t.ruleset(value, reference, reference, 0, 0, rules, *points, typ, rules, length)
								reference.Children = append(reference.Children, nested)
								nestedDeclarations = &nested.Children
							}
							t.parse(value, reference, reference, nested, rules, &reference.Children, length, points, nestedDeclarations)
						} else {
							t.parse(block, reference, reference, reference, [][]T{{}}, &reference.Children, 0, points, &reference.Children)
						}
					}
				}
			}

			index, offset, property = 0, 0, 0
			variable, ampersand = 1, 1
			typ = ""
			characters = characters[:0]
			length = pseudo
		// :
		case character == 58:
			length = 1 + len(characters)
			property = previous
			fallthrough
		default:
			if variable < 1 {
				if character == 123 {
					variable--
				} else if character == 125 {
					wasZero := variable == 0
					variable++
					if wasZero && t.prev() == 125 {
						continue
					}
				}
			}

			characters = append(characters, T(character))
			switch character * variable {
			// &
			case 38:
				if offset > 0 {
					ampersand = 1
				} else {
					characters = append(characters, '\f')
					ampersand = -1
				}
			// ,
			case 44:
				setPoint(points, index, (len(characters)-1)*ampersand)
				index++
				ampersand = 1
			// @
			case 64:
				// -
				if t.peek() == 45 {
					characters = append(characters, t.delimit(t.next())...)
				}

				atrule = t.peek()
				characters = append(characters, t.identifier(t.caret())...)
				typ = str(characters)
				length = len(characters)
				offset = length
				character++
			// -
			case 45:
				if previous == 45 && len(characters) == 2 {
					variable = 0
				}
			}
		}
	}
}

// orOne is value || 1.
func orOne(value int) int {
	if value == 0 {
		return 1
	}
	return value
}

// withSemicolon is characters + ';' in a new string.
func withSemicolon[T unit](characters []T) []T {
	out := make([]T, 0, len(characters)+1)
	out = append(out, characters...)
	return append(out, ';')
}

// ruleset makes a rule or an at-rule element. It also returns the selectors
// of the element as units, for the parser to use as the parent selectors.
//
// When props is not nil, the element takes it as its selectors. Stylis passes
// the parent selectors this way for the rule that it puts inside a nested
// at-rule.
func (t *tokenizer[T]) ruleset(value []T, root, parent *Element, index, offset int, rules [][]T, points []int, typ string, props [][]T, length int) (*Element, [][]T) {
	post := offset - 1
	rule := rules
	if offset != 0 {
		rule = [][]T{{}}
	}
	size := len(rule)

	if props == nil {
		props = [][]T{}
	}
	for i := 0; i < index; i++ {
		j := points[i]
		begin := post + 1
		post = abs(j)
		y := substr(value, begin, post)
		for x := 0; x < size; x++ {
			var z []T
			if j > 0 {
				joined := make([]T, 0, len(rule[x])+1+len(y))
				joined = append(joined, rule[x]...)
				joined = append(joined, ' ')
				joined = append(joined, y...)
				z = trim(joined)
			} else {
				z = trim(replaceAmpersand(y, rule[x]))
			}
			if len(z) != 0 {
				props = append(props, z)
			}
		}
	}

	nodeType := typ
	if offset == 0 {
		nodeType = RULESET
	}
	element := t.node(value, root, parent, nodeType, length)
	element.Props = strs(props)
	element.Children = []*Element{}
	return element, props
}

func (t *tokenizer[T]) comment(value []T, root, parent *Element) *Element {
	element := t.node(value, root, parent, COMMENT, 0)
	element.Prop = str([]T{T(t.char())})
	element.Text = str(substr(value, 2, -2))
	return element
}

func (t *tokenizer[T]) declaration(value []T, root, parent *Element, length int) *Element {
	element := t.node(value, root, parent, DECLARATION, length)
	element.Prop = str(substr(value, 0, length))
	element.Text = str(substr(value, length+1, -1))
	return element
}
