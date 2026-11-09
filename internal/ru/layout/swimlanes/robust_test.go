package swimlanes

import (
	"errors"
	"math"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// mutateLayoutData changes layout data at random places: sizes that are
// undefined, zero, negative, huge or infinite, parents that form cycles or
// are no groups, duplicate ids, edges to missing nodes, and so on.
func mutateLayoutData(rnd *rand.Rand, data *ru.LayoutData) {
	weird := []float64{ru.Undef, 0, -1, -50, 1e-9, 1e9, 1e300, math.Inf(1), math.Inf(-1), 0.5, 3}
	pickNum := func() float64 { return weird[rnd.Intn(len(weird))] }
	ids := []string{"", "no-such-node", DEFAULT_SWIMLANE_ID, "constructor", "__proto__", "null"}
	for _, n := range data.Nodes {
		ids = append(ids, n.ID)
	}
	pickID := func() string { return ids[rnd.Intn(len(ids))] }

	edits := 1 + rnd.Intn(6)
	for i := 0; i < edits; i++ {
		switch rnd.Intn(16) {
		case 0, 1:
			if len(data.Nodes) > 0 {
				n := data.Nodes[rnd.Intn(len(data.Nodes))]
				n.Width, n.Height = pickNum(), pickNum()
			}
		case 2:
			if len(data.Nodes) > 0 {
				n := data.Nodes[rnd.Intn(len(data.Nodes))]
				n.X, n.Y = pickNum(), pickNum()
			}
		case 3, 4:
			if len(data.Nodes) > 0 {
				data.Nodes[rnd.Intn(len(data.Nodes))].ParentID = pickID()
			}
		case 5:
			if len(data.Nodes) > 0 {
				data.Nodes[rnd.Intn(len(data.Nodes))].ID = pickID()
			}
		case 6:
			if len(data.Nodes) > 0 {
				n := data.Nodes[rnd.Intn(len(data.Nodes))]
				n.IsGroup = !n.IsGroup
			}
		case 7:
			if len(data.Nodes) > 0 {
				data.Nodes[rnd.Intn(len(data.Nodes))].Padding = pickNum()
			}
		case 8, 9:
			if len(data.Edges) > 0 {
				e := data.Edges[rnd.Intn(len(data.Edges))]
				if rnd.Intn(2) == 0 {
					e.Start = pickID()
				} else {
					e.End = pickID()
				}
			}
		case 10:
			if len(data.Edges) > 0 {
				data.Edges[rnd.Intn(len(data.Edges))].ID = pickID()
			}
		case 11:
			e := ru.NewEdge("extra-" + pickID())
			e.Start, e.End = pickID(), pickID()
			if rnd.Intn(2) == 0 {
				e.Label, e.HasLabel = "label", true
			}
			e.LabelNodeID = []string{"", "", pickID()}[rnd.Intn(3)]
			e.IsLayoutOnly = rnd.Intn(5) == 0
			data.Edges = append(data.Edges, e)
		case 12:
			data.Direction = []string{"", "TB", "LR", "BT", "RL", "TD", "sideways"}[rnd.Intn(7)]
		case 13:
			flowchart := js.NewObj[any]()
			flowchart.Set("nodeSpacing", []any{0.0, -20.0, math.NaN(), 1e9, "50", nil, true}[rnd.Intn(7)])
			flowchart.Set("rankSpacing", []any{0.0, -20.0, math.NaN(), 1e9, "50", nil, true}[rnd.Intn(7)])
			data.Config.Set("flowchart", flowchart)
		case 14:
			swimlane := js.NewObj[any]()
			swimlane.Set("ignoreCrossLaneEdges", rnd.Intn(2) == 0)
			swimlane.Set("optimizeRanksByCrossings", rnd.Intn(2) == 0)
			swimlane.Set("automaticLaneOrdering", rnd.Intn(2) == 0)
			data.Config.Set("swimlane", swimlane)
		case 15:
			if len(data.Nodes) > 1 {
				a, b := rnd.Intn(len(data.Nodes)), rnd.Intn(len(data.Nodes))
				data.Nodes[a], data.Nodes[b] = data.Nodes[b], data.Nodes[a]
			}
		}
	}
}

// TestMutatedLayoutData feeds mutated layout data to the prepare phase and
// to the layout core. The requirement is: no panic and no hang. An error
// that upstream throws too (a jsError) is a good result. A panic of the
// layout code is turned into an error by the layout core; the test counts
// these, because each one is layout data that upstream may handle.
func TestMutatedLayoutData(t *testing.T) {
	records := loadGolden(t)
	mutantsPerRecord := 2
	if testing.Short() {
		var subset []*goldenRecord
		for i, record := range records {
			if i%4 == 0 {
				subset = append(subset, record)
			}
		}
		records = subset
		mutantsPerRecord = 1
	}
	type job struct {
		name string
		data *ru.LayoutData
	}
	rnd := rand.New(rand.NewSource(0x73776c))
	var jobs []job
	for _, record := range records {
		if record.Raw == nil {
			continue
		}
		for i := 0; i < mutantsPerRecord; i++ {
			data := buildLayoutData(record.Raw)
			// The raw data has no sizes: give every node one first.
			for _, n := range data.Nodes {
				if !n.IsGroup {
					n.Width = 20 + float64(rnd.Intn(150))
					n.Height = 20 + float64(rnd.Intn(60))
				}
			}
			mutateLayoutData(rnd, data)
			jobs = append(jobs, job{name: record.Name, data: data})
		}
	}

	type outcome struct {
		problem  string
		internal string
	}
	outcomes := make([]outcome, len(jobs))
	var wg sync.WaitGroup
	slots := make(chan struct{}, runtime.GOMAXPROCS(0))
	for i, j := range jobs {
		wg.Add(1)
		slots <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			done := make(chan outcome, 1)
			go func() {
				var o outcome
				defer func() {
					if r := recover(); r != nil {
						o.problem = "panic"
					}
					done <- o
				}()
				if _, err := prepareSwimlaneLayout(j.data, nil); err != nil {
					return
				}
				// The measure phase gives the label nodes a size.
				for _, n := range j.data.Nodes {
					if n.IsEdgeLabel && n.Width == 0 {
						n.Width, n.Height = 30, 21
					}
				}
				_, err := runSwimlaneLayoutCore(j.data)
				var internal *internalError
				if errors.As(err, &internal) {
					o.internal = internal.Error() + "\n" + string(internal.stack)
				}
			}()
			select {
			case o := <-done:
				outcomes[i] = o
			case <-time.After(120 * time.Second):
				outcomes[i] = outcome{problem: "no result after 120 seconds"}
			}
		}()
	}
	wg.Wait()

	problems, internals := 0, 0
	seen := map[string]bool{}
	for i, o := range outcomes {
		if o.problem != "" {
			problems++
			if problems <= 5 {
				t.Errorf("%s (mutant %d): %s", jobs[i].name, i, o.problem)
			}
		}
		if o.internal != "" {
			internals++
			// One report per panic site.
			key := o.internal
			if len(key) > 400 {
				key = key[:400]
			}
			if !seen[key] && len(seen) < 6 {
				seen[key] = true
				t.Errorf("%s (mutant %d): %s", jobs[i].name, i, o.internal)
			}
		}
	}
	t.Logf("%d mutants, %d panics or hangs, %d internal errors", len(jobs), problems, internals)
}
