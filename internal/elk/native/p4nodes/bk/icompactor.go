// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p4nodes/bk/ICompactor.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package bk

// ICompactor is the interface ICompactor: the step of the node placer that
// gives each block its coordinate.
type ICompactor interface {
	// horizontalCompaction is horizontalCompaction(bal): it computes the
	// coordinates of the blocks of a layout and of their nodes.
	horizontalCompaction(bal *BKAlignedLayout)
}
