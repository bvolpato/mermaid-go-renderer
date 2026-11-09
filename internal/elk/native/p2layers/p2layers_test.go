package p2layers

import (
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nativetest"
)

func TestNetworkSimplexLayerer(t *testing.T) {
	nativetest.RunProcessor(t, "NetworkSimplexLayerer", func() lgraph.Processor { return NewNetworkSimplexLayerer() })
}

func TestLongestPathLayerer(t *testing.T) {
	nativetest.RunProcessor(t, "LongestPathLayerer", func() lgraph.Processor { return NewLongestPathLayerer() })
}

func TestLongestPathSourceLayerer(t *testing.T) {
	nativetest.RunProcessor(t, "LongestPathSourceLayerer", func() lgraph.Processor { return NewLongestPathSourceLayerer() })
}

func TestCoffmanGrahamLayerer(t *testing.T) {
	nativetest.RunProcessor(t, "CoffmanGrahamLayerer", func() lgraph.Processor { return NewCoffmanGrahamLayerer() })
}

func TestMinWidthLayerer(t *testing.T) {
	nativetest.RunProcessor(t, "MinWidthLayerer", func() lgraph.Processor { return NewMinWidthLayerer() })
}
