// Ported from non-layered-tidy-tree-layout 2.0.2, src/helpers.js (MIT), in
// the form that @mermaid-js/layout-tidy-tree 1.0.1 (MIT) bundles.
//
// The methods assignCoordinates and getSize of the class Layout are not
// here: layout does the work of both with assignLayout, and nothing else
// calls them.

package tidytree

import "github.com/bvolpato/mermaid-go-renderer/internal/js"

// boundingBox is the class BoundingBox.
type boundingBox struct {
	// gap is the gap between sibling nodes.
	gap float64
	// bottomPadding is the height reserved for connection drawing.
	bottomPadding float64
}

func (bb *boundingBox) addBoundingBox(width, height float64) (float64, float64) {
	return width + bb.gap, height + bb.bottomPadding
}

// removeBoundingBox returns the coordinate without the bounding box for a
// node.
func (bb *boundingBox) removeBoundingBox(x, y float64) (float64, float64) {
	return x + float64(bb.gap/2), y
}

// box is the bounding box that encompasses all the nodes. This is not the
// same bounding box concept as the boundingBox type.
type box struct {
	left, right, top, bottom float64
}

// treeLayout is the class Layout.
type treeLayout struct {
	bb *boundingBox
}

// layout lays out treeData. It returns the modified treeData and the
// bounding box encompassing all the nodes.
func (l *treeLayout) layout(treeData *tidyTreeNode) (*tidyTreeNode, *box) {
	tree := l.convert(treeData, 0)
	layout(tree)
	result, boundingBox := l.assignLayout(tree, treeData, nil)

	return result, boundingBox
}

// convert returns the tree to lay out, with bounding boxes added to each
// node.
func (l *treeLayout) convert(treeData *tidyTreeNode, y float64) *tree {
	if treeData == nil {
		return nil
	}

	width, height := l.bb.addBoundingBox(treeData.width, treeData.height)
	children := []*tree{}
	if len(treeData.children) != 0 {
		for i := 0; i < len(treeData.children); i++ {
			children = append(children, l.convert(treeData.children[i], y+height))
		}
	}

	return newTree(width, height, y, children)
}

// assignLayout assigns the x and y coordinates of the layout tree back to
// treeData, with bounding boxes removed, and computes the bounding box that
// encompasses all the nodes, in one pass.
func (l *treeLayout) assignLayout(tree *tree, treeData *tidyTreeNode, box0 *box) (*tidyTreeNode, *box) {
	x, y := l.bb.removeBoundingBox(tree.x, tree.y)

	treeData.x = x
	treeData.y = y

	width, height := treeData.width, treeData.height
	if box0 == nil {
		box0 = &box{left: x, right: x + width, top: y, bottom: y + height}
	}
	box0.left = js.Min(box0.left, x)
	box0.right = js.Max(box0.right, x+width)
	box0.top = js.Min(box0.top, y)
	box0.bottom = js.Max(box0.bottom, y+height)

	for i := 0; i < len(tree.c); i++ {
		l.assignLayout(tree.c[i], treeData.children[i], box0)
	}

	return treeData, box0
}
