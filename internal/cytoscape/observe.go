package cytoscape

import "sync/atomic"

// Observer sees the layouts of the process. The tests use it to record the
// inputs that real diagrams give to cytoscape. Each function gets the
// arguments of a call and returns the environment that the layout runs with:
// a wrapper of env can record what the layout reads. The functions must be
// safe for concurrent use.
type Observer struct {
	// CoseBilkent is called at the start of each call of CoseBilkent.
	CoseBilkent func(env Env, nodes []Node, edges []Edge) Env
	// Architecture is called at the start of each call of Architecture.
	Architecture func(env Env, in *ArchitectureInput) Env
}

var observer atomic.Pointer[Observer]

// SetObserver sets the observer of the process. nil ends the observation.
func SetObserver(o *Observer) { observer.Store(o) }
