package dom

import (
	"math/rand"
	"slices"
	"strings"
	"testing"
)

// TestClassAndIDLookups compares HasClass, ClassList, AppendClasses and ID
// with their definitions by GetAttr, for elements with the kinds of
// attributes that GetAttr tells apart.
func TestClassAndIDLookups(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	doc := NewDocument()
	values := []string{"", " ", "a", "a b", " a  b\tc\n", "a\fb\rc ", "é x", "a a", " a", "A", "x y"}
	names := []string{"", "a", "b", "c", "A", "é", "x", "a b", " a", "x y", "y"}
	for i := 0; i < 5000; i++ {
		var n *Node
		if rng.Intn(2) == 0 {
			n = doc.CreateElementNS(NSSVG, "g")
		} else {
			n = doc.CreateElement("div")
		}
		// Attributes in a random order, some with a prefix or a namespace.
		for k := rng.Intn(5); k > 0; k-- {
			value := values[rng.Intn(len(values))]
			switch rng.Intn(8) {
			case 0:
				n.Attr("class", value)
			case 1:
				n.Attr("id", value)
			case 2:
				n.Attr("xlink:class", value)
			case 3:
				n.SetAttributeRaw(NSXLink, "xlink", "id", value)
			case 4:
				n.SetAttributeRaw("", "", "CLASS", value)
			case 5:
				n.Attr("data-class", value)
			case 6:
				n.SetAttributeRaw("urn:x", "", "class", value)
			case 7:
				n.Attr("class", nil)
			}
		}
		class, hasClass := n.GetAttr("class")
		id, _ := n.GetAttr("id")
		if got := n.ID(); got != id {
			t.Fatalf("ID() = %q, GetAttr gives %q for %s", got, id, n.OuterHTML())
		}
		var wantList []string
		if hasClass {
			wantList = strings.FieldsFunc(class, isASCIISpace)
		}
		if got := n.ClassList(); !slices.Equal(got, wantList) || (got == nil) != (wantList == nil) {
			t.Fatalf("ClassList() = %q, want %q for %s", got, wantList, n.OuterHTML())
		}
		if got := n.AppendClasses(nil); !slices.Equal(got, wantList) {
			t.Fatalf("AppendClasses() = %q, want %q for %s", got, wantList, n.OuterHTML())
		}
		for _, name := range names {
			// The first definition of HasClass.
			want := false
			if hasClass {
				for v := class; len(v) > 0; {
					v = strings.TrimLeftFunc(v, isASCIISpace)
					end := strings.IndexFunc(v, isASCIISpace)
					if end < 0 {
						end = len(v)
					}
					if v[:end] == name {
						want = true
						break
					}
					v = v[end:]
				}
			}
			if got := n.HasClass(name); got != want {
				t.Fatalf("HasClass(%q) = %v, want %v for %s", name, got, want, n.OuterHTML())
			}
		}
	}
}
