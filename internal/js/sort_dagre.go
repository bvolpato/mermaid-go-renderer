// Ported from v8/third_party/v8/builtins/array-sort.tq, at the tags
// 14.8-lkgr (TimSort) and 15.6-lkgr (PowerSort) (PSF-2.0). The file is based
// on Objects/listobject.c of CPython. It has the copyright notice of the
// Python Software Foundation, and its license is
// third_party/v8/builtins/LICENSE of V8, which is the Python license.
// LICENSE-PSF.txt in this directory has that license.
//
// The copyright notice of array-sort.tq at the tag 14.8-lkgr:
//
//	Copyright (c) 2001, 2002, 2003, 2004, 2005, 2006, 2007, 2008, 2009, 2010,
//	2011, 2012, 2013, 2014, 2015, 2016, 2017, 2018 Python Software Foundation;
//	All Rights Reserved
//
// The copyright notice of array-sort.tq at the tag 15.6-lkgr:
//
//	Copyright (c) 2001, 2002, 2003, 2004, 2005, 2006, 2007, 2008, 2009, 2010,
//	2011, 2012, 2013, 2014, 2015, 2016, 2017, 2018, 2019, 2020, 2021, 2022, 2023,
//	2024, 2025, 2026 Python Software Foundation; All Rights Reserved
//
// Summary of the changes: this file is a translation of the two versions of
// array-sort.tq from Torque to Go. It sorts a Go slice with a comparator
// function. It leaves out the parts of the builtin that read and write a
// JavaScript array.

package js

import "math/bits"

// V8Sort names one implementation of Array.prototype.sort in V8.
//
// Every implementation is a stable sort, so all of them give the same result
// for a consistent comparator. They give different results for a comparator
// that is not a strict weak order. dagre has such comparators: a barycenter
// can be NaN, and lodash sortBy compares NaN keys inconsistently. For those
// inputs the result depends on the exact sequence of comparator calls, so
// the port must run the same algorithm as the engine.
type V8Sort int

const (
	// V8PowerSort is the builtin of V8 14.9 and later (Chromium 156): a
	// binary insertion sort for fewer than 16 elements, PowerSort otherwise.
	V8PowerSort V8Sort = iota
	// V8TimSort is the builtin of V8 7.0 to 14.8 (Node.js 26): a binary
	// insertion sort for fewer than 8 elements, TimSort otherwise.
	V8TimSort
)

const (
	v8MaxMergePending = 85
	v8MinGallopWins   = 7
	v8SortStateTemp   = 32
)

type v8SortState[T any] struct {
	work      []T
	temp      []T
	cmp       func(a, b T) float64
	minGallop int
	// The stack of pending runs. Run i starts at runBase[i] and has
	// runLength[i] elements. power[i] is the power of the boundary between
	// run i and run i+1 (PowerSort only).
	runBase   []int
	runLength []int
	power     [v8MaxMergePending]int
}

// SortV8 sorts s in place as Array.prototype.sort(cmp) does in the given V8
// version. The comparator returns a negative, zero or positive number. A NaN
// result counts as zero. The slice must not contain values that stand for
// undefined: V8 moves those to the end without a comparator call.
func SortV8[T any](mode V8Sort, s []T, cmp func(a, b T) float64) {
	n := len(s)
	if n < 2 {
		return
	}
	st := &v8SortState[T]{work: s, cmp: cmp, minGallop: v8MinGallopWins}
	switch mode {
	case V8TimSort:
		if n < 8 {
			st.binaryInsertionSort(0, 0, n)
		} else {
			st.timSortImpl(n)
		}
	default:
		if n < 16 {
			st.binaryInsertionSort(0, 0, n)
		} else {
			st.powerSortImpl(n)
		}
	}
}

// compare is SortCompareUserFn: a NaN result is +0.
func (st *v8SortState[T]) compare(x, y T) float64 {
	v := st.cmp(x, y)
	if v != v {
		return 0
	}
	return v
}

