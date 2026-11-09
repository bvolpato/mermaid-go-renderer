// Ported from mermaid 12.2.0, diagrams/block/layout.ts (MIT).

package block

import (
	"errors"
	"fmt"
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// blockPosition is BlockPosition: the column and the row of a block.
type blockPosition struct {
	px, py float64
}

// isInteger is Number.isInteger(x).
func isInteger(x float64) bool {
	return !math.IsInf(x, 0) && x == math.Floor(x)
}

// calculateBlockPosition is calculateBlockPosition(columns, position).
func calculateBlockPosition(columns, position float64) (blockPosition, error) {
	// Ensure that columns is a positive integer
	if columns == 0 || !isInteger(columns) {
		return blockPosition{}, errors.New("Columns must be an integer !== 0.")
	}

	// Ensure that position is a non-negative integer
	if position < 0 || !isInteger(position) {
		return blockPosition{}, errors.New("Position must be a non-negative integer." + js.Str(position))
	}

	if columns < 0 {
		// Auto columns is set
		return blockPosition{px: position, py: 0}, nil
	}
	if columns == 1 {
		// Auto columns is set
		return blockPosition{px: 0, py: position}, nil
	}
	// Calculate posX and posY
	px := math.Mod(position, columns)
	py := math.Floor(position / columns)
	return blockPosition{px: px, py: py}, nil
}

// getMaxChildSize is getMaxChildSize(block): the largest width for one
// column and the largest height of the children.
func getMaxChildSize(block *Block) (width, height float64) {
	maxWidth := 0.0
	maxHeight := 0.0
	// find max width of children
	for _, child := range block.Children {
		// child.size ?? { width: 0, height: 0, x: 0, y: 0 }
		var width, height float64
		if child.Size != nil {
			width, height = child.Size.Width, child.Size.Height
		}
		if child.Type == typeSpace {
			continue
		}
		normalizedWidth := width / nn(child.WidthInColumns, 1)
		if normalizedWidth > maxWidth {
			maxWidth = normalizedWidth
		}
		if height > maxHeight {
			maxHeight = height
		}
	}
	return maxWidth, maxHeight
}

// sizeWidthOr0 is `block?.size?.width || 0`.
func sizeWidthOr0(block *Block) float64 {
	if block.Size == nil || !js.Truthy(block.Size.Width) {
		return 0
	}
	return block.Size.Width
}

// setBlockSizes is setBlockSizes(block, db, siblingWidth, siblingHeight,
// padding).
func setBlockSizes(block *Block, db *DB, siblingWidth, siblingHeight, padding float64) {
	if block.Size == nil || !js.Truthy(block.Size.Width) {
		block.Size = &Size{
			Width:  siblingWidth,
			Height: siblingHeight,
			X:      0,
			Y:      0,
		}
	}
	maxWidth := 0.0
	maxHeight := 0.0

	if len(block.Children) > 0 {
		for _, child := range block.Children {
			setBlockSizes(child, db, 0, 0, padding)
		}
		// find max width of children
		maxWidth, maxHeight = getMaxChildSize(block)

		// set width of block to max width of children
		for _, child := range block.Children {
			if child.Size != nil {
				child.Size.Width = float64(maxWidth*nn(child.WidthInColumns, 1)) + float64(padding*(nn(child.WidthInColumns, 1)-1))
				child.Size.Height = maxHeight
				child.Size.X = 0
				child.Size.Y = 0
			}
		}
		for _, child := range block.Children {
			setBlockSizes(child, db, maxWidth, maxHeight, padding)
		}

		columns := nn(block.Columns, -1)
		numItems := 0.0
		for _, child := range block.Children {
			numItems += nn(child.WidthInColumns, 1)
		}

		// The width and height in number blocks
		xSize := float64(len(block.Children))
		if columns > 0 && columns < numItems {
			xSize = columns
		}

		ySize := math.Ceil(numItems / xSize)

		width := float64(xSize*(maxWidth+padding)) + padding
		height := float64(ySize*(maxHeight+padding)) + padding
		// If maxWidth
		if width < siblingWidth {
			width = siblingWidth
			height = siblingHeight
			childWidth := (siblingWidth - float64(xSize*padding) - padding) / xSize
			childHeight := (siblingHeight - float64(ySize*padding) - padding) / ySize

			// set width of block to max width of children
			for _, child := range block.Children {
				if child.Size != nil {
					child.Size.Width = childWidth
					child.Size.Height = childHeight
					child.Size.X = 0
					child.Size.Y = 0
				}
			}
		}

		if width < sizeWidthOr0(block) {
			width = sizeWidthOr0(block)

			// Grow children to fit
			num := float64(len(block.Children))
			if columns > 0 {
				num = js.Min(float64(len(block.Children)), columns)
			}
			if num > 0 {
				childWidth := (width - float64(num*padding) - padding) / num
				for _, child := range block.Children {
					if child.Size != nil {
						child.Size.Width = childWidth
					}
				}
			}
		}
		block.Size = &Size{
			Width:  width,
			Height: height,
			X:      0,
			Y:      0,
		}
	}
}

// rowStartX is
// `block?.size?.x ? block?.size?.x + (-block?.size?.width / 2 || 0) : -padding`:
// the value of startingPosX at the start of a row.
func rowStartX(block *Block, padding float64) float64 {
	if block.Size != nil && js.Truthy(block.Size.X) {
		half := float64(-block.Size.Width / 2)
		if !js.Truthy(half) {
			half = 0
		}
		return block.Size.X + half
	}
	return -padding
}

// layoutBlocks is layoutBlocks(block, db, padding).
func layoutBlocks(block *Block, db *DB, padding float64) error {
	columns := nn(block.Columns, -1)
	if len(block.Children) > 0 {
		// Pre-compute per-row max heights so y-positioning accounts for rows of different heights
		rowHeights := js.NewMap[float64, float64]()
		{
			colPos := 0.0
			for _, child := range block.Children {
				if child.Size == nil {
					continue
				}
				pos, err := calculateBlockPosition(columns, colPos)
				if err != nil {
					return err
				}
				py := pos.py
				currentMax := rowHeights.Get(py)
				if child.Size.Height > currentMax {
					rowHeights.Set(py, child.Size.Height)
				}
				filled := nn(child.WidthInColumns, 1)
				if columns > 0 {
					filled = js.Min(filled, columns-math.Mod(colPos, columns))
				}
				colPos += filled
			}
		}
		rowYOffsets := map[float64]float64{}
		{
			offset := 0.0
			rows := rowHeights.Keys()
			js.SortFunc(rows, func(a, b float64) float64 { return a - b })
			for _, row := range rows {
				rowYOffsets[row] = offset
				offset += rowHeights.Get(row) + padding
			}
		}

		columnPos := 0.0
		startingPosX := rowStartX(block, padding)
		rowPos := 0.0
		for _, child := range block.Children {
			parent := block

			if child.Size == nil {
				continue
			}
			width, height := child.Size.Width, child.Size.Height
			pos, err := calculateBlockPosition(columns, columnPos)
			if err != nil {
				return err
			}
			py := pos.py
			if py != rowPos {
				rowPos = py
				startingPosX = rowStartX(block, padding)
			}
			if parent.Size != nil {
				halfWidth := float64(width / 2)
				child.Size.X = startingPosX + padding + halfWidth

				startingPosX = child.Size.X + halfWidth

				// rowYOffsets.get(py) ?? 0
				rowYOffset := rowYOffsets[py]
				// rowHeights.get(py) ?? height
				rowHeight, ok := rowHeights.Lookup(py)
				if !ok {
					rowHeight = height
				}
				child.Size.Y = parent.Size.Y - float64(parent.Size.Height/2) + rowYOffset + float64(rowHeight/2) + padding
			}
			if child.hasChildren() {
				if err := layoutBlocks(child, db, padding); err != nil {
					return err
				}
			}
			columnsFilled := nn(child.WidthInColumns, 1)
			if columns > 0 {
				// Make sure overflowing lines do not affect later lines
				columnsFilled = js.Min(columnsFilled, columns-math.Mod(columnPos, columns))
			}
			columnPos += columnsFilled
		}
	}
	return nil
}

// boundsAcc is the { minX, minY, maxX, maxY } object of findBounds.
type boundsAcc struct {
	minX, minY, maxX, maxY float64
}

// findBounds is findBounds(block, { minX, minY, maxX, maxY }).
func findBounds(block *Block, acc boundsAcc) boundsAcc {
	if block.Size != nil && (block.IDUndefined || block.ID != "root") {
		x, y, width, height := block.Size.X, block.Size.Y, block.Size.Width, block.Size.Height
		if x-float64(width/2) < acc.minX {
			acc.minX = x - float64(width/2)
		}
		if y-float64(height/2) < acc.minY {
			acc.minY = y - float64(height/2)
		}
		if x+float64(width/2) > acc.maxX {
			acc.maxX = x + float64(width/2)
		}
		if y+float64(height/2) > acc.maxY {
			acc.maxY = y + float64(height/2)
		}
	}
	if block.hasChildren() {
		for _, child := range block.Children {
			acc = findBounds(child, acc)
		}
	}
	return acc
}

// Bounds is the result of layout: the box around every block.
type Bounds struct {
	X, Y, Width, Height float64
}

// maxLayoutSteps is the largest number of setBlockSizes calls of one
// layout. setBlockSizes visits every child two times, so its number of
// calls doubles with each level of nested blocks. Upstream has no limit: a
// text with some tens of nested blocks keeps the browser busy for hours. A
// layout with more calls is an error here.
const maxLayoutSteps = 1 << 26

// layoutSteps counts the calls of setBlockSizes for the block and the
// blocks inside it, where calls is the number of calls for the block. The
// count stops over maxLayoutSteps.
func layoutSteps(block *Block, calls float64, total *float64) {
	*total += calls
	if *total > maxLayoutSteps {
		return
	}
	for _, child := range block.Children {
		layoutSteps(child, 2*calls, total)
		if *total > maxLayoutSteps {
			return
		}
	}
}

// blockPadding is `getConfig()?.block?.padding ?? d` as a number. Upstream
// uses a value that is not a number as it is: a string then makes string
// concatenations of the sums of the layout. Here such a value is converted
// to a number.
func blockPadding(db *DB, d float64) float64 {
	v := db.ctx.Cfg("block", "padding")
	if js.IsNullish(v) {
		return d
	}
	return js.ToNumber(v)
}

// layout is layout(db): it sets the size and the position of every block
// of the hierarchy and returns the box around them. ok is false when the
// db has no root block (upstream returns undefined).
func layout(db *DB) (bounds Bounds, ok bool, err error) {
	if db.beforeLayout != nil {
		db.beforeLayout()
	}
	root := db.GetBlock("root")
	if root == nil {
		return Bounds{}, false, nil
	}

	steps := 0.0
	layoutSteps(root, 1, &steps)
	if steps > maxLayoutSteps {
		return Bounds{}, false, fmt.Errorf("block: the blocks are nested too deep for the layout (over %d steps)", maxLayoutSteps)
	}

	padding := blockPadding(db, 8)
	setBlockSizes(root, db, 0, 0, padding)
	if err := layoutBlocks(root, db, padding); err != nil {
		return Bounds{}, false, err
	}
	// Position blocks relative to parents

	acc := findBounds(root, boundsAcc{})

	height := acc.maxY - acc.minY
	width := acc.maxX - acc.minX
	return Bounds{X: acc.minX, Y: acc.minY, Width: width, Height: height}, true, nil
}
