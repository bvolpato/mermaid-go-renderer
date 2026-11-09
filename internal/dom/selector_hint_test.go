package dom

import (
	"math/rand"
	"testing"
)

// TestAncestorClassIsNeeded checks the promise of Selector.AncestorClass:
// when a selector matches an element, an ancestor of the element has the
// class. The style cascade leaves out the selectors whose class no ancestor
// has, so a wrong class would lose a rule.
func TestAncestorClassIsNeeded(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	compounds := []string{".a", ".b", ".c", "g", "rect", "g.a", "rect.b", "#x", "[data-k]", ".a.b", "*", ":not(.a)",
		":is(.a, .b)", ".c:first-child", "[style]", ".a[style*=\"red\"]", ":not([style])", ".b:nth-of-type(2)"}
	combinators := []string{" ", " > ", " + ", " ~ ", " ", " "}
	tags := []string{"g", "rect", "g"}
	classes := []string{"a", "b", "c", "a b", "", "b c"}
	matches, hinted := 0, 0
	for i := 0; i < 3000; i++ {
		// A random selector of one to four compounds.
		text := compounds[rng.Intn(len(compounds))]
		for k := rng.Intn(4); k > 0; k-- {
			text += combinators[rng.Intn(len(combinators))] + compounds[rng.Intn(len(compounds))]
		}
		list, err := ParseSelector(text)
		if err != nil || len(list) != 1 {
			t.Fatalf("selector %q: %v", text, err)
		}
		sel := list[0]
		// A random tree.
		doc := NewDocument()
		nodes := []*Node{doc.Body.Append("svg")}
		for k := 0; k < 25; k++ {
			n := nodes[rng.Intn(len(nodes))].Append(tags[rng.Intn(len(tags))])
			if cl := classes[rng.Intn(len(classes))]; cl != "" {
				n.Attr("class", cl)
			}
			switch rng.Intn(6) {
			case 0:
				n.Attr("id", "x")
			case 1:
				n.Attr("data-k", "1")
			case 2:
				n.Style("fill", "red")
			}
			nodes = append(nodes, n)
		}
		class := sel.AncestorClass()
		if class != "" {
			hinted++
		}
		for _, n := range nodes {
			if !sel.Match(n) {
				continue
			}
			matches++
			if class == "" {
				continue
			}
			found := false
			for p := n.Parent; p != nil; p = p.Parent {
				if p.HasClass(class) {
					found = true
				}
			}
			if !found {
				t.Fatalf("selector %q matches <%s class=%q>, and no ancestor has the class %q", text, n.Tag, n.AttrOr("class", ""), class)
			}
		}
		// A selector that reads the style attribute has no class: Match
		// must run, because it creates the attribute.
		if sel.readsStyleAttr() && class != "" {
			t.Fatalf("selector %q reads the style attribute and has the ancestor class %q", text, class)
		}
	}
	if matches < 1000 || hinted < 300 {
		t.Errorf("%d matches, %d selectors with an ancestor class", matches, hinted)
	}
}