func (st *v8SortState[T]) pushRun(base, length int) {
	st.runBase = append(st.runBase, base)
	st.runLength = append(st.runLength, length)
}

func (st *v8SortState[T]) getTempArray(requestedSize int) []T {
	minSize := max(v8SortStateTemp, requestedSize)
	if len(st.temp) >= minSize {
		return st.temp
	}
	st.temp = make([]T, minSize)
	return st.temp
}

// binaryInsertionSort sorts [low, high). The range [low, start) is sorted on entry.
func (st *v8SortState[T]) binaryInsertionSort(low, startArg, high int) {
	work := st.work
	start := startArg
	if low == startArg {
		start = startArg + 1
	}
	for ; start < high; start++ {
		// Set left to where work[start] belongs.
		left := low
		right := start
		pivot := work[right]
		for left < right {
			mid := left + ((right - left) >> 1)
			order := st.compare(pivot, work[mid])
			if order < 0 {
				right = mid
			} else {
				left = mid + 1
			}
		}
		for p := start; p > left; p-- {
			work[p] = work[p-1]
		}
		work[left] = pivot
	}
}

// countAndMakeRun returns the length of the run that begins at lowArg. A
// strictly descending run is reversed in place.
func (st *v8SortState[T]) countAndMakeRun(lowArg, high int) int {
	work := st.work
	low := lowArg + 1
	if low == high {
		return 1
	}
	runLength := 2
	elementLow := work[low]
	elementLowPred := work[low-1]
	order := st.compare(elementLow, elementLowPred)
	isDescending := order < 0

	previousElement := elementLow
	for idx := low + 1; idx < high; idx++ {
		currentElement := work[idx]
		order = st.compare(currentElement, previousElement)
		if isDescending {
			if order >= 0 {
				break
			}
		} else {
			if order < 0 {
				break
			}
		}
		previousElement = currentElement
		runLength++
	}
	if isDescending {
		lo, hi := lowArg, lowArg+runLength-1
		for lo < hi {
			work[lo], work[hi] = work[hi], work[lo]
			lo++
			hi--
		}
	}
	return runLength
}

// mergeAt merges the two runs at stack indices i and i + 1.
func (st *v8SortState[T]) mergeAt(i int) {
	stackSize := len(st.runBase)
	work := st.work

	baseA := st.runBase[i]
	lengthA := st.runLength[i]
	baseB := st.runBase[i+1]
	lengthB := st.runLength[i+1]

	// Record the length of the combined runs. If i is the third-last run,
	// also slide over the last run, which is not part of this merge.
	st.runLength[i] = lengthA + lengthB
	if i == stackSize-3 {
		st.runBase[i+1] = st.runBase[i+2]
		st.runLength[i+1] = st.runLength[i+2]
	}
	st.runBase = st.runBase[:stackSize-1]
	st.runLength = st.runLength[:stackSize-1]

	// Where does b start in a? Elements in a before that are in place.
	keyRight := work[baseB]
	k := st.gallopRight(work, keyRight, baseA, lengthA, 0)
	baseA = baseA + k
	lengthA = lengthA - k
	if lengthA == 0 {
		return
	}

	// Where does a end in b? Elements in b after that are in place.
	keyLeft := work[baseA+lengthA-1]
	lengthB = st.gallopLeft(work, keyLeft, baseB, lengthB, lengthB-1)
	if lengthB == 0 {
		return
	}

	if lengthA <= lengthB {
		st.mergeLow(baseA, lengthA, baseB, lengthB)
	} else {
		st.mergeHigh(baseA, lengthA, baseB, lengthB)
	}
}

