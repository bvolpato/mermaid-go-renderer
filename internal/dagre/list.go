// Ported from dagre-d3-es 7.0.14, dagre-d3-es/src/dagre/data/list.js (MIT).

package dagre

// fasEntry is a node of the graph that greedyFAS works on. It is also the
// entry of the doubly linked list: the JavaScript code stores the links
// _next and _prev on the same object.
type fasEntry struct {
	v   string
	in  float64
	out float64

	next *fasEntry
	prev *fasEntry
}

// list is a doubly linked list with a sentinel.
type list struct {
	sentinel *fasEntry
}

func newList() *list {
	sentinel := &fasEntry{}
	sentinel.next = sentinel
	sentinel.prev = sentinel
	return &list{sentinel: sentinel}
}

// dequeue removes and returns the oldest entry. It returns nil for an
// empty list.
func (l *list) dequeue() *fasEntry {
	sentinel := l.sentinel
	entry := sentinel.prev
	if entry != sentinel {
		unlink(entry)
		return entry
	}
	return nil
}

// enqueue adds an entry. It first removes the entry from the list that
// holds it.
func (l *list) enqueue(entry *fasEntry) {
	sentinel := l.sentinel
	if entry.prev != nil && entry.next != nil {
		unlink(entry)
	}
	entry.next = sentinel.next
	sentinel.next.prev = entry
	sentinel.next = entry
	entry.prev = sentinel
}

func unlink(entry *fasEntry) {
	entry.prev.next = entry.next
	entry.next.prev = entry.prev
	entry.next = nil
	entry.prev = nil
}
