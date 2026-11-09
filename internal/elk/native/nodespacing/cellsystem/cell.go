// Ported from plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/nodespacing/cellsystem/Cell.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

// Package cellsystem has the cell system of the node size calculation of
// ELK: cells with a padding and a minimum size, and containers that lay out
// their child cells in a strip or in a grid.
package cellsystem

import "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"

// Cell is the abstract class Cell: the basic component of the cell system. A
// cell has a padding, a minimum size, and a rectangle with its actual
// position and size. The flags tell whether the cell contributes to the
// minimum width and height of its container.
//
// A variable of the type Cell that has no cell must be the nil interface,
// not a nil pointer of a cell type.
type Cell interface {
	// Padding is getPadding(): the padding of the cell, to be changed by the
	// caller.
	Padding() *emath.ElkPadding
	// CellRectangle is getCellRectangle(): the size and position of the
	// cell with its padding, to be changed by the caller.
	CellRectangle() *emath.ElkRectangle
	// IsContributingToMinimumWidth is isContributingToMinimumWidth().
	IsContributingToMinimumWidth() bool
	// SetContributesToMinimumWidth is setContributesToMinimumWidth(flag).
	SetContributesToMinimumWidth(contributesToMinimumWidth bool)
	// IsContributingToMinimumHeight is isContributingToMinimumHeight().
	IsContributingToMinimumHeight() bool
	// SetContributesToMinimumHeight is setContributesToMinimumHeight(flag).
	SetContributesToMinimumHeight(contributesToMinimumHeight bool)
	// MinimumWidth is getMinimumWidth(): the minimum width with the padding.
	MinimumWidth() float64
	// MinimumHeight is getMinimumHeight(): the minimum height with the
	// padding.
	MinimumHeight() float64
}

// cellBase has the fields and the methods of the class Cell that are not
// abstract. Each cell type embeds it.
type cellBase struct {
	padding                    emath.ElkPadding
	cellRectangle              emath.ElkRectangle
	contributesToMinimumWidth  bool
	contributesToMinimumHeight bool
}

// Padding is getPadding().
func (c *cellBase) Padding() *emath.ElkPadding { return &c.padding }

// CellRectangle is getCellRectangle().
func (c *cellBase) CellRectangle() *emath.ElkRectangle { return &c.cellRectangle }

// IsContributingToMinimumWidth is isContributingToMinimumWidth().
func (c *cellBase) IsContributingToMinimumWidth() bool { return c.contributesToMinimumWidth }

// SetContributesToMinimumWidth is setContributesToMinimumWidth(flag).
func (c *cellBase) SetContributesToMinimumWidth(contributesToMinimumWidth bool) {
	c.contributesToMinimumWidth = contributesToMinimumWidth
}

// IsContributingToMinimumHeight is isContributingToMinimumHeight().
func (c *cellBase) IsContributingToMinimumHeight() bool { return c.contributesToMinimumHeight }

// SetContributesToMinimumHeight is setContributesToMinimumHeight(flag).
func (c *cellBase) SetContributesToMinimumHeight(contributesToMinimumHeight bool) {
	c.contributesToMinimumHeight = contributesToMinimumHeight
}
