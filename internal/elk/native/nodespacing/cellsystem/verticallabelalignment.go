// Ported from plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/nodespacing/cellsystem/VerticalLabelAlignment.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package cellsystem

import "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"

// VerticalLabelAlignment is the enum VerticalLabelAlignment.
type VerticalLabelAlignment int

// The constants of VerticalLabelAlignment, in the order of the Java enum.
const (
	VerticalLabelAlignmentTop VerticalLabelAlignment = iota
	VerticalLabelAlignmentCenter
	VerticalLabelAlignmentBottom
)

var namesVerticalLabelAlignment = []string{"TOP", "CENTER", "BOTTOM"}

// String returns the name of the Java constant.
func (e VerticalLabelAlignment) String() string {
	return prop.EnumName(namesVerticalLabelAlignment, int(e))
}

func init() {
	prop.RegisterEnum[VerticalLabelAlignment]("org.eclipse.elk.alg.common.nodespacing.cellsystem.VerticalLabelAlignment", namesVerticalLabelAlignment)
}
