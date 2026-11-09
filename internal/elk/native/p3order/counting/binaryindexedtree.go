// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p3order/counting/BinaryIndexedTree.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package counting

// BinaryIndexedTree is the class BinaryIndexedTree: a sorted list of
// integers from 0 up to the maximum number that is passed on creation. Add,
// RemoveAll and Rank run in O(log maxNum).
type BinaryIndexedTree struct {
	binarySums   []int
	numsPerIndex []int
	size         int
	maxNum       int
}

// NewBinaryIndexedTree is new BinaryIndexedTree(maxNum).
func NewBinaryIndexedTree(maxNum int) *BinaryIndexedTree {
	return &BinaryIndexedTree{
		maxNum:       maxNum,
		binarySums:   make([]int, maxNum+1),
		numsPerIndex: make([]int, maxNum),
	}
}

// Add is add(index): it increments the given index.
func (t *BinaryIndexedTree) Add(index int) {
	t.size++
	t.numsPerIndex[index]++
	i := index + 1
	for i < len(t.binarySums) {
		t.binarySums[i]++
		i += i & -i
	}
}

// Rank is rank(index): the sum of all entries before the given index.
func (t *BinaryIndexedTree) Rank(index int) int {
	i := index
	sum := 0
	for i > 0 {
		sum += t.binarySums[i]
		i -= i & -i
	}
	return sum
}

// Size is size().
func (t *BinaryIndexedTree) Size() int { return t.size }

// RemoveAll is removeAll(index): it removes all entries for one index.
func (t *BinaryIndexedTree) RemoveAll(index int) {
	numEntries := t.numsPerIndex[index]
	if numEntries == 0 {
		return
	}
	t.numsPerIndex[index] = 0
	t.size -= numEntries
	i := index + 1
	for i < len(t.binarySums) {
		t.binarySums[i] -= numEntries
		i += i & -i
	}
}

// Clear is clear().
func (t *BinaryIndexedTree) Clear() {
	t.binarySums = make([]int, t.maxNum+1)
	t.numsPerIndex = make([]int, t.maxNum)
	t.size = 0
}

// IsEmpty is isEmpty().
func (t *BinaryIndexedTree) IsEmpty() bool { return t.size == 0 }
