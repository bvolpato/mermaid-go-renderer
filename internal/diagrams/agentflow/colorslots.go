// Ported from mermaid 12.2.0, diagrams/agentflow/colorSlots.ts (MIT).

package agentflow

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// kinds is KINDS: the node kinds in the order of KIND_SLOT. The position of
// a kind is its palette slot, so "tool" is always the first palette colour,
// "task" the second, and so on.
var kinds = []string{"tool", "task", "decision", "input", "refdoc", "connector", "action"}

// kindSlot is KIND_SLOT: the palette slot of each kind.
var kindSlot = func() map[string]int {
	m := map[string]int{}
	for i, kind := range kinds {
		m[kind] = i
	}
	return m
}()

// kindCount is KIND_COUNT: the number of slots that the kinds take. The
// containers start above it.
var kindCount = len(kinds)

// kindClass is kindClass(kind): the class that marks the kind of a node.
func kindClass(kind string) string {
	return "af-kind-" + kind
}

// containerSlotCount is containerSlotCount(paletteLength): the number of
// slots that the containers cycle through. It is at least one.
func containerSlotCount(paletteLength int) int {
	return max(1, paletteLength-kindCount)
}

// containerSlot is containerSlot(n, paletteLength): the palette slot of
// the nth container.
func containerSlot(n, paletteLength int) int {
	slot := kindCount + (n % containerSlotCount(paletteLength))
	if paletteLength > 0 {
		return slot % paletteLength
	}
	return slot
}

// collapseSpaces is s.replace(/\s+/g, ' ').trim().
func collapseSpaces(s string) string {
	var b strings.Builder
	inSpace := false
	for _, r := range s {
		if js.IsWhiteSpace(r) {
			inSpace = true
			continue
		}
		if inSpace {
			b.WriteByte(' ')
			inSpace = false
		}
		b.WriteRune(r)
	}
	if inSpace {
		b.WriteByte(' ')
	}
	return js.TrimSpace(b.String())
}

// assignColorSlots is assignColorSlots(nodes, kindOf, containerOrder): it
// adds the class of its kind to every node, and gives every container its
// palette slot. kindOf returns "" when the id has no kind. containerOrder
// has the order in which the containers are declared.
func assignColorSlots(config *js.Obj[any], nodes []*ru.Node, kindOf func(id string) string, containerOrder *js.Map[string, int]) {
	paletteLength := 0
	if palette, ok := js.Path(config, "themeVariables", "borderColorArray").([]any); ok {
		paletteLength = len(palette)
	}

	// Containers that the walk did not reach still need a slot, and they must not all take
	// the same one. Counted after the declared ones so nothing shifts.
	fallbackOrdinal := containerOrder.Len()
	for _, node := range nodes {
		// A collapsed container is drawn as a single node (`isGroup` is false) but it is still
		// a container, and it takes its slot like one -- so collapsing a container does not
		// reshuffle the colours of the ones after it. Flowchart does the same for a collapsed
		// subgraph.
		if node.IsGroup || node.Shape == "collapsedGroup" {
			n, ok := containerOrder.Lookup(node.ID)
			if !ok {
				n = fallbackOrdinal
				fallbackOrdinal++
			}
			node.ColorIndex = float64(containerSlot(n, paletteLength))
			continue
		}
		kind := kindOf(node.ID)
		if _, ok := kindSlot[kind]; kind != "" && ok {
			// `${node.cssClasses ?? ''} ${kindClass(kind)}`
			classes := ""
			if node.CSSClassesSet {
				classes = node.CSSClasses
			}
			node.CSSClasses, node.CSSClassesSet = collapseSpaces(classes+" "+kindClass(kind)), true
		}
	}
}
