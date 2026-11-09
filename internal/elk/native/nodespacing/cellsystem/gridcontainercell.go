// Ported from plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/nodespacing/cellsystem/GridContainerCell.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package cellsystem

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
)

const (
	// rows is the number of rows of the grid.
	rows = containerAreaCount
	// columns is the number of columns of the grid.
	columns = rows
)

// GridContainerCell is the class GridContainerCell: a container that lays
// out its child cells in three rows of three columns. In the tabular mode
// the columns have the same widths in each row.
//
// The center cell can have a minimum size of its own. The container can be
// set to use only the center cell for its minimum size. The center cell
// rectangle is the part of the grid that the outer cells leave free.
type GridContainerCell struct {
	cellBase
	gap         float64
	tabular     bool
	symmetrical bool
	cells       [rows][columns]Cell
	// centerCellMinimumSize is the minimum size of the center cell, or nil.
	centerCellMinimumSize *emath.KVector
	// onlyCenterCellContributesToMinimumSize tells that the minimum size of
	// the container is that of the center cell.
	onlyCenterCellContributesToMinimumSize bool
	// centerCellRect is the part of the grid free of outer grid cells.
	centerCellRect emath.ElkRectangle
}

// NewGridContainerCell is new GridContainerCell(tabular, symmetrical, gap).
func NewGridContainerCell(tabular, symmetrical bool, gap float64) *GridContainerCell {
	return &GridContainerCell{tabular: tabular, symmetrical: symmetrical, gap: gap}
}

// Gap is getGap().
func (c *GridContainerCell) Gap() float64 { return c.gap }

// Cell is getCell(row, col): the cell of the area, or nil.
func (c *GridContainerCell) Cell(row, col ContainerArea) Cell { return c.cells[row][col] }

// SetCell is setCell(row, col, cell).
func (c *GridContainerCell) SetCell(row, col ContainerArea, cell Cell) { c.cells[row][col] = cell }

// SetCenterCellMinimumSize is setCenterCellMinimumSize(minimumSize). It
// keeps a copy of the vector.
func (c *GridContainerCell) SetCenterCellMinimumSize(minimumSize *emath.KVector) {
	c.centerCellMinimumSize = minimumSize.Clone()
}

// SetOnlyCenterCellContributesToMinimumSize is
// setOnlyCenterCellContributesToMinimumSize(contribution).
func (c *GridContainerCell) SetOnlyCenterCellContributesToMinimumSize(contribution bool) {
	c.onlyCenterCellContributesToMinimumSize = contribution
}

// CenterCellRectangle is getCenterCellRectangle(): a copy of the rectangle
// that the outer rows and columns leave free. It is valid after both layout
// methods ran.
func (c *GridContainerCell) CenterCellRectangle() *emath.ElkRectangle {
	return c.centerCellRect.Clone()
}

// MinimumWidth is getMinimumWidth().
func (c *GridContainerCell) MinimumWidth() float64 {
	width := 0.0

	if c.onlyCenterCellContributesToMinimumSize {
		// Check if an explicit minimum size for the center cell was set
		if c.centerCellMinimumSize != nil {
			width = c.centerCellMinimumSize.X
		} else if c.cells[1][1] != nil {
			width = c.cells[1][1].MinimumWidth()
		}
	} else if c.tabular {
		// Use aggregated widths
		width = c.sumWithGaps(c.minColumnWidths(noRow, true))
	} else {
		// Use maximum width over each row
		for _, area := range ValuesContainerArea() {
			width = max(width, c.sumWithGaps(c.minColumnWidths(int(area), true)))
		}
	}

	// If we don't have cells, we don't have width
	if width > 0 {
		return width + c.padding.Left + c.padding.Right
	}
	return 0
}

// MinimumHeight is getMinimumHeight().
func (c *GridContainerCell) MinimumHeight() float64 {
	height := 0.0

	if c.onlyCenterCellContributesToMinimumSize {
		// Check if an explicit minimum size for the center cell was set
		if c.centerCellMinimumSize != nil {
			height = c.centerCellMinimumSize.Y
		} else if c.cells[1][1] != nil {
			height = c.cells[1][1].MinimumHeight()
		}
	} else {
		// Minimum height of the different rows (this is independent of
		// whether we are in tabular mode or not)
		height = c.sumWithGaps(c.minRowHeights(true))
	}

	// If we don't have cells, we don't have height
	if height > 0 {
		return height + c.padding.Top + c.padding.Bottom
	}
	return 0
}

