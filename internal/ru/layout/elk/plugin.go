// Ported from mermaid 12.2.0, rendering-util/layout-algorithms/elk/plugin.ts
// and the ELK part of rendering-util/render.ts (elkLayoutLoaders) (MIT).

package elk

import "github.com/bvolpato/mermaid-go-renderer/internal/ru"

// LayoutLoaderDefinition is one layout that mermaid registers: the name of
// the `layout` config and the ELK algorithm that it runs.
type LayoutLoaderDefinition struct {
	Name      string
	Algorithm string
}

// layouts returns the ELK layouts: `elk`, which is elk.layered, and each
// algorithm of ELK_ALGORITHMS under its own name. Every entry has the same
// render function.
func layouts() []LayoutLoaderDefinition {
	list := []LayoutLoaderDefinition{{Name: "elk", Algorithm: "elk.layered"}}
	for _, algorithm := range ELK_ALGORITHMS {
		list = append(list, LayoutLoaderDefinition{Name: algorithm, Algorithm: algorithm})
	}
	return list
}

func init() {
	for _, layout := range layouts() {
		ru.RegisterLayout(layout.Name, layout.Algorithm, Render)
	}
}