// gallopLeft returns the offset k in 0..length such that
// array[base+k-1] < key <= array[base+k].
func (st *v8SortState[T]) gallopLeft(array []T, key T, base, length, hint int) int {
	lastOfs := 0
	offset := 1

	baseHintElement := array[base+hint]
	order := st.compare(baseHintElement, key)

	if order < 0 {
		// array[base+hint] < key: gallop right.
		maxOfs := length - hint
		for offset < maxOfs {
			offsetElement := array[base+hint+offset]
			order = st.compare(offsetElement, key)
			if order >= 0 {
				break
			}
			lastOfs = offset
			offset = (offset << 1) + 1
			if offset <= 0 {
				offset = maxOfs
			}
		}
		if offset > maxOfs {
			offset = maxOfs
		}
		lastOfs = lastOfs + hint
		offset = offset + hint
	} else {
		// key <= array[base+hint]: gallop left.
		maxOfs := hint + 1
		for offset < maxOfs {
			offsetElement := array[base+hint-offset]
			order = st.compare(offsetElement, key)
			if order < 0 {
				break
			}
			lastOfs = offset
			offset = (offset << 1) + 1
			if offset <= 0 {
				offset = maxOfs
			}
		}
		if offset > maxOfs {
			offset = maxOfs
		}
		tmp := lastOfs
		lastOfs = hint - offset
		offset = hint - tmp
	}

	lastOfs++
	for lastOfs < offset {
		m := lastOfs + ((offset - lastOfs) >> 1)
		order = st.compare(array[base+m], key)
		if order < 0 {
			lastOfs = m + 1
		} else {
			offset = m
		}
	}
	return offset
}

// gallopRight returns the offset k in 0..length such that
// array[base+k-1] <= key < array[base+k].
func (st *v8SortState[T]) gallopRight(array []T, key T, base, length, hint int) int {
	lastOfs := 0
	offset := 1

	baseHintElement := array[base+hint]
	order := st.compare(key, baseHintElement)

	if order < 0 {
		// key < array[base+hint]: gallop left.
		maxOfs := hint + 1
		for offset < maxOfs {
			offsetElement := array[base+hint-offset]
			order = st.compare(key, offsetElement)
			if order >= 0 {
				break
			}
			lastOfs = offset
			offset = (offset << 1) + 1
			if offset <= 0 {
				offset = maxOfs
			}
		}
		if offset > maxOfs {
			offset = maxOfs
		}
		tmp := lastOfs
		lastOfs = hint - offset
		offset = hint - tmp
	} else {
		// array[base+hint] <= key: gallop right.
		maxOfs := length - hint
		for offset < maxOfs {
			offsetElement := array[base+hint+offset]
			order = st.compare(key, offsetElement)
			if order < 0 {
				break
			}
			lastOfs = offset
			offset = (offset << 1) + 1
			if offset <= 0 {
				offset = maxOfs
			}
		}
		if offset > maxOfs {
			offset = maxOfs
		}
		lastOfs = lastOfs + hint
		offset = offset + hint
	}

	lastOfs++
	for lastOfs < offset {
		m := lastOfs + ((offset - lastOfs) >> 1)
		order = st.compare(key, array[base+m])
		if order < 0 {
			offset = m
		} else {
			lastOfs = m + 1
		}
	}
	return offset
}

// The exits of the merge loops, which are labels in the Torque source.
const (
	v8MergeSucceed = iota
	v8MergeCopyRest
)

