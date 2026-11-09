// Ported from plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/math/KVectorChain.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package emath

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// KVectorChain is a list of vectors: java.util.LinkedList<KVector> with the
// methods of KVectorChain. The elements are shared pointers, as in Java.
type KVectorChain struct {
	// Points has the vectors in order.
	Points []*KVector
}

// NewKVectorChain is new KVectorChain(vectors...).
func NewKVectorChain(vectors ...*KVector) *KVectorChain {
	c := &KVectorChain{}
	c.Points = append(c.Points, vectors...)
	return c
}

// Clone is new KVectorChain(collection): a new list with the same vectors.
func (c *KVectorChain) Clone() *KVectorChain {
	return &KVectorChain{Points: append([]*KVector(nil), c.Points...)}
}

// Len is size().
func (c *KVectorChain) Len() int { return len(c.Points) }

// IsEmpty is isEmpty().
func (c *KVectorChain) IsEmpty() bool { return len(c.Points) == 0 }

// Get is get(i).
func (c *KVectorChain) Get(i int) *KVector { return c.Points[i] }

// First is getFirst().
func (c *KVectorChain) First() *KVector { return c.Points[0] }

// Last is getLast().
func (c *KVectorChain) Last() *KVector { return c.Points[len(c.Points)-1] }

// Clear is clear().
func (c *KVectorChain) Clear() { c.Points = nil }

// Add is add(KVector) and addLast(KVector).
func (c *KVectorChain) Add(v *KVector) { c.Points = append(c.Points, v) }

// AddXY is add(x, y).
func (c *KVectorChain) AddXY(x, y float64) { c.Points = append(c.Points, &KVector{x, y}) }

// AddFirst is addFirst(KVector).
func (c *KVectorChain) AddFirst(v *KVector) { c.Insert(0, v) }

// Insert is add(index, KVector).
func (c *KVectorChain) Insert(index int, v *KVector) {
	c.Points = append(c.Points, nil)
	copy(c.Points[index+1:], c.Points[index:])
	c.Points[index] = v
}

// AddAll is addAll(vectors...).
func (c *KVectorChain) AddAll(vectors ...*KVector) { c.Points = append(c.Points, vectors...) }

// InsertAll is addAll(index, collection).
func (c *KVectorChain) InsertAll(index int, vectors []*KVector) {
	tail := append([]*KVector(nil), c.Points[index:]...)
	c.Points = append(append(c.Points[:index:index], vectors...), tail...)
}

// AddAllAsCopies is addAllAsCopies(index, chain).
func (c *KVectorChain) AddAllAsCopies(index int, chain []*KVector) {
	copies := make([]*KVector, len(chain))
	for i, v := range chain {
		copies[i] = v.Clone()
	}
	c.InsertAll(index, copies)
}

// RemoveFirst is removeFirst().
func (c *KVectorChain) RemoveFirst() *KVector {
	v := c.Points[0]
	c.Points = c.Points[1:]
	return v
}

// RemoveLast is removeLast().
func (c *KVectorChain) RemoveLast() *KVector {
	v := c.Points[len(c.Points)-1]
	c.Points = c.Points[:len(c.Points)-1]
	return v
}

// String is toString().
func (c *KVectorChain) String() string {
	var b strings.Builder
	b.WriteByte('(')
	for i, v := range c.Points {
		b.WriteString(js.Str(v.X) + "," + js.Str(v.Y))
		if i+1 < len(c.Points) {
			b.WriteString("; ")
		}
	}
	b.WriteByte(')')
	return b.String()
}

// Scale is scale(double).
func (c *KVectorChain) Scale(scale float64) *KVectorChain {
	for _, v := range c.Points {
		v.Scale(scale)
	}
	return c
}

// ScaleXY is scale(double, double).
func (c *KVectorChain) ScaleXY(sx, sy float64) *KVectorChain {
	for _, v := range c.Points {
		v.ScaleXY(sx, sy)
	}
	return c
}

// Offset is offset(KVector).
func (c *KVectorChain) Offset(offset *KVector) *KVectorChain {
	for _, v := range c.Points {
		v.Add(offset)
	}
	return c
}

// OffsetXY is offset(dx, dy).
func (c *KVectorChain) OffsetXY(dx, dy float64) *KVectorChain {
	for _, v := range c.Points {
		v.AddXY(dx, dy)
	}
	return c
}

// TotalLength is totalLength().
func (c *KVectorChain) TotalLength() float64 {
	length := 0.0
	for i := 1; i < len(c.Points); i++ {
		length += c.Points[i-1].Distance(c.Points[i])
	}
	return length
}

// ReverseChain is KVectorChain.reverse(chain): a new chain with copies of the
// vectors in reverse order.
func ReverseChain(chain *KVectorChain) *KVectorChain {
	result := &KVectorChain{Points: make([]*KVector, len(chain.Points))}
	for i, v := range chain.Points {
		result.Points[len(chain.Points)-1-i] = v.Clone()
	}
	return result
}
