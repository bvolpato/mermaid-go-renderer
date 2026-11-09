// Ported from plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/nodespacing/cellsystem/HorizontalLabelAlignment.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package cellsystem

import "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"

// HorizontalLabelAlignment is the enum HorizontalLabelAlignment.
type HorizontalLabelAlignment int

// The constants of HorizontalLabelAlignment, in the order of the Java enum.
const (
	HorizontalLabelAlignmentLeft HorizontalLabelAlignment = iota
	HorizontalLabelAlignmentCenter
	HorizontalLabelAlignmentRight
)

var namesHorizontalLabelAlignment = []string{"LEFT", "CENTER", "RIGHT"}

// String returns the name of the Java constant.
func (e HorizontalLabelAlignment) String() string {
	return prop.EnumName(namesHorizontalLabelAlignment, int(e))
}

func init() {
	prop.RegisterEnum[HorizontalLabelAlignment]("org.eclipse.elk.alg.common.nodespacing.cellsystem.HorizontalLabelAlignment", namesHorizontalLabelAlignment)
}
