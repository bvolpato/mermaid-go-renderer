// Ported from plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/nodespacing/cellsystem/LabelCell.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package cellsystem

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/adapters"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
)

// LabelLocation is the part of the enum NodeLabelLocation that a label cell
// reads. NodeLabelLocation is in the package nodespacing/internal, which
// imports this package, so the constructor takes this interface.
type LabelLocation interface {
	// HorizontalAlignment is getHorizontalAlignment().
	HorizontalAlignment() HorizontalLabelAlignment
	// VerticalAlignment is getVerticalAlignment().
	VerticalAlignment() VerticalLabelAlignment
}

// LabelCell is the class LabelCell: a cell that holds labels and places
// them. Its minimum size is the area that the labels need, with a gap
// between two labels.
//
// In the horizontal layout mode the labels are lines of text one below the
// other. In the vertical layout mode they are columns one next to the
// other.
type LabelCell struct {
	cellBase
	horizontalLayoutMode   bool
	horizontalAlignment    HorizontalLabelAlignment
	verticalAlignment      VerticalLabelAlignment
	gap                    float64
	labels                 []adapters.LabelAdapter
	minimumContentAreaSize emath.KVector
}

// NewLabelCell is new LabelCell(gap): a cell in the horizontal layout mode.
func NewLabelCell(gap float64) *LabelCell {
	return NewLabelCellMode(gap, true)
}

// NewLabelCellMode is new LabelCell(gap, horizontalLayoutMode).
func NewLabelCellMode(gap float64, horizontalLayoutMode bool) *LabelCell {
	return &LabelCell{
		gap:                  gap,
		horizontalLayoutMode: horizontalLayoutMode,
		horizontalAlignment:  HorizontalLabelAlignmentCenter,
		verticalAlignment:    VerticalLabelAlignmentCenter,
	}
}

// NewLabelCellLocation is new LabelCell(gap, nodeLabelLocation): a cell in
// the horizontal layout mode with the alignments of the location.
func NewLabelCellLocation(gap float64, nodeLabelLocation LabelLocation) *LabelCell {
	return NewLabelCellLocationMode(gap, nodeLabelLocation, true)
}

// NewLabelCellLocationMode is new LabelCell(gap, nodeLabelLocation,
// horizontalLayoutMode).
func NewLabelCellLocationMode(gap float64, nodeLabelLocation LabelLocation, horizontalLayoutMode bool) *LabelCell {
	return &LabelCell{
		gap:                  gap,
		horizontalLayoutMode: horizontalLayoutMode,
		horizontalAlignment:  nodeLabelLocation.HorizontalAlignment(),
		verticalAlignment:    nodeLabelLocation.VerticalAlignment(),
	}
}

// HorizontalAlignment is getHorizontalAlignment().
func (c *LabelCell) HorizontalAlignment() HorizontalLabelAlignment { return c.horizontalAlignment }

// SetHorizontalAlignment is setHorizontalAlignment(alignment). It returns
// the cell.
func (c *LabelCell) SetHorizontalAlignment(newHorizontalAlignment HorizontalLabelAlignment) *LabelCell {
	c.horizontalAlignment = newHorizontalAlignment
	return c
}

// VerticalAlignment is getVerticalAlignment().
func (c *LabelCell) VerticalAlignment() VerticalLabelAlignment { return c.verticalAlignment }

// SetVerticalAlignment is setVerticalAlignment(alignment). It returns the
// cell.
func (c *LabelCell) SetVerticalAlignment(newVerticalAlignment VerticalLabelAlignment) *LabelCell {
	c.verticalAlignment = newVerticalAlignment
	return c
}

// Labels is getLabels(): the labels of the cell. The slice shares the memory
// of the cell, so a caller can reorder the labels in place. Use AddLabel to
// add a label.
func (c *LabelCell) Labels() []adapters.LabelAdapter { return c.labels }

// SetLabels replaces the label list and keeps the minimum size. It stands
// for the Java code that clears the list of getLabels() and fills it again
// with the same labels in another order.
func (c *LabelCell) SetLabels(labels []adapters.LabelAdapter) { c.labels = labels }

// Gap returns the field gap: the space between two labels.
func (c *LabelCell) Gap() float64 { return c.gap }

// IsHorizontalLayoutMode returns the field horizontalLayoutMode.
func (c *LabelCell) IsHorizontalLayoutMode() bool { return c.horizontalLayoutMode }

// MinimumContentAreaSize returns the field minimumContentAreaSize: the space
// that the labels need, without the padding. AddLabel keeps it up to date.
func (c *LabelCell) MinimumContentAreaSize() *emath.KVector { return &c.minimumContentAreaSize }