// LayoutChildrenHorizontally is layoutChildrenHorizontally().
func (c *GridContainerCell) LayoutChildrenHorizontally() {
	// How we're going to do this depends on whether we're in tabular mode or
	// not. If so, the column widths across all rows are locked. The array of
	// the tabular mode is one array that each row changes, as in the Java
	// code.
	if c.tabular {
		colWidths := c.minColumnWidths(noRow, false)
		for _, row := range ValuesContainerArea() {
			c.applyWidthsToRow(row, &colWidths)
		}
	} else {
		for _, row := range ValuesContainerArea() {
			colWidths := c.minColumnWidths(int(row), false)
			c.applyWidthsToRow(row, &colWidths)
		}
	}
}

// LayoutChildrenVertically is layoutChildrenVertically().
func (c *GridContainerCell) LayoutChildrenVertically() {
	// Note that the code in this method does not depend on whether we're in
	// tabular mode or not, that's why it looks significantly different from
	// its horizontal counterpart
	cellRectangle := c.CellRectangle()
	cellPadding := c.Padding()

	rowHeights := c.minRowHeights(false)

	// Top row is top-aligned with our content area, bottom row is
	// bottom-aligned
	c.applyHeightToRow(ContainerAreaBegin,
		cellRectangle.Y+cellPadding.Top,
		&rowHeights)
	c.applyHeightToRow(ContainerAreaEnd,
		cellRectangle.Y+cellRectangle.Height-cellPadding.Bottom-rowHeights[2],
		&rowHeights)

	// Size of the content area and size of the available space in the
	// content area
	freeContentAreaHeight := cellRectangle.Height - cellPadding.Top - cellPadding.Bottom

	if rowHeights[0] > 0 {
		rowHeights[0] += c.gap
		freeContentAreaHeight -= rowHeights[0]
	}

	if rowHeights[2] > 0 {
		rowHeights[2] += c.gap
		freeContentAreaHeight -= rowHeights[2]
	}

	// Compute the center cell rectangle
	c.centerCellRect.Height = max(0, freeContentAreaHeight)
	c.centerCellRect.Y = cellRectangle.Y + cellPadding.Top + float64((c.centerCellRect.Height-freeContentAreaHeight)/2)

	// If the available space is larger than the current size of the center
	// cell, enlarge that thing
	rowHeights[1] = max(rowHeights[1], freeContentAreaHeight)

	// Place the center cell, possibly enlarging it in the process
	c.applyHeightToRow(ContainerAreaCenter,
		cellRectangle.Y+cellPadding.Top+rowHeights[0]-float64((rowHeights[1]-freeContentAreaHeight)/2),
		&rowHeights)
}

// noRow stands for the null row of minColumnWidths and minWidthOfColumn: the
// widths are those of the whole table.
const noRow = -1

// minColumnWidths is minColumnWidths(row, respectContributionFlag): the
// width of each column of one row, or of the whole table for noRow, with the
// symmetry applied.
func (c *GridContainerCell) minColumnWidths(row int, respectContributionFlag bool) [columns]float64 {
	colWidths := [columns]float64{
		c.minWidthOfColumn(ContainerAreaBegin, row, respectContributionFlag),
		c.minWidthOfColumn(ContainerAreaCenter, row, respectContributionFlag),
		c.minWidthOfColumn(ContainerAreaEnd, row, respectContributionFlag),
	}

	// If we are to be symmetrical, the outer cells need to be the same size
	if c.symmetrical {
		colWidths[0] = max(colWidths[0], colWidths[2])
		colWidths[2] = colWidths[0]
	}

	return colWidths
}

// minWidthOfColumn is minWidthOfColumn(column, row, respectContributionFlag):
// the minimum width of the column in one row, or the maximum over the rows
// for noRow.
func (c *GridContainerCell) minWidthOfColumn(column ContainerArea, row int, respectContributionFlag bool) float64 {
	maxMinWidth := 0.0

	if row == noRow {
		// Aggregate values for all rows
		for rowIndex := 0; rowIndex < rows; rowIndex++ {
			maxMinWidth = max(
				maxMinWidth,
				minWidthOfCell(c.cells[rowIndex][column], respectContributionFlag))
		}
	} else {
		// Only concentrate on the specified row
		maxMinWidth = minWidthOfCell(c.cells[row][column], respectContributionFlag)
	}

	// If this is the center column, we might have an explicit minimal width
	// for that
	if column == ContainerAreaCenter && c.centerCellMinimumSize != nil {
		maxMinWidth = max(maxMinWidth, c.centerCellMinimumSize.X)
	}

	return maxMinWidth
}

