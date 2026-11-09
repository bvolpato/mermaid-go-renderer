// Ported from mermaid 12.2.0, rendering-util/layout-algorithms/elk/index.ts
// (MIT).
//
// index.ts exports render of render.ts: the function that mermaid loads for
// an ELK layout. Here it is Render of render.go, which plugin.go registers
// with ru.RegisterLayout for `elk` and for each name of ELK_ALGORITHMS.

package elk
