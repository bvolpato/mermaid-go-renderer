// Ported from dagre-d3-es 7.0.14,
// dagre-d3-es/src/graphlib/data/priority-queue.js (MIT).

package alg

import "fmt"

// priorityQueue is a min-priority queue. The heap layout of the JavaScript
// class is kept as it is (children of i at 2i and 2i+1, parent at i>>1),
// because the removal order of equal priorities depends on it.
type priorityQueue struct {
	arr        []pqEntry
	keyIndices map[string]int
}

type pqEntry struct {
	key      string
	priority float64
}

func newPriorityQueue() *priorityQueue {
	return &priorityQueue{keyIndices: map[string]int{}}
}

func (q *priorityQueue) size() int { return len(q.arr) }

// priority returns the priority of key. The result is false when the key is
// not in the queue.
func (q *priorityQueue) priority(key string) (float64, bool) {
	index, ok := q.keyIndices[key]
	if ok {
		return q.arr[index].priority, true
	}
	return 0, false
}

func (q *priorityQueue) add(key string, priority float64) bool {
	if _, ok := q.keyIndices[key]; !ok {
		index := len(q.arr)
		q.keyIndices[key] = index
		q.arr = append(q.arr, pqEntry{key: key, priority: priority})
		q.decreaseIndex(index)
		return true
	}
	return false
}

func (q *priorityQueue) removeMin() string {
	q.swap(0, len(q.arr)-1)
	min := q.arr[len(q.arr)-1]
	q.arr = q.arr[:len(q.arr)-1]
	delete(q.keyIndices, min.key)
	q.heapify(0)
	return min.key
}

func (q *priorityQueue) decrease(key string, priority float64) error {
	index, ok := q.keyIndices[key]
	if !ok {
		return fmt.Errorf("priority queue has no key %q", key)
	}
	if priority > q.arr[index].priority {
		return fmt.Errorf("New priority is greater than current priority. Key: %s Old: %v New: %v", key, q.arr[index].priority, priority)
	}
	q.arr[index].priority = priority
	q.decreaseIndex(index)
	return nil
}

func (q *priorityQueue) heapify(i int) {
	arr := q.arr
	l := 2 * i
	r := l + 1
	largest := i
	if l < len(arr) {
		if arr[l].priority < arr[largest].priority {
			largest = l
		}
		if r < len(arr) {
			if arr[r].priority < arr[largest].priority {
				largest = r
			}
		}
		if largest != i {
			q.swap(i, largest)
			q.heapify(largest)
		}
	}
}

func (q *priorityQueue) decreaseIndex(index int) {
	arr := q.arr
	priority := arr[index].priority
	for index != 0 {
		parent := index >> 1
		if arr[parent].priority < priority {
			break
		}
		q.swap(index, parent)
		index = parent
	}
}

func (q *priorityQueue) swap(i, j int) {
	arr := q.arr
	origArrI := arr[i]
	origArrJ := arr[j]
	arr[i] = origArrJ
	arr[j] = origArrI
	q.keyIndices[origArrJ.key] = i
	q.keyIndices[origArrI.key] = j
}