// mergeLow merges the run of lengthA elements at baseA with the run of
// lengthB elements at baseB. It copies run A to the temp array and merges
// from left to right.
func (st *v8SortState[T]) mergeLow(baseA, lengthAArg, baseB, lengthBArg int) {
	lengthA := lengthAArg
	lengthB := lengthBArg

	work := st.work
	temp := st.getTempArray(lengthA)
	copy(temp[:lengthA], work[baseA:baseA+lengthA])

	dest := baseA
	cursorTemp := 0
	cursorB := baseB

	work[dest] = work[cursorB]
	dest++
	cursorB++

	exit := func() int {
		lengthB--
		if lengthB == 0 {
			return v8MergeSucceed
		}
		if lengthA == 1 {
			return v8MergeCopyRest
		}

		minGallop := st.minGallop
		for {
			nofWinsA := 0 // number of times A won in a row
			nofWinsB := 0 // number of times B won in a row

			// Do the straightforward thing until one run appears to win
			// consistently.
			for {
				order := st.compare(work[cursorB], temp[cursorTemp])
				if order < 0 {
					work[dest] = work[cursorB]
					dest++
					cursorB++

					nofWinsB++
					lengthB--
					nofWinsA = 0

					if lengthB == 0 {
						return v8MergeSucceed
					}
					if nofWinsB >= minGallop {
						break
					}
				} else {
					work[dest] = temp[cursorTemp]
					dest++
					cursorTemp++

					nofWinsA++
					lengthA--
					nofWinsB = 0

					if lengthA == 1 {
						return v8MergeCopyRest
					}
					if nofWinsA >= minGallop {
						break
					}
				}
			}

			// One run wins consistently. Gallop until neither run wins
			// consistently.
			minGallop++
			firstIteration := true
			for nofWinsA >= v8MinGallopWins || nofWinsB >= v8MinGallopWins || firstIteration {
				firstIteration = false

				minGallop = max(1, minGallop-1)
				st.minGallop = minGallop

				nofWinsA = st.gallopRight(temp, work[cursorB], cursorTemp, lengthA, 0)
				if nofWinsA > 0 {
					copy(work[dest:dest+nofWinsA], temp[cursorTemp:cursorTemp+nofWinsA])
					dest = dest + nofWinsA
					cursorTemp = cursorTemp + nofWinsA
					lengthA = lengthA - nofWinsA

					if lengthA == 1 {
						return v8MergeCopyRest
					}
					// lengthA == 0 is possible when the comparator is not consistent.
					if lengthA == 0 {
						return v8MergeSucceed
					}
				}
				work[dest] = work[cursorB]
				dest++
				cursorB++
				lengthB--
				if lengthB == 0 {
					return v8MergeSucceed
				}

				nofWinsB = st.gallopLeft(work, temp[cursorTemp], cursorB, lengthB, 0)
				if nofWinsB > 0 {
					copy(work[dest:dest+nofWinsB], work[cursorB:cursorB+nofWinsB])

					dest = dest + nofWinsB
					cursorB = cursorB + nofWinsB
					lengthB = lengthB - nofWinsB

					if lengthB == 0 {
						return v8MergeSucceed
					}
				}
				work[dest] = temp[cursorTemp]
				dest++
				cursorTemp++
				lengthA--
				if lengthA == 1 {
					return v8MergeCopyRest
				}
			}
			minGallop++ // penalty for leaving galloping mode
			st.minGallop = minGallop
		}
	}()

	if exit == v8MergeSucceed {
		if lengthA > 0 {
			copy(work[dest:dest+lengthA], temp[cursorTemp:cursorTemp+lengthA])
		}
		return
	}
	// CopyB: the last element of run A belongs at the end of the merge.
	copy(work[dest:dest+lengthB], work[cursorB:cursorB+lengthB])
	work[dest+lengthB] = temp[cursorTemp]
}

