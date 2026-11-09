// Ported from plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/nodespacing/cellsystem/ContainerArea.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package cellsystem

import "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"

// ContainerArea is the enum ContainerArea: the three areas of a container
// that has three areas in one direction.
type ContainerArea int

// The constants of ContainerArea, in the order of the Java enum.
const (
	// ContainerAreaBegin is the top row or the left column.
	ContainerAreaBegin ContainerArea = iota
	// ContainerAreaCenter is the center row or column.
	ContainerAreaCenter
	// ContainerAreaEnd is the bottom row or the right column.
	ContainerAreaEnd
)

// containerAreaCount is ContainerArea.values().length.
const containerAreaCount = 3

var namesContainerArea = []string{"BEGIN", "CENTER", "END"}

// String returns the name of the Java constant.
func (e ContainerArea) String() string { return prop.EnumName(namesContainerArea, int(e)) }

// ValuesContainerArea is ContainerArea.values().
func ValuesContainerArea() []ContainerArea {
	return []ContainerArea{ContainerAreaBegin, ContainerAreaCenter, ContainerAreaEnd}
}

func init() {
	prop.RegisterEnum[ContainerArea]("org.eclipse.elk.alg.common.nodespacing.cellsystem.ContainerArea", namesContainerArea)
}
