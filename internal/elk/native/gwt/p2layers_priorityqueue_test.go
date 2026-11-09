package gwt

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// pqElem is an element of the golden runs: the comparator sees the key only.
type pqElem struct {
	key, id int
}

// TestPriorityQueueGolden replays the runs of
// testdata/p2layers_priorityqueue.json, which
// tools/reference/gen-elk-gwt-priorityqueue.mjs writes from the
// PriorityQueue of elk-worker.js. It compares each polled element and the
// heap list after each operation.
func TestPriorityQueueGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/p2layers_priorityqueue.json")
	if err != nil {
		t.Fatal(err)
	}
	var runs []struct {
		Seed    int                 `json:"seed"`
		Ops     [][]json.RawMessage `json:"ops"`
		Results [][]json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal(data, &runs); err != nil {
		t.Fatal(err)
	}
	if len(runs) < 100 {
		t.Fatalf("the golden file has %d runs", len(runs))
	}
	operations := 0
	for _, run := range runs {
		q := NewPriorityQueue(func(x, y pqElem) int { return IntCompare(x.key, y.key) })
		for i, op := range run.Ops {
			operations++
			var kind string
			if err := json.Unmarshal(op[0], &kind); err != nil {
				t.Fatal(err)
			}
			polled := -2
			switch kind {
			case "o":
				var e pqElem
				mustUnmarshal(t, op[1], &e.key)
				mustUnmarshal(t, op[2], &e.id)
				q.Offer(e)
			case "a":
				var items [][2]int
				mustUnmarshal(t, op[1], &items)
				list := make([]pqElem, len(items))
				for k, it := range items {
					list[k] = pqElem{it[0], it[1]}
				}
				q.AddAll(list)
			case "p":
				head, okPeek := q.Peek()
				e, ok := q.Poll()
				if ok != okPeek || head != e {
					t.Fatalf("seed %d, operation %d: Peek gives %v %v, Poll gives %v %v", run.Seed, i, head, okPeek, e, ok)
				}
				polled = -1
				if ok {
					polled = e.id
				}
			case "c":
				q.Clear()
			default:
				t.Fatalf("unknown operation %q", kind)
			}
			var wantPolled *int
			var wantHeap []int
			mustUnmarshal(t, run.Results[i][0], &wantPolled)
			mustUnmarshal(t, run.Results[i][1], &wantHeap)
			if wantPolled != nil && *wantPolled != polled {
				t.Fatalf("seed %d, operation %d (%s): polled %d, want %d", run.Seed, i, kind, polled, *wantPolled)
			}
			heap := []int{}
			for _, e := range q.ToArray() {
				heap = append(heap, e.id)
			}
			if !reflect.DeepEqual(heap, wantHeap) {
				t.Fatalf("seed %d, operation %d (%s): heap %v, want %v", run.Seed, i, kind, heap, wantHeap)
			}
			if q.Len() != len(wantHeap) || q.IsEmpty() != (len(wantHeap) == 0) {
				t.Fatalf("seed %d, operation %d (%s): Len %d, IsEmpty %v, want %d elements", run.Seed, i, kind, q.Len(), q.IsEmpty(), len(wantHeap))
			}
		}
	}
	t.Logf("%d runs, %d operations", len(runs), operations)
}

func mustUnmarshal(t *testing.T, data json.RawMessage, v any) {
	t.Helper()
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatal(err)
	}
}
