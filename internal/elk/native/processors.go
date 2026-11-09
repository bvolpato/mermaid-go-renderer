// This file ports no upstream file. It links the processor packages into the
// native layout. A processor registers itself in an init function, so a
// blank import is enough.

package native

import (
	_ "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/compound"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/elknodespacing" // sets layered.NodeSpacing
	_ "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/hports"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/intermediate"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/labels"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/loops"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/loops/ordering"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/loops/routing"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodesize"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/p1cycles"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/p2layers"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/p3order"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/p4nodes"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/p4nodes/bk"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/p5edges"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/preserveorder"
)
