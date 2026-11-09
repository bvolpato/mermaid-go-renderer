// Ported from plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/math/ElkRectangle.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package emath

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// ElkRectangle is the class ElkRectangle.
type ElkRectangle struct {
	X, Y, Width, Height float64
}

// NewElkRectangle is new ElkRectangle(x, y, w, h).
func NewElkRectangle(x, y, w, h float64) *ElkRectangle { return &ElkRectangle{x, y, w, h} }

// Clone is the copy constructor.
func (r *ElkRectangle) Clone() *ElkRectangle { c := *r; return &c }

// String is toString().
func (r *ElkRectangle) String() string {
	return "Rect[x=" + js.Str(r.X) + ",y=" + js.Str(r.Y) + ",w=" + js.Str(r.Width) + ",h=" + js.Str(r.Height) + "]"
}

// SetRect is setRect(x, y, w, h).
func (r *ElkRectangle) SetRect(x, y, w, h float64) {
	r.X, r.Y, r.Width, r.Height = x, y, w, h
}

// Position is getPosition().
func (r *ElkRectangle) Position() *KVector { return &KVector{r.X, r.Y} }

// Center is getCenter().
func (r *ElkRectangle) Center() *KVector {
	return &KVector{r.X + float64(r.Width/2), r.Y + float64(r.Height/2)}
}

// TopLeft is getTopLeft().
func (r *ElkRectangle) TopLeft() *KVector { return &KVector{r.X, r.Y} }

// TopRight is getTopRight().
func (r *ElkRectangle) TopRight() *KVector { return &KVector{r.X + r.Width, r.Y} }

// BottomLeft is getBottomLeft().
func (r *ElkRectangle) BottomLeft() *KVector { return &KVector{r.X, r.Y + r.Height} }

// BottomRight is getBottomRight().
func (r *ElkRectangle) BottomRight() *KVector { return &KVector{r.X + r.Width, r.Y + r.Height} }

// MaxX is getMaxX().
func (r *ElkRectangle) MaxX() float64 { return r.X + r.Width }

// MaxY is getMaxY().
func (r *ElkRectangle) MaxY() float64 { return r.Y + r.Height }

// Union is union(other). Math.min and Math.max are the builtins min and max
// of Go: math.Min and math.Max give an infinity where JavaScript gives NaN
// for a NaN and an infinity.
func (r *ElkRectangle) Union(other *ElkRectangle) {
	x1 := min(r.X, other.X)
	y1 := min(r.Y, other.Y)
	x2 := max(r.X+r.Width, other.X+other.Width)
	y2 := max(r.Y+r.Height, other.Y+other.Height)
	if x2 < x1 {
		x1, x2 = x2, x1
	}
	if y2 < y1 {
		y1, y2 = y2, y1
	}
	r.SetRect(x1, y1, x2-x1, y2-y1)
}

// Move is move(offset).
func (r *ElkRectangle) Move(offset *KVector) {
	r.X += offset.X
	r.Y += offset.Y
}

// Intersects is intersects(rect).
func (r *ElkRectangle) Intersects(rect *ElkRectangle) bool {
	r1x1, r1y1 := r.X, r.Y
	r1x2, r1y2 := r.X+r.Width, r.Y+r.Height
	r2x1, r2y1 := rect.X, rect.Y
	r2x2, r2y2 := rect.X+rect.Width, rect.Y+rect.Height
	return r1x1 < r2x2 && r1x2 > r2x1 && r1y2 > r2y1 && r1y1 < r2y2
}
