// Ported from Spacing.java, ElkPadding.java and ElkMargin.java in
// plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/math of Eclipse ELK
// 0.9.1 (EPL-2.0).

package emath

import (
	"errors"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Spacing is the abstract class Spacing: four distances.
type Spacing struct {
	Top, Right, Bottom, Left float64
}

// ElkPadding is the class ElkPadding. LPadding of the layered algorithm is
// the same type.
type ElkPadding struct{ Spacing }

// ElkMargin is the class ElkMargin. LMargin of the layered algorithm is the
// same type.
type ElkMargin struct{ Spacing }

// NewElkPadding is new ElkPadding(top, right, bottom, left).
func NewElkPadding(top, right, bottom, left float64) *ElkPadding {
	return &ElkPadding{Spacing{top, right, bottom, left}}
}

// NewElkMargin is new ElkMargin(top, right, bottom, left).
func NewElkMargin(top, right, bottom, left float64) *ElkMargin {
	return &ElkMargin{Spacing{top, right, bottom, left}}
}

// Clone is clone().
func (p *ElkPadding) Clone() *ElkPadding { c := *p; return &c }

// Clone is clone().
func (m *ElkMargin) Clone() *ElkMargin { c := *m; return &c }

// Set is set(top, right, bottom, left).
func (s *Spacing) Set(top, right, bottom, left float64) {
	s.Top, s.Right, s.Bottom, s.Left = top, right, bottom, left
}

// Copy is copy(other) and set(Spacing).
func (s *Spacing) Copy(o *Spacing) *Spacing {
	s.Left, s.Right, s.Top, s.Bottom = o.Left, o.Right, o.Top, o.Bottom
	return s
}

// Add is add(other).
func (s *Spacing) Add(o *Spacing) *Spacing {
	s.Left += o.Left
	s.Right += o.Right
	s.Top += o.Top
	s.Bottom += o.Bottom
	return s
}

// SetLeftRight is setLeftRight(val).
func (s *Spacing) SetLeftRight(val float64) { s.Left, s.Right = val, val }

// SetTopBottom is setTopBottom(val).
func (s *Spacing) SetTopBottom(val float64) { s.Top, s.Bottom = val, val }

// Horizontal is getHorizontal().
func (s *Spacing) Horizontal() float64 { return s.Left + s.Right }

// Vertical is getVertical().
func (s *Spacing) Vertical() float64 { return s.Top + s.Bottom }

// Equals is equals(Object).
func (s *Spacing) Equals(o *Spacing) bool {
	return o != nil && s.Top == o.Top && s.Bottom == o.Bottom && s.Left == o.Left && s.Right == o.Right
}

// String is toString().
func (s *Spacing) String() string {
	return "[top=" + js.Str(s.Top) + ",left=" + js.Str(s.Left) + ",bottom=" + js.Str(s.Bottom) + ",right=" + js.Str(s.Right) + "]"
}

// Parse is parse(String): the text of an option value such as
// "[top=1,left=2,bottom=3,right=4]". It returns an error where Java throws an
// IllegalArgumentException.
func (s *Spacing) Parse(text string) error {
	body := stripDelims(text)
	if body == "" {
		return nil
	}
	for _, token := range splitAny(body, ",;") {
		kv := splitAny(token, "=")
		if len(kv) != 2 {
			return errors.New("expecting a list of key-value pairs")
		}
		key := gwt.Trim(kv[0])
		value, ok := gwt.ParseDouble(gwt.Trim(kv[1]))
		if !ok {
			return errors.New("the given string contains parts that cannot be parsed as numbers")
		}
		switch key {
		case "top":
			s.Top = value
		case "left":
			s.Left = value
		case "bottom":
			s.Bottom = value
		case "right":
			s.Right = value
		}
	}
	return nil
}
