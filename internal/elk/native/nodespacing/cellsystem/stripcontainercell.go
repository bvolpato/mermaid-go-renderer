// Ported from plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/nodespacing/cellsystem/StripContainerCell.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package cellsystem

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// Strip is the enum StripContainerCell.Strip: whether the child cells of a
// strip container are its rows or its columns.
type Strip int

// The constants of Strip, in the order of the Java enum.
const (
	// StripVertical makes the child cells the rows of the container.
	StripVertical Strip = iota
	// StripHorizontal makes the child cells the columns of the container.
	StripHorizontal
)

var namesStrip = []string{"VERTICAL", "HORIZONTAL"}

// String returns the name of the Java constant.
func (e Strip) String() string { return prop.EnumName(namesStrip, int(e)) }

func init() {
	prop.RegisterEnum[Strip]("org.eclipse.elk.alg.common.nodespacing.cellsystem.StripContainerCell$Strip", namesStrip)
}

// StripContainerCell is the class StripContainerCell: a container that lays
// out three child cells along a strip. The outer cells get their minimum
// size, and the center cell gets the rest, but at least its minimum size.
type StripContainerCell struct {
	cellBase
	containerMode Strip
	symmetrical   bool
	gap           float64
	cells         [containerAreaCount]Cell
}

// NewStripContainerCell is new StripContainerCell(mode, symmetrical, gap).
// With symmetrical the outer cells have the same size. The gap is the space
// between two cells that follow each other.
func NewStripContainerCell(mode Strip, symmetrical bool, gap float64) *StripContainerCell {
	return &StripContainerCell{containerMode: mode, symmetrical: symmetrical, gap: gap}
}

// ContainerMode is getContainerMode().
func (c *StripContainerCell) ContainerMode() Strip { return c.containerMode }

// Gap is getGap().
func (c *StripContainerCell) Gap() float64 { return c.gap }

// Cell is getCell(area): the cell of the area, or nil.
func (c *StripContainerCell) Cell(area ContainerArea) Cell { return c.cells[area] }

// SetCell is setCell(area, cell).
func (c *StripContainerCell) SetCell(area ContainerArea, cell Cell) { c.cells[area] = cell }

// MinimumWidth is getMinimumWidth().
func (c *StripContainerCell) MinimumWidth() float64 {
	width := 0.0

	if c.containerMode == StripVertical {
		// Take the maximum of the child cells. The Java code is a stream
		// with max().orElse(0): the maximum starts at negative infinity.
		maximum, present := math.Inf(-1), false
		for _, cell := range c.cells {
			if cell != nil && cell.IsContributingToMinimumWidth() {
				maximum = max(maximum, cell.MinimumWidth())
				present = true
			}
		}
		if present {
			width = maximum
		}
	} else {
		// Minimum widths of the different cells
		cellWidths := c.minCellWidths(true)

		// Keep track of how many cells we have
		activeCells := 0
		for _, cellWidth := range cellWidths {
			if cellWidth > 0 {
				width += cellWidth
				activeCells++
			}
		}

		// If there is more than a single cell, add necessary gaps
		if activeCells > 1 {
			width += float64(c.gap * float64(activeCells-1))
		}
	}

	// If we don't have cells, we don't have width
	if width > 0 {
		return width + c.padding.Left + c.padding.Right
	}
	return 0
}

// MinimumHeight is getMinimumHeight().
func (c *StripContainerCell) MinimumHeight() float64 {
	height := 0.0

	if c.containerMode == StripVertical {
		// Minimum heights of the different cells
		cellHeights := c.minCellHeights(true)

		// Keep track of how many cells we have
		activeCells := 0
		for _, cellHeight := range cellHeights {
			if cellHeight > 0 {
				height += cellHeight
				activeCells++
			}
		}

		// If there is more than a single cell, add necessary gaps
		if activeCells > 1 {
			height += float64(c.gap * float64(activeCells-1))
		}
	} else {
		// Take the maximum of the child cells
		maximum, present := math.Inf(-1), false
		for _, cell := range c.cells {
			if cell != nil && cell.IsContributingToMinimumHeight() {
				maximum = max(maximum, cell.MinimumHeight())
				present = true
			}
		}
		if present {
			height = maximum
		}
	}

	// If we don't have cells, we don't have height
	if height > 0 {
		return height + c.padding.Top + c.padding.Bottom
	}
	return 0
}