// minRowHeights is minRowHeights(respectContributionFlag): the height of
// each row, with the symmetry applied.
func (c *GridContainerCell) minRowHeights(respectContributionFlag bool) [rows]float64 {
	rowHeights := [rows]float64{
		c.minHeightOfRow(ContainerAreaBegin, respectContributionFlag),
		c.minHeightOfRow(ContainerAreaCenter, respectContributionFlag),
		c.minHeightOfRow(ContainerAreaEnd, respectContributionFlag),
	}

	// If we are to be symmetrical, the outer cells need to be the same size
	if c.symmetrical {
		rowHeights[0] = max(rowHeights[0], rowHeights[2])
		rowHeights[2] = rowHeights[0]
	}

	return rowHeights
}

// minHeightOfRow is minHeightOfRow(row, respectContributionFlag).
func (c *GridContainerCell) minHeightOfRow(row ContainerArea, respectContributionFlag bool) float64 {
	maxMinHeight := 0.0
	for column := 0; column < columns; column++ {
		maxMinHeight = max(
			maxMinHeight,
			minHeightOfCell(c.cells[row][column], respectContributionFlag))
	}

	// If this is the center row, we might have an explicit minimal height
	// for that
	if row == ContainerAreaCenter && c.centerCellMinimumSize != nil {
		maxMinHeight = max(maxMinHeight, c.centerCellMinimumSize.Y)
	}

	return maxMinHeight
}

// sumWithGaps is sumWithGaps(values): the sum of the values, with a gap
// between each pair of values greater than zero.
func (c *GridContainerCell) sumWithGaps(values [containerAreaCount]float64) float64 {
	sum := 0.0

	// Keep track of how many components > 0 we have
	activeComponents := 0
	for _, val := range values {
		if val > 0 {
			sum += val
			activeComponents++
		}
	}

	// If there is more than a single value, add necessary gaps
	if activeComponents > 1 {
		sum += float64(c.gap * float64(activeComponents-1))
	}

	return sum
}

// applyWidthsToRow is applyWidthsToRow(row, colWidths): it sets the x
// coordinates and the widths of the cells from the column widths. For the
// center row it also sets the x coordinate and the width of the center cell
// rectangle. It changes colWidths, as the Java code changes its array.
func (c *GridContainerCell) applyWidthsToRow(row ContainerArea, colWidths *[columns]float64) {
	cellRectangle := c.CellRectangle()
	cellPadding := c.Padding()

	// Left column is left-aligned with our content area, right column is
	// right-aligned
	c.applyWidthToColumn(ContainerAreaBegin,
		cellRectangle.X+cellPadding.Left,
		colWidths)
	c.applyWidthToColumn(ContainerAreaEnd,
		cellRectangle.X+cellRectangle.Width-cellPadding.Right-colWidths[2],
		colWidths)

	// Size of the content area and size of the available space in the
	// content area
	freeContentAreaWidth := cellRectangle.Width - cellPadding.Left - cellPadding.Right

	if colWidths[0] > 0 {
		colWidths[0] += c.gap
		freeContentAreaWidth -= colWidths[0]
	}

	if colWidths[2] > 0 {
		colWidths[2] += c.gap
		freeContentAreaWidth -= colWidths[2]
	}

	// Compute how wide the center cell can be
	centerWidth := max(0, freeContentAreaWidth)

	// If the available space is larger than the current size of the center
	// cell, enlarge that thing
	colWidths[1] = max(colWidths[1], freeContentAreaWidth)

	// Place the center cell, possibly enlarging it in the process
	c.applyWidthToColumn(ContainerAreaCenter,
		cellRectangle.X+cellPadding.Left+colWidths[0]-float64((colWidths[1]-freeContentAreaWidth)/2),
		colWidths)

	// If this is the center row, remember the center cell's data for the
	// center cell rectangle
	if row == ContainerAreaCenter {
		c.centerCellRect.Width = centerWidth
		c.centerCellRect.X = cellRectangle.X + cellPadding.Left + float64((centerWidth-freeContentAreaWidth)/2)
	}
}

// applyWidthToColumn is applyWidthToColumn(column, x, colWidths): it applies
// the horizontal layout to each cell of the column.
func (c *GridContainerCell) applyWidthToColumn(column ContainerArea, x float64, colWidths *[columns]float64) {
	for row := 0; row < rows; row++ {
		applyHorizontalLayout(c.cells[row][column], x, colWidths[column])
	}
}

// applyHeightToRow is applyHeightToRow(row, y, rowHeights): it applies the
// vertical layout to each cell of the row.
func (c *GridContainerCell) applyHeightToRow(row ContainerArea, y float64, rowHeights *[rows]float64) {
	for column := 0; column < columns; column++ {
		applyVerticalLayout(c.cells[row][column], y, rowHeights[row])
	}
}
