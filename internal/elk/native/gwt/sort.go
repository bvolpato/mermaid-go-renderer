// Ported from java.util.Arrays of the GWT runtime in elk-worker.js of elkjs
// 0.9.3 (mergeSort, insertionSort, merge) (Apache-2.0).
//
// GWT has the Apache License 2.0. elk-worker.js has no notice for this
// code. The Java source of GWT for it is
// user/super/com/google/gwt/emul/java/util/Arrays.java. Its copyright line,
// read from the main branch of https://github.com/gwtproject/gwt on
// 2026-11-09:
//
//	Copyright 2008 Google Inc.
//
// This file is a Go translation of that code. It is a changed version of
// it. LICENSE-GWT.txt in this directory has the license, and NOTICE in this
// directory has the license notice of the GWT files.

package gwt

import "slices"

// SortFunc is Arrays.sort(T[], Comparator), List.sort and Collections.sort of
// GWT: a stable merge sort, with an insertion sort for fewer than 7 elements.
//
// Some comparators of ELK are not transitive. The result then depends on the
// order of the comparisons, so ported code must not use another sort.
func SortFunc[T any](a []T, cmp func(x, y T) int) {
	SortRangeFunc(a, 0, len(a), cmp)
}

// SortRangeFunc sorts a[from:to].
func SortRangeFunc[T any](a []T, from, to int, cmp func(x, y T) int) {
	if to-from < 2 {
		return
	}
	temp := make([]T, to-from)
	copy(temp, a[from:to])
	mergeSort(temp, a, from, to, -from, cmp)
}

func mergeSort[T any](temp, array []T, low, high, ofs int, cmp func(x, y T) int) {
	length := high - low
	if length < 7 {
		insertionSort(array, low, high, cmp)
		return
	}
	tempLow := low + ofs
	tempHigh := high + ofs
	tempMid := tempLow + (tempHigh-tempLow)>>1
	mergeSort(array, temp, tempLow, tempMid, -ofs, cmp)
	mergeSort(array, temp, tempMid, tempHigh, -ofs, cmp)
	if cmp(temp[tempMid-1], temp[tempMid]) <= 0 {
		for low < high {
			array[low] = temp[tempLow]
			low++
			tempLow++
		}
		return
	}
	merge(temp, tempLow, tempMid, tempHigh, array, low, high, cmp)
}

func insertionSort[T any](array []T, low, high int, cmp func(x, y T) int) {
	for i := low + 1; i < high; i++ {
		for j := i; j > low && cmp(array[j-1], array[j]) > 0; j-- {
			array[j], array[j-1] = array[j-1], array[j]
		}
	}
}

func merge[T any](src []T, srcLow, srcMid, srcHigh int, dest []T, destLow, destHigh int, cmp func(x, y T) int) {
	topIdx := srcMid
	for destLow < destHigh {
		if topIdx >= srcHigh || srcLow < srcMid && cmp(src[srcLow], src[topIdx]) <= 0 {
			dest[destLow] = src[srcLow]
			srcLow++
		} else {
			dest[destLow] = src[topIdx]
			topIdx++
		}
		destLow++
	}
}

// SortFloats is Arrays.sort(double[]): Array.prototype.sort with
// Double.compare. The sort of V8 is stable.
func SortFloats(a []float64) {
	slices.SortStableFunc(a, DoubleCompare)
}

// SortInts is Arrays.sort(int[]).
func SortInts(a []int) {
	slices.Sort(a)
}