// mergeHigh merges the run of lengthA elements at baseA with the run of
// lengthB elements at baseB. It copies run B to the temp array and merges
// from right to left.
func (st *v8SortState[T]) mergeHigh(baseA, lengthAArg, baseB, lengthBArg int) {
	lengthA := lengthAArg
	lengthB := lengthBArg

	work := st.work
	temp := st.getTempArray(lengthB)
	copy(temp[:lengthB], work[baseB:baseB+lengthB])

	dest := baseB + lengthB - 1
	cursorTemp := lengthB - 1
	cursorA := baseA + lengthA - 1

	work[dest] = work[cursorA]
	dest--
	cursorA--

	exit := func() int {
		lengthA--
		if lengthA == 0 {
			return v8MergeSucceed
		}
		if lengthB == 1 {
			return v8MergeCopyRest
		}

		minGallop := st.minGallop
		for {
			nofWinsA := 0 // number of times A won in a row
			nofWinsB := 0 // number of times B won in a row

			for {
				order := st.compare(temp[cursorTemp], work[cursorA])
				if order < 0 {
					work[dest] = work[cursorA]
					dest--
					cursorA--

					nofWinsA++
					lengthA--
					nofWinsB = 0

					if lengthA == 0 {
						return v8MergeSucceed
					}
					if nofWinsA >= minGallop {
						break
					}
				} else {
					work[dest] = temp[cursorTemp]
					dest--
					cursorTemp--

					nofWinsB++
					lengthB--
					nofWinsA = 0

					if lengthB == 1 {
						return v8MergeCopyRest
					}
					if nofWinsB >= minGallop {
						break
					}
				}
			}

			minGallop++
			firstIteration := true
			for nofWinsA >= v8MinGallopWins || nofWinsB >= v8MinGallopWins || firstIteration {
				firstIteration = false

				minGallop = max(1, minGallop-1)
				st.minGallop = minGallop

				k := st.gallopRight(work, temp[cursorTemp], baseA, lengthA, lengthA-1)
				nofWinsA = lengthA - k

				if nofWinsA > 0 {
					dest = dest - nofWinsA
					cursorA = cursorA - nofWinsA
					copy(work[dest+1:dest+1+nofWinsA], work[cursorA+1:cursorA+1+nofWinsA])

					lengthA = lengthA - nofWinsA
					if lengthA == 0 {
						return v8MergeSucceed
					}
				}
				work[dest] = temp[cursorTemp]
				dest--
				cursorTemp--
				lengthB--
				if lengthB == 1 {
					return v8MergeCopyRest
				}

				k = st.gallopLeft(temp, work[cursorA], 0, lengthB, lengthB-1)
				nofWinsB = lengthB - k

				if nofWinsB > 0 {
					dest = dest - nofWinsB
					cursorTemp = cursorTemp - nofWinsB
					copy(work[dest+1:dest+1+nofWinsB], temp[cursorTemp+1:cursorTemp+1+nofWinsB])

					lengthB = lengthB - nofWinsB
					if lengthB == 1 {
						return v8MergeCopyRest
					}
					// lengthB == 0 is possible when the comparator is not consistent.
					if lengthB == 0 {
						return v8MergeSucceed
					}
				}
				work[dest] = work[cursorA]
				dest--
				cursorA--
				lengthA--
				if lengthA == 0 {
					return v8MergeSucceed
				}
			}
			minGallop++
			st.minGallop = minGallop
		}
	}()

	if exit == v8MergeSucceed {
		if lengthB > 0 {
			copy(work[dest-(lengthB-1):dest+1], temp[:lengthB])
		}
		return
	}
	// CopyA: the first element of run B belongs at the front of the merge.
	dest = dest - lengthA
	cursorA = cursorA - lengthA
	copy(work[dest+1:dest+1+lengthA], work[cursorA+1:cursorA+1+lengthA])
	work[dest] = temp[cursorTemp]
}

// v8ComputeMinRunLength is the TimSort minimum run length.
func v8ComputeMinRunLength(nArg int) int {
	n := nArg
	r := 0 // becomes 1 if any 1 bits are shifted off
	for n >= 64 {
		r = r | (n & 1)
		n = n >> 1
	}
	return n + r
}

// runInvariantEstablished reports whether
// run_length(n - 2) > run_length(n - 1) + run_length(n).
func (st *v8SortState[T]) runInvariantEstablished(n int) bool {
	if n < 2 {
		return true
	}
	return st.runLength[n-2] > st.runLength[n-1]+st.runLength[n]
}

// mergeCollapse merges adjacent runs until the TimSort stack invariants hold.
func (st *v8SortState[T]) mergeCollapse() {
	for len(st.runBase) > 1 {
		n := len(st.runBase) - 2

		if !st.runInvariantEstablished(n+1) || !st.runInvariantEstablished(n) {
			if n > 0 && st.runLength[n-1] < st.runLength[n+1] {
				n--
			}
			st.mergeAt(n)
		} else if st.runLength[n] <= st.runLength[n+1] {
			st.mergeAt(n)
		} else {
			break
		}
	}
}