// MinimumWidth is getMinimumWidth().
func (c *LabelCell) MinimumWidth() float64 {
	padding := c.Padding()
	return c.minimumContentAreaSize.X + padding.Left + padding.Right
}

// MinimumHeight is getMinimumHeight().
func (c *LabelCell) MinimumHeight() float64 {
	padding := c.Padding()
	return c.minimumContentAreaSize.Y + padding.Top + padding.Bottom
}

// AddLabel is addLabel(label).
func (c *LabelCell) AddLabel(label adapters.LabelAdapter) {
	c.labels = append(c.labels, label)

	// Update our minimum size (how this works depends on the layout mode)
	labelSize := label.Size()

	if c.horizontalLayoutMode {
		c.minimumContentAreaSize.X = max(c.minimumContentAreaSize.X, labelSize.X)
		c.minimumContentAreaSize.Y += labelSize.Y

		// If this is not our first label, insert a gap
		if len(c.labels) > 1 {
			c.minimumContentAreaSize.Y += c.gap
		}
	} else {
		c.minimumContentAreaSize.X += labelSize.X
		c.minimumContentAreaSize.Y = max(c.minimumContentAreaSize.Y, labelSize.Y)

		// If this is not our first label, insert a gap
		if len(c.labels) > 1 {
			c.minimumContentAreaSize.X += c.gap
		}
	}
}

// HasLabels is hasLabels().
func (c *LabelCell) HasLabels() bool { return len(c.labels) > 0 }

// ApplyLabelLayout is applyLabelLayout(): it sets the positions of the
// labels from the cell rectangle.
func (c *LabelCell) ApplyLabelLayout() {
	if c.horizontalLayoutMode {
		c.applyHorizontalModeLabelLayout()
	} else {
		c.applyVerticalModeLabelLayout()
	}
}

func (c *LabelCell) applyHorizontalModeLabelLayout() {
	cellRect := c.CellRectangle()
	cellPadding := c.Padding()

	// Calculate our starting y coordinate
	yPos := cellRect.Y

	if c.verticalAlignment == VerticalLabelAlignmentCenter {
		yPos += float64((cellRect.Height - c.minimumContentAreaSize.Y) / 2)
	} else if c.verticalAlignment == VerticalLabelAlignmentBottom {
		yPos += cellRect.Height - c.minimumContentAreaSize.Y
	}

	// Place them labels, I say!
	for _, label := range c.labels {
		labelSize := label.Size()
		labelPos := emath.KVector{}

		// Y coordinate
		labelPos.Y = yPos
		yPos += labelSize.Y + c.gap

		// X coordinate
		switch c.horizontalAlignment {
		case HorizontalLabelAlignmentLeft:
			labelPos.X = cellRect.X + cellPadding.Left
		case HorizontalLabelAlignmentCenter:
			labelPos.X = cellRect.X + cellPadding.Left + float64((cellRect.Width-labelSize.X)/2)
		case HorizontalLabelAlignmentRight:
			labelPos.X = cellRect.X + cellRect.Width - cellPadding.Right - labelSize.X
		}

		// Apply position
		label.SetPosition(&labelPos)
	}
}

func (c *LabelCell) applyVerticalModeLabelLayout() {
	cellRect := c.CellRectangle()
	cellPadding := c.Padding()

	// Calculate our starting x coordinate
	xPos := cellRect.X

	if c.horizontalAlignment == HorizontalLabelAlignmentCenter {
		xPos += float64((cellRect.Width - c.minimumContentAreaSize.X) / 2)
	} else if c.horizontalAlignment == HorizontalLabelAlignmentRight {
		xPos += cellRect.Width - c.minimumContentAreaSize.X
	}

	// Place them labels, I say!
	for _, label := range c.labels {
		labelSize := label.Size()
		labelPos := emath.KVector{}

		// X coordinate
		labelPos.X = xPos
		xPos += labelSize.X + c.gap

		// Y coordinate
		switch c.verticalAlignment {
		case VerticalLabelAlignmentTop:
			labelPos.Y = cellRect.Y + cellPadding.Top
		case VerticalLabelAlignmentCenter:
			labelPos.Y = cellRect.Y + cellPadding.Top + float64((cellRect.Height-labelSize.Y)/2)
		case VerticalLabelAlignmentBottom:
			labelPos.Y = cellRect.Y + cellRect.Height - cellPadding.Bottom - labelSize.Y
		}

		// Apply position
		label.SetPosition(&labelPos)
	}
}
