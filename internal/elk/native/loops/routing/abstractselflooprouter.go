// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/loops/routing/AbstractSelfLoopRouter.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

// Package routing has the classes that route the self loops of a node
// (org.eclipse.elk.alg.layered.intermediate.loops.routing), and the
// processor SelfLoopRouter that uses them.
//
// Only the orthogonal self loop router is ported. The layout stops for a
// graph with self loops whose edge routing is POLYLINE or SPLINES.
package routing

import "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/loops"

// AbstractSelfLoopRouter is the class AbstractSelfLoopRouter: a router of
// self loops.
type AbstractSelfLoopRouter interface {
	// RouteSelfLoops is routeSelfLoops(slHolder): it routes the self loops
	// of the holder. The loop routes and the routing slots are known.
	RouteSelfLoops(slHolder *loops.SelfLoopHolder)
}