// mergeForceCollapse merges all runs on the stack until one remains.
func (st *v8SortState[T]) mergeForceCollapse() {
	for len(st.runBase) > 1 {
		n := len(st.runBase) - 2
		if n > 0 && st.runLength[n-1] < st.runLength[n+1] {
			n--
		}
		st.mergeAt(n)
	}
}

func (st *v8SortState[T]) timSortImpl(length int) {
	if length < 2 {
		return
	}
	remaining := length

	// March over the array once, left to right. Find natural runs and
	// extend short natural runs to minRunLength elements.
	low := 0
	minRunLength := v8ComputeMinRunLength(remaining)
	for remaining != 0 {
		currentRunLength := st.countAndMakeRun(low, low+remaining)

		if currentRunLength < minRunLength {
			forcedRunLength := min(minRunLength, remaining)
			st.binaryInsertionSort(low, low+currentRunLength, low+forcedRunLength)
			currentRunLength = forcedRunLength
		}

		st.pushRun(low, currentRunLength)
		st.mergeCollapse()

		low = low + currentRunLength
		remaining = remaining - currentRunLength
	}

	st.mergeForceCollapse()
}

// v8NodePower is the depth of the boundary between two adjacent runs in an
// optimal binary merge tree. The first run starts at s1 and has n1
// elements, the second has n2 elements, and n is the array length. This is
// the 64-bit path of the Torque macro.
func v8NodePower(s1, n1, n2, n int) int {
	a := int64(2*s1 + n1)
	b := a + int64(n1) + int64(n2)
	aScaled := (a << 30) / int64(n)
	bScaled := (b << 30) / int64(n)
	a32 := uint32(int32(aScaled))
	b32 := uint32(int32(bScaled))
	return bits.LeadingZeros32(a32 ^ b32)
}

func (st *v8SortState[T]) powerSortImpl(length int) {
	if length < 2 {
		return
	}
	remaining := length

	// The minimum run lengths are balanced: each is floor(length/2^e) or
	// ceil(length/2^e), and they sum to length (minrun_next in CPython).
	mrStep := length
	mrMask := 0
	for mrStep >= 64 {
		mrStep = mrStep >> 1
		mrMask = (mrMask << 1) | 1
	}
	mrRemainder := length & mrMask
	mrThreshold := mrMask + 1
	mrAccum := 0

	low := 0
	for remaining != 0 {
		mrAccum = mrAccum + mrRemainder
		minRunLength := mrStep
		if mrAccum >= mrThreshold {
			mrAccum = mrAccum - mrThreshold
			minRunLength = minRunLength + 1
		}
		currentRunLength := st.countAndMakeRun(low, low+remaining)

		if currentRunLength < minRunLength {
			forcedRunLength := min(minRunLength, remaining)
			st.binaryInsertionSort(low, low+currentRunLength, low+forcedRunLength)
			currentRunLength = forcedRunLength
		}

		stackSize := len(st.runBase)
		if stackSize == 0 {
			st.pushRun(low, currentRunLength)
		} else {
			prevRunBase := st.runBase[stackSize-1]
			prevRunLen := st.runLength[stackSize-1]

			p := v8NodePower(prevRunBase, prevRunLen, currentRunLength, length)

			for stackSize > 1 && st.power[stackSize-2] > p {
				st.mergeAt(stackSize - 2)
				stackSize = len(st.runBase)
			}

			st.power[stackSize-1] = p
			st.pushRun(low, currentRunLength)
		}

		low = low + currentRunLength
		remaining = remaining - currentRunLength
	}

	// Merge the remaining runs.
	for len(st.runBase) > 1 {
		n := len(st.runBase) - 2
		if n > 0 && st.runLength[n-1] < st.runLength[n+1] {
			n--
		}
		st.mergeAt(n)
	}
}
