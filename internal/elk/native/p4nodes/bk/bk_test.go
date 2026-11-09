package bk

import (
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/p4nodes/internal/p4test"
)

func TestBKNodePlacer(t *testing.T) {
	p4test.RunProcessor(t, "BKNodePlacer", 150, func() lgraph.Processor { return NewBKNodePlacer() })
}

func TestBKNodePlacerExtra(t *testing.T) {
	p4test.RunExtra(t, "testdata/bknodeplacer.jsonl.gz", "BKNodePlacer", 400, func() lgraph.Processor { return NewBKNodePlacer() })
}
