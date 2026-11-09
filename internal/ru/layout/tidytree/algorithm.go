// Ported from non-layered-tidy-tree-layout 2.0.2, src/algorithm.js (MIT), in
// the form that @mermaid-js/layout-tidy-tree 1.0.1 (MIT) bundles.

package tidytree

// typeError is a TypeError of JavaScript. The layout throws it for a tree
// whose sizes break the order of the contours. executeTidyTreeLayout
// catches it and rejects with it.
type typeError string

func (e typeError) Error() string { return string(e) }

// tree is the class Tree.
type tree struct {
	w, h, y float64
	c       []*tree
	cs      int

	x, prelim, mod, shift, change float64
	// tl is the left thread and tr is the right thread.
	tl, tr *tree
	// el and er are the extreme left and right nodes.
	el, er *tree
	// msel and mser are the sums of modifiers at the extreme nodes.
	msel, mser float64
}

// newTree is the constructor of Tree.
func newTree(width, height, y float64, children []*tree) *tree {
	return &tree{w: width, h: height, y: y, c: children, cs: len(children)}
}

func setExtremes(tree *tree) {
	if tree.cs == 0 {
		tree.el = tree
		tree.er = tree
		tree.mser = 0
		tree.msel = 0
	} else {
		tree.el = tree.c[0].el
		tree.msel = tree.c[0].msel
		tree.er = tree.c[tree.cs-1].er
		tree.mser = tree.c[tree.cs-1].mser
	}
}

func bottom(tree *tree) float64 {
	return tree.y + tree.h
}

// iyl is the class IYL: a linked list of the indexes of left siblings and
// their lowest vertical coordinate.
type iyl struct {
	lowY  float64
	index int
	next  *iyl
}

func updateIYL(minY float64, i int, ih *iyl) *iyl {
	// Remove siblings that are hidden by the new subtree.
	for ih != nil && minY >= ih.lowY {
		// Prepend the new subtree
		ih = ih.next
	}
	return &iyl{lowY: minY, index: i, next: ih}
}

func distributeExtra(tree *tree, i, si int, distance float64) {
	// Are there intermediate children?
	if si != i-1 {
		nr := float64(i - si)
		tree.c[si+1].shift += distance / nr
		tree.c[i].shift -= distance / nr
		tree.c[i].change -= distance - distance/nr
	}
}

func moveSubtree(tree *tree, i, si int, distance float64) {
	// Move subtree by changing mod.
	tree.c[i].mod += distance
	tree.c[i].msel += distance
	tree.c[i].mser += distance
	distributeExtra(tree, i, si, distance)
}

func nextLeftContour(tree *tree) *tree {
	if tree.cs == 0 {
		return tree.tl
	}
	return tree.c[0]
}

func nextRightContour(tree *tree) *tree {
	if tree.cs == 0 {
		return tree.tr
	}
	return tree.c[tree.cs-1]
}

func setLeftThread(tree *tree, i int, cl *tree, modsumcl float64) {
	li := tree.c[0].el
	li.tl = cl
	// Change mod so that the sum of modifier after following thread is correct.
	diff := modsumcl - cl.mod - tree.c[0].msel
	li.mod += diff
	// Change preliminary x coordinate so that the node does not move.
	li.prelim -= diff
	// Update extreme node and its sum of modifiers.
	tree.c[0].el = tree.c[i].el
	tree.c[0].msel = tree.c[i].msel
}

// setRightThread is symmetrical to setLeftThread.
func setRightThread(tree *tree, i int, sr *tree, modsumsr float64) {
	ri := tree.c[i].er
	ri.tr = sr
	diff := modsumsr - sr.mod - tree.c[i].mser
	ri.mod += diff
	ri.prelim -= diff
	tree.c[i].er = tree.c[i-1].er
	tree.c[i].mser = tree.c[i-1].mser
}

// seperate keeps the spelling of upstream.
func seperate(tree *tree, i int, ih *iyl) {
	// Right contour node of left siblings and its sum of modifiers.
	sr := tree.c[i-1]
	mssr := sr.mod
	// Left contour node of current subtree and its sum of modifiers.
	cl := tree.c[i]
	mscl := cl.mod
	for sr != nil && cl != nil {
		if ih == nil {
			panic(typeError("Cannot read properties of null (reading 'lowY')"))
		}
		if bottom(sr) > ih.lowY {
			ih = ih.next
		}
		// How far to the left of the right side of sr is the left side of cl.
		distance := mssr + sr.prelim + sr.w - (mscl + cl.prelim)
		if distance > 0 {
			mscl += distance
			if ih == nil {
				panic(typeError("Cannot read properties of null (reading 'index')"))
			}
			moveSubtree(tree, i, ih.index, distance)
		}
		sy := bottom(sr)
		cy := bottom(cl)
		// Advance highest node(s) and sum(s) of modifiers
		if sy <= cy {
			sr = nextRightContour(sr)
			if sr != nil {
				mssr += sr.mod
			}
		}
		if sy >= cy {
			cl = nextLeftContour(cl)
			if cl != nil {
				mscl += cl.mod
			}
		}
	}
	// Set threads if required.
	if sr == nil && cl != nil {
		setLeftThread(tree, i, cl, mscl)
	} else if sr != nil && cl == nil {
		setRightThread(tree, i, sr, mssr)
	}
}

func positionRoot(tree *tree) {
	// Position root between children, taking into account their mod.
	tree.prelim = float64((tree.c[0].prelim+
		tree.c[0].mod+
		tree.c[tree.cs-1].mod+
		tree.c[tree.cs-1].prelim+
		tree.c[tree.cs-1].w)/
		2) -
		float64(tree.w/2)
}

func firstWalk(tree *tree) {
	if tree.cs == 0 {
		setExtremes(tree)
		return
	}

	firstWalk(tree.c[0])
	ih := updateIYL(bottom(tree.c[0].el), 0, nil)
	for i := 1; i < tree.cs; i++ {
		firstWalk(tree.c[i])
		minY := bottom(tree.c[i].er)
		seperate(tree, i, ih)
		ih = updateIYL(minY, i, ih)
	}
	positionRoot(tree)
	setExtremes(tree)
}

func addChildSpacing(tree *tree) {
	d := 0.0
	modsumdelta := 0.0
	for i := 0; i < tree.cs; i++ {
		d += tree.c[i].shift
		modsumdelta += d + tree.c[i].change
		tree.c[i].mod += modsumdelta
	}
}

func secondWalk(tree *tree, modsum float64) {
	modsum += tree.mod
	// Set absolute (no-relative) horizontal coordinates.
	tree.x = tree.prelim + modsum
	addChildSpacing(tree)
	for i := 0; i < tree.cs; i++ {
		secondWalk(tree.c[i], modsum)
	}
}

// layout is layout(tree): the two walks of the algorithm.
func layout(tree *tree) {
	firstWalk(tree)
	secondWalk(tree, 0)
}