// LayoutChildrenHorizontally is layoutChildrenHorizontally().
func (c *StripContainerCell) LayoutChildrenHorizontally() {
	cellRectangle := c.CellRectangle()
	cellPadding := c.Padding()

	if c.containerMode == StripVertical {
		// Each child cell begins at our left border (plus padding) and is as
		// large as our content area
		xPos := cellRectangle.X + cellPadding.Left
		width := cellRectangle.Width - cellPadding.Left - cellPadding.Right

		for _, childCell := range c.cells {
			applyHorizontalLayout(childCell, xPos, width)
		}
	} else {
		cellWidths := c.minCellWidths(false)

		// Left cell is left-aligned with our content area, right cell is
		// right-aligned
		applyHorizontalLayout(c.cells[0],
			cellRectangle.X+cellPadding.Left,
			cellWidths[0])
		applyHorizontalLayout(c.cells[2],
			cellRectangle.X+cellRectangle.Width-cellPadding.Right-cellWidths[2],
			cellWidths[2])

		// Size of the content area and size of the available space in the
		// content area
		freeContentAreaWidth := cellRectangle.Width - cellPadding.Left - cellPadding.Right

		if cellWidths[0] > 0 {
			freeContentAreaWidth -= cellWidths[0] + c.gap

			// We add the gap here because that will spare us to check if
			// cellWidths[0] is zero later on
			cellWidths[0] += c.gap
		}

		if cellWidths[2] > 0 {
			freeContentAreaWidth -= cellWidths[2] + c.gap
		}

		// If the available space is larger than the current size of the
		// center cell, enlarge that thing
		cellWidths[1] = max(cellWidths[1], freeContentAreaWidth)

		// Place the center cell, possibly enlarging it in the process
		applyHorizontalLayout(c.cells[1],
			cellRectangle.X+cellPadding.Left+cellWidths[0]-float64((cellWidths[1]-freeContentAreaWidth)/2),
			cellWidths[1])
	}

	// Layout container cells recursively
	for _, childCell := range c.cells {
		if container, ok := childCell.(ContainerCell); ok {
			container.LayoutChildrenHorizontally()
		}
	}
}

// LayoutChildrenVertically is layoutChildrenVertically().
func (c *StripContainerCell) LayoutChildrenVertically() {
	cellRectangle := c.CellRectangle()
	cellPadding := c.Padding()

	if c.containerMode == StripVertical {
		cellHeights := c.minCellHeights(false)

		// Top cell is top-aligned with our content area, bottom cell is
		// bottom-aligned
		applyVerticalLayout(c.cells[0],
			cellRectangle.Y+cellPadding.Top,
			cellHeights[0])
		applyVerticalLayout(c.cells[2],
			cellRectangle.Y+cellRectangle.Height-cellPadding.Bottom-cellHeights[2],
			cellHeights[2])

		// Size of the content area and size of the available space in the
		// content area
		contentAreaHeight := cellRectangle.Height - cellPadding.Top - cellPadding.Bottom
		contentAreaFreeHeight := contentAreaHeight

		if cellHeights[0] > 0 {
			// We add the gap here because that will spare us to check if
			// cellHeights[0] is zero later on
			cellHeights[0] += c.gap
			contentAreaFreeHeight -= cellHeights[0]
		}

		if cellHeights[2] > 0 {
			contentAreaFreeHeight -= cellHeights[2] + c.gap
		}

		// If the available space is larger than the current size of the
		// center cell, enlarge that thing
		cellHeights[1] = max(cellHeights[1], contentAreaFreeHeight)

		// Place the center cell, possibly enlarging it in the process
		applyVerticalLayout(c.cells[1],
			cellRectangle.Y+cellPadding.Top+cellHeights[0]-float64((cellHeights[1]-contentAreaFreeHeight)/2),
			cellHeights[1])
	} else {
		// Each child cell begins at our top border (plus padding) and is as
		// large as our content area
		yPos := cellRectangle.Y + cellPadding.Top
		height := cellRectangle.Height - cellPadding.Top - cellPadding.Bottom

		for _, childCell := range c.cells {
			applyVerticalLayout(childCell, yPos, height)
		}
	}

	// Layout container cells recursively
	for _, childCell := range c.cells {
		if container, ok := childCell.(ContainerCell); ok {
			container.LayoutChildrenVertically()
		}
	}
}

// minCellWidths is minCellWidths(respectContributionFlag): the width of each
// cell, with the symmetry applied.
func (c *StripContainerCell) minCellWidths(respectContributionFlag bool) [containerAreaCount]float64 {
	cellWidths := [containerAreaCount]float64{
		minWidthOfCell(c.cells[0], respectContributionFlag),
		minWidthOfCell(c.cells[1], respectContributionFlag),
		minWidthOfCell(c.cells[2], respectContributionFlag),
	}

	// If we are to be symmetrical, the outer cells need to be the same size
	if c.symmetrical {
		cellWidths[0] = max(cellWidths[0], cellWidths[2])
		cellWidths[2] = cellWidths[0]
	}

	return cellWidths
}

// minCellHeights is minCellHeights(respectContributionFlag): the height of
// each cell, with the symmetry applied.
func (c *StripContainerCell) minCellHeights(respectContributionFlag bool) [containerAreaCount]float64 {
	cellHeights := [containerAreaCount]float64{
		minHeightOfCell(c.cells[0], respectContributionFlag),
		minHeightOfCell(c.cells[1], respectContributionFlag),
		minHeightOfCell(c.cells[2], respectContributionFlag),
	}

	// If we are to be symmetrical, the outer cells need to be the same size
	if c.symmetrical {
		cellHeights[0] = max(cellHeights[0], cellHeights[2])
		cellHeights[2] = cellHeights[0]
	}

	return cellHeights
}
