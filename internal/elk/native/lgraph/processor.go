// Ported from Eclipse ELK 0.9.1, in part: the interfaces of
// plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/alg/ILayoutProcessor.java
// and
// plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/IHierarchyAwareLayoutProcessor.java
// (EPL-2.0). The registry of the ported processors and the state of one
// layout port no upstream file.
//
// The processor interface of the layered algorithm
// (org.eclipse.elk.core.alg.ILayoutProcessor with an LGraph), the registry of
// the ported processors, and the state of one layout.

package lgraph

import (
	"fmt"
	"sort"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
)

// Processor is ILayoutProcessor<LGraph>: one step of the layered algorithm.
// A phase of the algorithm (ILayoutPhase) is a Processor too.
//
// Process changes the graph in place. It must be a faithful port: a later
// processor reads each field and each property that this one leaves,
// including the id fields.
//
// Where the Java code throws an exception, and where the port does not cover
// a case, call Bail. The layout then stops and the caller uses elkjs.
type Processor interface {
	Process(graph *LGraph, ctx *Context)
}

// HierarchyAware is IHierarchyAwareLayoutProcessor: a processor that runs one
// time, for the root graph, and handles the nested graphs itself.
type HierarchyAware interface {
	Processor
	HierarchyAware()
}

// Factory makes a new processor. The layered algorithm makes one processor
// of each kind for one layout run and uses it for each graph of a hierarchy,
// as the AlgorithmAssembler of ELK does with its cache.
type Factory func() Processor

var registry = map[string]Factory{}

// Register makes a processor known under the simple name of its Java class,
// for example "LongEdgeSplitter". Call it from an init function of the
// package that ports the processor.
func Register(name string, f Factory) {
	if _, dup := registry[name]; dup {
		panic("lgraph: processor " + name + " is registered twice")
	}
	registry[name] = f
}

// Lookup returns the factory of a processor, or nil when no linked package
// ports it.
func Lookup(name string) Factory { return registry[name] }

// Registered returns the names of the ported processors in order.
func Registered() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// FixedNow is the value of Date.now() that the JavaScript bridge of
// internal/elk gives to elkjs. A random generator without a seed reads it.
const FixedNow = 1700000000000

// Context is the state of one layout. elkjs keeps this state in global
// variables. A Context is not safe for concurrent use: each layout has its
// own.
type Context struct {
	// RandomCount is the number of java.util.Random objects that the layout
	// made without a seed (uniqueSeed of GWT, counted from the start of the
	// layout).
	RandomCount int
}

// NewContext returns the state for a new layout.
func NewContext() *Context { return &Context{} }

// NewRandom is new Random(): a generator whose seed is the clock plus the
// number of generators made before. The bridge fixes the clock, so the k-th
// generator of a layout has the seed FixedNow + k.
func (c *Context) NewRandom() *gwt.Random {
	r := gwt.NewRandomFromDouble(float64(FixedNow + c.RandomCount))
	c.RandomCount++
	return r
}

// BailError is the panic value of Bail.
type BailError struct {
	Reason string
}

func (e *BailError) Error() string { return "elk native: " + e.Reason }

// Bail stops the layout. Use it where Java throws an exception, and for each
// case that the port does not cover. native.Layout recovers and reports that
// it did not lay out the graph, so the caller uses elkjs, which gives the
// exact result or the exact error.
func Bail(format string, args ...any) {
	panic(&BailError{Reason: fmt.Sprintf(format, args...)})
}
