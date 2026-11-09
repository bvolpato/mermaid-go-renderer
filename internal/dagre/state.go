// Ported from lodash-es 4.18.1, lodash-es/uniqueId.js (MIT).

package dagre

import (
	"strconv"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// State is the state that the JavaScript code keeps outside of one layout.
// One State must not be used by two layouts at the same time.
type State struct {
	// IDCounter is the counter of lodash uniqueId: the number of ids made
	// so far. dagre names its dummy nodes, its reversed edges and the roots
	// of its layer graphs with uniqueId. In a browser the counter is a
	// module global, so it keeps its value from one layout to the next in
	// one page. To follow that, use one State for all layouts of a render.
	IDCounter int

	// Sort selects the Array.prototype.sort implementation. The zero value
	// is the builtin of Chromium 156 (V8 14.9 and later). The choice
	// changes the result only when a comparator is not consistent, which
	// needs a NaN barycenter (an edge weight of 0) or NaN coordinates.
	Sort js.V8Sort
}

// UniqueID is lodash uniqueId: it returns prefix plus the next counter value.
func (s *State) UniqueID(prefix string) string {
	s.IDCounter++
	return prefix + strconv.Itoa(s.IDCounter)
}
