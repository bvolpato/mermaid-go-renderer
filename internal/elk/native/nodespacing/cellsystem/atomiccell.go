// Ported from plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/nodespacing/cellsystem/AtomicCell.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package cellsystem

import "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"

// AtomicCell is the class AtomicCell: a cell that holds a minimum size. It
// stands for something that is placed in the cell system later.
type AtomicCell struct {
	cellBase
	// minimumContentAreaSize is the minimum size of the content area, without
	// the padding.
	minimumContentAreaSize emath.KVector
}

// NewAtomicCell is new AtomicCell().
func NewAtomicCell() *AtomicCell { return &AtomicCell{} }

// MinimumContentAreaSize is getMinimumContentAreaSize(): the minimum size of
// the content area without the padding, to be changed by the caller.
func (c *AtomicCell) MinimumContentAreaSize() *emath.KVector { return &c.minimumContentAreaSize }

// SetMinimumContentAreaSize is setMinimumContentAreaSize(size,
// includesPadding). With includesPadding the padding is subtracted from the
// size.
func (c *AtomicCell) SetMinimumContentAreaSize(newMinimumContentAreaSize *emath.KVector, includesPadding bool) {
	if includesPadding {
		padding := c.Padding()
		c.minimumContentAreaSize.X = newMinimumContentAreaSize.X - padding.Left - padding.Right
		c.minimumContentAreaSize.Y = newMinimumContentAreaSize.Y - padding.Top - padding.Bottom
	} else {
		c.minimumContentAreaSize.Set(newMinimumContentAreaSize)
	}
}

// MinimumWidth is getMinimumWidth().
func (c *AtomicCell) MinimumWidth() float64 {
	padding := c.Padding()
	return c.minimumContentAreaSize.X + padding.Left + padding.Right
}

// MinimumHeight is getMinimumHeight().
func (c *AtomicCell) MinimumHeight() float64 {
	padding := c.Padding()
	return c.minimumContentAreaSize.Y + padding.Top + padding.Bottom
}
