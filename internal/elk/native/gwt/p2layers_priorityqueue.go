// Ported from java.util.PriorityQueue of the GWT runtime in elk-worker.js of
// elkjs 0.9.3 ($offer, $poll_0, $removeAtIndex_0, $mergeHeaps, $addAll_4,
// $makeHeap) (Apache-2.0).
//
// GWT has the Apache License 2.0. elk-worker.js has no notice for this
// code. The Java source of GWT for it is
// user/super/com/google/gwt/emul/java/util/PriorityQueue.java. Its
// copyright line, read from the main branch of
// https://github.com/gwtproject/gwt on 2026-11-09:
//
//	Copyright 2008 Google Inc.
//
// This file is a Go translation of that code. It is a changed version of
// it. LICENSE-GWT.txt in this directory has the license, and NOTICE in this
// directory has the license notice of the GWT files.

package gwt

// PriorityQueue is java.util.PriorityQueue of GWT: a binary heap in a list,
// with a comparator. The element that the comparator puts first is at the
// head.
//
// The order of elements that the comparator calls equal depends on the heap
// operations. The JVM class has another heap algorithm, so ported code must
// use this type.
type PriorityQueue[T any] struct {
	heap []T
	cmp  func(a, b T) int
}

// NewPriorityQueue is new PriorityQueue(comparator).
func NewPriorityQueue[T any](cmp func(a, b T) int) *PriorityQueue[T] {
	return &PriorityQueue[T]{cmp: cmp}
}

// Len is size().
func (q *PriorityQueue[T]) Len() int { return len(q.heap) }

// IsEmpty is isEmpty().
func (q *PriorityQueue[T]) IsEmpty() bool { return len(q.heap) == 0 }

// Clear is clear().
func (q *PriorityQueue[T]) Clear() { q.heap = q.heap[:0] }

// Offer is offer(e) and add(e).
func (q *PriorityQueue[T]) Offer(e T) bool {
	node := len(q.heap)
	q.heap = append(q.heap, e)
	for node > 0 {
		childNode := node
		node = (node - 1) / 2
		if q.cmp(q.heap[node], e) <= 0 {
			q.heap[childNode] = e
			return true
		}
		q.heap[childNode] = q.heap[node]
	}
	q.heap[node] = e
	return true
}

// AddAll is addAll(c): it adds the elements at the end of the list and makes
// the heap again.
func (q *PriorityQueue[T]) AddAll(c []T) bool {
	oldSize := len(q.heap)
	q.heap = append(q.heap, c...)
	if oldSize != len(q.heap) {
		q.makeHeap(0)
		return true
	}
	return false
}

// Peek is peek(). ok is false for an empty queue.
func (q *PriorityQueue[T]) Peek() (head T, ok bool) {
	if len(q.heap) == 0 {
		return head, false
	}
	return q.heap[0], true
}

// Poll is poll(): it removes the head and returns it. ok is false for an
// empty queue.
func (q *PriorityQueue[T]) Poll() (head T, ok bool) {
	if len(q.heap) == 0 {
		return head, false
	}
	head = q.heap[0]
	q.removeAtIndex(0)
	return head, true
}

// ToArray is toArray(): the elements in the order of the heap list.
func (q *PriorityQueue[T]) ToArray() []T {
	return append([]T(nil), q.heap...)
}

func (q *PriorityQueue[T]) makeHeap(node int) {
	if node*2+1 >= len(q.heap) {
		return
	}
	q.makeHeap(2*node + 1)
	rightChild := 2*node + 2
	if rightChild < len(q.heap) {
		q.makeHeap(rightChild)
	}
	q.mergeHeaps(node)
}

func (q *PriorityQueue[T]) mergeHeaps(node int) {
	heapSize := len(q.heap)
	value := q.heap[node]
	for node*2+1 < heapSize {
		leftChild := 2*node + 1
		rightChild := leftChild + 1
		smallestChild := leftChild
		if rightChild < heapSize && q.cmp(q.heap[rightChild], q.heap[leftChild]) < 0 {
			smallestChild = rightChild
		}
		if q.cmp(value, q.heap[smallestChild]) < 0 {
			break
		}
		q.heap[node] = q.heap[smallestChild]
		node = smallestChild
	}
	q.heap[node] = value
}

func (q *PriorityQueue[T]) removeAtIndex(index int) {
	last := len(q.heap) - 1
	lastValue := q.heap[last]
	var zero T
	q.heap[last] = zero
	q.heap = q.heap[:last]
	if index < len(q.heap) {
		q.heap[index] = lastValue
		q.mergeHeaps(index)
	}
}
