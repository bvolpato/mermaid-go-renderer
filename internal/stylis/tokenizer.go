// Ported from stylis/src/Tokenizer.js (stylis 4.3.6).

package stylis

// Element is a node of the tree that Compile returns.
//
// In stylis the props and children fields hold an array for a rule or an
// at-rule, and a string for a declaration or a comment. The port has one
// field for each form.
type Element struct {
	Value  string
	Root   *Element
	Parent *Element
	Type   string
	// Props are the selectors of a rule or an at-rule.
	Props []string
	// Prop is the property name of a declaration. For a comment, it is the
	// character that follows the comment.
	Prop string
	// Children are the elements inside a rule or an at-rule.
	Children []*Element
	// Text is the value of a declaration or the text of a comment.
	Text string
	// Line and Column give the position after the element. Length is an
	// offset in Value. Column and Length count UTF-16 code units, as they do
	// in stylis.
	Line   int
	Column int
	Length int
	Return string
}

// tokenizer holds the state that Tokenizer.js keeps in module variables.
type tokenizer[T unit] struct {
	line       int
	column     int
	length     int
	position   int
	character  int
	characters []T
}

// node makes an element. The siblings field of stylis is used only by the
// prefixer, which mermaid does not use, so the port does not keep it.
func (t *tokenizer[T]) node(value []T, root, parent *Element, typ string, length int) *Element {
	return &Element{Value: str(value), Root: root, Parent: parent, Type: typ, Line: t.line, Column: t.column, Length: length}
}

func (t *tokenizer[T]) char() int {
	return t.character
}

func (t *tokenizer[T]) prev() int {
	if t.position > 0 {
		t.position--
		t.character = charat(t.characters, t.position)
	} else {
		t.character = 0
	}
	t.column--
	if t.character == 10 {
		t.column = 1
		t.line--
	}
	return t.character
}

func (t *tokenizer[T]) next() int {
	if t.position < t.length {
		t.character = charat(t.characters, t.position)
		t.position++
	} else {
		t.character = 0
	}
	t.column++
	if t.character == 10 {
		t.column = 1
		t.line++
	}
	return t.character
}

func (t *tokenizer[T]) peek() int {
	return charat(t.characters, t.position)
}

func (t *tokenizer[T]) caret() int {
	return t.position
}

func (t *tokenizer[T]) slice(begin, end int) []T {
	return substr(t.characters, begin, end)
}

// token gives the class of a character.
func token(typ int) int {
	switch typ {
	// \0 \t \n \r \s whitespace token
	case 0, 9, 10, 13, 32:
		return 5
	// ! + , / > @ ~ isolate token
	case 33, 43, 44, 47, 62, 64, 126,
		// ; { } breakpoint token
		59, 123, 125:
		return 4
	// : accompanied token
	case 58:
		return 3
	// " ' ( [ opening delimit token
	case 34, 39, 40, 91:
		return 2
	// ) ] closing delimit token
	case 41, 93:
		return 1
	}
	return 0
}

func (t *tokenizer[T]) alloc(value []T) {
	t.line, t.column = 1, 1
	t.characters = value
	t.length = len(value)
	t.position = 0
}

func (t *tokenizer[T]) dealloc() {
	t.characters = nil
}

func (t *tokenizer[T]) delimit(typ int) []T {
	begin := t.position - 1
	switch typ {
	case 91:
		typ += 2
	case 40:
		typ++
	}
	return trim(t.slice(begin, t.delimiter(typ)))
}

// whitespace skips white space. It reports whether stylis returns ' ' and
// not the empty string.
func (t *tokenizer[T]) whitespace(typ int) bool {
	for {
		t.character = t.peek()
		if t.character == 0 {
			break
		}
		if t.character < 33 {
			t.next()
		} else {
			break
		}
	}
	return !(token(typ) > 2 || token(t.character) > 3)
}

func (t *tokenizer[T]) escaping(index, count int) []T {
	for {
		count--
		if count == 0 || t.next() == 0 {
			break
		}
		// not 0-9 A-F a-f
		if t.character < 48 || t.character > 102 || (t.character > 57 && t.character < 65) || (t.character > 70 && t.character < 97) {
			break
		}
	}
	end := t.caret()
	if count < 6 && t.peek() == 32 && t.next() == 32 {
		end++
	}
	return t.slice(index, end)
}

func (t *tokenizer[T]) delimiter(typ int) int {
	for t.next() != 0 {
		switch {
		// ] ) " '
		case t.character == typ:
			return t.position
		// " '
		case t.character == 34, t.character == 39:
			if typ != 34 && typ != 39 {
				t.delimiter(t.character)
			}
		// (
		case t.character == 40:
			if typ == 41 {
				t.delimiter(typ)
			}
		// \
		case t.character == 92:
			t.next()
		}
	}
	return t.position
}

func (t *tokenizer[T]) commenter(typ, index int) []T {
	for t.next() != 0 {
		// //
		if typ+t.character == 47+10 {
			break
		}
		// /*
		if typ+t.character == 42+42 && t.peek() == 47 {
			break
		}
	}
	text := append(toUnits[T]("/*"), t.slice(index, t.position-1)...)
	text = append(text, '*')
	if typ == 47 {
		return append(text, '/')
	}
	return append(text, T(t.next()))
}

func (t *tokenizer[T]) identifier(index int) []T {
	for token(t.peek()) == 0 {
		t.next()
	}
	return t.slice(index, t.position)
}
